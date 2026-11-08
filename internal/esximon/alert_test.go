package esximon

import (
	"testing"
)

func TestThresholdAlertItems(t *testing.T) {
	cfg := AlertConfig{
		CPUTempC:           80,
		CPUUsagePercent:    90,
		MemoryUsagePercent: 90,
		DiskTempC:          55,
		DiskUsagePercent:   90,
	}
	m := HostMetrics{
		CPUTemp: CPUTemperature{MaxC: 82},
		Runtime: RuntimeUsage{
			CPUCapacityMHz:     10000,
			CPUUsagePercent:    91,
			MemoryTotalBytes:   1000,
			MemoryUsagePercent: 92,
		},
		Disks: []DiskHealth{
			{Device: "naa.hot", Model: "hot disk", TempC: 56, UsedBytes: 950, CapacityBytes: 1000},
			{Device: "naa.ok", Model: "ok disk", TempC: 40, UsedBytes: 100, CapacityBytes: 1000},
		},
	}

	items := thresholdAlertItems(m, cfg)
	got := map[string]bool{}
	for _, item := range items {
		got[item.Key] = true
	}
	for _, key := range []string{"cpu_temp", "cpu_usage", "memory_usage", "disk_temp:naa.hot", "disk_usage:naa.hot"} {
		if !got[key] {
			t.Fatalf("missing alert key %q in %#v", key, items)
		}
	}
	if got["disk_temp:naa.ok"] || got["disk_usage:naa.ok"] {
		t.Fatalf("ok disk should not alert: %#v", items)
	}
}

func TestThresholdAlertNeedsConsecutiveSamples(t *testing.T) {
	item := thresholdAlertItem{Key: "cpu_temp", Metric: "CPU 温度"}
	state := thresholdAlertState{}
	for i := 1; i <= 4; i++ {
		var due []thresholdAlertItem
		state, due = advanceThresholdAlertState(state, []thresholdAlertItem{item}, measuredAll, 5)
		if len(due) != 0 {
			t.Fatalf("attempt %d should not alert: %#v", i, due)
		}
		if state.Items[item.Key].Count != i {
			t.Fatalf("attempt %d count = %d", i, state.Items[item.Key].Count)
		}
	}

	state, due := advanceThresholdAlertState(state, []thresholdAlertItem{item}, measuredAll, 5)
	if len(due) != 1 || due[0].Key != item.Key {
		t.Fatalf("5th attempt should alert: %#v", due)
	}
	record := state.Items[item.Key]
	record.Notified = true
	state.Items[item.Key] = record

	state, due = advanceThresholdAlertState(state, []thresholdAlertItem{item}, measuredAll, 5)
	if len(due) != 0 {
		t.Fatalf("notified active alert should not repeat: %#v", due)
	}
	if state.Items[item.Key].Count != 6 {
		t.Fatalf("continued count = %d", state.Items[item.Key].Count)
	}
}

func TestThresholdAlertResetsAfterRecovery(t *testing.T) {
	item := thresholdAlertItem{Key: "cpu_usage", Metric: "CPU 使用率"}
	state := thresholdAlertState{}
	state, _ = advanceThresholdAlertState(state, []thresholdAlertItem{item}, measuredAll, 5)
	state, _ = advanceThresholdAlertState(state, []thresholdAlertItem{item}, measuredAll, 5)
	if state.Items[item.Key].Count != 2 {
		t.Fatalf("count before recovery = %d", state.Items[item.Key].Count)
	}

	state, due := advanceThresholdAlertState(state, nil, measuredAll, 5)
	if len(due) != 0 || len(state.Items) != 0 {
		t.Fatalf("recovery should clear state, state=%#v due=%#v", state, due)
	}

	state, due = advanceThresholdAlertState(state, []thresholdAlertItem{item}, measuredAll, 5)
	if len(due) != 0 || state.Items[item.Key].Count != 1 {
		t.Fatalf("after recovery should start over, state=%#v due=%#v", state, due)
	}
}

// measuredAll 表示测试里涉及的指标本轮都测到了值。
var measuredAll = map[string]bool{"cpu_temp": true, "cpu_usage": true, "memory_usage": true}

func TestThresholdAlertKeepsStateWhenMetricMissing(t *testing.T) {
	item := thresholdAlertItem{Key: "disk_temp:nvme0", Metric: "磁盘温度"}
	state := thresholdAlertState{Items: map[string]thresholdAlertRecord{item.Key: {Count: 7, Notified: true}}}

	// 本轮该盘温度没采到：保留"已通知"记录，不计数也不清零。
	state, due := advanceThresholdAlertState(state, nil, map[string]bool{"cpu_temp": true}, 5)
	if len(due) != 0 {
		t.Fatalf("missing metric should not alert: %#v", due)
	}
	if rec := state.Items[item.Key]; rec.Count != 7 || !rec.Notified {
		t.Fatalf("missing metric should keep record, got %#v", rec)
	}

	// 下一轮又测到且仍超阈值：不应重复告警。
	state, due = advanceThresholdAlertState(state, []thresholdAlertItem{item}, map[string]bool{item.Key: true}, 5)
	if len(due) != 0 || state.Items[item.Key].Count != 8 {
		t.Fatalf("still-hot disk should not re-alert, state=%#v due=%#v", state, due)
	}

	// 测到且回落：记录清掉。
	state, _ = advanceThresholdAlertState(state, nil, map[string]bool{item.Key: true}, 5)
	if len(state.Items) != 0 {
		t.Fatalf("measured recovery should clear state: %#v", state)
	}
}

func TestMeasuredAlertKeysSkipsSentinels(t *testing.T) {
	m := HostMetrics{
		CPUTemp: CPUTemperature{MaxC: -1},
		Runtime: RuntimeUsage{CPUUsagePercent: 12, MemoryUsagePercent: -1},
		Disks: []DiskHealth{
			{Device: "a", TempC: 40, UsedBytes: -1},
			{Device: "b", TempC: -1, UsedBytes: 50, CapacityBytes: 100},
		},
	}
	got := measuredAlertKeys(m)
	want := map[string]bool{"cpu_usage": true, "disk_temp:a": true, "disk_usage:b": true}
	if len(got) != len(want) {
		t.Fatalf("measured = %#v, want %#v", got, want)
	}
	for k := range want {
		if !got[k] {
			t.Fatalf("measured missing %q: %#v", k, got)
		}
	}
}
