// Package jobmonitor 把 scheduler.Observer 落到 DB + 经 notify Hub 告警，
// 与 scheduler 解耦（scheduler 不依赖 DB / notify）。
package jobmonitor

import (
	"context"
	"fmt"
	"time"

	"github.com/LemonZuo/homer/internal/logx"
	"github.com/LemonZuo/homer/internal/model"
	"github.com/LemonZuo/homer/internal/notify"
	"github.com/LemonZuo/homer/internal/scheduler"
	"gorm.io/gorm"
)

type Monitor struct {
	db    *gorm.DB
	alert notify.Notifier
}

// New 构造监视器；alert 用于失败告警（通常是 hub.For(notify.ModuleSchedAlrt)）。
func New(db *gorm.DB, alert notify.Notifier) *Monitor {
	return &Monitor{db: db, alert: alert}
}

// Record 持久化每次执行结果（重启后面板/healthz 仍可见最近状态）。
func (m *Monitor) Record(name string, r scheduler.Run, consecFails int) {
	start, end := r.Start, r.End
	st := model.SchedulerJobState{
		Name:        name,
		LastStart:   &start,
		LastEnd:     &end,
		LastOK:      model.BoolFlag(r.OK),
		LastErr:     r.Err,
		LastTrigger: r.Trigger,
		ConsecFails: consecFails,
	}
	if err := m.db.Save(&st).Error; err != nil {
		logx.Error("jobmonitor persist failed", "job", name, "err", err)
	}
}

// Alert 连续失败达阈值时推送告警；无可用通道则只落日志（Record 已落库）。
func (m *Monitor) Alert(name string, r scheduler.Run, consecFails int) {
	if m.alert == nil || !m.alert.Enabled() {
		logx.Warn("jobmonitor alert: no channel", "job", name, "consec_fails", consecFails, "err", r.Err)
		return
	}
	text := fmt.Sprintf("任务：%s\n连续失败：%d 次\n时间：%s\n错误：%s",
		name, consecFails, r.End.Format("2006-01-02 15:04:05"), r.Err)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := m.alert.Send(ctx, notify.Message{Title: "homer 任务失败告警", Text: text}); err != nil {
		logx.Error("jobmonitor alert send failed", "job", name, "err", err)
	} else {
		logx.Warn("jobmonitor alert sent", "job", name, "consec_fails", consecFails)
	}
}

// Hydrate 用持久化状态预热 scheduler（须在 sched.Start 前、注册完任务后调用）。
func (m *Monitor) Hydrate(s *scheduler.Scheduler) {
	var rows []model.SchedulerJobState
	if err := m.db.Find(&rows).Error; err != nil {
		logx.Error("jobmonitor hydrate failed", "err", err)
		return
	}
	for _, st := range rows {
		var last *scheduler.Run
		if st.LastStart != nil && st.LastEnd != nil {
			last = &scheduler.Run{
				Start:   *st.LastStart,
				End:     *st.LastEnd,
				OK:      bool(st.LastOK),
				Err:     st.LastErr,
				Trigger: st.LastTrigger,
			}
		}
		s.Seed(st.Name, last, st.ConsecFails)
	}
}
