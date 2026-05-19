package logx

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
	"time"
)

func TestParseLevel(t *testing.T) {
	cases := map[string]slog.Level{
		"debug":   slog.LevelDebug,
		"DEBUG":   slog.LevelDebug,
		" warn":   slog.LevelWarn,
		"warning": slog.LevelWarn,
		"error":   slog.LevelError,
		"info":    slog.LevelInfo,
		"":        slog.LevelInfo,
		"bogus":   slog.LevelInfo,
	}
	for in, want := range cases {
		if got := parseLevel(in); got != want {
			t.Errorf("parseLevel(%q)=%v want %v", in, got, want)
		}
	}
}

func TestLevelLabel(t *testing.T) {
	cases := map[slog.Level]string{
		slog.LevelDebug:     "DEBUG",
		slog.LevelInfo:      "INFO ",
		slog.LevelWarn:      "WARN ",
		slog.LevelError:     "ERROR",
		slog.LevelError + 4: "ERROR",
	}
	for lv, want := range cases {
		if got := levelLabel(lv); got != want {
			t.Errorf("levelLabel(%v)=%q want %q", lv, got, want)
		}
	}
}

func TestNeedsQuoting(t *testing.T) {
	quoted := []string{"", "has space", "a=b", `quote"`, `back\slash`, "tab\tx"}
	for _, s := range quoted {
		if !needsQuoting(s) {
			t.Errorf("needsQuoting(%q) = false, want true", s)
		}
	}
	for _, s := range []string{"plain", "a.b.c", "123", "kebab-case"} {
		if needsQuoting(s) {
			t.Errorf("needsQuoting(%q) = true, want false", s)
		}
	}
}

func TestFormatValue(t *testing.T) {
	if got := formatValue(slog.StringValue("plain")); got != "plain" {
		t.Fatalf("plain => %q", got)
	}
	if got := formatValue(slog.StringValue("has space")); got != `"has space"` {
		t.Fatalf("spaced => %q want quoted", got)
	}
	if got := formatValue(slog.IntValue(42)); got != "42" {
		t.Fatalf("int => %q", got)
	}
}

func TestHandlerHandleFormat(t *testing.T) {
	var buf bytes.Buffer
	h := newHandler(&buf, slog.LevelInfo)

	if h.Enabled(context.Background(), slog.LevelDebug) {
		t.Fatal("debug should be disabled at info level")
	}

	r := slog.NewRecord(time.Date(2026, 5, 19, 1, 2, 3, 0, time.UTC), slog.LevelInfo, "hello", 0)
	r.AddAttrs(slog.String("k", "v"), slog.Int("n", 7), slog.String("empty", ""))
	if err := h.Handle(context.Background(), r); err != nil {
		t.Fatalf("Handle: %v", err)
	}
	line := buf.String()
	for _, want := range []string{"2026/05/19 - 01:02:03.000", "| INFO  |", "hello", "k=v", "n=7", `empty=""`} {
		if !strings.Contains(line, want) {
			t.Fatalf("line %q missing %q", line, want)
		}
	}
	if !strings.HasSuffix(line, "\n") {
		t.Fatal("line should end with newline")
	}
	// 首个属性前应为 " | " 分隔。
	if !strings.Contains(line, "hello | k=v") {
		t.Fatalf("expected ' | ' before first attr: %q", line)
	}
}

func TestHandlerWithAttrsAndGroup(t *testing.T) {
	var buf bytes.Buffer
	base := newHandler(&buf, slog.LevelInfo)

	// 无 attrs / 空 group 返回自身,不分配新 handler。
	if base.WithAttrs(nil) != slog.Handler(base) {
		t.Fatal("WithAttrs(nil) should return receiver")
	}
	if base.WithGroup("") != slog.Handler(base) {
		t.Fatal("WithGroup(\"\") should return receiver")
	}

	h := base.WithAttrs([]slog.Attr{slog.String("component", "sms")}).WithGroup("req")
	r := slog.NewRecord(time.Now(), slog.LevelWarn, "msg", 0)
	r.AddAttrs(slog.String("id", "42"))
	if err := h.Handle(context.Background(), r); err != nil {
		t.Fatalf("Handle: %v", err)
	}
	line := buf.String()
	if !strings.Contains(line, "component=sms") {
		t.Fatalf("fixed attr missing: %q", line)
	}
	if !strings.Contains(line, "req.id=42") {
		t.Fatalf("group prefix not applied: %q", line)
	}
}
