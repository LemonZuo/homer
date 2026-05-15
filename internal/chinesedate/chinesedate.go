// Package chinesedate 提供公历到农历、生肖的转换。
// 用于生日提醒：保存时把公历日期换成农历字符串以便后续按农历匹配。
package chinesedate

import (
	"strings"
	"time"

	"github.com/6tail/lunar-go/calendar"
)

// LunarString 把 t 转成形如「腊月初八」的农历字符串（仅月+日，不含年份）。
// 与老 Java ChineseDateUtils.getFullChineseDay 1:1 对齐：拼接「中文月+中文日」，
// 不带年份，并剔除"闰"字（闰月对外当普通月份处理），以便按农历跨年匹配。
func LunarString(t time.Time) string {
	solar := calendar.NewSolarFromYmd(t.Year(), int(t.Month()), t.Day())
	lunar := solar.GetLunar()
	s := lunar.GetMonthInChinese() + "月" + lunar.GetDayInChinese()
	return strings.ReplaceAll(s, "闰", "")
}

// Zodiac 返回农历年的生肖（鼠/牛/虎…）。
func Zodiac(t time.Time) string {
	solar := calendar.NewSolarFromYmd(t.Year(), int(t.Month()), t.Day())
	return solar.GetLunar().GetYearShengXiao()
}
