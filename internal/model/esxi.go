package model

import "time"

// EsxiHostKind 写入 esxi_sample / esxi_state 的 host_kind 列常量。固定 "esxi"，
// 保留列名以便未来扩展(例如增加 fnos 远端、IPMI 等其他来源)。
const EsxiHostKind = "esxi"

// EsxiHost ESXi 监控目标机器。自带 SSH 凭证维度,与 UPS / ACME 完全解耦——
// 这样 ESXi 可以单独配低权账号(主要跑 esxcli/vsish 只读命令)而不影响其他模块。
type EsxiHost struct {
	ID         int64     `gorm:"primaryKey;column:id" json:"id"`
	Name       string    `gorm:"column:name;size:64;uniqueIndex" json:"name"`
	Endpoint   string    `gorm:"column:endpoint;size:512" json:"endpoint"`
	AuthJSON   string    `gorm:"column:auth_json;type:text" json:"auth_json"`
	ConfigJSON string    `gorm:"column:config_json;type:text" json:"config_json"`
	Enabled    BoolFlag  `gorm:"column:enabled;type:varchar(1);default:'1';index" json:"enabled"`
	CreatedAt  time.Time `gorm:"column:created_at;autoCreateTime" json:"created_at"`
	UpdatedAt  time.Time `gorm:"column:updated_at;autoUpdateTime" json:"updated_at"`
}

func (EsxiHost) TableName() string { return "esxi_host" }

// EsxiSSHCredential ESXi 模块专属 SSH 凭证。结构与 UPSSSHCredential 一致,
// 物理上分表,允许给 ESXi 单独配只允许执行采集命令的低权账号。
type EsxiSSHCredential struct {
	ID         int64     `gorm:"primaryKey;column:id" json:"id"`
	Name       string    `gorm:"column:name;size:64;uniqueIndex" json:"name"`
	Username   string    `gorm:"column:username;size:128" json:"username"`
	AuthType   string    `gorm:"column:auth_type;size:16" json:"auth_type"`
	Password   string    `gorm:"column:password;type:text" json:"password"`
	PrivateKey string    `gorm:"column:private_key;type:text" json:"private_key"`
	Passphrase string    `gorm:"column:passphrase;type:text" json:"passphrase"`
	CreatedAt  time.Time `gorm:"column:created_at;autoCreateTime" json:"created_at"`
	UpdatedAt  time.Time `gorm:"column:updated_at;autoUpdateTime" json:"updated_at"`
	RefCount   int64     `gorm:"-" json:"ref_count"`
}

func (EsxiSSHCredential) TableName() string { return "esxi_ssh_credential" }

// MCE 健康状态枚举,对齐 ESXi `/hardware/health/mce` 的 Health state 输出。
const (
	EsxiMCEStateGreen  = "Green"
	EsxiMCEStateYellow = "Yellow"
	EsxiMCEStateRed    = "Red"
)

// EsxiSample 一次采样的标量趋势快照。默认保留 7 天(cleanup job 清理)。
// 新采样会先做关键指标完整性校验,完整才写入;数值列仍保留 -1 作为旧行/非关键字段
// 的无数据哨兵,前端拿到 -1 直接当"无数据"处理。
type EsxiSample struct {
	ID                  int64  `gorm:"primaryKey;column:id" json:"id"`
	HostKind            string `gorm:"column:host_kind;size:16;index:idx_host_time,priority:1" json:"host_kind"`
	HostID              int64  `gorm:"column:host_id;index:idx_host_time,priority:2" json:"host_id"`
	HostName            string `gorm:"column:host_name;size:64" json:"host_name"`
	CPUMaxC             int    `gorm:"column:cpu_max_c" json:"cpu_max_c"`
	CPUAvgC             int    `gorm:"column:cpu_avg_c" json:"cpu_avg_c"`
	CPUTjMaxC           int    `gorm:"column:cpu_tjmax_c" json:"cpu_tjmax_c"`
	MCEState            string `gorm:"column:mce_state;size:16" json:"mce_state"`
	MCECorrectedTotal   int64  `gorm:"column:mce_corrected_total" json:"mce_corrected_total"`
	MCEUncorrectedTotal int64  `gorm:"column:mce_uncorrected_total" json:"mce_uncorrected_total"`
	DiskMaxC            int    `gorm:"column:disk_max_c" json:"disk_max_c"`
	VMTotal             int    `gorm:"column:vm_total" json:"vm_total"`
	VMPoweredOn         int    `gorm:"column:vm_powered_on" json:"vm_powered_on"`
	// 明细 JSON 列,给历史曲线"每核 / 每盘单独画线"用。
	// CPUTempPerCoreJSON:[{"id":0,"temp_c":54}, ...]
	// DiskTempPerDiskJSON:[{"device":"t10.XXX","temp_c":35}, ...]
	// 缺数据落空字符串 ""(不是 NULL),与 EsxiState 的 *_json 列约定一致。
	CPUTempPerCoreJSON  string    `gorm:"column:cpu_temp_json;type:text" json:"cpu_temp_json"`
	DiskTempPerDiskJSON string    `gorm:"column:disk_temp_json;type:text" json:"disk_temp_json"`
	SampledAt           time.Time `gorm:"column:sampled_at;index:idx_host_time,priority:3,sort:desc;index:idx_sampled_at" json:"sampled_at"`
}

func (EsxiSample) TableName() string { return "esxi_sample" }

// EsxiState 每台 ESXi 一条,存最近一次完整快照 + 告警去抖时间戳。
// 变长结构(每核温度数组、磁盘列表、VM 列表等)按 prompt 范式存 JSON 列,
// 由 sampler 序列化后整块写入,前端直接 JSON.parse 渲染。
type EsxiState struct {
	HostKind      string     `gorm:"primaryKey;column:host_kind;size:16" json:"host_kind"`
	HostID        int64      `gorm:"primaryKey;column:host_id" json:"host_id"`
	HostName      string     `gorm:"column:host_name;size:64" json:"host_name"`
	Reachable     BoolFlag   `gorm:"column:reachable;type:varchar(1);default:'0'" json:"reachable"`
	LastError     string     `gorm:"column:last_error;size:512" json:"last_error"`
	PlatformJSON  string     `gorm:"column:platform_json;type:text" json:"platform_json"`
	CPUStaticJSON string     `gorm:"column:cpu_static_json;type:text" json:"cpu_static_json"`
	MemoryJSON    string     `gorm:"column:memory_json;type:text" json:"memory_json"`
	RuntimeJSON   string     `gorm:"column:runtime_json;type:text" json:"runtime_json"`
	CPUTempJSON   string     `gorm:"column:cpu_temp_json;type:text" json:"cpu_temp_json"`
	MCEJSON       string     `gorm:"column:mce_json;type:text" json:"mce_json"`
	DiskJSON      string     `gorm:"column:disk_json;type:text" json:"disk_json"`
	USBJSON       string     `gorm:"column:usb_json;type:text" json:"usb_json"`
	VMJSON        string     `gorm:"column:vm_json;type:text" json:"vm_json"`
	BootJSON      string     `gorm:"column:boot_json;type:text" json:"boot_json"`
	NICJSON       string     `gorm:"column:nic_json;type:text" json:"nic_json"`
	TopologyJSON  string     `gorm:"column:topology_json;type:text" json:"topology_json"`
	LastAlertAt   *time.Time `gorm:"column:last_alert_at" json:"last_alert_at"`
	SampledAt     *time.Time `gorm:"column:sampled_at" json:"sampled_at"`
	UpdatedAt     time.Time  `gorm:"column:updated_at;autoUpdateTime" json:"updated_at"`
}

func (EsxiState) TableName() string { return "esxi_state" }
