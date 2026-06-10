package esximon

import (
	"testing"

	"github.com/LemonZuo/homer/internal/model"
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
		Disks: []DiskTemperature{
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

func TestThresholdAlertDiffOnlyNewItems(t *testing.T) {
	cfg := AlertConfig{
		CPUTempC:           80,
		CPUUsagePercent:    90,
		MemoryUsagePercent: 90,
		DiskTempC:          55,
		DiskUsagePercent:   90,
	}
	prevMetrics := HostMetrics{
		CPUTemp: CPUTemperature{MaxC: 82},
		Runtime: RuntimeUsage{CPUCapacityMHz: 10000, CPUUsagePercent: 91},
	}
	prev := model.EsxiState{
		CPUTempJSON: mustJSON(prevMetrics.CPUTemp),
		RuntimeJSON: mustJSON(prevMetrics.Runtime),
	}
	cur := HostMetrics{
		CPUTemp: CPUTemperature{MaxC: 83},
		Runtime: RuntimeUsage{
			CPUCapacityMHz:     10000,
			CPUUsagePercent:    92,
			MemoryTotalBytes:   1000,
			MemoryUsagePercent: 95,
		},
	}

	prevItems := thresholdAlertItems(metricsFromState(&prev), cfg)
	prevSet := map[string]struct{}{}
	for _, item := range prevItems {
		prevSet[item.Key] = struct{}{}
	}
	var newKeys []string
	for _, item := range thresholdAlertItems(cur, cfg) {
		if _, ok := prevSet[item.Key]; !ok {
			newKeys = append(newKeys, item.Key)
		}
	}
	if len(newKeys) != 1 || newKeys[0] != "memory_usage" {
		t.Fatalf("new keys = %#v", newKeys)
	}
}
