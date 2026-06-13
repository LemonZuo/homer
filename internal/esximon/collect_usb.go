package esximon

// ESXi USB 控制器、直通设备和 VM 持有设备采集。

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/LemonZuo/homer/internal/logx"
	"golang.org/x/crypto/ssh"
)

// collectUSB 拉控制器 / arbitrator / 可直通设备,VM 持有部分用调用方共享的 VMShallow 列表
// (避免在这里再跑一次 getallvms)。
func collectUSB(client *ssh.Client, vms []VMShallow, opts CollectOptions) USBState {
	u := USBState{}
	if out, err := runEsxiRetry(client, "usb controllers", "lspci | grep -i usb", defaultCmdTimeout, 2, func(out string) bool {
		return len(parseUSBControllers(out)) > 0
	}); err == nil {
		u.Controllers = parseUSBControllers(out)
		u.controllersKnown = len(u.Controllers) > 0
		if !u.controllersKnown {
			logx.Warn("esxi usb controllers parsed 0", "bytes", len(out))
		}
	} else {
		logx.Warn("esxi usb controllers fetch failed", "err", err.Error())
	}
	if out, err := runEsxiRetry(client, "usb arbitrator status", "/etc/init.d/usbarbitrator status", defaultCmdTimeout, 2, func(out string) bool {
		low := strings.ToLower(out)
		return strings.Contains(low, "running") || strings.Contains(low, "stopped")
	}); err == nil {
		u.ArbitratorRunning = strings.Contains(strings.ToLower(out), "running")
		u.arbitratorKnown = true
	} else {
		logx.Warn("esxi usb arbitrator status failed", "err", err.Error())
	}
	if out, err := runEsxiRetry(client, "usb passthrough list", "localcli hardware usb passthrough device list", defaultCmdTimeout, 2, func(out string) bool {
		return strings.Contains(out, "VendorId") && strings.Contains(out, "ProductId")
	}); err == nil {
		u.AvailableForPassthrough = parseUSBPassthrough(out)
		u.passthroughKnown = true
	} else {
		logx.Warn("esxi usb passthrough list failed", "err", err.Error())
	}
	if opts.SkipUSBVMOwned {
		u.VMOwned = opts.PreviousUSB.VMOwned
		u.VMOwnedLastFullSuccessAt = opts.PreviousUSB.VMOwnedLastFullSuccessAt
		u.vmOwnedKnown = true
	} else {
		var ok bool
		u.VMOwned, ok = collectUSBVMOwned(client, vms)
		u.vmOwnedKnown = ok
		if ok {
			u.VMOwnedLastFullSuccessAt = time.Now()
		} else {
			u.VMOwnedLastFullSuccessAt = opts.PreviousUSB.VMOwnedLastFullSuccessAt
		}
	}
	if len(u.VMOwned) > 0 && len(u.AvailableForPassthrough) > 0 {
		u.AvailableForPassthrough = filterVMOwnedUSB(u.AvailableForPassthrough, u.VMOwned)
	}
	return u
}

// parseUSBControllers 解 `lspci | grep -i usb` 一行一控制器,形如:
//
//	0000:00:14.0 USB controller: Intel Cannon Lake PCH USB 3.1 xHCI Host Controller
var lspciLineRe = regexp.MustCompile(`^(\S+)\s+USB\s+controller:\s+(.+)$`)

func parseUSBControllers(out string) []USBController {
	var list []USBController
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if m := lspciLineRe.FindStringSubmatch(line); len(m) == 3 {
			list = append(list, USBController{PCIAddr: m[1], Name: strings.TrimSpace(m[2])})
		}
	}
	return list
}

// parseUSBPassthrough 解 `localcli hardware usb passthrough device list` 表格。
// 第 1 行是表头(Bus  Dev  VendorId  ProductId  Enabled  Can Connect to VM  Name),
// 第 2 行是分隔线(全 -),之后每行一台设备。
func parseUSBPassthrough(out string) []USBPassthroughDevice {
	var list []USBPassthroughDevice
	lines := strings.Split(out, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "Bus") || strings.HasPrefix(line, "---") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 7 {
			continue
		}
		bus := parseIntDefault(fields[0], 0)
		dev := parseIntDefault(fields[1], 0)
		vid := fields[2]
		pid := fields[3]
		enabled := strings.EqualFold(fields[4], "true")
		// fields[5] = Can Connect (yes/no)
		name := strings.Join(fields[6:], " ")
		list = append(list, USBPassthroughDevice{
			Bus: bus, Dev: dev, VID: vid, PID: pid, Name: name, Enabled: enabled,
		})
	}
	return list
}

// collectUSBVMOwned 对每台 VM 取 device.getdevices,筛 VirtualUSB + USBBackingInfo path:。
// 这是 prompt 里强调的"物理直通"判据 —— `backing` 含 deviceName = "path:..."。
//
// 以前是「for VM 各起 1 个 SSH session」,14 VM 会被 ESXi sshd 的 MaxStartups 限速;
// 现在把所有 VM 的 device.getdevices 合到一次 session,用 `===VM===<id>` 行分段。
func collectUSBVMOwned(client *ssh.Client, vms []VMShallow) ([]USBVMOwned, bool) {
	if vms == nil {
		return nil, false
	}
	if len(vms) == 0 {
		return nil, true
	}
	devByVM, full := batchVMDevices(client, vms)
	var out []USBVMOwned
	for _, v := range vms {
		devOut, ok := devByVM[v.ID]
		if !ok || devOut == "" {
			continue
		}
		out = append(out, extractVirtualUSB(v.ID, v.Name, devOut)...)
	}
	return out, full
}

// batchVMDevices 把所有 VM 的 vim-cmd vmsvc/device.getdevices 合到一次 session,
// 用 `===VM===<id>` 行做分段标志。device.getdevices 单次几百毫秒,N VM 串起来很贵。
// 输出可能很大(每 VM 几 KB),给 20s 超时留余量。
func batchVMDevices(client *ssh.Client, vms []VMShallow) (map[int]string, bool) {
	res := map[int]string{}
	if len(vms) == 0 {
		return res, true
	}
	var b strings.Builder
	for _, v := range vms {
		b.WriteString("printf '===VM===%d\\n' ")
		b.WriteString(strconv.Itoa(v.ID))
		b.WriteString("; vim-cmd vmsvc/device.getdevices ")
		b.WriteString(strconv.Itoa(v.ID))
		b.WriteString("; ")
	}
	out, err := runEsxiRetry(client, "vm devices batch", b.String(), 35*time.Second, 2, func(out string) bool {
		return len(parseBatchVMDevices(out)) >= len(vms)
	})
	if err != nil {
		logx.Warn("esxi vm devices batch failed", "err", err.Error(), "bytes", len(out))
	}
	res = parseBatchVMDevices(out)
	for _, v := range vms {
		if res[v.ID] != "" {
			continue
		}
		cmd := "vim-cmd vmsvc/device.getdevices " + strconv.Itoa(v.ID)
		devOut, err := runEsxiRetry(client, "vm devices "+strconv.Itoa(v.ID), cmd, 10*time.Second, 2, func(out string) bool {
			return strings.Contains(out, "(vim.vm.device.")
		})
		if err != nil {
			logx.Warn("esxi vm devices retry failed", "vm_id", v.ID, "vm", v.Name, "err", err.Error())
			continue
		}
		res[v.ID] = devOut
	}
	return res, len(res) >= len(vms)
}

func parseBatchVMDevices(out string) map[int]string {
	res := map[int]string{}
	currentID := -1
	var buf strings.Builder
	flush := func() {
		if currentID >= 0 {
			res[currentID] = buf.String()
		}
		buf.Reset()
	}
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "===VM===") {
			flush()
			idStr := strings.TrimRight(strings.TrimPrefix(line, "===VM==="), "\r")
			if id, err := strconv.Atoi(idStr); err == nil {
				currentID = id
			} else {
				currentID = -1
			}
			continue
		}
		if currentID < 0 {
			continue
		}
		buf.WriteString(line)
		buf.WriteByte('\n')
	}
	flush()
	return res
}

// extractVirtualUSB 在 device.getdevices 输出里找 VirtualUSB 段:
//
//	(vim.vm.device.VirtualUSB) {
//	   key = 41000,
//	   deviceInfo = (vim.Description) {
//	      label = "USB 41001",
//	      summary = "Cyber Power System UT1050EGC"
//	   },
//	   backing = (vim.vm.device.VirtualUSB.USBBackingInfo) {
//	      deviceName = "path:0/1/6",
//	   }
//
// 只关心带 `deviceName = "path:..."` 的(物理直通,排除 xHCI controller 等)。
func extractVirtualUSB(vmID int, vmName, out string) []USBVMOwned {
	var list []USBUVMScan
	const marker = "(vim.vm.device.VirtualUSB)"
	idx := 0
	for {
		i := strings.Index(out[idx:], marker)
		if i < 0 {
			break
		}
		start := idx + i
		end := advanceBalancedBlock(out, start)
		seg := out[start:end]
		list = append(list, scanVirtualUSBSegment(vmID, vmName, seg))
		idx = end
	}
	var owned []USBVMOwned
	for _, l := range list {
		if l.HasPath {
			owned = append(owned, USBVMOwned{
				VMID: vmID, VMName: vmName, Label: l.Label, Summary: l.Summary, Path: l.Path, VID: l.VID, PID: l.PID,
			})
		}
	}
	return owned
}

type USBUVMScan struct {
	Label   string
	Summary string
	Path    string
	VID     string
	PID     string
	HasPath bool
}

func advanceBalancedBlock(s string, from int) int {
	if from >= len(s) {
		return len(s)
	}
	depth := 0
	seenOpen := false
	for i := from; i < len(s); i++ {
		switch s[i] {
		case '{':
			depth++
			seenOpen = true
		case '}':
			if depth > 0 {
				depth--
			}
			if seenOpen && depth == 0 {
				for i+1 < len(s) && (s[i+1] == ',' || s[i+1] == '\r' || s[i+1] == '\n') {
					i++
				}
				return i + 1
			}
		}
	}
	return len(s)
}

func scanVirtualUSBSegment(_ int, _, seg string) USBUVMScan {
	r := USBUVMScan{}
	for _, line := range strings.Split(seg, "\n") {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "label"):
			if m := quotedRe.FindStringSubmatch(line); len(m) == 2 {
				r.Label = m[1]
			}
		case strings.HasPrefix(line, "summary"):
			if m := quotedRe.FindStringSubmatch(line); len(m) == 2 {
				r.Summary = m[1]
			}
		case strings.HasPrefix(line, "deviceName"):
			if m := quotedRe.FindStringSubmatch(line); len(m) == 2 {
				v := m[1]
				if strings.HasPrefix(v, "path:") {
					r.Path = strings.TrimPrefix(v, "path:")
					r.HasPath = true
				}
			}
		case strings.HasPrefix(line, "vendor"):
			r.VID = parseDecimalUSBID(line)
		case strings.HasPrefix(line, "product"):
			r.PID = parseDecimalUSBID(line)
		}
	}
	return r
}

func parseDecimalUSBID(line string) string {
	idx := strings.Index(line, "=")
	if idx < 0 {
		return ""
	}
	val := strings.Trim(strings.TrimSpace(line[idx+1:]), ",")
	n, err := strconv.Atoi(val)
	if err != nil || n < 0 {
		return ""
	}
	return fmt.Sprintf("%04x", n)
}

func filterVMOwnedUSB(avail []USBPassthroughDevice, owned []USBVMOwned) []USBPassthroughDevice {
	ownedIDs := map[string]struct{}{}
	for _, d := range owned {
		if d.VID == "" || d.PID == "" {
			continue
		}
		ownedIDs[strings.ToLower(d.VID)+":"+strings.ToLower(d.PID)] = struct{}{}
	}
	if len(ownedIDs) == 0 {
		return avail
	}
	out := make([]USBPassthroughDevice, 0, len(avail))
	for _, d := range avail {
		key := strings.ToLower(d.VID) + ":" + strings.ToLower(d.PID)
		if _, ok := ownedIDs[key]; ok {
			continue
		}
		out = append(out, d)
	}
	return out
}
