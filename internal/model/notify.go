package model

import "time"

// NotifyChannel 一个出站通知通道；type 决定 config_json 的结构：
//   wework:  {"corp_id","agent_id","secret","tag_id"}
//   email:   {"api_key","from","to"}
//   webhook: {"url"}
// 凭证当前明文存储（P0 加密待办）。
type NotifyChannel struct {
	ID         int64     `gorm:"primaryKey;column:id" json:"id"`
	Name       string    `gorm:"column:name;size:64;uniqueIndex" json:"name"`
	Type       string    `gorm:"column:type;size:16" json:"type"`
	ConfigJSON string    `gorm:"column:config_json;type:text" json:"config_json"`
	Enabled    BoolFlag  `gorm:"column:enabled;type:varchar(1);default:'1'" json:"enabled"`
	CreatedAt  time.Time `gorm:"column:created_at;autoCreateTime" json:"created_at"`
	UpdatedAt  time.Time `gorm:"column:updated_at;autoUpdateTime" json:"updated_at"`
}

func (NotifyChannel) TableName() string { return "notify_channel" }

// NotifyBinding 模块 → 通道映射。一个模块可绑多个通道（扇出）。
// module 取值见 notify 包的模块常量（birthday/event/bypass/scheduler_alert）。
type NotifyBinding struct {
	ID        int64 `gorm:"primaryKey;column:id" json:"id"`
	Module    string `gorm:"column:module;size:32;uniqueIndex:uk_notify_bind" json:"module"`
	ChannelID int64  `gorm:"column:channel_id;uniqueIndex:uk_notify_bind" json:"channel_id"`
}

func (NotifyBinding) TableName() string { return "notify_binding" }
