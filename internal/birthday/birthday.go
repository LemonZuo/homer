// Package birthday 生日提醒相关的业务逻辑：消息构造 + 周期/手动触发的推送。
package birthday

import (
	"fmt"
	"time"

	"github.com/LemonZuo/homer/internal/chinesedate"
	"github.com/LemonZuo/homer/internal/model"
)

// daysUntilLunar 返回从 from 起到下一个农历命中日（含今天）的天数；找不到则返回 -1。
func daysUntilLunar(target string, from time.Time) int {
	for i := 0; i <= 366; i++ {
		d := from.AddDate(0, 0, i)
		if chinesedate.LunarString(d) == target {
			return i
		}
	}
	return -1
}

// 与老 Java 实现保持 1:1：
//   - 当日：     敬请关注今天是:{name}的生日,{date},农历:{chineseBirthday}
//   - 非当日：   敬请关注:{name},将于:{date},农历:{chineseBirthday}, 生日!!!
//
// 其中 {date} 是“下一次农历命中”对应的公历日期（yyyy-MM-dd），
// 等价于老项目里被覆写后的 remindBirthday。
func BuildMessage(it *model.BirthdayRemind) string {
	d := daysUntilLunar(it.ChineseBirthday, time.Now())
	if d < 0 {
		// 老项目正常不会出现（农历日一年内必命中）；兜底用录入的公历生日。
		return fmt.Sprintf("敬请关注:%s,将于:%s,农历:%s, 生日!!!",
			it.Name, it.Birthday, it.ChineseBirthday)
	}
	dateStr := time.Now().AddDate(0, 0, d).Format("2006-01-02")
	if d == 0 {
		return fmt.Sprintf("敬请关注今天是:%s的生日,%s,农历:%s",
			it.Name, dateStr, it.ChineseBirthday)
	}
	return fmt.Sprintf("敬请关注:%s,将于:%s,农历:%s, 生日!!!",
		it.Name, dateStr, it.ChineseBirthday)
}
