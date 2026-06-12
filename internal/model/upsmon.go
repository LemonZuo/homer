package model

import "time"

// UPSHostKind 写入 ups_sample / ups_state 的 host_kind 列常量。
// 现在恒为 "ups"——历史上曾用 "ssh"/"fnos" 区分,迁移到 ups_host 表后单一来源。
const UPSHostKind = "ups"

// UPSHost UPS 监控目标机器。自带 SSH 凭证维度,与 ACME 部署目标完全解耦。
// auth_json/config_json 结构详见 sql 注释;连接语义复用 internal/sshlike,UPS 适配在 internal/upsmon/sshtarget.go。
type UPSHost struct {
	ID         int64     `gorm:"primaryKey;column:id" json:"id"`
	Name       string    `gorm:"column:name;size:64;uniqueIndex;comment:机器名称" json:"name"`
	Endpoint   string    `gorm:"column:endpoint;size:512;comment:SSH 入口" json:"endpoint"`
	AuthJSON   string    `gorm:"column:auth_json;type:text;comment:认证配置" json:"auth_json"`
	ConfigJSON string    `gorm:"column:config_json;type:text;comment:扩展配置" json:"config_json"`
	Enabled    BoolFlag  `gorm:"column:enabled;type:varchar(1);default:'1';index;comment:是否启用" json:"enabled"`
	CreatedAt  time.Time `gorm:"column:created_at;autoCreateTime" json:"created_at"`
	UpdatedAt  time.Time `gorm:"column:updated_at;autoUpdateTime" json:"updated_at"`
}

func (UPSHost) TableName() string { return "ups_host" }

// UPSSSHCredential UPS 模块专属 SSH 凭证。结构与 SSHCredential 一致,但物理上分表,
// 这样 UPS 可以用一个只允许执行 upsc 的低权账号,跟 ACME 的强权 SSH 凭证隔离。
type UPSSSHCredential struct {
	ID         int64     `gorm:"primaryKey;column:id" json:"id"`
	Name       string    `gorm:"column:name;size:64;uniqueIndex;comment:凭证名称" json:"name"`
	Username   string    `gorm:"column:username;size:128;comment:用户名" json:"username"`
	AuthType   string    `gorm:"column:auth_type;size:16;comment:认证类型" json:"auth_type"`
	Password   string    `gorm:"column:password;type:text;comment:密码" json:"password"`
	PrivateKey string    `gorm:"column:private_key;type:text;comment:私钥" json:"private_key"`
	Passphrase string    `gorm:"column:passphrase;type:text;comment:私钥口令" json:"passphrase"`
	CreatedAt  time.Time `gorm:"column:created_at;autoCreateTime" json:"created_at"`
	UpdatedAt  time.Time `gorm:"column:updated_at;autoUpdateTime" json:"updated_at"`
	RefCount   int64     `gorm:"-" json:"ref_count"`
}

func (UPSSSHCredential) TableName() string { return "ups_ssh_credential" }

// UPS 监控相关常量。供电类型枚举与 NUT 的 ups.status 映射:
//   - mains       <- OL(on-line,市电)
//   - battery     <- OB(on-battery)
//   - low_battery <- LB(low-battery,任意叠加状态)
//   - unknown     <- 无 ups.status 或解析失败
const (
	UPSPowerMains      = "mains"
	UPSPowerBattery    = "battery"
	UPSPowerLowBattery = "low_battery"
	UPSPowerUnknown    = "unknown"
)

// UPSSample 一次采样的快照。30 秒一条,默认保留 7 天(cleanup job 清理)。
// 电气指标列(input_voltage / output_voltage / load_percent / real_power /
// battery_voltage / battery_nominal_voltage)在 NUT 缺字段时存 -1,
// battery_type 缺数据时存空串,与 battery_percent 一致的哨兵约定。
type UPSSample struct {
	ID                    int64     `gorm:"primaryKey;column:id" json:"id"`
	HostKind              string    `gorm:"column:host_kind;size:16;index:idx_host_ups_time,priority:1;comment:主机类型" json:"host_kind"`
	HostID                int64     `gorm:"column:host_id;index:idx_host_ups_time,priority:2;comment:主机 ID" json:"host_id"`
	HostName              string    `gorm:"column:host_name;size:64;comment:主机名" json:"host_name"`
	UPSName               string    `gorm:"column:ups_name;size:64;index:idx_host_ups_time,priority:3;comment:UPS 名称" json:"ups_name"`
	Mfr                   string    `gorm:"column:mfr;size:128;comment:品牌" json:"mfr"`
	Model                 string    `gorm:"column:model;size:128;comment:型号" json:"model"`
	PowerSource           string    `gorm:"column:power_source;size:16;comment:供电状态" json:"power_source"`
	BatteryPercent        int       `gorm:"column:battery_percent;comment:剩余电量" json:"battery_percent"`
	RuntimeMinutes        int       `gorm:"column:runtime_minutes;comment:续航分钟" json:"runtime_minutes"`
	BatteryVoltage        float32   `gorm:"column:battery_voltage;type:decimal(5,1);comment:电池端电压" json:"battery_voltage"`
	BatteryNominalVoltage float32   `gorm:"column:battery_nominal_voltage;type:decimal(5,1);comment:电池标称电压" json:"battery_nominal_voltage"`
	BatteryType           string    `gorm:"column:battery_type;size:16;comment:电池类型" json:"battery_type"`
	InputVoltage          float32   `gorm:"column:input_voltage;type:decimal(6,1);comment:输入电压" json:"input_voltage"`
	OutputVoltage         float32   `gorm:"column:output_voltage;type:decimal(6,1);comment:输出电压" json:"output_voltage"`
	LoadPercent           int       `gorm:"column:load_percent;comment:负载百分比" json:"load_percent"`
	RealPower             int       `gorm:"column:real_power;comment:实时功率" json:"real_power"`
	RawStatus             string    `gorm:"column:raw_status;size:64;comment:原始状态" json:"raw_status"`
	SampledAt             time.Time `gorm:"column:sampled_at;index:idx_host_ups_time,priority:4,sort:desc;index:idx_sampled_at;comment:采样时刻" json:"sampled_at"`
}

func (UPSSample) TableName() string { return "ups_sample" }

// UPSState 每个 UPS 一条,作"最近一次状态 + 告警去抖时间戳"用。
// last_alert_at 标识"切到 OB/LB 时发了告警的时刻",切回 OL 时清空。
type UPSState struct {
	HostKind                  string     `gorm:"primaryKey;column:host_kind;size:16;comment:主机类型" json:"host_kind"`
	HostID                    int64      `gorm:"primaryKey;column:host_id;comment:主机 ID" json:"host_id"`
	UPSName                   string     `gorm:"primaryKey;column:ups_name;size:64;comment:UPS 名称" json:"ups_name"`
	HostName                  string     `gorm:"column:host_name;size:64;comment:主机名" json:"host_name"`
	Mfr                       string     `gorm:"column:mfr;size:128;comment:品牌" json:"mfr"`
	Model                     string     `gorm:"column:model;size:128;comment:型号" json:"model"`
	LastPowerSource           string     `gorm:"column:last_power_source;size:16;comment:最近供电状态" json:"last_power_source"`
	LastBatteryPercent        int        `gorm:"column:last_battery_percent;comment:最近电量" json:"last_battery_percent"`
	LastRuntimeMinutes        int        `gorm:"column:last_runtime_minutes;comment:最近续航分钟" json:"last_runtime_minutes"`
	LastBatteryVoltage        float32    `gorm:"column:last_battery_voltage;type:decimal(5,1);comment:最近电池端电压" json:"last_battery_voltage"`
	LastBatteryNominalVoltage float32    `gorm:"column:last_battery_nominal_voltage;type:decimal(5,1);comment:最近电池标称电压" json:"last_battery_nominal_voltage"`
	LastBatteryType           string     `gorm:"column:last_battery_type;size:16;comment:最近电池类型" json:"last_battery_type"`
	LastInputVoltage          float32    `gorm:"column:last_input_voltage;type:decimal(6,1);comment:最近输入电压" json:"last_input_voltage"`
	LastOutputVoltage         float32    `gorm:"column:last_output_voltage;type:decimal(6,1);comment:最近输出电压" json:"last_output_voltage"`
	LastLoadPercent           int        `gorm:"column:last_load_percent;comment:最近负载百分比" json:"last_load_percent"`
	LastRealPower             int        `gorm:"column:last_real_power;comment:最近实时功率" json:"last_real_power"`
	LastRawStatus             string     `gorm:"column:last_raw_status;size:64;comment:最近原始状态" json:"last_raw_status"`
	LastAlertAt               *time.Time `gorm:"column:last_alert_at;comment:最近告警时刻" json:"last_alert_at"`
	UpdatedAt                 time.Time  `gorm:"column:updated_at;autoUpdateTime;comment:更新时刻" json:"updated_at"`
}

func (UPSState) TableName() string { return "ups_state" }
