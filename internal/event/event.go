// Package event 一次性事项提醒：消息构造 + 周期/手动触发的推送。
// 与生日提醒隔离：独立 wework.Client、独立 cron、表 event_reminder。
package event

import (
	"fmt"
	"strings"
	"time"

	"github.com/LemonZuo/homer/internal/model"
)

// BuildMessage 构造企业微信文本：当天 / 还有 N 天 两种措辞。
// today 单独传入便于测试，调用方一般传 time.Now()。
func BuildMessage(it *model.EventReminder, target, today time.Time) string {
	days := daysBetween(today, target)
	var b strings.Builder
	if days <= 0 {
		fmt.Fprintf(&b, "事项提醒（今日）：%s\n日期：%s", it.Title, target.Format("2006-01-02"))
	} else {
		fmt.Fprintf(&b, "事项提醒：%s\n日期：%s（还有 %d 天）", it.Title, target.Format("2006-01-02"), days)
	}
	if strings.TrimSpace(it.Remark) != "" {
		fmt.Fprintf(&b, "\n备注：%s", it.Remark)
	}
	return b.String()
}

// daysBetween 返回从 from 到 to 的整天差（忽略时分秒，使用 from 的时区）。
func daysBetween(from, to time.Time) int {
	loc := from.Location()
	f := time.Date(from.Year(), from.Month(), from.Day(), 0, 0, 0, 0, loc)
	t := time.Date(to.Year(), to.Month(), to.Day(), 0, 0, 0, 0, loc)
	return int(t.Sub(f).Hours() / 24)
}
