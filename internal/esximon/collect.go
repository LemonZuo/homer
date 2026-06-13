package esximon

// ESXi 一轮完整采集入口。

import (
	"strings"
	"time"

	"github.com/LemonZuo/homer/internal/logx"
	"golang.org/x/crypto/ssh"
)

// CollectAll 在 client 上跑所有 ESXi 数据采集命令并解析。
// 单条命令失败用空值兜底,不阻断后续。
func CollectAll(client *ssh.Client) HostMetrics {
	return CollectAllWithOptions(client, CollectOptions{})
}

func CollectAllWithOptions(client *ssh.Client, opts CollectOptions) HostMetrics {
	snap := HostMetrics{
		CPUTemp: CPUTemperature{TjMaxC: -1, MaxC: -1, AvgC: -1},
		Runtime: RuntimeUsage{CPUUsagePercent: -1, MemoryUsagePercent: -1},
		MCE:     MCEHealth{State: ""},
		CPU:     CPUStatic{TjMaxC: -1},
		Boot:    newHostBoot(),
	}

	if opts.SkipStatic {
		snap.Platform = opts.PreviousPlatform
		snap.CPU = opts.PreviousCPU
		snap.Memory = opts.PreviousMemory
	} else {
		staticOK := true
		if out, err := runEsxiRetry(client, "platform", "esxcli hardware platform get", defaultCmdTimeout, 2, func(out string) bool {
			p := parsePlatform(out)
			return p.Vendor != "" || p.Product != "" || p.UUID != ""
		}); err == nil {
			snap.Platform = parsePlatform(out)
		} else {
			staticOK = false
			logx.Warn("esxi platform fetch failed", "err", err.Error())
		}
		if out, err := runEsxiRetry(client, "version", "vmware -v; esxcli system version get", defaultCmdTimeout, 2, func(out string) bool {
			return versionRe.MatchString(out)
		}); err == nil {
			parseVersionInto(&snap.Platform, out)
		} else {
			staticOK = false
			logx.Warn("esxi version fetch failed", "err", err.Error())
		}
		if out, err := runEsxiRetry(client, "cpu list", "esxcli hardware cpu list", defaultCmdTimeout, 2, func(out string) bool {
			c := parseCPUStatic(out)
			return c.Brand != "" || c.Family > 0 || c.ModelID > 0
		}); err == nil {
			snap.CPU = parseCPUStatic(out)
		} else {
			staticOK = false
			logx.Warn("esxi cpu list failed", "err", err.Error())
		}
		// 补核心数(cpu list 没这字段)。
		if out, err := runEsxiRetry(client, "cpu global", "esxcli hardware cpu global get", defaultCmdTimeout, 2, func(out string) bool {
			kv := parseKV(out)
			return parseIntDefault(kv["cpu_cores"], 0) > 0
		}); err == nil {
			fillCoresFromGlobal(&snap.CPU, out)
		} else {
			staticOK = false
			logx.Warn("esxi cpu global failed", "err", err.Error())
		}
		// smbiosDump 给真实 CPU 型号/当前频率/核心数,覆盖 esxcli 的 vendor 名。
		// 远端 awk 裁出 Type 4 块(约 30 行) —— smbiosDump 全量约 700 行/35 KiB,
		// 没必要全拉回来,且大输出在 SSH 窗口流控下偶发拖慢。
		smbiosCmd := `smbiosDump 2>/dev/null | awk '/Processor Info \(Type 4\)/{p=NR+30} NR<=p'`
		if out, err := runEsxiRetry(client, "smbios cpu", smbiosCmd, 12*time.Second, 2, func(out string) bool {
			return strings.Contains(out, "Processor Info") && strings.Contains(out, "Type 4")
		}); err == nil {
			logx.Debug("esxi smbios slice", "bytes", len(out))
			fillFromSmbios(&snap.CPU, out)
		} else {
			staticOK = false
			logx.Warn("esxi smbios fetch failed", "err", err.Error())
		}
		if out, err := runEsxiRetry(client, "cpu tjmax", "vsish -e get /hardware/msr/pcpu/0/addr/0x1A2", defaultCmdTimeout, 2, func(out string) bool {
			return decodeTjMax(out) > 0
		}); err == nil {
			if tj := decodeTjMax(out); tj > 0 {
				snap.CPU.TjMaxC = tj
			}
		} else {
			staticOK = false
			logx.Warn("esxi cpu tjmax failed", "err", err.Error())
		}
		if out, err := runEsxiRetry(client, "memory", "esxcli hardware memory get", defaultCmdTimeout, 2, func(out string) bool {
			return parseMemory(out).TotalBytes > 0
		}); err == nil {
			snap.Memory = parseMemory(out)
		} else {
			staticOK = false
			logx.Warn("esxi memory fetch failed", "err", err.Error())
		}
		// vsish 拿可用内存(esxcli 没这字段);同时兜底总内存。
		if out, err := runEsxiRetry(client, "memory comprehensive", "vsish -e cat /memory/comprehensive", defaultCmdTimeout, 2, func(out string) bool {
			var m MemoryInfo
			fillMemoryFromVsish(&m, out)
			return m.TotalBytes > 0 || m.FreeBytes > 0
		}); err == nil {
			fillMemoryFromVsish(&snap.Memory, out)
		} else {
			staticOK = false
			logx.Warn("esxi memory comprehensive failed", "err", err.Error())
		}
		if staticOK && platformUsable(snap.Platform) && cpuStaticUsable(snap.CPU) && memoryUsable(snap.Memory) {
			snap.Platform.StaticLastFullSuccessAt = time.Now()
		}
	}
	if out, err := runEsxiRetry(client, "host summary runtime", "vim-cmd hostsvc/hostsummary", 12*time.Second, 2, func(out string) bool {
		u := parseRuntimeUsage(out, snap.CPU, snap.Memory)
		return u.CPUUsagePercent >= 0 || u.MemoryUsagePercent >= 0
	}); err == nil {
		snap.Runtime = parseRuntimeUsage(out, snap.CPU, snap.Memory)
	} else {
		logx.Warn("esxi runtime usage failed", "err", err.Error())
	}

	// CPU 温度:遍历 0..15 核,失败即停。
	snap.CPUTemp = collectCPUTemp(client, snap.CPU.TjMaxC, snap.CPU.Cores)

	// MCE。
	if out, err := runEsxiRetry(client, "mce health", "vsish -e cat /hardware/health/mce", defaultCmdTimeout, 2, func(out string) bool {
		return parseMCE(out).State != ""
	}); err == nil {
		snap.MCE = parseMCE(out)
	} else {
		logx.Warn("esxi mce fetch failed", "err", err.Error())
	}

	// 磁盘 SMART(逐盘遍历)。
	snap.Disks = collectDisks(client, opts)

	// vim-cmd vmsvc/getallvms 跑一次,USB owned 和 VM 列表共用,
	// 省一次 session 也避免两边对 VM 列表看法不一致。
	var vmsShallow []VMShallow
	var guestOS map[int]string
	if out, err := runEsxiRetry(client, "vm list", "vim-cmd vmsvc/getallvms", 12*time.Second, 2, func(out string) bool {
		return strings.Contains(out, "Vmid") || len(parseVMListShallow(out)) > 0
	}); err == nil {
		vmsShallow = parseVMListShallow(out)
		if vmsShallow == nil {
			vmsShallow = []VMShallow{}
		}
		guestOS = parseVMGuestOS(out)
	} else {
		logx.Warn("esxi vm list failed", "err", err.Error())
	}

	// USB:控制器 + arbitrator + 可直通 + VM 持有。
	snap.USB = collectUSB(client, vmsShallow, opts)

	// VM 列表 + 电源态。
	snap.VMs = collectVMs(client, vmsShallow, guestOS, opts)

	// 主机启动信息(uptime / boot epoch / crash dump)。
	snap.Boot = collectHostBoot(client)

	// 网卡列表 + 收发计数。
	snap.NICs = collectNICs(client, opts)

	// 网络拓扑(vSwitch / Portgroup / VM vNIC 边)。
	// 传入开机 VM 名单做交叉验证:`esxcli network vm list` 偶发输出截断时,
	// validator 能发现缺台并触发重试,避免拓扑图上 VM 时多时少。
	if opts.SkipTopology {
		snap.Topology = opts.PreviousTopology
		snap.Topology.Skipped = true
	} else {
		snap.Topology = collectNetTopology(client, snap.VMs)
		if topologyFullyCollected(snap) {
			snap.Topology.LastFullSuccessAt = time.Now()
		}
	}

	return snap
}
