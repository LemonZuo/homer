package birthday

import (
	"context"
	"time"

	"github.com/LemonZuo/homer/internal/chinesedate"
	"github.com/LemonZuo/homer/internal/logx"
	"github.com/LemonZuo/homer/internal/model"
	"github.com/LemonZuo/homer/internal/notify"

	"gorm.io/gorm"
)

// 与老 Java 实现保持一致：提前 15/10/5/3/2/1/0 天各提醒一次。
var offsets = []int{15, 10, 5, 3, 2, 1, 0}

// RunOnce 执行一次扫描；通常由 scheduler 每天调用一次。
// 对每个偏移日，匹配 chinese_birthday 命中且 enabled='1' 的记录，逐条推送。
// 推送文案统一通过 BuildMessage 生成，与手动触发一致。
func RunOnce(db *gorm.DB, notifier notify.Notifier) {
	if notifier == nil || !notifier.Enabled() {
		logx.Warn("birthday skip: notifier not configured")
		return
	}
	today := time.Now()
	for _, d := range offsets {
		target := today.AddDate(0, 0, d)
		lunar := chinesedate.LunarString(target)
		var items []model.BirthdayRemind
		if err := db.Where("enabled = ? AND chinese_birthday = ?", "1", lunar).Find(&items).Error; err != nil {
			logx.Error("birthday query failed", "offset", d, "err", err)
			continue
		}
		for _, it := range items {
			msg := BuildMessage(&it)
			if err := notifier.Send(context.Background(), notify.Message{Text: msg}); err != nil {
				logx.Error("birthday send failed", "name", it.Name, "err", err)
			} else {
				logx.Info("birthday reminder sent", "name", it.Name, "offset", d)
			}
		}
	}
}
