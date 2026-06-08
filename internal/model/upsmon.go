package model

import "time"

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
	HostKind              string    `gorm:"column:host_kind;size:16;index:idx_host_ups_time,priority:1" json:"host_kind"`
	HostID                int64     `gorm:"column:host_id;index:idx_host_ups_time,priority:2" json:"host_id"`
	HostName              string    `gorm:"column:host_name;size:64" json:"host_name"`
	UPSName               string    `gorm:"column:ups_name;size:64;index:idx_host_ups_time,priority:3" json:"ups_name"`
	Mfr                   string    `gorm:"column:mfr;size:128" json:"mfr"`
	Model                 string    `gorm:"column:model;size:128" json:"model"`
	PowerSource           string    `gorm:"column:power_source;size:16" json:"power_source"`
	BatteryPercent        int       `gorm:"column:battery_percent" json:"battery_percent"`
	RuntimeMinutes        int       `gorm:"column:runtime_minutes" json:"runtime_minutes"`
	BatteryVoltage        float32   `gorm:"column:battery_voltage;type:decimal(5,1)" json:"battery_voltage"`
	BatteryNominalVoltage float32   `gorm:"column:battery_nominal_voltage;type:decimal(5,1)" json:"battery_nominal_voltage"`
	BatteryType           string    `gorm:"column:battery_type;size:16" json:"battery_type"`
	InputVoltage          float32   `gorm:"column:input_voltage;type:decimal(6,1)" json:"input_voltage"`
	OutputVoltage         float32   `gorm:"column:output_voltage;type:decimal(6,1)" json:"output_voltage"`
	LoadPercent           int       `gorm:"column:load_percent" json:"load_percent"`
	RealPower             int       `gorm:"column:real_power" json:"real_power"`
	RawStatus             string    `gorm:"column:raw_status;size:64" json:"raw_status"`
	SampledAt             time.Time `gorm:"column:sampled_at;index:idx_host_ups_time,priority:4,sort:desc;index:idx_sampled_at" json:"sampled_at"`
}

func (UPSSample) TableName() string { return "ups_sample" }

// UPSState 每个 UPS 一条,作"最近一次状态 + 告警去抖时间戳"用。
// last_alert_at 标识"切到 OB/LB 时发了告警的时刻",切回 OL 时清空。
type UPSState struct {
	HostKind                  string     `gorm:"primaryKey;column:host_kind;size:16" json:"host_kind"`
	HostID                    int64      `gorm:"primaryKey;column:host_id" json:"host_id"`
	UPSName                   string     `gorm:"primaryKey;column:ups_name;size:64" json:"ups_name"`
	HostName                  string     `gorm:"column:host_name;size:64" json:"host_name"`
	Mfr                       string     `gorm:"column:mfr;size:128" json:"mfr"`
	Model                     string     `gorm:"column:model;size:128" json:"model"`
	LastPowerSource           string     `gorm:"column:last_power_source;size:16" json:"last_power_source"`
	LastBatteryPercent        int        `gorm:"column:last_battery_percent" json:"last_battery_percent"`
	LastRuntimeMinutes        int        `gorm:"column:last_runtime_minutes" json:"last_runtime_minutes"`
	LastBatteryVoltage        float32    `gorm:"column:last_battery_voltage;type:decimal(5,1)" json:"last_battery_voltage"`
	LastBatteryNominalVoltage float32    `gorm:"column:last_battery_nominal_voltage;type:decimal(5,1)" json:"last_battery_nominal_voltage"`
	LastBatteryType           string     `gorm:"column:last_battery_type;size:16" json:"last_battery_type"`
	LastInputVoltage          float32    `gorm:"column:last_input_voltage;type:decimal(6,1)" json:"last_input_voltage"`
	LastOutputVoltage         float32    `gorm:"column:last_output_voltage;type:decimal(6,1)" json:"last_output_voltage"`
	LastLoadPercent           int        `gorm:"column:last_load_percent" json:"last_load_percent"`
	LastRealPower             int        `gorm:"column:last_real_power" json:"last_real_power"`
	LastRawStatus             string     `gorm:"column:last_raw_status;size:64" json:"last_raw_status"`
	LastAlertAt               *time.Time `gorm:"column:last_alert_at" json:"last_alert_at"`
	UpdatedAt                 time.Time  `gorm:"column:updated_at;autoUpdateTime" json:"updated_at"`
}

func (UPSState) TableName() string { return "ups_state" }
