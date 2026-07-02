package upsmon

import (
	"strings"
	"testing"

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
