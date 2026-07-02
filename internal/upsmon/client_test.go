package upsmon

import (
	"reflect"
	"testing"

	"github.com/LemonZuo/homer/internal/model"
)

func TestParseUPSCOutputFullReading(t *testing.T) {
	raw := `
battery.charge: 100
battery.runtime: 1200
battery.voltage: 13.6
battery.voltage.nominal: 12
battery.type: PbAc
device.mfr: CPS
device.model: UT1050EGC
input.voltage: 228.0
output.voltage: 228.0
ups.load: 21
ups.realpower: 63
ups.status: OL CHRG
`
	r, ok := parseUPSCOutput("ups", raw)
	if !ok {
		t.Fatal("expected ok")
	}
	if r.Name != "ups" || r.Mfr != "CPS" || r.Model != "UT1050EGC" {
		t.Fatalf("identity = %+v", r)
	}
	if r.PowerSource != model.UPSPowerMains {
		t.Fatalf("power source = %q", r.PowerSource)
	}
	if r.BatteryPercent != 100 || r.RuntimeMinutes != 20 {
		t.Fatalf("battery = %d%%, runtime = %dmin", r.BatteryPercent, r.RuntimeMinutes)
	}
	if r.BatteryVoltage != 13.6 || r.BatteryNominalVoltage != 12 || r.BatteryType != "PbAc" {
		t.Fatalf("battery volt = %+v", r)
	}
	if r.InputVoltage != 228 || r.OutputVoltage != 228 {
		t.Fatalf("voltage = %v / %v", r.InputVoltage, r.OutputVoltage)
	}
	if r.LoadPercent != 21 || r.RealPower != 63 {
		t.Fatalf("load = %d, power = %d", r.LoadPercent, r.RealPower)
	}
	if r.RawStatus != "OL CHRG" {
		t.Fatalf("raw status = %q", r.RawStatus)
	}
}

func TestParseUPSCOutputPowerFallbackChain(t *testing.T) {
	// 第三档:realpower/power 都缺,nominal × load 估算(四舍五入)
	r, ok := parseUPSCOutput("u", "ups.status: OL\nups.load: 21\nups.realpower.nominal: 300\n")
	if !ok {
		t.Fatal("expected ok")
	}
	if r.RealPower != 63 {
		t.Fatalf("estimated power = %d, want 63", r.RealPower)
	}

	// 第二档:ups.power 回退
	r, _ = parseUPSCOutput("u", "ups.status: OL\nups.power: 88\n")
	if r.RealPower != 88 {
		t.Fatalf("apparent power fallback = %d, want 88", r.RealPower)
	}

	// 第一档优先:realpower 存在时 power 不覆盖
	r, _ = parseUPSCOutput("u", "ups.realpower: 60\nups.power: 88\nups.status: OL\n")
	if r.RealPower != 60 {
		t.Fatalf("realpower should win, got %d", r.RealPower)
	}

	// nominal 缺 load 时不估算,保持 -1
	r, _ = parseUPSCOutput("u", "ups.status: OL\nups.realpower.nominal: 300\n")
	if r.RealPower != -1 {
		t.Fatalf("power should stay -1 without load, got %d", r.RealPower)
	}
}

func TestParseUPSCOutputMissingFieldsKeepSentinel(t *testing.T) {
	r, ok := parseUPSCOutput("u", "ups.status: OB\n")
	if !ok {
		t.Fatal("expected ok")
	}
	if r.BatteryPercent != -1 || r.RuntimeMinutes != -1 || r.LoadPercent != -1 || r.RealPower != -1 {
		t.Fatalf("missing numeric fields must stay -1: %+v", r)
	}
	if r.InputVoltage != -1 || r.OutputVoltage != -1 || r.BatteryVoltage != -1 {
		t.Fatalf("missing voltages must stay -1: %+v", r)
	}
}

func TestParseUPSCOutputClampsPercent(t *testing.T) {
	r, _ := parseUPSCOutput("u", "battery.charge: 130\nups.load: 250\n")
	if r.BatteryPercent != 100 || r.LoadPercent != 100 {
		t.Fatalf("clamp failed: battery=%d load=%d", r.BatteryPercent, r.LoadPercent)
	}
}

func TestParseUPSCOutputRejectsEmptyAndGarbage(t *testing.T) {
	if _, ok := parseUPSCOutput("u", ""); ok {
		t.Fatal("empty output must not be ok")
	}
	if _, ok := parseUPSCOutput("u", "   \n\t\n"); ok {
		t.Fatal("blank output must not be ok")
	}
	// 有冒号但没有任何 NUT 字段命中 → matched=0 → not ok
	if _, ok := parseUPSCOutput("u", "Error: Driver not connected\n"); ok {
		t.Fatal("no matched NUT field must not be ok")
	}
}

func TestParseUPSCOutputMfrFallback(t *testing.T) {
	// device.mfr 缺失时 ups.mfr 兜底;两者都有时 device.* 先出现先占位
	r, _ := parseUPSCOutput("u", "ups.mfr: FallbackVendor\nups.status: OL\n")
	if r.Mfr != "FallbackVendor" {
		t.Fatalf("mfr fallback = %q", r.Mfr)
	}
	r, _ = parseUPSCOutput("u", "device.mfr: Primary\nups.mfr: Secondary\nups.status: OL\n")
	if r.Mfr != "Primary" {
		t.Fatalf("device.mfr should win, got %q", r.Mfr)
	}
}

func TestMapPowerSource(t *testing.T) {
	cases := []struct {
		status string
		want   string
	}{
		{"OL", model.UPSPowerMains},
		{"OL CHRG", model.UPSPowerMains},
		{"OB", model.UPSPowerBattery},
		{"DISCHRG", model.UPSPowerBattery},
		{"OB DISCHRG", model.UPSPowerBattery},
		// 严重度优先:LB > OB > OL
		{"OB LB", model.UPSPowerLowBattery},
		{"OL OB LB", model.UPSPowerLowBattery},
		{"ol lb", model.UPSPowerLowBattery}, // 大小写不敏感
		{"", model.UPSPowerUnknown},
		{"FSD RB", model.UPSPowerUnknown}, // 无已知 token
	}
	for _, c := range cases {
		if got := mapPowerSource(c.status); got != c.want {
			t.Errorf("mapPowerSource(%q) = %q, want %q", c.status, got, c.want)
		}
	}
}

func TestSplitUPSNames(t *testing.T) {
	got := splitUPSNames("ups1\nups2 ups3\n\nups1\n")
	want := []string{"ups1", "ups2", "ups3"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	if splitUPSNames("") != nil {
		t.Fatal("empty input must return nil")
	}
	if splitUPSNames("  \n  ") != nil {
		t.Fatal("blank input must return nil")
	}
}
