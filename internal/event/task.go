package event

import (
	"context"
	"fmt"
	"time"

	"github.com/LemonZuo/homer/internal/logx"
	"github.com/LemonZuo/homer/internal/model"
	"github.com/LemonZuo/homer/internal/notify"

	"gorm.io/gorm"
)

// RunOnce 扫描启用中的事项，命中提醒窗口的逐条推送。
// 窗口：[event_date - lead_days, event_date]；过期事项跳过。
// 去重：last_notified_at 与今天同日则不再推送（所以失败后手动重跑不会重复推送已成功的条目）。
// 查询失败或有推送失败时返回 error，交给 scheduler / jobmonitor 记失败并告警；
// 未配置通知通道视为未启用，不算失败。
func RunOnce(db *gorm.DB, notifier notify.Notifier) error {
	if notifier == nil || !notifier.Enabled() {
		logx.Warn("event skip: notifier not configured")
		return nil
	}
	now := time.Now()
	loc := now.Location()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)

	var items []model.EventReminder
	if err := db.Where("enabled = ?", "1").Find(&items).Error; err != nil {
		return fmt.Errorf("查询事项提醒失败：%w", err)
	}
	sent, failed := 0, 0

	for _, it := range items {
		target, ok := shouldSendReminder(it, today, loc)
		if !ok {
			continue
		}
		msg := BuildMessage(&it, target, now)
		if err := notifier.Send(context.Background(), notify.Message{Text: msg}); err != nil {
			failed++
			logx.Error("event send failed", "title", it.Title, "err", err)
			continue
		}
		sent++
		logx.Info("event reminder sent", "title", it.Title)
		if err := db.Model(&model.EventReminder{}).Where("id = ?", it.ID).Update("last_notified_at", now).Error; err != nil {
			logx.Error("event mark notified failed", "title", it.Title, "err", err)
		}
	}
	if failed > 0 {
		return fmt.Errorf("事项提醒发送失败 %d 条（成功 %d 条），详见服务端日志", failed, sent)
	}
	return nil
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
