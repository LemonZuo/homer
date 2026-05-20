package event

import (
	"context"
	"time"

	"github.com/LemonZuo/homer/internal/logx"
	"github.com/LemonZuo/homer/internal/model"
	"github.com/LemonZuo/homer/internal/notify"

	"gorm.io/gorm"
)

// RunOnce 扫描启用中的事项，命中提醒窗口的逐条推送。
// 窗口：[event_date - lead_days, event_date]；过期事项跳过。
// 去重：last_notified_at 与今天同日则不再推送。
func RunOnce(db *gorm.DB, notifier notify.Notifier) {
	if notifier == nil || !notifier.Enabled() {
		logx.Warn("event skip: notifier not configured")
		return
	}
	now := time.Now()
	loc := now.Location()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)

	var items []model.EventReminder
	if err := db.Where("enabled = ?", "1").Find(&items).Error; err != nil {
		logx.Error("event query failed", "err", err)
		return
	}

	for _, it := range items {
		target, ok := shouldSendReminder(it, today, loc)
		if !ok {
			continue
		}
		msg := BuildMessage(&it, target, now)
		if err := notifier.Send(context.Background(), notify.Message{Text: msg}); err != nil {
			logx.Error("event send failed", "title", it.Title, "err", err)
			continue
		}
		logx.Info("event reminder sent", "title", it.Title)
		if err := db.Model(&model.EventReminder{}).Where("id = ?", it.ID).Update("last_notified_at", now).Error; err != nil {
			logx.Error("event mark notified failed", "title", it.Title, "err", err)
		}
	}
}

// shouldSendReminder 判定一条提醒今天是否应该推送，返回解析后的事件日期供消息组装复用。
// 跳过条件：日期解析失败 / 已过期 / 未到提醒窗口 / 今天已推过。
func shouldSendReminder(it model.EventReminder, today time.Time, loc *time.Location) (time.Time, bool) {
	target, err := time.ParseInLocation("2006-01-02", it.EventDate, loc)
	if err != nil {
		logx.Error("event date parse failed", "title", it.Title, "date", it.EventDate, "err", err)
		return target, false
	}
	if target.Before(today) {
		return target, false
	}
	lead := it.LeadDays
	if lead < 0 {
		lead = 0
	}
	if today.Before(target.AddDate(0, 0, -lead)) {
		return target, false
	}
	if it.LastNotifiedAt != nil {
		last := it.LastNotifiedAt.In(loc)
		lastDay := time.Date(last.Year(), last.Month(), last.Day(), 0, 0, 0, 0, loc)
		if !lastDay.Before(today) {
			return target, false
		}
	}
	return target, true
}
