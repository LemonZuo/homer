package birthday

import (
	"slices"
	"strings"
	"time"

	"github.com/6tail/lunar-go/LunarUtil"
	"github.com/6tail/lunar-go/calendar"
	"github.com/LemonZuo/homer/internal/model"
)

// LunarString 把 t 转成形如「腊月初八」的农历字符串（仅月+日，不含年份）。
// 与老 Java ChineseDateUtils.getFullChineseDay 1:1 对齐：拼接「中文月+中文日」，
// 不带年份，并剔除"闰"字（闰月对外当普通月份处理），以便按农历跨年匹配。
// 只用于录入时回填 chinese_birthday；提醒命中判定走 lunarBirthdayOn。
func LunarString(t time.Time) string {
	solar := calendar.NewSolarFromYmd(t.Year(), int(t.Month()), t.Day())
	lunar := solar.GetLunar()
	s := lunar.GetMonthInChinese() + "月" + lunar.GetDayInChinese()
	return strings.ReplaceAll(s, "闰", "")
}

// lunarDay 是某个公历日对应的农历信息，供生日命中判定用。
type lunarDay struct {
	month     int  // 1..12
	day       int  // 1..30
	leap      bool // 是否闰月
	monthDays int  // 当月天数（29 或 30）
}

func lunarDayOf(t time.Time) lunarDay {
	lunar := calendar.NewSolarFromYmd(t.Year(), int(t.Month()), t.Day()).GetLunar()
	m := lunar.GetMonth()
	ld := lunarDay{month: m, day: lunar.GetDay(), monthDays: 30}
	if m < 0 {
		ld.month, ld.leap = -m, true
	}
	if lm := calendar.NewLunarMonthFromYm(lunar.GetYear(), m); lm != nil {
		ld.monthDays = lm.GetDayCount()
	}
	return ld
}

// parseLunarMonthDay 把「腊月三十」解析成 (12, 30)；「闰」字忽略。
func parseLunarMonthDay(s string) (month, day int, ok bool) {
	mText, dText, found := strings.Cut(strings.ReplaceAll(strings.TrimSpace(s), "闰", ""), "月")
	if !found {
		return 0, 0, false
	}
	month = slices.Index(LunarUtil.MONTH, mText)
	day = slices.Index(LunarUtil.DAY, dText)
	if month < 1 || day < 1 {
		return 0, 0, false
	}
	return month, day, true
}

// matches 判断这一天是否是农历生日 month/day：
//   - 只认非闰月：闰月出生的录入时已去掉「闰」字，按同名普通月过；闰月年份不会再提醒第二遍。
//   - 生日日大于当月天数（如「三十」遇到只有 29 天的小月）时，当月最后一天视为生日。
func (ld lunarDay) matches(month, day int) bool {
	if ld.leap || ld.month != month {
		return false
	}
	if day > ld.monthDays {
		return ld.day == ld.monthDays
	}
	return ld.day == day
}

// lunarBirthdayOn 判断公历日 t 是否命中农历生日 target（形如「腊月三十」）。
func lunarBirthdayOn(target string, t time.Time) bool {
	month, day, ok := parseLunarMonthDay(target)
	return ok && lunarDayOf(t).matches(month, day)
}

// Zodiac 返回农历年的生肖（鼠/牛/虎…）。
func Zodiac(t time.Time) string {
	solar := calendar.NewSolarFromYmd(t.Year(), int(t.Month()), t.Day())
	return solar.GetLunar().GetYearShengXiao()
}

// FillDerivedFields 根据公历生日回填农历生日和生肖。
func FillDerivedFields(b *model.BirthdayRemind) error {
	if b.Birthday == "" {
		return nil
	}
	t, err := time.ParseInLocation("2006-01-02", b.Birthday, time.Local)
	if err != nil {
		return err
	}
	b.ChineseBirthday = LunarString(t)
	b.Zodiac = Zodiac(t)
	return nil
}
