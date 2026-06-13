package esximon

// ESXi 虚拟机列表和电源态采集。

import (
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/LemonZuo/homer/internal/logx"
	"golang.org/x/crypto/ssh"
)

// VMShallow 仅 (id, name) 给 USB 路径用。
type VMShallow struct {
	ID   int
	Name string
}

// collectVMs 用 CollectAll 已经拉过一次的 VMShallow 列表 + guestOS 映射,
// 自己只负责 power.getstate 合批查询和组装结果(避免重复跑 getallvms)。
//
// vim-cmd vmsvc/getallvms 输出:第一行表头,后续每行 5 列:
//
//	Vmid  Name  File  Guest OS  Version  Annotation
//	"Name" 可能含空格 —— 由 vim-cmd 用空格对齐,这里偷懒按"首数字 + 空格 + 名字 + 空格 + 路径开始[/vmfs/...]"切。
func collectVMs(client *ssh.Client, shallow []VMShallow, guestOS map[int]string, opts CollectOptions) []VM {
	if opts.SkipVMPower {
		return opts.PreviousVMs
	}
	if shallow == nil {
		// 调用方拿到的 getallvms 失败 —— 返回 nil,buildSample 会写 vm_total/vm_powered_on=-1。
		return nil
	}
	powerByID, full := batchVMPowerState(client, shallow)
	vms := make([]VM, 0, len(shallow))
	now := time.Now()
	prevByID := vmByID(opts.PreviousVMs)
	for _, s := range shallow {
		state, ok := powerByID[s.ID]
		if !ok {
			state = "unknown"
		}
		vm := VM{
			ID:      s.ID,
			Name:    s.Name,
			GuestOS: guestOS[s.ID],
			State:   state,
		}
		if full {
			vm.PowerStateLastFullSuccessAt = now
		} else if prev, ok := prevByID[s.ID]; ok {
			vm.PowerStateLastFullSuccessAt = prev.PowerStateLastFullSuccessAt
			if vm.State == "unknown" {
				vm.State = prev.State
			}
		}
		vms = append(vms, vm)
	}
	return vms
}

func vmByID(vms []VM) map[int]VM {
	m := make(map[int]VM, len(vms))
	for _, v := range vms {
		if v.ID != 0 {
			m[v.ID] = v
		}
	}
	return m
}

// batchVMPowerState 把多 VM 的 power.getstate 合并到一次 SSH session,
// 远端用 `===VM===<id>` 行作分段标志。每次 vim-cmd 启动很慢(~1s),合批省得最明显。
func batchVMPowerState(client *ssh.Client, vms []VMShallow) (map[int]string, bool) {
	res := map[int]string{}
	if len(vms) == 0 {
		return res, true
	}
	var b strings.Builder
	for _, v := range vms {
		b.WriteString("printf '===VM===%d\\n' ")
		b.WriteString(strconv.Itoa(v.ID))
		b.WriteString("; vim-cmd vmsvc/power.getstate ")
		b.WriteString(strconv.Itoa(v.ID))
		b.WriteString("; ")
	}
	// 14 VM × 每次 ~1s = 14s,远超默认 8s,这里给 35s,并在批量不完整时逐 VM 补采。
	out, err := runEsxiRetry(client, "vm power batch", b.String(), 35*time.Second, 2, func(out string) bool {
		return knownPowerCount(parseBatchVMPowerState(out)) == len(vms)
	})
	if err != nil {
		logx.Warn("esxi vm power batch failed", "err", err.Error(), "bytes", len(out))
	}
	res = parseBatchVMPowerState(out)
	for _, v := range vms {
		if st := res[v.ID]; st != "" && st != "unknown" {
			continue
		}
		cmd := "vim-cmd vmsvc/power.getstate " + strconv.Itoa(v.ID)
		powerOut, err := runEsxiRetry(client, "vm power "+strconv.Itoa(v.ID), cmd, 8*time.Second, 2, func(out string) bool {
			return mapVMPowerState(out) != "unknown"
		})
		if err != nil {
			logx.Warn("esxi vm power retry failed", "vm_id", v.ID, "vm", v.Name, "err", err.Error())
			continue
		}
		res[v.ID] = mapVMPowerState(powerOut)
	}
	return res, knownPowerCount(res) == len(vms)
}

func parseBatchVMPowerState(out string) map[int]string {
	res := map[int]string{}
	currentID := -1
	var buf strings.Builder
	flush := func() {
		if currentID >= 0 {
			res[currentID] = mapVMPowerState(buf.String())
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

func knownPowerCount(states map[int]string) int {
	n := 0
	for _, st := range states {
		if st != "" && st != "unknown" {
			n++
		}
	}
	return n
}

func parseVMListShallow(out string) []VMShallow {
	var list []VMShallow
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimRight(line, "\r")
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		id, err := strconv.Atoi(fields[0])
		if err != nil {
			continue
		}
		// 名字到下一个出现 "[" 之前的字段
		nameTokens := []string{}
		for _, f := range fields[1:] {
			if strings.HasPrefix(f, "[") {
				break
			}
			nameTokens = append(nameTokens, f)
		}
		if len(nameTokens) == 0 {
			continue
		}
		list = append(list, VMShallow{ID: id, Name: strings.Join(nameTokens, " ")})
	}
	return list
}

// parseVMGuestOS 从 vim-cmd 输出里抽 Guest OS 列:
// 输出每行 file 列形如 "[datastore] vmfolder/vm.vmx",紧随其后是 Guest OS、Version、Annotation。
// 偷懒:从 ".vmx" 之后再分词,前 1~3 个 token 是 Guest OS(可能含 "Other 5.x or later Linux (64-bit)" 之类空格)。
// 这里简化为:取 ".vmx]" 之后到 "vmx-NN" 之前的 token 串。
var vmGuestOSRe = regexp.MustCompile(`(?m)^(\d+)\s+.+\.vmx\]?\s+(.+?)\s+vmx-\d+`)

func parseVMGuestOS(out string) map[int]string {
	m := map[int]string{}
	for _, mm := range vmGuestOSRe.FindAllStringSubmatch(out, -1) {
		id, err := strconv.Atoi(mm[1])
		if err != nil {
			continue
		}
		m[id] = strings.TrimSpace(mm[2])
	}
	return m
}

func mapVMPowerState(out string) string {
	s := strings.ToLower(strings.TrimSpace(out))
	switch {
	case strings.Contains(s, "powered on"):
		return "powered_on"
	case strings.Contains(s, "powered off"):
		return "powered_off"
	case strings.Contains(s, "suspended"):
		return "suspended"
	default:
		return "unknown"
	}
}

// --- 小工具 ---
