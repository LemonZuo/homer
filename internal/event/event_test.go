package event

import (
	"strings"
	"testing"
	"time"

	"github.com/LemonZuo/homer/internal/model"
)

func d(y, m, day int) time.Time {
	return time.Date(y, time.Month(m), day, 0, 0, 0, 0, time.Local)
}

func TestDaysBetween(t *testing.T) {
	if got := daysBetween(d(2026, 5, 19), d(2026, 5, 19)); got != 0 {
		t.Fatalf("same day = %d want 0", got)
	}
	if got := daysBetween(d(2026, 5, 19), d(2026, 5, 22)); got != 3 {
		t.Fatalf("3 days = %d", got)
	}
	if got := daysBetween(d(2026, 5, 22), d(2026, 5, 19)); got != -3 {
		t.Fatalf("past = %d want -3", got)
	}
	// 时分秒应被忽略。
	from := time.Date(2026, 5, 19, 23, 59, 0, 0, time.Local)
	to := time.Date(2026, 5, 20, 0, 1, 0, 0, time.Local)
	if got := daysBetween(from, to); got != 1 {
		t.Fatalf("ignores time-of-day, got %d", got)
	}
}

func TestBuildMessage(t *testing.T) {
	today := d(2026, 5, 19)

	t.Run("today wording", func(t *testing.T) {
		msg := BuildMessage(&model.EventReminder{Title: "缴费"}, d(2026, 5, 19), today)
		if !strings.Contains(msg, "（今日）") || !strings.Contains(msg, "缴费") {
			t.Fatalf("unexpected: %q", msg)
		}
		if strings.Contains(msg, "还有") {
			t.Fatalf("today msg should not say 还有: %q", msg)
		}
	})

	t.Run("past also counts as today", func(t *testing.T) {
		msg := BuildMessage(&model.EventReminder{Title: "x"}, d(2026, 5, 18), today)
		if !strings.Contains(msg, "（今日）") {
			t.Fatalf("days<=0 should use 今日 wording: %q", msg)
		}
	})

	t.Run("future with day count", func(t *testing.T) {
		msg := BuildMessage(&model.EventReminder{Title: "体检"}, d(2026, 5, 24), today)
		if !strings.Contains(msg, "还有 5 天") {
			t.Fatalf("expected '还有 5 天': %q", msg)
		}
	})

	t.Run("remark appended only when non-blank", func(t *testing.T) {
		with := BuildMessage(&model.EventReminder{Title: "t", Remark: "带身份证"}, d(2026, 5, 20), today)
		if !strings.Contains(with, "备注：带身份证") {
			t.Fatalf("remark missing: %q", with)
		}
		without := BuildMessage(&model.EventReminder{Title: "t", Remark: "  "}, d(2026, 5, 20), today)
		if strings.Contains(without, "备注") {
			t.Fatalf("blank remark should be omitted: %q", without)
		}
	})
}
