package model

import "time"

// ModuleSetting 只保存业务开关；设备、凭证和部署配置仍归各业务管理。
type ModuleSetting struct {
	Key       string    `gorm:"column:module_key;primaryKey;size:32" json:"key"`
	Enabled   bool      `gorm:"column:enabled;not null;default:false" json:"enabled"`
	Version   uint64    `gorm:"column:version;not null;default:1" json:"version"`
	UpdatedAt time.Time `gorm:"column:updated_at;autoUpdateTime" json:"updated_at"`
}

func (ModuleSetting) TableName() string { return "module_setting" }
