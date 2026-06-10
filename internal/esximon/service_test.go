package esximon

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/LemonZuo/homer/internal/model"
)

func TestStickyUSBJSONKeepsPreviousControllers(t *testing.T) {
	prev := USBState{
		Controllers: []USBController{{
			PCIAddr: "0000:00:14.0",
			Name:    "Intel USB xHCI Host Controller",
		}},
		ArbitratorRunning: true,
		AvailableForPassthrough: []USBPassthroughDevice{{
			Bus: 1, Dev: 2, VID: "old", PID: "dev", Name: "old device", Enabled: true,
		}},
	}
	prevJSON, err := json.Marshal(prev)
	if err != nil {
		t.Fatal(err)
	}

	cur := USBState{
		ArbitratorRunning: true,
		AvailableForPassthrough: []USBPassthroughDevice{{
			Bus: 1, Dev: 3, VID: "152d", PID: "a576", Name: "JMicron", Enabled: true,
		}},
		arbitratorKnown:  true,
		passthroughKnown: true,
	}
	gotJSON := stickyUSBJSON(cur, string(prevJSON))

	var got USBState
	if err := json.Unmarshal([]byte(gotJSON), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Controllers) != 1 || got.Controllers[0].PCIAddr != "0000:00:14.0" {
		t.Fatalf("controllers were not preserved: %#v", got.Controllers)
	}
	if len(got.AvailableForPassthrough) != 1 || got.AvailableForPassthrough[0].PID != "a576" {
		t.Fatalf("passthrough devices were not refreshed: %#v", got.AvailableForPassthrough)
	}
}

func TestStickyUSBJSONClearsKnownEmptyPassthrough(t *testing.T) {
	prev := USBState{
		Controllers: []USBController{{PCIAddr: "0000:00:14.0", Name: "Intel USB"}},
		AvailableForPassthrough: []USBPassthroughDevice{{
			Bus: 1, Dev: 2, VID: "old", PID: "dev", Name: "old device", Enabled: true,
		}},
	}
	prevJSON, err := json.Marshal(prev)
	if err != nil {
		t.Fatal(err)
	}

	cur := USBState{
		Controllers:      prev.Controllers,
		controllersKnown: true,
		passthroughKnown: true,
	}
	gotJSON := stickyUSBJSON(cur, string(prevJSON))

	var got USBState
	if err := json.Unmarshal([]byte(gotJSON), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Controllers) != 1 {
		t.Fatalf("controllers = %#v", got.Controllers)
	}
	if len(got.AvailableForPassthrough) != 0 {
		t.Fatalf("known empty passthrough should clear previous values: %#v", got.AvailableForPassthrough)
	}
}

func TestSampleMissingMetricsRequiresCompleteCurrentData(t *testing.T) {
	m := HostMetrics{
		CPU:     CPUStatic{Cores: 2},
		CPUTemp: CPUTemperature{TjMaxC: 100, MaxC: -1, AvgC: -1},
		Disks:   []DiskHealth{{Device: "disk-a", TempC: -1}},
		VMs:     []VM{{ID: 1, Name: "a", State: "powered_on"}, {ID: 2, Name: "b", State: "unknown"}},
	}

	got := sampleMissingMetrics(m)
	want := []string{"cpu_temperature", "disk_health", "mce_health", "vm_power_state"}
	if len(got) != len(want) {
		t.Fatalf("missing len = %d, %#v", len(got), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("missing[%d] = %q, want %q; all=%#v", i, got[i], want[i], got)
		}
	}
}

func TestBuildStateIncompleteKeepsPreviousSnapshot(t *testing.T) {
	prevCPU := CPUTemperature{
		TjMaxC: 100,
		Cores:  []CPUCore{{ID: 0, TempC: 50, HeadroomC: 50}, {ID: 1, TempC: 51, HeadroomC: 49}},
		MaxC:   51,
		AvgC:   50,
	}
	prevDisks := []DiskHealth{
		{Device: "disk-a", TempC: 35},
		{Device: "cdrom", TempC: -1},
		{Device: "disk-b", TempC: 48},
	}
	prevVMs := []VM{
		{ID: 1, Name: "a", State: "powered_on"},
		{ID: 2, Name: "b", State: "powered_off"},
	}
	prevMCE := MCEHealth{State: "Green", CorrectedTotal: 3}
	prevAt := time.Unix(50, 0)
	prev := model.EsxiState{
		CPUTempJSON: mustJSON(prevCPU),
		DiskJSON:    mustJSON(prevDisks),
		VMJSON:      mustJSON(prevVMs),
		MCEJSON:     mustJSON(prevMCE),
		SampledAt:   &prevAt,
	}
	r := HostResult{
		HostKind: "esxi",
		HostID:   1,
		HostName: "host",
		OK:       true,
		Metrics: HostMetrics{
			CPUTemp: CPUTemperature{TjMaxC: 100, MaxC: -1, AvgC: -1},
			Disks:   []DiskHealth{{Device: "disk-a", TempC: -1}},
			VMs:     []VM{{ID: 1, Name: "a", State: "unknown"}, {ID: 2, Name: "b", State: "unknown"}},
			MCE:     MCEHealth{},
		},
	}

	got := buildState(r, &prev, time.Unix(100, 0), false)
	if got.SampledAt == nil || !got.SampledAt.Equal(prevAt) {
		t.Fatalf("incomplete state should keep previous sampled_at: %#v", got.SampledAt)
	}
	if got.CPUTempJSON != prev.CPUTempJSON || got.DiskJSON != prev.DiskJSON || got.VMJSON != prev.VMJSON || got.MCEJSON != prev.MCEJSON {
		t.Fatalf("incomplete state should keep previous JSON: %#v", got)
	}
}

func TestBuildSampleUsesCurrentCompleteMetrics(t *testing.T) {
	r := HostResult{
		HostKind: "esxi",
		HostID:   1,
		HostName: "host",
		OK:       true,
		Metrics: HostMetrics{
			CPU: CPUStatic{Cores: 2},
			CPUTemp: CPUTemperature{
				TjMaxC: 100,
				Cores:  []CPUCore{{ID: 0, TempC: 50, HeadroomC: 50}, {ID: 1, TempC: 52, HeadroomC: 48}},
				MaxC:   52,
				AvgC:   51,
			},
			Disks: []DiskHealth{
				{Device: "disk-a", TempC: 35},
				{Device: "disk-b", TempC: 48},
			},
			VMs: []VM{
				{ID: 1, Name: "a", State: "powered_on"},
				{ID: 2, Name: "b", State: "powered_off"},
			},
			MCE: MCEHealth{State: "Green", CorrectedTotal: 3},
		},
	}

	got := buildSample(r, time.Unix(100, 0))
	if got.CPUMaxC != 52 || got.CPUAvgC != 51 || got.CPUTjMaxC != 100 {
		t.Fatalf("cpu sample failed: %#v", got)
	}
	if got.DiskMaxC != 48 {
		t.Fatalf("disk max = %d", got.DiskMaxC)
	}
	if got.VMTotal != 2 || got.VMPoweredOn != 1 {
		t.Fatalf("vm total/on = %d/%d", got.VMTotal, got.VMPoweredOn)
	}
	if got.MCEState != "Green" || got.MCECorrectedTotal != 3 {
		t.Fatalf("mce sample failed: %#v", got)
	}

	var disks []DiskTempPoint
	if err := json.Unmarshal([]byte(got.DiskTempPerDiskJSON), &disks); err != nil {
		t.Fatal(err)
	}
	if len(disks) != 2 {
		t.Fatalf("disk points = %#v", disks)
	}
}

func TestMergeHostMetricsCompletesPartialAttempts(t *testing.T) {
	base := HostMetrics{
		CPU:     CPUStatic{Cores: 2},
		CPUTemp: CPUTemperature{TjMaxC: 100, Cores: []CPUCore{{ID: 0, TempC: 50}, {ID: 1, TempC: 51}}, MaxC: 51, AvgC: 50},
		Disks: []DiskHealth{
			{Device: "disk-a", TempC: -1, UsedBytes: -1, FreeBytes: -1, ThresholdC: -1, Status: "unknown"},
			{Device: "disk-b", TempC: 42, UsedBytes: -1, FreeBytes: -1, ThresholdC: -1, Status: "ok"},
		},
		VMs: []VM{{ID: 1, Name: "a", State: "unknown"}},
	}
	next := HostMetrics{
		CPU: CPUStatic{Cores: 2},
		Disks: []DiskHealth{
			{Device: "disk-a", TempC: 35, UsedBytes: 10, FreeBytes: 20, ThresholdC: 70, Status: "ok"},
			{Device: "disk-b", TempC: 42, UsedBytes: 30, FreeBytes: 40, ThresholdC: 70, Status: "ok"},
		},
		MCE: MCEHealth{State: "Green"},
		VMs: []VM{{ID: 1, Name: "a", State: "powered_on"}},
	}

	got := mergeHostMetrics(base, next)
	if missing := sampleMissingMetrics(got); len(missing) != 0 {
		t.Fatalf("merged metrics should be complete, missing=%#v got=%#v", missing, got)
	}
	if got.Disks[0].TempC != 35 || got.VMs[0].State != "powered_on" || got.MCE.State != "Green" {
		t.Fatalf("merge did not fill missing fields: %#v", got)
	}
}

func TestBuildSampleUsesCurrentValidZeroVMOn(t *testing.T) {
	r := HostResult{
		HostKind: "esxi",
		HostID:   1,
		HostName: "host",
		OK:       true,
		Metrics: HostMetrics{
			CPU:     CPUStatic{Cores: 1},
			CPUTemp: CPUTemperature{TjMaxC: 100, Cores: []CPUCore{{ID: 0, TempC: 50}}, MaxC: 50, AvgC: 50},
			Disks:   []DiskHealth{{Device: "disk-a", TempC: 35}},
			MCE:     MCEHealth{State: "Green"},
			VMs: []VM{
				{ID: 1, Name: "a", State: "powered_off"},
				{ID: 2, Name: "b", State: "powered_off"},
			},
		},
	}

	got := buildSample(r, time.Unix(100, 0))
	if got.VMTotal != 2 || got.VMPoweredOn != 0 {
		t.Fatalf("valid all-off VM state should not fallback: %d/%d", got.VMTotal, got.VMPoweredOn)
	}
}
