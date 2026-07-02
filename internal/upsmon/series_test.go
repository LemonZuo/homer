package upsmon

import (
	"testing"
	"time"
)

func TestPickBucket(t *testing.T) {
	cases := []struct {
		window time.Duration
		want   int
	}{
		{1 * time.Hour, 60},
		{6 * time.Hour, 60},
		{12 * time.Hour, 300},
		{24 * time.Hour, 300},
		{3 * 24 * time.Hour, 900},
		{7 * 24 * time.Hour, 1800},
	}
	for _, c := range cases {
		if got := pickBucket(c.window); got != c.want {
			t.Errorf("pickBucket(%v) = %d, want %d", c.window, got, c.want)
		}
	}
}

func TestParseRange(t *testing.T) {
	cases := []struct {
		in   string
		want time.Duration
	}{
		{"1h", time.Hour},
		{"6h", 6 * time.Hour},
		{"12h", 12 * time.Hour},
		{"24h", 24 * time.Hour},
		{"3d", 3 * 24 * time.Hour},
		{"7d", 7 * 24 * time.Hour},
		{"", 24 * time.Hour},      // 缺省
		{"30d", 24 * time.Hour},   // 未知值回落
		{" 24h ", 24 * time.Hour}, // 容忍空白
		{"nonsense", 24 * time.Hour},
	}
	for _, c := range cases {
		if got := parseRange(c.in); got != c.want {
			t.Errorf("parseRange(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}
