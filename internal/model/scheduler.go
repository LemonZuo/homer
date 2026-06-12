package model

import "time"

// SchedulerJobState 持久化每个任务的「上次执行结果」，重启后面板与 /healthz 仍可见。
// 历史环形仍只在内存（重启清空），这里只存最近一次状态 + 连续失败计数（用于告警防抖）。
type SchedulerJobState struct {
	Name        string     `gorm:"primaryKey;column:name;size:64;comment:任务名" json:"name"`
	LastStart   *time.Time `gorm:"column:last_start;comment:最近开始时间" json:"last_start,omitempty"`
	LastEnd     *time.Time `gorm:"column:last_end;comment:最近结束时间" json:"last_end,omitempty"`
	LastOK      BoolFlag   `gorm:"column:last_ok;type:varchar(1);comment:是否成功" json:"last_ok"`
	LastErr     string     `gorm:"column:last_err;type:text;comment:错误信息" json:"last_err,omitempty"`
	LastTrigger string     `gorm:"column:last_trigger;size:16;comment:触发方式" json:"last_trigger"`
	ConsecFails int        `gorm:"column:consec_fails;comment:连续失败次数" json:"consec_fails"`
	UpdatedAt   time.Time  `gorm:"column:updated_at;autoUpdateTime" json:"updated_at"`
}

func (SchedulerJobState) TableName() string { return "scheduler_job_state" }
