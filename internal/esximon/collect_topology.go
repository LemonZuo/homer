package esximon

// ESXi 网络拓扑采集和解析。

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/LemonZuo/homer/internal/logx"
	"github.com/LemonZuo/homer/internal/sshx"
	"golang.org/x/crypto/ssh"
)

// collectNetTopology 抓 vSwitch + 每个 VM 的 vNIC 边 + VMkernel 端口。
// vSwitch list 一次拿全;`esxcli network vm list` 给出每台正在跑的 VM 的 World ID,
// 再对每个 World ID 跑一次 `esxcli network vm port list -w <wid>`。
// 注意:这里用的 ID 是 World ID(esxcli 体系),与 VMShallow.ID(vim-cmd 的 Vmid)不是同一个值。
// 返回给前端的 VMNICLink.VMID 使用 vim-cmd VM ID,只在 VM 名称匹配失败时用 World ID 兜底。
// expectVMs 是已知开机的 VM 名单(来自 vim-cmd 电源态):`esxcli network vm list`
// 偶发把表输出截断时行数会变少,用名单做 validator 让 runEsxiRetry 能感知并重试。
func collectNetTopology(client *ssh.Client, vms []VM) NetTopology {
	topo := NetTopology{Collected: true, VMPortsCollected: true}
	expectVMs := poweredOnVMNames(vms)
	vmByName := poweredOnVMByName(vms)
	if out, err := runEsxiRetry(client, "vswitch list", "esxcli network vswitch standard list", defaultCmdTimeout, 2, func(s string) bool {
		return strings.Contains(s, "Uplinks:") || strings.Contains(s, "vSwitch")
	}); err == nil {
		topo.VSwitches = parseVSwitchList(out)
		topo.VSwitchCollected = true
	} else {
		logx.Warn("esxi vswitch list failed", "err", err.Error())
	}
	topo.VMKPorts, topo.VMKCollected, topo.VMKFullCollected = collectVMKPorts(client)
	listOut, err := runEsxiRetry(client, "vm net list", "esxcli network vm list", defaultCmdTimeout, 3, func(s string) bool {
		if strings.TrimSpace(s) == "" {
			// 没有开机 VM 时输出为空是合法的;有 VM 该开机却输出为空就是截断
			return len(expectVMs) == 0
		}
		if !strings.Contains(s, "World ID") {
			return false
		}
		return vmNetListCovers(parseVMNetList(s), expectVMs)
	})
	if err != nil {
		// 重试后依然缺台:照常解析已有的行(部分数据比没有强),
		// 完整性由 service 层的 topologyComplete 判定并回退 prev。
		logx.Warn("esxi vm net list incomplete", "err", err.Error())
		topo.VMPortsCollected = false
	} else {
		topo.VMNetCollected = true
	}
	for _, vm := range parseVMNetList(listOut) {
		cmd := fmt.Sprintf("esxcli network vm port list -w %d", vm.worldID)
		out, err := runEsxiRetry(client, "vm port "+vm.name, cmd, defaultCmdTimeout, 2, func(s string) bool {
			return strings.Contains(s, "Port ID:") || strings.TrimSpace(s) == ""
		})
		if err != nil {
			logx.Warn("esxi vm port list failed", "vm", vm.name, "err", err.Error())
			topo.VMPortsCollected = false
			continue
		}
		links := parseVMPortList(out)
		vmID := vm.worldID
		if vmInfo, ok := vmByName[vm.name]; ok {
			vmID = vmInfo.ID
			if vmNICLinksNeedGuestIP(links) {
				guestNICs, guestOK := collectVMGuestNIC(client, vmInfo)
				links = fillVMNICGuestIPs(links, guestNICs)
				if !guestOK {
					topo.VMPortsCollected = false
				}
			}
		}
		for _, link := range links {
			link.VMID = vmID
			link.VMName = vm.name
			topo.VMNICs = append(topo.VMNICs, link)
		}
	}
	return topo
}

type vmGuestNIC struct {
	MAC string
	IP  string
}

func vmNICLinksNeedGuestIP(links []VMNICLink) bool {
	for _, link := range links {
		if link.MAC != "" && link.IP == "" {
			return true
		}
	}
	return false
}

func collectVMGuestNIC(client *ssh.Client, vm VM) ([]vmGuestNIC, bool) {
	if vm.ID <= 0 || vm.State != "powered_on" {
		return nil, true
	}
	cmd := "vim-cmd vmsvc/get.guest " + strconv.Itoa(vm.ID)
	out, err := runEsxiRetry(client, "vm guest "+strconv.Itoa(vm.ID), cmd, 12*time.Second, 2, func(out string) bool {
		return strings.TrimSpace(out) != ""
	})
	if err != nil {
		logx.Warn("esxi vm guest fetch failed", "vm_id", vm.ID, "vm", vm.Name, "err", err.Error(), "bytes", len(out))
		return parseVMGuestNICs(out), false
	}
	return parseVMGuestNICs(out), true
}

func collectVMKPorts(client *ssh.Client) ([]VMKPort, bool, bool) {
	out, err := runEsxiRetry(client, "vmkernel interface list", "esxcli network ip interface list", defaultCmdTimeout, 2, func(s string) bool {
		return strings.Contains(s, "Name:") || strings.Contains(s, "vmk")
	})
	if err != nil {
		logx.Warn("esxi vmkernel interface list failed", "err", err.Error())
		return nil, false, false
	}
	ports := parseIPInterfaceList(out)
	full := true
	for i := range ports {
		if ports[i].Name == "" {
			continue
		}
		cmd := "esxcli network ip interface ipv4 get -i " + sshx.ShellQuote(ports[i].Name)
		ipOut, err := runEsxiRetry(client, "vmkernel ipv4 "+ports[i].Name, cmd, defaultCmdTimeout, 2, func(s string) bool {
			return strings.Contains(s, ports[i].Name) || strings.Contains(s, "IPv4 Address")
		})
		if err != nil {
			logx.Warn("esxi vmkernel ipv4 get failed", "vmk", ports[i].Name, "err", err.Error())
			full = false
			continue
		}
		ports[i].IPv4 = parseIPv4Get(ipOut, ports[i].Name)
	}
	return ports, true, full
}

// vmNetListCovers 检查 `esxcli network vm list` 的解析结果是否覆盖了全部期望 VM。
// 名字按精确匹配;expectVMs 里有而列表里没有的视为缺台(输出截断或 VM 刚好关机,
// 后者会在下一轮自愈,宁可多重试一次也不让拓扑缺 VM)。
func vmNetListCovers(entries []vmNetEntry, expectVMs []string) bool {
	if len(expectVMs) == 0 {
		return true
	}
	seen := make(map[string]bool, len(entries))
	for _, e := range entries {
		seen[e.name] = true
	}
	for _, want := range expectVMs {
		if !seen[want] {
			return false
		}
	}
	return true
}

// vmNetEntry 是 parseVMNetList 的内部返回行(World ID + Name)。
type vmNetEntry struct {
	worldID int
	name    string
}

// parseVMNetList 解析 `esxcli network vm list`:World ID + Name。
// 列顺序固定:World ID | Name | Num Ports | Networks(可含逗号空格)。
// 名字可能含空格,用"Num Ports 必是单一纯数字"做切分锚点 ——
// 从后往前找第一个不含逗号的纯数字字段,它就是 Num Ports。
func parseVMNetList(out string) []vmNetEntry {
	var list []vmNetEntry
	for _, line := range strings.Split(out, "\n") {
		trim := strings.TrimSpace(line)
		if trim == "" || strings.HasPrefix(trim, "----") || strings.HasPrefix(trim, "World ID") {
			continue
		}
		fields := strings.Fields(trim)
		if len(fields) < 4 {
			continue
		}
		wid, err := strconv.Atoi(fields[0])
		if err != nil {
			continue
		}
		numIdx := -1
		for i := len(fields) - 1; i >= 1; i-- {
			if strings.Contains(fields[i], ",") {
				continue
			}
			if _, e := strconv.Atoi(fields[i]); e == nil {
				numIdx = i
				break
			}
		}
		if numIdx < 2 {
			continue
		}
		name := strings.Join(fields[1:numIdx], " ")
		if name == "" {
			continue
		}
		list = append(list, vmNetEntry{worldID: wid, name: name})
	}
	return list
}

// parseVSwitchList 解析 `esxcli network vswitch standard list`。
// 输出按 vSwitch 分块,每块内"Name:/Uplinks:/Portgroups:"等 KV 行;块之间空行分隔。
// Uplinks 与 Portgroups 形如 "vmnic0, vmnic1" / "VM Network, Management Network"。
func parseVSwitchList(out string) []VSwitchInfo {
	var list []VSwitchInfo
	var cur *VSwitchInfo
	flush := func() {
		if cur != nil && cur.Name != "" {
			list = append(list, *cur)
		}
		cur = nil
	}
	for _, line := range strings.Split(out, "\n") {
		raw := line
		trim := strings.TrimSpace(raw)
		if trim == "" {
			flush()
			continue
		}
		// 块开头:列名(无前导空格,无冒号,例如 "vSwitch0")。
		if !strings.HasPrefix(raw, " ") && !strings.HasPrefix(raw, "\t") && !strings.Contains(trim, ":") {
			flush()
			cur = &VSwitchInfo{Name: trim}
			continue
		}
		if cur == nil {
			continue
		}
		colon := strings.Index(trim, ":")
		if colon < 0 {
			continue
		}
		key := strings.TrimSpace(trim[:colon])
		val := strings.TrimSpace(trim[colon+1:])
		switch key {
		case "Name":
			cur.Name = val
		case "Uplinks":
			cur.Uplinks = splitCSV(val)
		case "Portgroups":
			cur.Portgroups = splitCSV(val)
		}
	}
	flush()
	return list
}

// parseIPInterfaceList 解析 `esxcli network ip interface list`。
// 输出按 vmk 分块,每块内是 KV 行;标准 vSwitch 下 Portset 就是 vSwitch 名。
func parseIPInterfaceList(out string) []VMKPort {
	var list []VMKPort
	var cur *VMKPort
	flush := func() {
		if cur != nil && cur.Name != "" {
			list = append(list, *cur)
		}
		cur = nil
	}
	for _, line := range strings.Split(out, "\n") {
		raw := line
		trim := strings.TrimSpace(raw)
		if trim == "" {
			flush()
			continue
		}
		if !strings.HasPrefix(raw, " ") && !strings.HasPrefix(raw, "\t") && !strings.Contains(trim, ":") {
			flush()
			if strings.HasPrefix(trim, "vmk") {
				cur = &VMKPort{Name: trim}
			}
			continue
		}
		colon := strings.Index(trim, ":")
		if colon < 0 {
			continue
		}
		key := strings.TrimSpace(trim[:colon])
		val := strings.TrimSpace(trim[colon+1:])
		if cur == nil && (key == "Name" || strings.HasPrefix(val, "vmk")) {
			cur = &VMKPort{}
		}
		if cur == nil {
			continue
		}
		switch key {
		case "Name":
			if cur.Name != "" && cur.Name != val {
				flush()
				cur = &VMKPort{}
			}
			cur.Name = val
		case "Portset":
			cur.VSwitch = val
		case "Portgroup":
			cur.Portgroup = val
		case "MAC Address":
			cur.MAC = val
		case "Enabled":
			cur.Enabled = strings.EqualFold(val, "true") || strings.EqualFold(val, "yes")
		}
	}
	flush()
	return list
}

// parseIPv4Get 解析 `esxcli network ip interface ipv4 get -i vmkX` 的表格输出。
func parseIPv4Get(out, name string) string {
	for _, line := range strings.Split(out, "\n") {
		trim := strings.TrimSpace(line)
		if trim == "" || strings.HasPrefix(trim, "----") || strings.HasPrefix(trim, "Name ") {
			continue
		}
		fields := strings.Fields(trim)
		if len(fields) >= 2 && fields[0] == name {
			if fields[1] != "0.0.0.0" && fields[1] != "N/A" {
				return fields[1]
			}
			return ""
		}
		colon := strings.Index(trim, ":")
		if colon < 0 {
			continue
		}
		key := strings.TrimSpace(trim[:colon])
		val := strings.TrimSpace(trim[colon+1:])
		if key == "IPv4 Address" && val != "0.0.0.0" && val != "N/A" {
			return val
		}
	}
	return ""
}

// parseVMPortList 解析 `esxcli network vm port list -w <wid>`。
// 输出按 vNIC 分块,块之间空行分隔。每块的 KV 包括
// Port ID / vSwitch / Portgroup / MAC Address / IP Address / Team Uplink。
func parseVMPortList(out string) []VMNICLink {
	var list []VMNICLink
	var cur *VMNICLink
	flush := func() {
		if cur != nil && (cur.MAC != "" || cur.VSwitch != "") {
			list = append(list, *cur)
		}
		cur = nil
	}
	for _, line := range strings.Split(out, "\n") {
		trim := strings.TrimSpace(line)
		if trim == "" {
			flush()
			continue
		}
		colon := strings.Index(trim, ":")
		if colon < 0 {
			continue
		}
		key := strings.TrimSpace(trim[:colon])
		val := strings.TrimSpace(trim[colon+1:])
		switch key {
		case "Port ID":
			flush()
			cur = &VMNICLink{}
		case "vSwitch":
			if cur != nil {
				cur.VSwitch = val
			}
		case "Portgroup":
			if cur != nil {
				cur.Portgroup = val
			}
		case "MAC Address":
			if cur != nil {
				cur.MAC = val
			}
		case "IP Address":
			if cur != nil && val != "0.0.0.0" {
				cur.IP = val
			}
		case "Team Uplink":
			if cur != nil {
				cur.TeamUplink = val
			}
		}
	}
	flush()
	return list
}

func parseBatchVMGuestNICs(out string) map[int][]vmGuestNIC {
	res := map[int][]vmGuestNIC{}
	currentID := -1
	var buf strings.Builder
	flush := func() {
		if currentID >= 0 {
			if nics := parseVMGuestNICs(buf.String()); len(nics) > 0 {
				res[currentID] = nics
			}
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

func parseVMGuestNICs(out string) []vmGuestNIC {
	var list []vmGuestNIC
	var cur *vmGuestNIC
	inIPList := false
	flush := func() {
		if cur != nil && cur.MAC != "" && cur.IP != "" {
			list = append(list, *cur)
		}
		cur = nil
		inIPList = false
	}
	for _, line := range strings.Split(out, "\n") {
		trim := strings.TrimSpace(line)
		if strings.Contains(trim, "(vim.vm.GuestInfo.NicInfo)") && strings.HasSuffix(trim, "{") {
			flush()
			cur = &vmGuestNIC{}
			continue
		}
		if cur == nil {
			continue
		}
		if strings.HasPrefix(trim, "ipAddress = (string)") {
			inIPList = true
			continue
		}
		if inIPList {
			if strings.HasPrefix(trim, "]") {
				inIPList = false
				continue
			}
			if ip := firstQuoted(trim); ip != "" && usefulGuestIP(ip) && cur.IP == "" {
				cur.IP = ip
			}
			continue
		}
		if strings.HasPrefix(trim, "macAddress =") {
			cur.MAC = firstQuoted(trim)
			continue
		}
		if strings.HasPrefix(trim, "}") {
			flush()
		}
	}
	flush()
	return list
}

func fillVMNICGuestIPs(links []VMNICLink, guestNICs []vmGuestNIC) []VMNICLink {
	if len(links) == 0 || len(guestNICs) == 0 {
		return links
	}
	ipByMAC := make(map[string]string, len(guestNICs))
	for _, nic := range guestNICs {
		if nic.MAC == "" || nic.IP == "" {
			continue
		}
		ipByMAC[strings.ToLower(nic.MAC)] = nic.IP
	}
	for i := range links {
		if links[i].IP != "" || links[i].MAC == "" {
			continue
		}
		if ip := ipByMAC[strings.ToLower(links[i].MAC)]; ip != "" {
			links[i].IP = ip
		}
	}
	return links
}

func usefulGuestIP(ip string) bool {
	ip = strings.TrimSpace(ip)
	if ip == "" || ip == "0.0.0.0" || ip == "::" {
		return false
	}
	low := strings.ToLower(ip)
	return !strings.HasPrefix(low, "fe80:")
}
