package esximon

// ESXi 采集数据结构。

import (
	"time"
)

// PlatformInfo 主机标识 + ESXi 版本(启动时一次就够,但我们每轮都顺手刷一次)。
type PlatformInfo struct {
	Vendor                  string    `json:"vendor"`
	Product                 string    `json:"product"`
	Serial                  string    `json:"serial"`
	UUID                    string    `json:"uuid"`
	IPMISupported           bool      `json:"ipmi_supported"`
	ESXiVersion             string    `json:"esxi_version"`
	ESXiBuild               int64     `json:"esxi_build"`
	StaticLastFullSuccessAt time.Time `json:"static_last_full_success_at,omitzero"`
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
	HealthStatus              string    `json:"smart_health"`                 // OK / 厂商私有
	PowerOnHours              int64     `json:"smart_power_on_hours"`         // 通电小时
	PowerCycleCount           int64     `json:"smart_power_cycle_count"`      // 开机次数
	ReallocatedSectors        int64     `json:"smart_reallocated_sectors"`    // 重映射扇区数
	UncorrectableErrors       int64     `json:"smart_uncorrectable_errors"`   // SSD/HDD 二选一名,合并入此字段
	MediaWearoutValue         int       `json:"smart_media_wearout"`          // SSD 独有,normalized 100=新,0=磨损完
	ReadErrorCount            int64     `json:"smart_read_error_count"`       // HDD 独有
	PendingSectorReallocation int64     `json:"smart_pending_sector_realloc"` // HDD 独有
	SMARTLastFullSuccessAt    time.Time `json:"smart_last_full_success_at,omitzero"`
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
	Controllers              []USBController        `json:"controllers"`
	ArbitratorRunning        bool                   `json:"arbitrator_running"`
	AvailableForPassthrough  []USBPassthroughDevice `json:"available_for_passthrough"`
	VMOwned                  []USBVMOwned           `json:"vm_owned"`
	VMOwnedLastFullSuccessAt time.Time              `json:"vm_owned_last_full_success_at,omitzero"`

	controllersKnown bool
	arbitratorKnown  bool
	passthroughKnown bool
	vmOwnedKnown     bool
}

// VM 单台虚拟机。
type VM struct {
	ID                          int       `json:"id"`
	Name                        string    `json:"name"`
	GuestOS                     string    `json:"guest_os"`
	State                       string    `json:"state"` // powered_on / powered_off / suspended / unknown
	PowerStateLastFullSuccessAt time.Time `json:"power_state_last_full_success_at,omitzero"`
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

	RxBytes                int64     `json:"rx_bytes"`
	TxBytes                int64     `json:"tx_bytes"`
	RxErrors               int64     `json:"rx_errors"`
	TxErrors               int64     `json:"tx_errors"`
	RxDropped              int64     `json:"rx_dropped"`
	TxDropped              int64     `json:"tx_dropped"`
	StatsLastFullSuccessAt time.Time `json:"stats_last_full_success_at,omitzero"`
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
	VSwitches         []VSwitchInfo `json:"vswitches"`
	VMNICs            []VMNICLink   `json:"vm_nics"`
	VMKPorts          []VMKPort     `json:"vmk_ports,omitempty"`
	LastFullSuccessAt time.Time     `json:"last_full_success_at,omitzero"`
	Skipped           bool          `json:"-"`
	Collected         bool          `json:"-"`
	VSwitchCollected  bool          `json:"-"`
	VMNetCollected    bool          `json:"-"`
	VMPortsCollected  bool          `json:"-"`
	VMKCollected      bool          `json:"-"`
	VMKFullCollected  bool          `json:"-"`
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

type CollectOptions struct {
	SkipTopology     bool
	PreviousTopology NetTopology
	SkipStatic       bool
	PreviousPlatform PlatformInfo
	PreviousCPU      CPUStatic
	PreviousMemory   MemoryInfo
	SkipVMPower      bool
	PreviousVMs      []VM
	SkipUSBVMOwned   bool
	PreviousUSB      USBState
	SkipDiskSMART    bool
	PreviousDisks    []DiskHealth
	SkipNICStats     bool
	PreviousNICs     []NIC
}
