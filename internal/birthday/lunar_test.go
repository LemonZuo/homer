package birthday

import (
	"testing"
	"time"

	"github.com/LemonZuo/homer/internal/model"
)

func TestFillDerivedFields(t *testing.T) {
	item := &model.BirthdayRemind{Birthday: "2024-02-10"}
	if err := FillDerivedFields(item); err != nil {
		t.Fatalf("FillDerivedFields() error = %v", err)
	}
	if item.ChineseBirthday != "正月初一" {
		t.Fatalf("ChineseBirthday = %q, want %q", item.ChineseBirthday, "正月初一")
	}
	if item.Zodiac != "龙" {
		t.Fatalf("Zodiac = %q, want %q", item.Zodiac, "龙")
	}
}

func TestLunarStringRemovesLeapMonthPrefix(t *testing.T) {
	got := LunarString(time.Date(2023, 3, 22, 0, 0, 0, 0, time.Local))
	if got != "二月初一" {
		t.Fatalf("LunarString() = %q, want %q", got, "二月初一")
	}
}

func day(s string) time.Time {
	t, _ := time.ParseInLocation("2006-01-02", s, time.Local)
	return t
}

func TestLunarBirthdayOnSmallMonthFallsBackToLastDay(t *testing.T) {
	// 2024 腊月是大月：三十当天命中。
	if !lunarBirthdayOn("腊月三十", day("2024-02-09")) {
		t.Fatal("腊月三十 should match 2024-02-09")
	}
	// 2025、2026 的腊月只有 29 天：回落到廿九，旧实现按字符串匹配整年都不会提醒。
	for _, s := range []string{"2025-01-28", "2026-02-16"} {
		if !lunarBirthdayOn("腊月三十", day(s)) {
			t.Fatalf("腊月三十 should fall back to 廿九 on %s", s)
		}
	}
	// 廿九生日照常只命中廿九。
	if !lunarBirthdayOn("腊月廿九", day("2026-02-16")) || lunarBirthdayOn("腊月廿九", day("2026-02-17")) {
		t.Fatal("腊月廿九 should match only 2026-02-16")
	}
}

func TestLunarBirthdayOnSkipsLeapMonth(t *testing.T) {
	// 2025 有闰六月：只在普通六月提醒，闰六月不再提醒第二遍。
	if !lunarBirthdayOn("六月初一", day("2025-06-25")) {
		t.Fatal("六月初一 should match regular month 2025-06-25")
	}
	if lunarBirthdayOn("六月初一", day("2025-07-25")) {
		t.Fatal("六月初一 must not match leap month 2025-07-25")
	}
	// 闰月出生的录入时带「闰」也能解析，同样按普通月过。
	if !lunarBirthdayOn("闰六月初一", day("2025-06-25")) {
		t.Fatal("闰六月初一 should be treated as 六月初一")
	}
}

func TestDaysUntilLunarUsesSameRule(t *testing.T) {
	if got := daysUntilLunar("腊月三十", day("2026-02-10")); got != 6 {
		t.Fatalf("daysUntilLunar = %d, want 6 (2026-02-16, 腊月廿九)", got)
	}
}

func TestParseLunarMonthDay(t *testing.T) {
	cases := []struct {
		in         string
		month, day int
		ok         bool
	}{
		{"正月初一", 1, 1, true},
		{"冬月十五", 11, 15, true},
		{"腊月三十", 12, 30, true},
		{"", 0, 0, false},
		{"十三月初一", 0, 0, false},
		{"正月三十一", 0, 0, false},
	}
	for _, c := range cases {
		m, d, ok := parseLunarMonthDay(c.in)
		if ok != c.ok || m != c.month || d != c.day {
			t.Errorf("parseLunarMonthDay(%q) = %d,%d,%v", c.in, m, d, ok)
		}
	}
}
