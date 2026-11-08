package birthday

import (
	"context"
	"fmt"
	"time"

	"github.com/LemonZuo/homer/internal/logx"
	"github.com/LemonZuo/homer/internal/model"
	"github.com/LemonZuo/homer/internal/notify"

	"gorm.io/gorm"
)

// 与老 Java 实现保持一致：提前 15/10/5/3/2/1/0 天各提醒一次。
var offsets = []int{15, 10, 5, 3, 2, 1, 0}

// RunOnce 执行一次扫描；通常由 scheduler 每天调用一次。
// 对每个偏移日，找出农历生日命中该日（含小月回落、跳过闰月）且 enabled='1' 的记录，逐条推送。
// 推送文案统一通过 BuildMessage 生成，与手动触发一致。
// 查询失败或有推送失败时返回 error，交给 scheduler / jobmonitor 记失败并告警；
// 未配置通知通道视为未启用，不算失败。
func RunOnce(db *gorm.DB, notifier notify.Notifier) error {
	if notifier == nil || !notifier.Enabled() {
		logx.Warn("birthday skip: notifier not configured")
		return nil
	}
	var items []model.BirthdayRemind
	if err := db.Where("enabled = ?", "1").Find(&items).Error; err != nil {
		return fmt.Errorf("查询生日提醒失败：%w", err)
	}
	type parsed struct{ month, day int }
	dates := make([]parsed, len(items))
	for i, it := range items {
		month, day, ok := parseLunarMonthDay(it.ChineseBirthday)
		if !ok {
			// 解析失败的记录永远不会提醒，必须留痕，不能静默跳过。
			logx.Warn("birthday skip: unparseable chinese_birthday",
				"id", it.ID, "name", it.Name, "chinese_birthday", it.ChineseBirthday)
			continue
		}
		dates[i] = parsed{month, day}
	}
	today := time.Now()
	sent, failed := 0, 0
	for _, d := range offsets {
		target := lunarDayOf(today.AddDate(0, 0, d))
		for i, it := range items {
			if dates[i].month == 0 || !target.matches(dates[i].month, dates[i].day) {
				continue
			}
			msg := BuildMessage(&it)
			if err := notifier.Send(context.Background(), notify.Message{Text: msg}); err != nil {
				failed++
				logx.Error("birthday send failed", "name", it.Name, "err", err)
				continue
			}
			sent++
			logx.Info("birthday reminder sent", "name", it.Name, "offset", d)
		}
	}
	if failed > 0 {
		return fmt.Errorf("生日提醒发送失败 %d 条（成功 %d 条），详见服务端日志", failed, sent)
	}
	return nil
}
