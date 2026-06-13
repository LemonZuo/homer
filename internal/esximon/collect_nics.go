package esximon

// ESXi 物理网卡采集。

import (
	"strconv"
	"strings"
	"time"

	"github.com/LemonZuo/homer/internal/logx"
	"github.com/LemonZuo/homer/internal/sshx"
	"golang.org/x/crypto/ssh"
)

// collectNICs 抓物理网卡列表 + 每张卡的 stats。
// list 给基本/链路信息一次拿全,stats 按设备名循环。
// vmnic 数量个位数,串行调用足够,无需 batch。
func collectNICs(client *ssh.Client, opts CollectOptions) []NIC {
	listOut, err := runEsxiRetry(client, "nic list", "esxcli network nic list", defaultCmdTimeout, 2, func(s string) bool {
		return strings.Contains(s, "vmnic")
	})
	if err != nil {
		logx.Warn("esxi nic list failed", "err", err.Error())
		return nil
	}
	nics := parseNICList(listOut)
	if len(nics) == 0 {
		return nil
	}
	prevByName := nicByName(opts.PreviousNICs)
	if opts.SkipNICStats {
		for i := range nics {
			if prev, ok := prevByName[nics[i].Name]; ok {
				fillNICStatsFromPrev(&nics[i], prev)
			}
		}
		return nics
	}
	statsFull := true
	for i := range nics {
		statsCmd := "esxcli network nic stats get -n " + sshx.ShellQuote(nics[i].Name)
		out, err := runEsxiRetry(client, "nic stats "+nics[i].Name, statsCmd, defaultCmdTimeout, 2, func(s string) bool {
			return strings.Contains(s, "Packets received") || strings.Contains(s, "Bytes received")
		})
		if err != nil {
			statsFull = false
			logx.Warn("esxi nic stats failed", "nic", nics[i].Name, "err", err.Error())
			if prev, ok := prevByName[nics[i].Name]; ok {
				fillNICStatsFromPrev(&nics[i], prev)
			}
			continue
		}
		fillNICStats(&nics[i], out)
	}
	if statsFull {
		now := time.Now()
		for i := range nics {
			nics[i].StatsLastFullSuccessAt = now
		}
	} else {
		for i := range nics {
			if nics[i].StatsLastFullSuccessAt.IsZero() {
				if prev, ok := prevByName[nics[i].Name]; ok {
					nics[i].StatsLastFullSuccessAt = prev.StatsLastFullSuccessAt
				}
			}
		}
	}
	return nics
}

func nicByName(nics []NIC) map[string]NIC {
	m := make(map[string]NIC, len(nics))
	for _, n := range nics {
		if n.Name != "" {
			m[n.Name] = n
		}
	}
	return m
}

func fillNICStatsFromPrev(n *NIC, prev NIC) {
	n.RxBytes = prev.RxBytes
	n.TxBytes = prev.TxBytes
	n.RxErrors = prev.RxErrors
	n.TxErrors = prev.TxErrors
	n.RxDropped = prev.RxDropped
	n.TxDropped = prev.TxDropped
	n.StatsLastFullSuccessAt = prev.StatsLastFullSuccessAt
}

// parseNICList 解析 `esxcli network nic list`。
// 表头固定 10 列,实测一行的描述字段(Description)可能含空格,要从末尾倒着切。
// 列顺序:Name PCI Driver AdminStatus LinkStatus Speed Duplex MAC MTU Description...
func parseNICList(out string) []NIC {
	var nics []NIC
	for _, line := range strings.Split(out, "\n") {
		trim := strings.TrimSpace(line)
		if trim == "" || strings.HasPrefix(trim, "----") || strings.HasPrefix(trim, "Name ") {
			continue
		}
		fields := strings.Fields(trim)
		if len(fields) < 9 || !strings.HasPrefix(fields[0], "vmnic") {
			continue
		}
		n := newNIC()
		n.Name = fields[0]
		n.Driver = fields[2]
		n.AdminStatus = fields[3]
		n.LinkStatus = fields[4]
		n.SpeedMbps = parseIntDefault(fields[5], -1)
		n.Duplex = fields[6]
		n.MAC = fields[7]
		n.MTU = parseIntDefault(fields[8], -1)
		if len(fields) > 9 {
			n.Description = strings.Join(fields[9:], " ")
		}
		nics = append(nics, n)
	}
	return nics
}

// fillNICStats 把 `esxcli network nic stats get -n vmnicX` 的输出注入 NIC。
// 输出每行形如 "   Packets received: 638456",冒号分割。
func fillNICStats(n *NIC, out string) {
	for _, line := range strings.Split(out, "\n") {
		trim := strings.TrimSpace(line)
		colon := strings.Index(trim, ":")
		if colon < 0 {
			continue
		}
		key := strings.TrimSpace(trim[:colon])
		val := strings.TrimSpace(trim[colon+1:])
		v, err := strconv.ParseInt(val, 10, 64)
		if err != nil {
			continue
		}
		switch key {
		case "Bytes received":
			n.RxBytes = v
		case "Bytes sent":
			n.TxBytes = v
		case "Receive packets dropped":
			n.RxDropped = v
		case "Transmit packets dropped":
			n.TxDropped = v
		case "Total receive errors":
			n.RxErrors = v
		case "Total transmit errors":
			n.TxErrors = v
		}
	}
}

// poweredOnVMNames 从 VM 电源态列表里挑出开机 VM 的名字,给拓扑采集做交叉验证。
func poweredOnVMNames(vms []VM) []string {
	var names []string
	for _, v := range vms {
		if v.State == "powered_on" {
			names = append(names, v.Name)
		}
	}
	return names
}

func poweredOnVMByName(vms []VM) map[string]VM {
	m := make(map[string]VM, len(vms))
	for _, v := range vms {
		if v.State == "powered_on" && v.Name != "" {
			m[v.Name] = v
		}
	}
	return m
}
