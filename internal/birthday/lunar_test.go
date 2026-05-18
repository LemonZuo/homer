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
