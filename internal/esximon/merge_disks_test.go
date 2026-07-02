package esximon

import (
	"testing"
	"time"
)

func TestMergeDiskHealthNewerSMARTWins(t *testing.T) {
	t0 := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	t1 := t0.Add(time.Hour)
	base := DiskHealth{Device: "t10.a", TempC: 35, Status: "ok", PowerOnHours: 100, SMARTLastFullSuccessAt: t0}
	next := DiskHealth{Device: "t10.a", TempC: 47, Status: "warning", PowerOnHours: 101, SMARTLastFullSuccessAt: t1}
	got := mergeDiskHealth(base, next)
	if got.TempC != 47 || got.Status != "warning" || got.PowerOnHours != 101 {
		t.Fatalf("newer must win: %+v", got)
	}
	if !got.SMARTLastFullSuccessAt.Equal(t1) {
		t.Fatalf("timestamp = %v", got.SMARTLastFullSuccessAt)
	}
}

func TestMergeDiskHealthOlderSMARTOnlyFillsGaps(t *testing.T) {
	t0 := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	base := DiskHealth{
		Device: "t10.a", TempC: -1, ThresholdC: -1, Status: "unknown",
		UsedBytes: -1, FreeBytes: -1, SMARTLastFullSuccessAt: t0,
	}
	next := DiskHealth{
		Device: "t10.a", Model: "870 EVO", Type: "SATA-SSD", CapacityBytes: 1000,
		UsedBytes: 500, FreeBytes: 500, TempC: 35, ThresholdC: 70, Status: "ok",
		Datastores: []string{"ds1"}, SMARTLastFullSuccessAt: t0.Add(-time.Hour),
	}
	got := mergeDiskHealth(base, next)
	// 旧 SMART 不整体覆盖,只补缺失字段
	if got.TempC != 35 || got.ThresholdC != 70 || got.Status != "ok" {
		t.Fatalf("gap fill = %+v", got)
	}
	if got.Model != "870 EVO" || got.CapacityBytes != 1000 || got.UsedBytes != 500 {
		t.Fatalf("static fill = %+v", got)
	}
	if len(got.Datastores) != 1 {
		t.Fatalf("datastores = %v", got.Datastores)
	}
	if !got.SMARTLastFullSuccessAt.Equal(t0) {
		t.Fatalf("base timestamp must be kept: %v", got.SMARTLastFullSuccessAt)
	}
}

func TestMergeDiskHealthList(t *testing.T) {
	t1 := time.Date(2026, 7, 1, 1, 0, 0, 0, time.UTC)
	base := []DiskHealth{
		{Device: "t10.a", TempC: 35},
		{Device: "t10.b", TempC: 43},
	}
	next := []DiskHealth{
		{Device: "t10.a", TempC: 36, SMARTLastFullSuccessAt: t1},
		{Device: "t10.c", TempC: 52}, // base 没有的新盘要追加
	}
	got := mergeDiskHealthList(base, next)
	if len(got) != 3 {
		t.Fatalf("len = %d: %v", len(got), got)
	}
	if got[0].Device != "t10.a" || got[0].TempC != 36 {
		t.Fatalf("merged a = %+v", got[0])
	}
	if got[1].Device != "t10.b" || got[1].TempC != 43 {
		t.Fatalf("untouched b = %+v", got[1])
	}
	if got[2].Device != "t10.c" {
		t.Fatalf("appended c = %+v", got[2])
	}
	// base 为空 → 直接用 next
	if got2 := mergeDiskHealthList(nil, next); len(got2) != 2 {
		t.Fatalf("empty base = %v", got2)
	}
}

func TestMergeVMStates(t *testing.T) {
	t0 := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	t1 := t0.Add(time.Minute)
	base := []VM{
		{ID: 1, Name: "fnOS", State: "on", PowerStateLastFullSuccessAt: t0},
		{ID: 2, Name: "", State: "unknown"},
	}
	next := []VM{
		{ID: 1, State: "off", PowerStateLastFullSuccessAt: t1},
		{ID: 2, Name: "win2022", GuestOS: "windows2019srvNext-64", State: "on", PowerStateLastFullSuccessAt: t1},
		{ID: 3, Name: "ghost"}, // base 没有的 VM 不追加(以 base 名单为准)
	}
	got := mergeVMStates(base, next)
	if len(got) != 2 {
		t.Fatalf("len = %d: %v", len(got), got)
	}
	if got[0].State != "off" || got[0].Name != "fnOS" {
		t.Fatalf("vm1 = %+v", got[0])
	}
	// 空名字/unknown 态被 next 补齐
	if got[1].Name != "win2022" || got[1].GuestOS != "windows2019srvNext-64" || got[1].State != "on" {
		t.Fatalf("vm2 = %+v", got[1])
	}
	// base 为 nil → 用 next;base 为空切片 → 保持空
	if got2 := mergeVMStates(nil, next); len(got2) != 3 {
		t.Fatalf("nil base = %v", got2)
	}
	if got3 := mergeVMStates([]VM{}, next); len(got3) != 0 {
		t.Fatalf("empty base = %v", got3)
	}
}

func TestMergeNIC(t *testing.T) {
	t0 := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	t1 := t0.Add(time.Minute)
	base := NIC{
		Name: "vmnic0", Driver: "ne1000", MAC: "d8:bb:c1:c1:b6:49", MTU: 1500,
		LinkStatus: "Up", SpeedMbps: 1000,
		RxBytes: 100, TxBytes: 200, StatsLastFullSuccessAt: t0,
	}
	// 新统计 → 覆盖流量计数
	next := NIC{Name: "vmnic0", MTU: -1, SpeedMbps: -1, RxBytes: 150, TxBytes: 250, StatsLastFullSuccessAt: t1}
	got := mergeNIC(base, next)
	if got.RxBytes != 150 || got.TxBytes != 250 || !got.StatsLastFullSuccessAt.Equal(t1) {
		t.Fatalf("stats = %+v", got)
	}
	// 空串/负值不覆盖静态字段
	if got.Driver != "ne1000" || got.MAC != "d8:bb:c1:c1:b6:49" || got.MTU != 1500 || got.SpeedMbps != 1000 {
		t.Fatalf("static overwritten = %+v", got)
	}
	// 旧统计不覆盖
	older := NIC{Name: "vmnic0", RxBytes: 1, StatsLastFullSuccessAt: t0.Add(-time.Hour)}
	got2 := mergeNIC(got, older)
	if got2.RxBytes != 150 {
		t.Fatalf("old stats must not overwrite: %+v", got2)
	}
}

func TestMergeNICList(t *testing.T) {
	base := []NIC{{Name: "vmnic0", Driver: "ne1000"}}
	next := []NIC{
		{Name: "vmnic0", LinkStatus: "Up"},
		{Name: "vmnic1", Driver: "igc-community"},
	}
	got := mergeNICList(base, next)
	if len(got) != 2 {
		t.Fatalf("len = %d: %v", len(got), got)
	}
	if got[0].Driver != "ne1000" || got[0].LinkStatus != "Up" {
		t.Fatalf("merged = %+v", got[0])
	}
	if got[1].Name != "vmnic1" {
		t.Fatalf("appended = %+v", got[1])
	}
	if got2 := mergeNICList(nil, next); len(got2) != 2 {
		t.Fatal("empty base must return next")
	}
	if got3 := mergeNICList(base, nil); len(got3) != 1 {
		t.Fatal("empty next must return base")
	}
}
