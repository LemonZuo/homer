package model

import "time"

// SmsForwarder 一个「短信转发器」(SmsForwarder Android) 服务端配置。
// 支持配置多台、前端按需切换；secret 为服务端「客户端安全措施=签名校验」的密钥。
type SmsForwarder struct {
	ID             int64     `gorm:"primaryKey;column:id" json:"id"`
	Name           string    `gorm:"column:name;size:64;uniqueIndex" json:"name"`
	ServerURL      string    `gorm:"column:server_url;size:512" json:"server_url"`
	Secret         string    `gorm:"column:secret;size:512" json:"secret"`
	TimeoutSeconds int       `gorm:"column:timeout_seconds;default:30" json:"timeout_seconds"`
	Enabled        BoolFlag  `gorm:"column:enabled;type:varchar(1);default:'1'" json:"enabled"`
	CreatedAt      time.Time `gorm:"column:created_at;autoCreateTime" json:"created_at"`
	UpdatedAt      time.Time `gorm:"column:updated_at;autoUpdateTime" json:"updated_at"`
}

func (SmsForwarder) TableName() string { return "sms_forwarder" }
