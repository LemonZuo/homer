package upsmon

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/LemonZuo/homer/internal/model"
)

func TestComposeMessageBatteryAlert(t *testing.T) {
	st := model.UPSState{
		HostName:           "fnOS",
		UPSName:            "ups",
		Mfr:                "CPS",
		Model:              "UT1050EGC",
		LastBatteryPercent: 87,
		LastRuntimeMinutes: 42,
		LastRawStatus:      "OB DISCHRG",
	}
	title, body := composeMessage("switched_to_battery", st)
	if title != "UPS 切到电池供电" {
		t.Fatalf("title = %q", title)
	}
	for _, want := range []string{"fnOS 上的 ups", "CPS UT1050EGC", "87%", "42 分钟", "OB DISCHRG"} {
		if !strings.Contains(body, want) {
			t.Fatalf("body missing %q:\n%s", want, body)
		}
	}
}

func TestComposeMessageDeviceOptional(t *testing.T) {
	st := model.UPSState{HostName: "h", UPSName: "u", LastBatteryPercent: 10, LastRawStatus: "LB"}
	_, body := composeMessage("low_battery", st)
	if strings.Contains(body, "()") || strings.Contains(body, ":(") {
		t.Fatalf("empty device must not render parens:\n%s", body)
	}
}

func TestComposeMessageRestoredOmitsRuntime(t *testing.T) {
	st := model.UPSState{HostName: "h", UPSName: "u", LastBatteryPercent: 95, LastRawStatus: "OL CHRG"}
	title, body := composeMessage("restored_mains", st)
	if title != "UPS 已恢复市电" {
		t.Fatalf("title = %q", title)
	}
	if strings.Contains(body, "预估续航") {
		t.Fatalf("restored message should omit runtime:\n%s", body)
	}
}

func TestComposeMessageUnknownKindFallback(t *testing.T) {
	st := model.UPSState{LastRawStatus: "RB"}
	title, body := composeMessage("someday_new_kind", st)
	if title != "UPS 状态变化" || body != "RB" {
		t.Fatalf("fallback = %q / %q", title, body)
	}
}

func TestFmtRuntime(t *testing.T) {
	cases := []struct {
		min  int
		want string
	}{
		{-1, "未知"},
		{0, "0 分钟"},
		{59, "59 分钟"},
		{60, "1 小时 0 分"},
		{135, "2 小时 15 分"},
	}
	for _, c := range cases {
		if got := fmtRuntime(c.min); got != c.want {
			t.Errorf("fmtRuntime(%d) = %q, want %q", c.min, got, c.want)
		}
	}
}

// nilStore 场景不测 Process 全链路(依赖 store.MarkAlerted → DB),
// 但静默分支(nil receiver / 同状态 / 未知态)不触 DB,可直接验证。
func TestNotifierProcessSilentBranches(t *testing.T) {
	// nil Notifier 不 panic
	var n *Notifier
	n.Process(nil, model.UPSState{})

	out := &fakeOut{}
	n2 := &Notifier{out: out}
	// prev=nil → prevSrc=unknown → 静默
	n2.Process(nil, model.UPSState{LastPowerSource: model.UPSPowerMains})
	// 同状态 → 静默
	prev := model.UPSState{LastPowerSource: model.UPSPowerMains}
	n2.Process(&prev, model.UPSState{LastPowerSource: model.UPSPowerMains})
	// curr unknown → 静默
	n2.Process(&prev, model.UPSState{LastPowerSource: model.UPSPowerUnknown})
	if len(out.titles()) != 0 {
		t.Fatalf("silent branches must not send, got %v", out.titles())
	}
}

func upsState(src string) model.UPSState {
	return model.UPSState{HostKind: "ups", HostID: 1, HostName: "h", UPSName: "u", LastPowerSource: src}
}

type fakeClock struct{ t time.Time }

func (c *fakeClock) now() time.Time          { return c.t }
func (c *fakeClock) advance(d time.Duration) { c.t = c.t.Add(d) }

func TestNotifierRetriesFailedAlert(t *testing.T) {
	out := &fakeOut{err: errors.New("network down")}
	clk := &fakeClock{t: time.Date(2026, 11, 7, 10, 0, 0, 0, time.Local)}
	n := &Notifier{out: out, now: clk.now}

	mains, battery := upsState(model.UPSPowerMains), upsState(model.UPSPowerBattery)
	n.Process(&mains, battery) // 发送失败
	if len(out.titles()) != 0 {
		t.Fatalf("failed send must not be recorded: %v", out.titles())
	}

	// 下一轮 DB 里已是 battery;未到重试时间不重发。
	out.err = nil
	clk.advance(30 * time.Second)
	n.Process(&battery, battery)
	if len(out.titles()) != 0 {
		t.Fatalf("should wait for retry interval, got %v", out.titles())
	}

	clk.advance(alertRetryInterval)
	n.Process(&battery, battery)
	if got := out.titles(); len(got) != 1 || got[0] != "UPS 切到电池供电" {
		t.Fatalf("retry should resend switched_to_battery, got %v", got)
	}

	// 已送达后不再重复。
	clk.advance(alertRetryInterval)
	n.Process(&battery, battery)
	if len(out.titles()) != 1 {
		t.Fatalf("delivered alert must not repeat: %v", out.titles())
	}
}

func TestNotifierUnknownRoundDoesNotSwallowSwitch(t *testing.T) {
	out := &fakeOut{}
	n := &Notifier{out: out}
	mains, unknown, battery := upsState(model.UPSPowerMains), upsState(model.UPSPowerUnknown), upsState(model.UPSPowerBattery)

	n.Process(&mains, mains)
	n.Process(&mains, unknown)   // 一轮未知:不告警、不覆盖基线
	n.Process(&unknown, battery) // DB 上一轮是 unknown,但基线仍是 mains
	if got := out.titles(); len(got) != 1 || got[0] != "UPS 切到电池供电" {
		t.Fatalf("switch after unknown round should alert, got %v", got)
	}
}

func TestNotifierRestartUsesPersistedState(t *testing.T) {
	// 重启前已报过电池供电,停机期间恢复市电:新进程首轮应补发"已恢复市电"。
	out := &fakeOut{}
	n := &Notifier{out: out}
	battery, mains := upsState(model.UPSPowerBattery), upsState(model.UPSPowerMains)
	n.Process(&battery, mains)
	if got := out.titles(); len(got) != 1 || got[0] != "UPS 已恢复市电" {
		t.Fatalf("restored after restart should alert, got %v", got)
	}
}

func TestNotifierBaselineRetriesAndAcceptsMains(t *testing.T) {
	out := &fakeOut{err: errors.New("down")}
	clk := &fakeClock{t: time.Date(2026, 11, 7, 10, 0, 0, 0, time.Local)}
	n := &Notifier{out: out, now: clk.now}
	battery := upsState(model.UPSPowerBattery)

	n.ProcessBaseline(battery)
	out.err = nil
	clk.advance(alertRetryInterval)
	n.Process(&battery, battery)
	if got := out.titles(); len(got) != 1 || got[0] != "UPS 当前使用电池供电" {
		t.Fatalf("baseline alert should be retried, got %v", got)
	}

	// 基线为市电时直接接受,不发任何告警。
	n2 := &Notifier{out: out}
	out.reset()
	mains := upsState(model.UPSPowerMains)
	n2.ProcessBaseline(mains)
	n2.Process(&mains, mains)
	if len(out.titles()) != 0 {
		t.Fatalf("mains baseline must stay silent, got %v", out.titles())
	}
}
