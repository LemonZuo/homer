// Package esximon 通过 SSH 远程执行 ESXi 的 esxcli / vsish / vim-cmd 命令,
// 采集平台、CPU 温度、磁盘 SMART、MCE、USB、VM 等信息。机器来源 esxi_host 表,
// 凭证来源 esxi_ssh_credential,与 UPS / ACME 完全解耦。
//
// 数据采集策略:每轮 SSH 连接复用,**串行**地跑 N 条命令(esxcli/vsish 都很轻),
// 不在远端拼 JSON —— 直接把每条命令的纯文本拉回本地用 Go 解析,避开 busybox
// 转义陷阱。详见 prompt/6_ESXI_SSH_MONITORING.md。
package esximon

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/LemonZuo/homer/internal/logx"
	"github.com/LemonZuo/homer/internal/sshx"
	"golang.org/x/crypto/ssh"
)

// --- 数据形态(对应 prompt 里的 JSON 范式) ---

// PlatformInfo 主机标识 + ESXi 版本(启动时一次就够,但我们每轮都顺手刷一次)。
type PlatformInfo struct {
	Vendor        string `json:"vendor"`
	Product       string `json:"product"`
	Serial        string `json:"serial"`
	UUID          string `json:"uuid"`
	IPMISupported bool   `json:"ipmi_supported"`
	ESXiVersion   string `json:"esxi_version"`
	ESXiBuild     int64  `json:"esxi_build"`
}

// CPUStatic 静态 CPU 信息。
type CPUStatic struct {
	Brand    string `json:"brand"`
	Family   int    `json:"family"`
	ModelID  int    `json:"model"`
	Stepping int    `json:"stepping"`
	Cores    int    `json:"cores"`
	FreqMHz  int    `json:"freq_mhz"`
	L2KB     int    `json:"l2_kb"`
	L3KB     int    `json:"l3_kb"`
	TjMaxC   int    `json:"tjmax_c"`
}

// MemoryInfo 内存。只保留总量和可用,合并进平台卡展示。
type MemoryInfo struct {
	TotalBytes int64 `json:"mem_total_bytes"`
	FreeBytes  int64 `json:"mem_free_bytes"`
}

// RuntimeUsage 主机运行时资源使用率。CPU 用量来自 hostsummary 的 MHz,
// 内存用量来自 hostsummary 的 MiB;缺数据用 -1。
type RuntimeUsage struct {
	CPUUsedMHz         int   `json:"cpu_used_mhz"`
	CPUCapacityMHz     int   `json:"cpu_capacity_mhz"`
	CPUUsagePercent    int   `json:"cpu_usage_percent"`
	MemoryUsedBytes    int64 `json:"memory_used_bytes"`
	MemoryTotalBytes   int64 `json:"memory_total_bytes"`
	MemoryUsagePercent int   `json:"memory_usage_percent"`
}

// CPUCore 每核温度。
type CPUCore struct {
	ID        int `json:"id"`
	TempC     int `json:"temp_c"`
	HeadroomC int `json:"headroom_c"`
}

// CPUTemperature 全核温度聚合。
type CPUTemperature struct {
	TjMaxC int       `json:"tjmax_c"`
	Cores  []CPUCore `json:"cores"`
	MaxC   int       `json:"max_c"`
	AvgC   int       `json:"avg_c"`
}

// MCEHealth Machine Check Error 健康状态。
type MCEHealth struct {
	State            string `json:"state"` // Green / Yellow / Red
	CorrectedTotal   int64  `json:"corrected_total"`
	CorrectedEWMA    int64  `json:"corrected_ewma"`
	PeriodSeconds    int    `json:"period_seconds"`
	UncorrectedTotal int64  `json:"uncorrected_total"`
}

// DiskHealth 单块盘温度 + 容量/用量 + SMART 属性。
// 字段命名沿用历史(原本只有温度),后续如需重命名再说。
type DiskHealth struct {
	Device        string   `json:"device"`
	Model         string   `json:"model"`
	Type          string   `json:"type"`           // SATA-SSD / SATA-HDD / NVMe / unknown
	CapacityBytes int64    `json:"capacity_bytes"` // 物理设备容量,0 表示无数据
	UsedBytes     int64    `json:"used_bytes"`     // 单 extent VMFS datastore 已用量,-1 表示无法唯一关联
	FreeBytes     int64    `json:"free_bytes"`     // 单 extent VMFS datastore 可用量,-1 表示无法唯一关联
	Datastores    []string `json:"datastores,omitempty"`
	TempC         int      `json:"temp_c"`      // -1 表示无数据
	ThresholdC    int      `json:"threshold_c"` // -1 表示无数据
	Status        string   `json:"status"`      // ok / warning / critical / unknown

	// SMART 属性。统一规则:优先取 Raw 列,Raw=N/A 时回退 Value 列(NVMe 都走 Value)。
	// 未拿到/不适用统一兜底:string→"",int64→-1,int→-1。
	HealthStatus              string `json:"smart_health"`                 // OK / 厂商私有
	PowerOnHours              int64  `json:"smart_power_on_hours"`         // 通电小时
	PowerCycleCount           int64  `json:"smart_power_cycle_count"`      // 开机次数
	ReallocatedSectors        int64  `json:"smart_reallocated_sectors"`    // 重映射扇区数
	UncorrectableErrors       int64  `json:"smart_uncorrectable_errors"`   // SSD/HDD 二选一名,合并入此字段
	MediaWearoutValue         int    `json:"smart_media_wearout"`          // SSD 独有,normalized 100=新,0=磨损完
	ReadErrorCount            int64  `json:"smart_read_error_count"`       // HDD 独有
	PendingSectorReallocation int64  `json:"smart_pending_sector_realloc"` // HDD 独有
}

// USBController USB 控制器(实测 TS80X 只有一个 xHCI)。
type USBController struct {
	PCIAddr string `json:"pci_addr"`
	Name    string `json:"name"`
}

// USBPassthroughDevice 可直通设备(未被 VM 持有)。
type USBPassthroughDevice struct {
	Bus     int    `json:"bus"`
	Dev     int    `json:"dev"`
	VID     string `json:"vid"`
	PID     string `json:"pid"`
	Name    string `json:"name"`
	Enabled bool   `json:"enabled"`
}

// USBVMOwned VM 持有的物理直通 USB 设备。
type USBVMOwned struct {
	VMID    int    `json:"vm_id"`
	VMName  string `json:"vm_name"`
	Label   string `json:"label"`
	Summary string `json:"summary"`
	Path    string `json:"path"`
	VID     string `json:"vid,omitempty"`
	PID     string `json:"pid,omitempty"`
}

// USBState USB 完整状态。
type USBState struct {
	Controllers             []USBController        `json:"controllers"`
	ArbitratorRunning       bool                   `json:"arbitrator_running"`
	AvailableForPassthrough []USBPassthroughDevice `json:"available_for_passthrough"`
	VMOwned                 []USBVMOwned           `json:"vm_owned"`

	controllersKnown bool
	arbitratorKnown  bool
	passthroughKnown bool
}

// VM 单台虚拟机。
type VM struct {
	ID      int    `json:"id"`
	Name    string `json:"name"`
	GuestOS string `json:"guest_os"`
	State   string `json:"state"` // powered_on / powered_off / suspended / unknown
}

// HostBoot 主机启动信息。
// UptimeSeconds = -1 表示采集失败。
// LastCrashAt 为零值表示没找到 /var/core/vmkernel-zdump.* 文件。
type HostBoot struct {
	UptimeSeconds  int64     `json:"uptime_seconds"`
	BootedAt       time.Time `json:"booted_at"`
	CrashDumpCount int       `json:"crash_dump_count"`
	LastCrashAt    time.Time `json:"last_crash_at,omitzero"`
}

// NIC 单张物理网卡:基本信息 + 链路状态 + 收发计数。
// SpeedMbps = -1 / Duplex = "" 表示链路 Down 或数据缺失。
// 计数字段为 -1 表示 stats 命令失败,0 表示真的没流量。
type NIC struct {
	Name        string `json:"name"`
	Driver      string `json:"driver"`
	MAC         string `json:"mac"`
	MTU         int    `json:"mtu"`
	Description string `json:"description"`
	AdminStatus string `json:"admin_status"` // Up / Down
	LinkStatus  string `json:"link_status"`  // Up / Down
	SpeedMbps   int    `json:"speed_mbps"`
	Duplex      string `json:"duplex"` // Full / Half / ""

	RxBytes   int64 `json:"rx_bytes"`
	TxBytes   int64 `json:"tx_bytes"`
	RxErrors  int64 `json:"rx_errors"`
	TxErrors  int64 `json:"tx_errors"`
	RxDropped int64 `json:"rx_dropped"`
	TxDropped int64 `json:"tx_dropped"`
}

// VSwitchInfo 标准 vSwitch 一条。
// Uplinks/Portgroups 数组保留 esxcli 列表顺序,前端不重排。
type VSwitchInfo struct {
	Name       string   `json:"name"`
	Uplinks    []string `json:"uplinks"`
	Portgroups []string `json:"portgroups"`
}

// VMNICLink 一个 VM 的一张 vNIC 接到拓扑里的"边"。
// TeamUplink 是当前 active 的 pNIC(标准 vSwitch + 默认 team 策略下足够稳定)。
// MAC/IP 空串表示 vm port list 没拿到。
type VMNICLink struct {
	VMID       int    `json:"vmid"`
	VMName     string `json:"vm_name"`
	VSwitch    string `json:"vswitch"`
	Portgroup  string `json:"portgroup"`
	MAC        string `json:"mac"`
	IP         string `json:"ip,omitempty"`
	TeamUplink string `json:"team_uplink"`
}

// VMKPort 是 VMkernel 网卡(vmk0/vmk1...)接入的 Portgroup。
// IPv4 为空表示只拿到接口归属,没拿到 IPv4 地址。
type VMKPort struct {
	Name      string `json:"name"`
	VSwitch   string `json:"vswitch"`
	Portgroup string `json:"portgroup"`
	MAC       string `json:"mac"`
	IPv4      string `json:"ipv4,omitempty"`
	Enabled   bool   `json:"enabled"`
}

// NetTopology 网络拓扑:vSwitch 全集 + VM vNIC 边 + VMkernel 端口。
// 前端按 pNIC → vSwitch → Portgroup → VMs/VMkernel → pNIC 渲染。
type NetTopology struct {
	VSwitches    []VSwitchInfo `json:"vswitches"`
	VMNICs       []VMNICLink   `json:"vm_nics"`
	VMKPorts     []VMKPort     `json:"vmk_ports,omitempty"`
	VMKCollected bool          `json:"-"`
}

func newHostBoot() HostBoot {
	return HostBoot{UptimeSeconds: -1}
}

func newNIC() NIC {
	return NIC{
		MTU:       -1,
		SpeedMbps: -1,
		RxBytes:   -1,
		TxBytes:   -1,
		RxErrors:  -1,
		TxErrors:  -1,
		RxDropped: -1,
		TxDropped: -1,
	}
}

// HostMetrics 一台 ESXi 一轮的完整采集结果。
// 任何一段子采集失败(esxcli/vsish 单点错误)都不会让整轮挂掉。调用方会对关键
// 指标做完整性判定:完整才写 esxi_sample,不完整则保留 esxi_state 的上一轮有效值。
type HostMetrics struct {
	Platform PlatformInfo   `json:"platform"`
	CPU      CPUStatic      `json:"cpu_static"`
	Memory   MemoryInfo     `json:"memory"`
	Runtime  RuntimeUsage   `json:"runtime_usage"`
	CPUTemp  CPUTemperature `json:"cpu_temperature"`
	MCE      MCEHealth      `json:"mce_health"`
	Disks    []DiskHealth   `json:"disk_health"`
	USB      USBState       `json:"usb"`
	VMs      []VM           `json:"vms"`
	Boot     HostBoot       `json:"boot"`
	NICs     []NIC          `json:"nics"`
	Topology NetTopology    `json:"net_topology"`
}

// --- 主入口:在已建立的 ssh.Client 上跑一整轮 ---

// 非交互非登录 SSH session 经常缺路径,统一显式注入(ESXi 实际不需要,
// 但加上没坏处,并且如果走 bastion + Linux jump host 这里就能兜住)。
const esxiPathPrefix = "export PATH=/bin:/sbin:/usr/lib/vmware/bin:/usr/lib/vmware/vsan/bin:$PATH; "

// CollectAll 在 client 上跑所有 ESXi 数据采集命令并解析。
// 单条命令失败用空值兜底,不阻断后续。
func CollectAll(client *ssh.Client) HostMetrics {
	snap := HostMetrics{
		CPUTemp: CPUTemperature{TjMaxC: -1, MaxC: -1, AvgC: -1},
		Runtime: RuntimeUsage{CPUUsagePercent: -1, MemoryUsagePercent: -1},
		MCE:     MCEHealth{State: ""},
		CPU:     CPUStatic{TjMaxC: -1},
		Boot:    newHostBoot(),
	}

	if out, err := runEsxiRetry(client, "platform", "esxcli hardware platform get", defaultCmdTimeout, 2, func(out string) bool {
		p := parsePlatform(out)
		return p.Vendor != "" || p.Product != "" || p.UUID != ""
	}); err == nil {
		snap.Platform = parsePlatform(out)
	} else {
		logx.Warn("esxi platform fetch failed", "err", err.Error())
	}
	if out, err := runEsxiRetry(client, "version", "vmware -v; esxcli system version get", defaultCmdTimeout, 2, func(out string) bool {
		return versionRe.MatchString(out)
	}); err == nil {
		parseVersionInto(&snap.Platform, out)
	} else {
		logx.Warn("esxi version fetch failed", "err", err.Error())
	}
	if out, err := runEsxiRetry(client, "cpu list", "esxcli hardware cpu list", defaultCmdTimeout, 2, func(out string) bool {
		c := parseCPUStatic(out)
		return c.Brand != "" || c.Family > 0 || c.ModelID > 0
	}); err == nil {
		snap.CPU = parseCPUStatic(out)
	} else {
		logx.Warn("esxi cpu list failed", "err", err.Error())
	}
	// 补核心数(cpu list 没这字段)。
	if out, err := runEsxiRetry(client, "cpu global", "esxcli hardware cpu global get", defaultCmdTimeout, 2, func(out string) bool {
		kv := parseKV(out)
		return parseIntDefault(kv["cpu_cores"], 0) > 0
	}); err == nil {
		fillCoresFromGlobal(&snap.CPU, out)
	} else {
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
		logx.Warn("esxi smbios fetch failed", "err", err.Error())
	}
	if out, err := runEsxiRetry(client, "cpu tjmax", "vsish -e get /hardware/msr/pcpu/0/addr/0x1A2", defaultCmdTimeout, 2, func(out string) bool {
		return decodeTjMax(out) > 0
	}); err == nil {
		if tj := decodeTjMax(out); tj > 0 {
			snap.CPU.TjMaxC = tj
		}
	} else {
		logx.Warn("esxi cpu tjmax failed", "err", err.Error())
	}
	if out, err := runEsxiRetry(client, "memory", "esxcli hardware memory get", defaultCmdTimeout, 2, func(out string) bool {
		return parseMemory(out).TotalBytes > 0
	}); err == nil {
		snap.Memory = parseMemory(out)
	} else {
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
		logx.Warn("esxi memory comprehensive failed", "err", err.Error())
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
	snap.Disks = collectDisks(client)

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
	snap.USB = collectUSB(client, vmsShallow)

	// VM 列表 + 电源态。
	snap.VMs = collectVMs(client, vmsShallow, guestOS)

	// 主机启动信息(uptime / boot epoch / crash dump)。
	snap.Boot = collectHostBoot(client)

	// 网卡列表 + 收发计数。
	snap.NICs = collectNICs(client)

	// 网络拓扑(vSwitch / Portgroup / VM vNIC 边)。
	// 传入开机 VM 名单做交叉验证:`esxcli network vm list` 偶发输出截断时,
	// validator 能发现缺台并触发重试,避免拓扑图上 VM 时多时少。
	snap.Topology = collectNetTopology(client, poweredOnVMNames(snap.VMs))

	return snap
}

// 单条命令默认超时:ESXi 上单条 esxcli/vsish 多数 < 2s,8s 已经留了很大裕量。
// 用来防"某条命令偶发 hang 拖死整轮",避免一台机器一轮采样里只有部分子项有值。
const defaultCmdTimeout = 8 * time.Second

// runEsxi 在 client 上跑一条 shell 命令,前面统一加 PATH 注入。
// stderr 一律丢弃 —— ESXi 的 vsish/esxcli 偶尔会往 stderr 打 banner,影响 stdout 解析。
// 单条命令带 8s 超时;超时后命令在远端可能还在跑,session 由 ssh.Client 关闭时统一回收。
func runEsxi(client *ssh.Client, cmd string) (string, error) {
	return runEsxiTimeout(client, cmd, defaultCmdTimeout)
}

// runEsxiTimeout 同 runEsxi,但允许调用方为合批命令指定更长的超时。
//
// 末尾 `; true`:合批里某条 esxcli 偶发非零退出(典型如 NVMe 盘的 smart get 字段格式不同)
// 不应该让 sshx.Run 因为退出码非零而把已收集的 stdout 全部丢弃 —— stdout 里
// `===DEV===` 分段大概率已经写入大半,调用方按 stdout 是否空判定才是真的失败信号。
func runEsxiTimeout(client *ssh.Client, cmd string, timeout time.Duration) (string, error) {
	// 调用方拼合批命令时常在末尾留 `; `(方便循环里追加),
	// 但如果 cmd 末尾正好是 `;`,再附加 `; true` 会生成 `;;` —— POSIX/BusyBox sh
	// 对 `;;` 在非 case 上下文直接报语法错误,整段 group 不执行,stdout 为空,
	// 远端进程以 status 2 退出 —— 这正是之前看到 `smart batch failed err=Process exited with status 2`
	// 但 stdout 全空、partial 兜底打不上的根因。
	trimmed := strings.TrimRight(cmd, " \t\n;")
	full := esxiPathPrefix + "{ " + trimmed + "; true; } 2>/dev/null"
	type result struct {
		out string
		err error
	}
	ch := make(chan result, 1)
	go func() {
		out, err := sshx.Run(client, full, nil)
		ch <- result{out, err}
	}()
	select {
	case r := <-ch:
		return r.out, r.err
	case <-time.After(timeout):
		return "", fmt.Errorf("esxi command timeout after %s", timeout)
	}
}

func runEsxiRetry(client *ssh.Client, name, cmd string, timeout time.Duration, attempts int, ok func(string) bool) (string, error) {
	if attempts <= 0 {
		attempts = 1
	}
	var lastOut string
	var lastErr error
	for i := 1; i <= attempts; i++ {
		out, err := runEsxiTimeout(client, cmd, timeout)
		lastOut = out
		if err == nil && (ok == nil || ok(out)) {
			return out, nil
		}
		if err != nil {
			lastErr = err
		} else {
			lastErr = fmt.Errorf("validator failed")
		}
		if i < attempts {
			logx.Warn("esxi command retry", "name", name, "attempt", i, "err", lastErr.Error(), "bytes", len(out))
			time.Sleep(time.Duration(i) * 150 * time.Millisecond)
		}
	}
	return lastOut, fmt.Errorf("%s failed after %d attempts: %w", name, attempts, lastErr)
}

// --- 解析:平台 / 版本 ---

// parseKV 把 ESXi 一行 "Key: Value" 风格的输出拆成 map(小写 key、去空白 value)。
// 字段名规范:全小写,空格替换为下划线,与 prompt 里描述一致。
func parseKV(text string) map[string]string {
	m := map[string]string{}
	for _, line := range strings.Split(text, "\n") {
		idx := strings.IndexByte(line, ':')
		if idx <= 0 {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(line[:idx]))
		key = strings.ReplaceAll(key, " ", "_")
		val := strings.TrimSpace(line[idx+1:])
		if key == "" || val == "" {
			continue
		}
		m[key] = val
	}
	return m
}

func parsePlatform(out string) PlatformInfo {
	kv := parseKV(out)
	p := PlatformInfo{
		Vendor:  kv["vendor_name"],
		Product: kv["product_name"],
		Serial:  kv["serial_number"],
		UUID:    kv["uuid"],
	}
	if v, ok := kv["ipmi_supported"]; ok {
		p.IPMISupported = strings.EqualFold(v, "true")
	}
	// 部分版本字段名变体兜底
	if p.Vendor == "" {
		p.Vendor = kv["vendor"]
	}
	if p.Product == "" {
		p.Product = kv["product"]
	}
	return p
}

// vmware -v 形态:"VMware ESXi 7.0.3 build-24784741"
var versionRe = regexp.MustCompile(`ESXi\s+(\S+)\s+build-(\d+)`)

func parseVersionInto(p *PlatformInfo, out string) {
	if m := versionRe.FindStringSubmatch(out); len(m) == 3 {
		p.ESXiVersion = m[1]
		if b, err := strconv.ParseInt(m[2], 10, 64); err == nil {
			p.ESXiBuild = b
		}
	}
}

// --- 解析:CPU 静态 ---

// esxcli hardware cpu list 输出形如多块、每块多行 "  Key: Value",取第一块即代表平台 CPU。
//
// 注意 ESXi 7.0 之前 `Brand` 字段返回的是 vendor("GenuineIntel"),不是 model 名。
// CPU 真实型号、Current Speed、Core Count 由 fillFromSmbios 从 smbiosDump 覆盖。
//
// L2/L3 Cache Size 字段单位是 byte(实测 262144/8388608),
// 不能用 parseSizeKB(那是按 KB/MB/GB 文本单位推断),也不能用 Contains 匹配,
// 否则会被 `L2 Cache Line Size: 64` 或 `L2 Cache CPU Count: 1` 这类同前缀字段误覆盖
// (parseKV 把多个 CPU 块合并后 map 遍历顺序不定,最后赋值的赢)。
func parseCPUStatic(out string) CPUStatic {
	c := CPUStatic{TjMaxC: -1}
	kv := parseKV(out)
	if v, ok := kv["brand"]; ok {
		c.Brand = v
	} else if v, ok := kv["vendor"]; ok {
		c.Brand = v
	}
	c.Family = parseIntDefault(kv["family"], 0)
	c.ModelID = parseIntDefault(kv["model"], 0)
	c.Stepping = parseIntDefault(kv["stepping"], 0)
	if v, ok := kv["core_speed"]; ok {
		c.FreqMHz = parseFreqMHz(v)
	}
	if v, ok := kv["l2_cache_size"]; ok {
		if n, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64); err == nil {
			c.L2KB = int(n / 1024)
		}
	}
	if v, ok := kv["l3_cache_size"]; ok {
		if n, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64); err == nil {
			c.L3KB = int(n / 1024)
		}
	}
	if v, ok := kv["number_of_cpu_cores"]; ok {
		c.Cores = parseIntDefault(v, 0)
	}
	return c
}

// fillCoresFromGlobal 从 `esxcli hardware cpu global get` 补核心数。
// 输出:CPU Packages: 1 / CPU Cores: 4 / CPU Threads: 4 / Hyperthreading ...
func fillCoresFromGlobal(c *CPUStatic, out string) {
	kv := parseKV(out)
	if v, ok := kv["cpu_cores"]; ok {
		if n := parseIntDefault(v, 0); n > 0 {
			c.Cores = n
		}
	}
}

// fillFromSmbios 解 `smbiosDump` 输出里第一个 Processor Info (Type 4) 块,
// 取真实 CPU 型号、当前频率、核心数。优先级高于 esxcli cpu list / global get。
//
// 块结构示例(2 空格缩进):
//
//	Processor Info (Type 4): #74
//	  Socket: "U3E1"
//	  ...
//	  Version: "Intel(R) Xeon(R) E-2224G CPU @ 3.50GHz"
//	  ...
//	  Current Speed: 3500 MHz
//	  Core Count: 4
//
// 第一个顶格行(下一个 Type X)即块结束。
func fillFromSmbios(c *CPUStatic, out string) {
	inBlock := false
	matched := false
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, "Processor Info") && strings.Contains(line, "Type 4") {
			inBlock = true
			matched = true
			continue
		}
		if !inBlock {
			continue
		}
		// 块内行必须以空格起首;空行/顶格行视为块结束。
		if line == "" || !strings.HasPrefix(line, " ") {
			return
		}
		trim := strings.TrimSpace(line)
		idx := strings.Index(trim, ":")
		if idx <= 0 {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(trim[:idx]))
		val := strings.Trim(strings.TrimSpace(trim[idx+1:]), "\"")
		switch key {
		case "version":
			if val != "" && !strings.EqualFold(val, "To Be Filled By O.E.M.") {
				c.Brand = val
				logx.Debug("esxi smbios brand", "brand", val)
			}
		case "current speed":
			if mhz := parseFreqMHz(val); mhz > 0 {
				c.FreqMHz = mhz
			}
		case "core count":
			if n := parseIntDefault(val, 0); n > 0 {
				c.Cores = n
			}
		}
	}
	if !matched {
		logx.Warn("esxi smbios: Type 4 block not found", "bytes", len(out))
	}
}

// decodeTjMax 解 MSR_TEMPERATURE_TARGET(0x1A2)bits 23:16。
// 输入是 vsish 返回的 hex 或 dec 文本(忽略大小写、前后空白)。
func decodeTjMax(out string) int {
	s := strings.TrimSpace(out)
	if s == "" {
		return -1
	}
	n, err := parseUint64Auto(s)
	if err != nil {
		return -1
	}
	return int((n >> 16) & 0xFF)
}

// --- 解析:内存 ---

func parseMemory(out string) MemoryInfo {
	kv := parseKV(out)
	var m MemoryInfo
	for k, v := range kv {
		if strings.Contains(k, "physical_memory") {
			m.TotalBytes = parseBytes(v)
		}
	}
	return m
}

// fillMemoryFromVsish 解 `vsish -e cat /memory/comprehensive`:
//
//	Comprehensive {
//	   Physical memory estimate:134045160 KB
//	   ...
//	   Free:75142516 KB
//	}
//
// 取 Free 作为可用内存;Physical 作为总内存兜底(esxcli 没拿到时)。
func fillMemoryFromVsish(m *MemoryInfo, out string) {
	for _, line := range strings.Split(out, "\n") {
		trim := strings.TrimSpace(line)
		idx := strings.Index(trim, ":")
		if idx <= 0 {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(trim[:idx]))
		val := strings.TrimSpace(trim[idx+1:])
		switch key {
		case "free":
			m.FreeBytes = parseBytes(val)
		case "physical memory estimate":
			if m.TotalBytes <= 0 {
				m.TotalBytes = parseBytes(val)
			}
		}
	}
}

// parseRuntimeUsage 解 `vim-cmd hostsvc/hostsummary` 中 quickStats/hardware 的运行时用量:
// overallCpuUsage(MHz), overallMemoryUsage(MiB), cpuMhz, numCpuCores, memorySize(bytes)。
func parseRuntimeUsage(out string, cpu CPUStatic, mem MemoryInfo) RuntimeUsage {
	u := RuntimeUsage{CPUUsagePercent: -1, MemoryUsagePercent: -1}
	cpuUsedMHz := -1
	memUsedMiB := int64(-1)
	cpuMHz := 0
	cpuCores := 0
	memTotalBytes := int64(0)

	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(strings.TrimSuffix(line, ","))
		idx := strings.Index(line, "=")
		if idx <= 0 {
			continue
		}
		key := strings.TrimSpace(line[:idx])
		val := strings.TrimSpace(line[idx+1:])
		val = strings.TrimSuffix(val, ",")
		switch key {
		case "overallCpuUsage":
			cpuUsedMHz = parseIntDefault(val, -1)
		case "overallMemoryUsage":
			memUsedMiB = parseInt64Default(val, -1)
		case "cpuMhz":
			cpuMHz = parseIntDefault(val, 0)
		case "numCpuCores":
			cpuCores = parseIntDefault(val, 0)
		case "memorySize":
			memTotalBytes = parseInt64Default(val, 0)
		}
	}

	if cpuMHz <= 0 {
		cpuMHz = cpu.FreqMHz
	}
	if cpuCores <= 0 {
		cpuCores = cpu.Cores
	}
	if cpuUsedMHz >= 0 {
		u.CPUUsedMHz = cpuUsedMHz
		if cpuMHz > 0 && cpuCores > 0 {
			u.CPUCapacityMHz = cpuMHz * cpuCores
			u.CPUUsagePercent = percentInt(int64(cpuUsedMHz), int64(u.CPUCapacityMHz))
		}
	}

	if memTotalBytes <= 0 {
		memTotalBytes = mem.TotalBytes
	}
	if memUsedMiB >= 0 {
		u.MemoryUsedBytes = memUsedMiB * 1024 * 1024
		if memTotalBytes > 0 {
			u.MemoryTotalBytes = memTotalBytes
			u.MemoryUsagePercent = percentInt(u.MemoryUsedBytes, memTotalBytes)
		}
	}
	return u
}

// --- 采集 + 解析:CPU 温度 ---

// collectCPUTemp 通过单次 SSH session 拉所有核(0..15)的 MSR 0x1A2 / 0x19C,
// 远端用一段 shell 循环读取并按 `CORE=<n> TJ=<v> DRO=<v>` 一行一核打印,
// 失败核打印空 TJ/DRO,本地遇空即停(与逐核串行的旧行为保持一致)。
// 这样原本 N*2 次 ssh.NewSession 压缩到 1 次,典型 4 核机器省下 ~7 次 session 开销。
func collectCPUTemp(client *ssh.Client, fallbackTjMax, expectedCores int) CPUTemperature {
	res := CPUTemperature{TjMaxC: fallbackTjMax, MaxC: -1, AvgC: -1}
	script := `for i in 0 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15; do
  tj=$(vsish -e get /hardware/msr/pcpu/$i/addr/0x1A2 2>/dev/null)
  dro=$(vsish -e get /hardware/msr/pcpu/$i/addr/0x19C 2>/dev/null)
  printf 'CORE=%s TJ=%s DRO=%s\n' "$i" "$tj" "$dro"
  if [ -z "$tj" ] || [ -z "$dro" ]; then break; fi
done`
	// 16 核 × 2 次 vsish,本地 shell 循环,典型 < 2s;给 15s 留余量,
	// 防止默认 8s 在多核机器上偶发被截断。
	out, err := runEsxiRetry(client, "cpu temperature", script, 15*time.Second, 3, func(out string) bool {
		got := len(parseCPUTempOutput(out, fallbackTjMax).Cores)
		if expectedCores > 0 {
			return got >= expectedCores
		}
		return got > 0
	})
	if err != nil {
		logx.Warn("esxi cpu temperature failed", "err", err.Error())
		return res
	}
	return parseCPUTempOutput(out, fallbackTjMax)
}

func parseCPUTempOutput(out string, fallbackTjMax int) CPUTemperature {
	res := CPUTemperature{TjMaxC: fallbackTjMax, MaxC: -1, AvgC: -1}
	var sum int
	maxC := -1
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "CORE=") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 3 {
			break
		}
		idStr := strings.TrimPrefix(fields[0], "CORE=")
		tjStr := strings.TrimPrefix(fields[1], "TJ=")
		droStr := strings.TrimPrefix(fields[2], "DRO=")
		id, err := strconv.Atoi(idStr)
		if err != nil {
			break
		}
		if tjStr == "" || droStr == "" {
			break
		}
		t, err := parseUint64Auto(tjStr)
		if err != nil {
			break
		}
		ss, err := parseUint64Auto(droStr)
		if err != nil {
			break
		}
		tjmax := int((t >> 16) & 0xFF)
		dro := int((ss >> 16) & 0x7F)
		temp := tjmax - dro
		if res.TjMaxC <= 0 {
			res.TjMaxC = tjmax
		}
		res.Cores = append(res.Cores, CPUCore{ID: id, TempC: temp, HeadroomC: dro})
		if temp > maxC {
			maxC = temp
		}
		sum += temp
	}
	if len(res.Cores) > 0 {
		res.MaxC = maxC
		res.AvgC = sum / len(res.Cores)
	}
	return res
}

// --- 解析:MCE ---

// vsish -e cat /hardware/health/mce 输出形如:
//
//	Machine check error stats {
//	   Total corrected errors since boot: 0
//	   EWMA of corrected errors per period: 0
//	   Period in seconds: 120
//	   Total uncorrected errors since boot: 0
//	   Health state: 0 -> Green
//	}
var mceStateRe = regexp.MustCompile(`Health state:\s*\d+\s*->\s*(\w+)`)

func parseMCE(out string) MCEHealth {
	m := MCEHealth{State: ""}
	if mm := mceStateRe.FindStringSubmatch(out); len(mm) == 2 {
		m.State = mm[1]
	}
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "Total corrected errors"):
			m.CorrectedTotal = parseTrailingInt64(line)
		case strings.HasPrefix(line, "EWMA"):
			m.CorrectedEWMA = parseTrailingInt64(line)
		case strings.HasPrefix(line, "Period in seconds"):
			m.PeriodSeconds = int(parseTrailingInt64(line))
		case strings.HasPrefix(line, "Total uncorrected errors"):
			m.UncorrectedTotal = parseTrailingInt64(line)
		}
	}
	return m
}

// parseTrailingInt64 从 "Some prefix: 1234" / "...errors since boot: 0" 取尾部数字。
func parseTrailingInt64(line string) int64 {
	idx := strings.LastIndexByte(line, ':')
	if idx < 0 {
		return 0
	}
	tail := strings.TrimSpace(line[idx+1:])
	tail = strings.TrimSuffix(tail, "}")
	tail = strings.TrimSpace(tail)
	// 兜底:取首串数字
	for i := 0; i < len(tail); i++ {
		if tail[i] < '0' || tail[i] > '9' {
			tail = tail[:i]
			break
		}
	}
	n, _ := strconv.ParseInt(tail, 10, 64)
	return n
}

// --- 采集 + 解析:磁盘 ---

var deviceIDRe = regexp.MustCompile(`(?m)^(t10\.\S+|naa\.\S+|mpx\.\S+|eui\.\S+)`)

type diskDeviceInfo struct {
	Model         string
	Type          string
	CapacityBytes int64
}

type storageFilesystem struct {
	Name      string
	UUID      string
	Type      string
	SizeBytes int64
	FreeBytes int64
}

type vmfsExtent struct {
	VolumeName string
	UUID       string
	Device     string
	Partition  int
}

type diskUsage struct {
	Known      bool
	UsedBytes  int64
	FreeBytes  int64
	Datastores []string
}

func collectDisks(client *ssh.Client) []DiskHealth {
	// list 命令本身在多盘机器上偶发 5-10s(要走 SCSI inquiry),给 15s 留余量。
	listOut, err := runEsxiRetry(client, "disk device list", "esxcli storage core device list", 15*time.Second, 2, func(out string) bool {
		return len(deviceIDRe.FindAllString(out, -1)) > 0
	})
	if err != nil {
		logx.Warn("esxi collectDisks: list failed", "err", err.Error())
		return nil
	}
	rawIDs := deviceIDRe.FindAllString(listOut, -1)
	if len(rawIDs) == 0 {
		// list 跑通但 regex 没匹配 —— 通常是设备前缀不在已知的 t10./naa./mpx./eui. 里。
		// 截短一下输出方便看,512 字节就够看到几行了。
		head := listOut
		if len(head) > 512 {
			head = head[:512]
		}
		logx.Warn("esxi collectDisks: list parsed 0 devices", "bytes", len(listOut), "head", head)
		return nil
	}
	devInfo := parseDeviceInventory(listOut)
	usageByDevice := collectDiskUsage(client)

	// 去重保序
	seen := map[string]struct{}{}
	ids := make([]string, 0, len(rawIDs))
	for _, id := range rawIDs {
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		if !isDiskDevice(devInfo[id]) {
			continue
		}
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		logx.Warn("esxi collectDisks: no smart-capable disk devices", "n_raw", len(rawIDs))
		return nil
	}

	// 把所有盘的 SMART 合并到一次 SSH session,用 `===DEV===<id>` 行做分段标志。
	// 远端逐盘跑 esxcli smart get,stderr 已被外层 runEsxi 重定向丢掉。
	// SMART 单盘 1~2s,N 盘合批后总耗时随盘数线性增长(8 盘可能 12-15s),
	// 这里给到 25s 上限,远高于默认 8s 单命令超时,避免被截断成空结果。
	var b strings.Builder
	for _, id := range ids {
		b.WriteString(`printf '===DEV===%s\n' `)
		b.WriteString(sshx.ShellQuote(id))
		b.WriteString("; esxcli storage core device smart get -d ")
		b.WriteString(sshx.ShellQuote(id))
		b.WriteString("; ")
	}
	smartAll, err := runEsxiRetry(client, "disk smart batch", b.String(), 30*time.Second, 2, func(out string) bool {
		return strings.Count(out, "===DEV===") >= len(ids)
	})
	if err != nil {
		// 即便 ssh.Run 返回非零(NVMe 等盘的 smart get 偶尔以 status 2 退出),
		// stdout 里大概率已经吐出了大半 `===DEV===` 段,优先按 stdout 是否含分段判定。
		if !strings.Contains(smartAll, "===DEV===") {
			logx.Warn("esxi collectDisks: smart batch failed", "err", err.Error(), "n_dev", len(ids))
			return nil
		}
		logx.Warn("esxi collectDisks: smart batch partial", "err", err.Error(), "n_dev", len(ids), "bytes", len(smartAll))
	}
	smartByID := splitSMARTOutput(smartAll)
	retryMissingSMART(client, ids, smartByID)
	// ATA 盘走 vsish 拿真实 6-byte raw,修正 esxcli 截断到 1 字节的 attribute 9/12 等。
	// NVMe 不支持 vsish smart 路径,这里直接跳过(返回 map 不包含 NVMe 盘)。
	ataAttrsByID := collectATASmartBuffers(client, ids)
	logx.Debug("esxi collectDisks", "n_dev", len(ids), "smart_bytes", len(smartAll), "smart_segments", len(smartByID), "ata_vsish", len(ataAttrsByID))

	var out []DiskHealth
	for _, id := range ids {
		smart := smartByID[id]
		attrs := parseSMARTAttrs(smart)
		if ata, ok := ataAttrsByID[id]; ok {
			// vsish 给到非 -1 的字段优先覆盖 esxcli 解析结果(esxcli Raw 列截断到低 1 字节)
			if ata.PowerOnHours >= 0 {
				attrs.PowerOnHours = ata.PowerOnHours
			}
			if ata.PowerCycleCount >= 0 {
				attrs.PowerCycleCount = ata.PowerCycleCount
			}
			if ata.ReallocatedSectors >= 0 {
				attrs.ReallocatedSectors = ata.ReallocatedSectors
			}
			if ata.PendingSectorReallocation >= 0 {
				attrs.PendingSectorReallocation = ata.PendingSectorReallocation
			}
			if ata.UncorrectableErrors >= 0 {
				attrs.UncorrectableErrors = ata.UncorrectableErrors
			}
		}
		info := devInfo[id]
		usage := usageByDevice[id]
		usedBytes := int64(-1)
		freeBytes := int64(-1)
		if usage.Known {
			usedBytes = usage.UsedBytes
			freeBytes = usage.FreeBytes
		}
		out = append(out, DiskHealth{
			Device:                    id,
			Model:                     info.Model,
			Type:                      info.Type,
			CapacityBytes:             info.CapacityBytes,
			UsedBytes:                 usedBytes,
			FreeBytes:                 freeBytes,
			Datastores:                usage.Datastores,
			TempC:                     attrs.TempC,
			ThresholdC:                attrs.ThresholdC,
			Status:                    classifyDisk(info.Type, attrs.TempC),
			HealthStatus:              attrs.HealthStatus,
			PowerOnHours:              attrs.PowerOnHours,
			PowerCycleCount:           attrs.PowerCycleCount,
			ReallocatedSectors:        attrs.ReallocatedSectors,
			UncorrectableErrors:       attrs.UncorrectableErrors,
			MediaWearoutValue:         attrs.MediaWearoutValue,
			ReadErrorCount:            attrs.ReadErrorCount,
			PendingSectorReallocation: attrs.PendingSectorReallocation,
		})
	}
	return out
}

func isDiskDevice(info diskDeviceInfo) bool {
	model := strings.ToLower(info.Model)
	devType := strings.ToLower(info.Type)
	if strings.Contains(model, "dvd") || strings.Contains(model, "cd-rom") || strings.Contains(model, "cdrom") {
		return false
	}
	if strings.Contains(devType, "cd-rom") || strings.Contains(devType, "cdrom") || strings.Contains(devType, "optical") {
		return false
	}
	return true
}

func retryMissingSMART(client *ssh.Client, ids []string, smartByID map[string]string) {
	for _, id := range ids {
		if parseSMARTAttrs(smartByID[id]).TempC >= 0 {
			continue
		}
		cmd := "esxcli storage core device smart get -d " + sshx.ShellQuote(id)
		out, err := runEsxiRetry(client, "disk smart "+id, cmd, 10*time.Second, 2, func(out string) bool {
			return parseSMARTAttrs(out).TempC >= 0
		})
		if err != nil {
			logx.Warn("esxi collectDisks: smart retry failed", "device", id, "err", err.Error())
			continue
		}
		smartByID[id] = out
	}
}

func collectDiskUsage(client *ssh.Client) map[string]diskUsage {
	fsOut, fsErr := runEsxiRetry(client, "storage filesystem list", "esxcli storage filesystem list", defaultCmdTimeout, 2, func(out string) bool {
		return len(parseStorageFilesystems(out)) > 0
	})
	extentOut, extentErr := runEsxiRetry(client, "storage vmfs extent list", "esxcli storage vmfs extent list", defaultCmdTimeout, 2, func(out string) bool {
		return len(parseVMFSExtents(out)) > 0
	})
	if fsErr != nil || extentErr != nil {
		if fsErr != nil {
			logx.Warn("esxi collectDisks: filesystem list failed", "err", fsErr.Error())
		}
		if extentErr != nil {
			logx.Warn("esxi collectDisks: vmfs extent list failed", "err", extentErr.Error())
		}
		return nil
	}
	return mapDiskUsage(parseStorageFilesystems(fsOut), parseVMFSExtents(extentOut))
}

// splitSMARTOutput 按 `===DEV===<id>` 标记切合批后的 SMART 输出,返回 id→该盘原始 SMART 文本。
func splitSMARTOutput(out string) map[string]string {
	res := map[string]string{}
	var currentID string
	var buf strings.Builder
	flush := func() {
		if currentID != "" {
			res[currentID] = buf.String()
		}
		buf.Reset()
	}
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "===DEV===") {
			flush()
			currentID = strings.TrimPrefix(line, "===DEV===")
			currentID = strings.TrimRight(currentID, "\r")
			continue
		}
		if currentID == "" {
			continue
		}
		buf.WriteString(line)
		buf.WriteByte('\n')
	}
	flush()
	return res
}

// parseDeviceInventory 扫 `esxcli storage core device list` 输出,
// 把每个设备的 Model / Type / Size 映射出来(同一段以设备 id 起首,后续若干行缩进键值)。
func parseDeviceInventory(out string) map[string]diskDeviceInfo {
	devices := map[string]diskDeviceInfo{}
	var current string
	for _, line := range strings.Split(out, "\n") {
		trim := strings.TrimSpace(line)
		if trim == "" {
			continue
		}
		if isStorageDeviceID(trim) {
			current = strings.Fields(trim)[0]
			if _, ok := devices[current]; !ok {
				devices[current] = diskDeviceInfo{}
			}
			continue
		}
		if current == "" {
			continue
		}
		idx := strings.IndexByte(trim, ':')
		if idx <= 0 {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(trim[:idx]))
		val := strings.TrimSpace(trim[idx+1:])
		info := devices[current]
		switch key {
		case "model":
			if val != "" {
				info.Model = val
			}
		case "device type":
			if val != "" {
				info.Type = val
			}
		case "size":
			info.CapacityBytes = parseESXiDeviceSize(val)
		}
		devices[current] = info
	}
	return devices
}

func isStorageDeviceID(s string) bool {
	return strings.HasPrefix(s, "t10.") ||
		strings.HasPrefix(s, "naa.") ||
		strings.HasPrefix(s, "mpx.") ||
		strings.HasPrefix(s, "eui.")
}

// parseESXiDeviceSize 解 `esxcli storage core device list` 的 Size 字段。
// 该字段裸数字单位是 MiB;如果输出带 GB/Bytes 等单位,交给 parseBytes。
func parseESXiDeviceSize(s string) int64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	low := strings.ToLower(s)
	if strings.Contains(low, "b") {
		return parseBytes(s)
	}
	num := extractLeadingFloat(s)
	if num == "" {
		return 0
	}
	f, err := strconv.ParseFloat(num, 64)
	if err != nil {
		return 0
	}
	return int64(f * 1024 * 1024)
}

// parseStorageFilesystems 解 `esxcli storage filesystem list`。
// 只保留已挂载 VMFS datastore;Size/Free 在该命令里是 bytes。
func parseStorageFilesystems(out string) []storageFilesystem {
	var list []storageFilesystem
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) < 7 {
			continue
		}
		if fields[0] == "Mount" || strings.HasPrefix(fields[0], "---") {
			continue
		}
		freeBytes := parseBytes(fields[len(fields)-1])
		sizeBytes := parseBytes(fields[len(fields)-2])
		fsType := fields[len(fields)-3]
		mounted := fields[len(fields)-4]
		uuid := fields[len(fields)-5]
		if !strings.EqualFold(mounted, "true") || !strings.HasPrefix(strings.ToLower(fsType), "vmfs") {
			continue
		}
		name := strings.Join(fields[1:len(fields)-5], " ")
		if name == "" || uuid == "" || sizeBytes <= 0 {
			continue
		}
		list = append(list, storageFilesystem{
			Name:      name,
			UUID:      uuid,
			Type:      fsType,
			SizeBytes: sizeBytes,
			FreeBytes: freeBytes,
		})
	}
	return list
}

// parseVMFSExtents 解 `esxcli storage vmfs extent list`,用于把 datastore 关联回 device。
func parseVMFSExtents(out string) []vmfsExtent {
	var list []vmfsExtent
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) < 5 {
			continue
		}
		if fields[0] == "Volume" || strings.HasPrefix(fields[0], "---") {
			continue
		}
		partition := parseIntDefault(fields[len(fields)-1], -1)
		device := fields[len(fields)-2]
		uuid := fields[len(fields)-4]
		name := strings.Join(fields[:len(fields)-4], " ")
		if name == "" || uuid == "" || device == "" {
			continue
		}
		list = append(list, vmfsExtent{
			VolumeName: name,
			UUID:       uuid,
			Device:     device,
			Partition:  partition,
		})
	}
	return list
}

func mapDiskUsage(filesystems []storageFilesystem, extents []vmfsExtent) map[string]diskUsage {
	fsByUUID := map[string]storageFilesystem{}
	fsByName := map[string]storageFilesystem{}
	for _, fs := range filesystems {
		fsByUUID[strings.ToLower(fs.UUID)] = fs
		fsByName[fs.Name] = fs
	}

	type datastoreExtent struct {
		fs      storageFilesystem
		devices map[string]struct{}
	}
	byDatastore := map[string]datastoreExtent{}
	for _, ext := range extents {
		fs, ok := fsByUUID[strings.ToLower(ext.UUID)]
		if !ok {
			fs, ok = fsByName[ext.VolumeName]
		}
		if !ok {
			continue
		}
		key := strings.ToLower(fs.UUID)
		if key == "" {
			key = "name:" + fs.Name
		}
		item := byDatastore[key]
		if item.devices == nil {
			item.fs = fs
			item.devices = map[string]struct{}{}
		}
		item.devices[ext.Device] = struct{}{}
		byDatastore[key] = item
	}

	usageByDevice := map[string]diskUsage{}
	for _, item := range byDatastore {
		usedBytes := item.fs.SizeBytes - item.fs.FreeBytes
		if usedBytes < 0 {
			usedBytes = 0
		}
		if len(item.devices) != 1 {
			// 多 extent datastore 不能可靠拆分到单盘,只记录关联名称,不写用量。
			for dev := range item.devices {
				u := usageByDevice[dev]
				u.Datastores = appendUniqueString(u.Datastores, item.fs.Name)
				usageByDevice[dev] = u
			}
			continue
		}
		for dev := range item.devices {
			u := usageByDevice[dev]
			u.Known = true
			u.UsedBytes += usedBytes
			u.FreeBytes += item.fs.FreeBytes
			u.Datastores = appendUniqueString(u.Datastores, item.fs.Name)
			usageByDevice[dev] = u
		}
	}
	return usageByDevice
}

func appendUniqueString(list []string, item string) []string {
	if item == "" {
		return list
	}
	for _, v := range list {
		if v == item {
			return list
		}
	}
	return append(list, item)
}

// parseSMARTAttrs 把 `esxcli storage core device smart get` 的输出解析成统一属性。
//
// 列布局实测三家差异(prompt 6_ESXI_SSH_MONITORING.md 16.4 + 16 节):
//
//	ATA SSD (Samsung 870):  Drive Temperature 66 0  49  34   → Raw=34°C 是真值
//	ATA HDD (WDC):          Drive Temperature 48 0  N/A 45   → Raw=45°C 是真值
//	NVMe (Samsung 990 PRO): Drive Temperature 46 82 N/A N/A  → Raw=N/A,Value=46 才是真值
//
// 取值统一:Raw 优先,Raw=N/A 时回退 Value(吃下 NVMe);否则返回 -1。
// 行结构:`<参数名 token1..tokenN> <Value> <Threshold> <Worst> <Raw>`,末尾固定 4 列。
// 参数名可能是 1~多 token,例如 "Pending Sector Reallocation Count"(4 个 token)。
func parseSMARTAttrs(out string) SMARTAttrs {
	attrs := newSMARTAttrs()
	for _, line := range strings.Split(out, "\n") {
		trim := strings.TrimSpace(line)
		if trim == "" {
			continue
		}
		fields := strings.Fields(trim)
		// 最少要有 1 个名字 token + 4 个数据列 = 5 列。
		if len(fields) < 5 {
			continue
		}
		// 跳过表头和分隔线。
		if fields[0] == "Parameter" || strings.HasPrefix(fields[0], "---") {
			continue
		}
		nameTokens := fields[:len(fields)-4]
		name := strings.Join(nameTokens, " ")
		valueCol := fields[len(fields)-4]
		thresholdCol := fields[len(fields)-3]
		rawCol := fields[len(fields)-1]

		switch name {
		case "Health Status":
			if valueCol != "" && valueCol != "N/A" {
				attrs.HealthStatus = valueCol
			}
		case "Power-on Hours":
			attrs.PowerOnHours = smartInt64Pick(rawCol, valueCol)
		case "Power Cycle Count":
			attrs.PowerCycleCount = smartInt64Pick(rawCol, valueCol)
		case "Reallocated Sector Count":
			attrs.ReallocatedSectors = smartInt64Pick(rawCol, valueCol)
		case "Uncorrectable Error Count", "Uncorrectable Sector Count":
			attrs.UncorrectableErrors = smartInt64Pick(rawCol, valueCol)
		case "Media Wearout Indicator":
			// SSD 独有。Value 是 normalized(100=新,0=磨损完),跨厂商可比较;Raw 各家含义不同,不存。
			attrs.MediaWearoutValue = parseIntDefault(valueCol, -1)
		case "Read Error Count":
			attrs.ReadErrorCount = smartInt64Pick(rawCol, valueCol)
		case "Pending Sector Reallocation Count":
			attrs.PendingSectorReallocation = smartInt64Pick(rawCol, valueCol)
		case "Drive Temperature":
			attrs.TempC = smartIntPick(rawCol, valueCol)
			attrs.ThresholdC = parseIntDefault(thresholdCol, -1)
		}
	}
	return attrs
}

// SMARTAttrs 是 parseSMARTAttrs 的中间产物,collectDisks 取需要的字段写到 DiskHealth。
type SMARTAttrs struct {
	HealthStatus              string
	PowerOnHours              int64
	PowerCycleCount           int64
	ReallocatedSectors        int64
	UncorrectableErrors       int64
	MediaWearoutValue         int
	ReadErrorCount            int64
	PendingSectorReallocation int64
	TempC                     int
	ThresholdC                int
}

func newSMARTAttrs() SMARTAttrs {
	return SMARTAttrs{
		PowerOnHours:              -1,
		PowerCycleCount:           -1,
		ReallocatedSectors:        -1,
		UncorrectableErrors:       -1,
		MediaWearoutValue:         -1,
		ReadErrorCount:            -1,
		PendingSectorReallocation: -1,
		TempC:                     -1,
		ThresholdC:                -1,
	}
}

func smartInt64Pick(raw, value string) int64 {
	if v := parseInt64Default(raw, -1); v >= 0 {
		return v
	}
	return parseInt64Default(value, -1)
}

func smartIntPick(raw, value string) int {
	if v := parseIntDefault(raw, -1); v >= 0 {
		return v
	}
	return parseIntDefault(value, -1)
}

// ataSMARTAttrs 是 ATA 盘从 vsish valuesBuffer 解析出的真实 6-byte raw 值。
// 作用:覆盖 esxcli 输出的 Raw 列(对 ATA 盘只暴露低 1 字节,Power-on Hours 等会被截到 0-255)。
// NVMe 不走这条路(vsish 该节点 Not supported),所有字段保持 -1。
type ataSMARTAttrs struct {
	PowerOnHours              int64
	PowerCycleCount           int64
	ReallocatedSectors        int64
	PendingSectorReallocation int64
	UncorrectableErrors       int64
}

func newATASMARTAttrs() ataSMARTAttrs {
	return ataSMARTAttrs{
		PowerOnHours:              -1,
		PowerCycleCount:           -1,
		ReallocatedSectors:        -1,
		PendingSectorReallocation: -1,
		UncorrectableErrors:       -1,
	}
}

// collectATASmartBuffers 对每块 ATA 盘合批跑 vsish,读 /storage/scsifw/devices/<id>/smart/valuesBuffer。
// vsish 返回的 512 字节就是 ATA SMART data 结构:bytes[0:2]=revision + 30 个 12 字节 attribute entry。
// NVMe 设备前缀以 "t10.NVMe" 起手,直接跳过(vsish 该路径返回 "Not supported")。
func collectATASmartBuffers(client *ssh.Client, ids []string) map[string]ataSMARTAttrs {
	var ataIDs []string
	for _, id := range ids {
		if strings.HasPrefix(id, "t10.ATA_") {
			ataIDs = append(ataIDs, id)
		}
	}
	if len(ataIDs) == 0 {
		return nil
	}
	var b strings.Builder
	for _, id := range ataIDs {
		b.WriteString(`printf '===DEV===%s\n' `)
		b.WriteString(sshx.ShellQuote(id))
		b.WriteString("; vsish -e get /storage/scsifw/devices/")
		b.WriteString(sshx.ShellQuote(id))
		b.WriteString("/smart/valuesBuffer; ")
	}
	out, err := runEsxiRetry(client, "disk vsish smart batch", b.String(), 20*time.Second, 2, func(s string) bool {
		return strings.Count(s, "===DEV===") >= len(ataIDs)
	})
	if err != nil && !strings.Contains(out, "===DEV===") {
		logx.Warn("esxi vsish smart batch failed", "err", err.Error(), "n", len(ataIDs))
		return nil
	}
	segs := splitSMARTOutput(out)
	res := map[string]ataSMARTAttrs{}
	for id, seg := range segs {
		if attrs, ok := parseATASMARTBuffer(seg); ok {
			res[id] = attrs
		}
	}
	return res
}

// parseATASMARTBuffer 把 vsish 输出(每行形如 `[N]: 0xXX`)还原成 512 字节 ATA SMART data,
// 然后按 12 字节为一组扫 30 个 attribute,取关注的 attribute id 的 6 字节 raw(little endian)。
// 不关心 value/worst 列(那些 esxcli 已经给到了)。
// attribute ID 含义(SATA 标准):5=Reallocated / 9=Power-on Hours / 12=Power Cycle / 197=Pending Realloc / 198=Offline Uncorr。
func parseATASMARTBuffer(text string) (ataSMARTAttrs, bool) {
	attrs := newATASMARTAttrs()
	var buf [512]byte
	got := 0
	for _, line := range strings.Split(text, "\n") {
		trim := strings.TrimSpace(line)
		if !strings.HasPrefix(trim, "[") {
			continue
		}
		rb := strings.IndexByte(trim, ']')
		if rb < 2 {
			continue
		}
		idx, err := strconv.Atoi(trim[1:rb])
		if err != nil || idx < 0 || idx >= 512 {
			continue
		}
		hx := strings.Index(trim, "0x")
		if hx < 0 {
			continue
		}
		end := hx + 2
		for end < len(trim) {
			c := trim[end]
			if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
				break
			}
			end++
		}
		n, err := strconv.ParseUint(trim[hx+2:end], 16, 8)
		if err != nil {
			continue
		}
		buf[idx] = byte(n)
		got++
	}
	// 解析过少说明 vsish 输出截断/格式异常,放弃覆盖,沿用 esxcli。
	if got < 50 {
		return attrs, false
	}
	for i := 0; i < 30; i++ {
		off := 2 + i*12
		aid := buf[off]
		if aid == 0 {
			continue
		}
		var raw int64
		for j := 0; j < 6; j++ {
			raw |= int64(buf[off+5+j]) << (8 * j)
		}
		switch aid {
		case 5:
			attrs.ReallocatedSectors = raw
		case 9:
			attrs.PowerOnHours = raw
		case 12:
			attrs.PowerCycleCount = raw
		case 197:
			attrs.PendingSectorReallocation = raw
		case 198:
			attrs.UncorrectableErrors = raw
		}
	}
	return attrs, true
}

// classifyDisk 给磁盘按温度评 ok/warning/critical(基于 prompt 表格)。
func classifyDisk(devType string, temp int) string {
	if temp < 0 {
		return "unknown"
	}
	t := strings.ToUpper(devType)
	switch {
	case strings.Contains(t, "NVME"):
		switch {
		case temp >= 80:
			return "critical"
		case temp >= 70:
			return "warning"
		}
	case strings.Contains(t, "SSD"):
		switch {
		case temp >= 70:
			return "critical"
		case temp >= 60:
			return "warning"
		}
	case strings.Contains(t, "HDD"), strings.Contains(t, "ATA"):
		switch {
		case temp >= 55:
			return "critical"
		case temp >= 50:
			return "warning"
		}
	}
	return "ok"
}

// --- 采集 + 解析:USB ---

// collectUSB 拉控制器 / arbitrator / 可直通设备,VM 持有部分用调用方共享的 VMShallow 列表
// (避免在这里再跑一次 getallvms)。
func collectUSB(client *ssh.Client, vms []VMShallow) USBState {
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
	u.VMOwned = collectUSBVMOwned(client, vms)
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
func collectUSBVMOwned(client *ssh.Client, vms []VMShallow) []USBVMOwned {
	if len(vms) == 0 {
		return nil
	}
	devByVM := batchVMDevices(client, vms)
	var out []USBVMOwned
	for _, v := range vms {
		devOut, ok := devByVM[v.ID]
		if !ok || devOut == "" {
			continue
		}
		out = append(out, extractVirtualUSB(v.ID, v.Name, devOut)...)
	}
	return out
}

// batchVMDevices 把所有 VM 的 vim-cmd vmsvc/device.getdevices 合到一次 session,
// 用 `===VM===<id>` 行做分段标志。device.getdevices 单次几百毫秒,N VM 串起来很贵。
// 输出可能很大(每 VM 几 KB),给 20s 超时留余量。
func batchVMDevices(client *ssh.Client, vms []VMShallow) map[int]string {
	res := map[int]string{}
	if len(vms) == 0 {
		return res
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
	return res
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

var quotedRe = regexp.MustCompile(`"([^"]*)"`)

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

// --- 采集 + 解析:VM ---

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
func collectVMs(client *ssh.Client, shallow []VMShallow, guestOS map[int]string) []VM {
	if shallow == nil {
		// 调用方拿到的 getallvms 失败 —— 返回 nil,buildSample 会写 vm_total/vm_powered_on=-1。
		return nil
	}
	powerByID := batchVMPowerState(client, shallow)
	vms := make([]VM, 0, len(shallow))
	for _, s := range shallow {
		state, ok := powerByID[s.ID]
		if !ok {
			state = "unknown"
		}
		vms = append(vms, VM{
			ID:      s.ID,
			Name:    s.Name,
			GuestOS: guestOS[s.ID],
			State:   state,
		})
	}
	return vms
}

// batchVMPowerState 把多 VM 的 power.getstate 合并到一次 SSH session,
// 远端用 `===VM===<id>` 行作分段标志。每次 vim-cmd 启动很慢(~1s),合批省得最明显。
func batchVMPowerState(client *ssh.Client, vms []VMShallow) map[int]string {
	res := map[int]string{}
	if len(vms) == 0 {
		return res
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
	return res
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

func parseIntDefault(s string, def int) int {
	s = strings.TrimSpace(s)
	if s == "" {
		return def
	}
	if n, err := strconv.Atoi(s); err == nil {
		return n
	}
	// 尝试取前导整数(如 "3504 MHz")
	end := 0
	for end < len(s) && (s[end] >= '0' && s[end] <= '9') {
		end++
	}
	if end == 0 {
		return def
	}
	if n, err := strconv.Atoi(s[:end]); err == nil {
		return n
	}
	return def
}

func parseInt64Default(s string, def int64) int64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return def
	}
	if n, err := strconv.ParseInt(s, 10, 64); err == nil {
		return n
	}
	end := 0
	for end < len(s) && (s[end] >= '0' && s[end] <= '9') {
		end++
	}
	if end == 0 {
		return def
	}
	if n, err := strconv.ParseInt(s[:end], 10, 64); err == nil {
		return n
	}
	return def
}

func percentInt(used, total int64) int {
	if used < 0 || total <= 0 {
		return -1
	}
	return int((used*100 + total/2) / total)
}

// parseUint64Auto 支持 "0x..." 与十进制。
func parseUint64Auto(s string) (uint64, error) {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "0x") || strings.HasPrefix(s, "0X") {
		return strconv.ParseUint(s[2:], 16, 64)
	}
	return strconv.ParseUint(s, 10, 64)
}

// parseFreqMHz 把 "3504 MHz" / "3504000000 Hz" / "3.5 GHz" 之类统一成 MHz。
func parseFreqMHz(s string) int {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	low := strings.ToLower(s)
	var f float64
	num := extractLeadingFloat(s)
	if num != "" {
		if v, err := strconv.ParseFloat(num, 64); err == nil {
			f = v
		}
	}
	switch {
	case strings.Contains(low, "ghz"):
		return int(f * 1000)
	case strings.Contains(low, "hz") && !strings.Contains(low, "mhz") && !strings.Contains(low, "khz"):
		return int(f / 1_000_000)
	default:
		// 没有显式单位:ESXi 的 `Core Speed` 字段实际是 Hz(几十亿那种大整数)。
		// 合理 CPU 主频范围 < 100000 MHz,超过即视为 Hz。
		if f > 100000 {
			return int(f / 1_000_000)
		}
		return int(f)
	}
}

// parseSizeKB 把 "256 KB" / "8 MB" / "8192 KB" / "1 GiB" 统一成 KB。
func parseSizeKB(s string) int {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	low := strings.ToLower(s)
	num := extractLeadingFloat(s)
	var f float64
	if num != "" {
		if v, err := strconv.ParseFloat(num, 64); err == nil {
			f = v
		}
	}
	switch {
	case strings.Contains(low, "gb") || strings.Contains(low, "gib"):
		return int(f * 1024 * 1024)
	case strings.Contains(low, "mb") || strings.Contains(low, "mib"):
		return int(f * 1024)
	default:
		return int(f)
	}
}

// parseBytes 把 "137262243840 Bytes" / "131072 MB" 等统一成 bytes。
func parseBytes(s string) int64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	low := strings.ToLower(s)
	num := extractLeadingFloat(s)
	var f float64
	if num != "" {
		if v, err := strconv.ParseFloat(num, 64); err == nil {
			f = v
		}
	}
	switch {
	case strings.Contains(low, "gb") || strings.Contains(low, "gib"):
		return int64(f * 1024 * 1024 * 1024)
	case strings.Contains(low, "mb") || strings.Contains(low, "mib"):
		return int64(f * 1024 * 1024)
	case strings.Contains(low, "kb") || strings.Contains(low, "kib"):
		return int64(f * 1024)
	default:
		return int64(f)
	}
}

// extractLeadingFloat 取字符串前导可解析为 float 的子串(允许小数点)。
func extractLeadingFloat(s string) string {
	end := 0
	dot := false
	for end < len(s) {
		c := s[end]
		if c >= '0' && c <= '9' {
			end++
			continue
		}
		if c == '.' && !dot {
			dot = true
			end++
			continue
		}
		break
	}
	return s[:end]
}

// collectHostBoot 一气呵成抓 uptime + 当前 epoch + zdump 数,
// 后端用 (now - uptime) 算 booted_at,避免依赖远端时区。
// 输出格式(以行为单位的 KV)便于解析,无 zdump 时 LATEST 留空。
func collectHostBoot(client *ssh.Client) HostBoot {
	boot := newHostBoot()
	cmd := strings.Join([]string{
		`u=$(esxcli system stats uptime get 2>/dev/null)`,
		`n=$(date +%s)`,
		`c=$(ls /var/core/vmkernel-zdump.* 2>/dev/null | wc -l)`,
		`m=$(ls /var/core/vmkernel-zdump.* 2>/dev/null | xargs -n1 stat -c '%Y' 2>/dev/null | sort -n | tail -1)`,
		`printf 'UPTIME_US=%s\nNOW_EPOCH=%s\nZDUMP_COUNT=%s\nZDUMP_LATEST=%s\n' "$u" "$n" "$c" "$m"`,
	}, "; ")
	out, err := runEsxiRetry(client, "host boot", cmd, defaultCmdTimeout, 2, func(s string) bool {
		return strings.Contains(s, "UPTIME_US=") && strings.Contains(s, "NOW_EPOCH=")
	})
	if err != nil {
		logx.Warn("esxi host boot fetch failed", "err", err.Error())
		return boot
	}
	return parseHostBoot(out)
}

// parseHostBoot 解析 collectHostBoot 的 KV 输出。
// uptime 单位是微秒(esxcli system stats uptime get 实测),换算成秒。
func parseHostBoot(out string) HostBoot {
	boot := newHostBoot()
	kv := map[string]string{}
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if i := strings.Index(line, "="); i > 0 {
			kv[line[:i]] = line[i+1:]
		}
	}
	uptimeUS, _ := strconv.ParseInt(kv["UPTIME_US"], 10, 64)
	nowEpoch, _ := strconv.ParseInt(kv["NOW_EPOCH"], 10, 64)
	if uptimeUS > 0 {
		boot.UptimeSeconds = uptimeUS / 1_000_000
		if nowEpoch > 0 {
			boot.BootedAt = time.Unix(nowEpoch-boot.UptimeSeconds, 0).UTC()
		}
	}
	if c, err := strconv.Atoi(kv["ZDUMP_COUNT"]); err == nil {
		boot.CrashDumpCount = c
	}
	if m, _ := strconv.ParseInt(kv["ZDUMP_LATEST"], 10, 64); m > 0 {
		boot.LastCrashAt = time.Unix(m, 0).UTC()
	}
	return boot
}

// collectNICs 抓物理网卡列表 + 每张卡的 stats。
// list 给基本/链路信息一次拿全,stats 按设备名循环。
// vmnic 数量个位数,串行调用足够,无需 batch。
func collectNICs(client *ssh.Client) []NIC {
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
	for i := range nics {
		statsCmd := "esxcli network nic stats get -n " + sshx.ShellQuote(nics[i].Name)
		out, err := runEsxiRetry(client, "nic stats "+nics[i].Name, statsCmd, defaultCmdTimeout, 2, func(s string) bool {
			return strings.Contains(s, "Packets received") || strings.Contains(s, "Bytes received")
		})
		if err != nil {
			logx.Warn("esxi nic stats failed", "nic", nics[i].Name, "err", err.Error())
			continue
		}
		fillNICStats(&nics[i], out)
	}
	return nics
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

// collectNetTopology 抓 vSwitch + 每个 VM 的 vNIC 边 + VMkernel 端口。
// vSwitch list 一次拿全;`esxcli network vm list` 给出每台正在跑的 VM 的 World ID,
// 再对每个 World ID 跑一次 `esxcli network vm port list -w <wid>`。
// 注意:这里用的 ID 是 World ID(esxcli 体系),与 VMShallow.ID(vim-cmd 的 Vmid)不是同一个值。
// expectVMs 是已知开机的 VM 名单(来自 vim-cmd 电源态):`esxcli network vm list`
// 偶发把表输出截断时行数会变少,用名单做 validator 让 runEsxiRetry 能感知并重试。
func collectNetTopology(client *ssh.Client, expectVMs []string) NetTopology {
	topo := NetTopology{}
	if out, err := runEsxiRetry(client, "vswitch list", "esxcli network vswitch standard list", defaultCmdTimeout, 2, func(s string) bool {
		return strings.Contains(s, "Uplinks:") || strings.Contains(s, "vSwitch")
	}); err == nil {
		topo.VSwitches = parseVSwitchList(out)
	} else {
		logx.Warn("esxi vswitch list failed", "err", err.Error())
	}
	topo.VMKPorts, topo.VMKCollected = collectVMKPorts(client)
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
	}
	for _, vm := range parseVMNetList(listOut) {
		cmd := fmt.Sprintf("esxcli network vm port list -w %d", vm.worldID)
		out, err := runEsxiRetry(client, "vm port "+vm.name, cmd, defaultCmdTimeout, 2, func(s string) bool {
			return strings.Contains(s, "Port ID:") || strings.TrimSpace(s) == ""
		})
		if err != nil {
			logx.Warn("esxi vm port list failed", "vm", vm.name, "err", err.Error())
			continue
		}
		for _, link := range parseVMPortList(out) {
			link.VMID = vm.worldID
			link.VMName = vm.name
			topo.VMNICs = append(topo.VMNICs, link)
		}
	}
	return topo
}

func collectVMKPorts(client *ssh.Client) ([]VMKPort, bool) {
	out, err := runEsxiRetry(client, "vmkernel interface list", "esxcli network ip interface list", defaultCmdTimeout, 2, func(s string) bool {
		return strings.Contains(s, "Name:") || strings.Contains(s, "vmk")
	})
	if err != nil {
		logx.Warn("esxi vmkernel interface list failed", "err", err.Error())
		return nil, false
	}
	ports := parseIPInterfaceList(out)
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
			continue
		}
		ports[i].IPv4 = parseIPv4Get(ipOut, ports[i].Name)
	}
	return ports, true
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

// splitCSV 拆 "a, b, c" 风格的列表,去空格、丢空段。
func splitCSV(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}
