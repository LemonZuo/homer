package model

import "time"

// EventReminder 一次性事项提醒：事项当天前 lead_days 天起每天推送一次。
// 与生日提醒隔离：独立企业微信应用 + 独立 cron + 独立去重字段。
type EventReminder struct {
	ID             int64      `gorm:"primaryKey;column:id" json:"id"`
	Title          string     `gorm:"column:title;size:128;comment:事项标题" json:"title"`
	EventDate      string     `gorm:"column:event_date;size:10;comment:事项日期" json:"event_date"`
	LeadDays       int        `gorm:"column:lead_days;default:5;comment:提前天数" json:"lead_days"`
	Remark         string     `gorm:"column:remark;size:255;comment:备注" json:"remark"`
	Enabled        BoolFlag   `gorm:"column:enabled;type:varchar(1);default:'1';comment:是否启用" json:"enabled"`
	LastNotifiedAt *time.Time `gorm:"column:last_notified_at;comment:最近推送时间" json:"last_notified_at,omitempty"`
	CreatedAt      time.Time  `gorm:"column:created_at;autoCreateTime" json:"created_at"`
	UpdatedAt      time.Time  `gorm:"column:updated_at;autoUpdateTime" json:"updated_at"`
}

func (EventReminder) TableName() string { return "event_reminder" }
