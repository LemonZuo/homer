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

func TestStickyTopologyJSONKeepsPreviousVMKPortsOnly(t *testing.T) {
	prev := NetTopology{
		VSwitches: []VSwitchInfo{{Name: "oldSwitch"}},
		VMNICs: []VMNICLink{{
			VMName:    "oldVM",
			VSwitch:   "oldSwitch",
			Portgroup: "Old Network",
			MAC:       "00:0c:29:00:00:01",
		}},
		VMKPorts: []VMKPort{{
			Name:      "vmk0",
			VSwitch:   "vSwitch0",
			Portgroup: "Management Network",
			MAC:       "00:50:56:6a:11:22",
			IPv4:      "192.168.8.138",
			Enabled:   true,
		}},
	}
	cur := NetTopology{
		VSwitches: []VSwitchInfo{{Name: "vSwitch0"}},
		VMNICs: []VMNICLink{{
			VMName:    "fnOS",
			VSwitch:   "vSwitch0",
			Portgroup: "VM Network",
			MAC:       "00:0c:29:86:f2:a0",
		}},
	}
	gotJSON := stickyTopologyJSON(cur, true, mustJSON(prev))

	var got NetTopology
	if err := json.Unmarshal([]byte(gotJSON), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.VSwitches) != 1 || got.VSwitches[0].Name != "vSwitch0" {
		t.Fatalf("current vswitch should be kept, got %#v", got.VSwitches)
	}
	if len(got.VMNICs) != 1 || got.VMNICs[0].VMName != "fnOS" {
		t.Fatalf("current vm_nics should be kept, got %#v", got.VMNICs)
	}
	if len(got.VMKPorts) != 1 || got.VMKPorts[0].Name != "vmk0" || got.VMKPorts[0].IPv4 != "192.168.8.138" {
		t.Fatalf("previous vmk_ports should be preserved, got %#v", got.VMKPorts)
	}
}

func TestStickyTopologyJSONUsesKnownEmptyVMKPorts(t *testing.T) {
	prev := NetTopology{
		VSwitches: []VSwitchInfo{{Name: "vSwitch0"}},
		VMKPorts:  []VMKPort{{Name: "vmk0", IPv4: "192.168.8.138", Enabled: true}},
	}
	cur := NetTopology{
		VSwitches:    []VSwitchInfo{{Name: "vSwitch0"}},
		VMNICs:       []VMNICLink{{VMName: "fnOS", VSwitch: "vSwitch0", Portgroup: "VM Network", MAC: "00:0c:29:86:f2:a0"}},
		VMKCollected: true,
	}
	gotJSON := stickyTopologyJSON(cur, true, mustJSON(prev))

	var got NetTopology
	if err := json.Unmarshal([]byte(gotJSON), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.VMKPorts) != 0 {
		t.Fatalf("known empty vmk_ports should clear previous values: %#v", got.VMKPorts)
	}
}

func TestStickyTopologyJSONKeepsPreviousVMNICIP(t *testing.T) {
	prev := NetTopology{
		VSwitches: []VSwitchInfo{{Name: "vSwitch0"}},
		VMNICs: []VMNICLink{{
			VMName:    "fnOS",
			VSwitch:   "vSwitch0",
			Portgroup: "VM Network",
			MAC:       "00:0c:29:86:f2:a0",
			IP:        "192.168.8.21",
		}},
	}
	cur := NetTopology{
		VSwitches: []VSwitchInfo{{Name: "vSwitch0"}},
		VMNICs: []VMNICLink{{
			VMName:    "fnOS",
			VSwitch:   "vSwitch0",
			Portgroup: "VM Network",
			MAC:       "00:0C:29:86:F2:A0",
		}},
	}
	gotJSON := stickyTopologyJSON(cur, true, mustJSON(prev))

	var got NetTopology
	if err := json.Unmarshal([]byte(gotJSON), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.VMNICs) != 1 || got.VMNICs[0].IP != "192.168.8.21" {
		t.Fatalf("previous vmnic ip should be preserved, got %#v", got.VMNICs)
	}
}

func TestStickyTopologyJSONKeepsPreviousFullSuccessAt(t *testing.T) {
	lastSuccess := time.Date(2026, 6, 12, 10, 0, 0, 0, time.Local)
	prev := NetTopology{
		VSwitches:         []VSwitchInfo{{Name: "vSwitch0"}},
		VMNICs:            []VMNICLink{{VMName: "fnOS", MAC: "00:0c:29:86:f2:a0"}},
		LastFullSuccessAt: lastSuccess,
	}
	cur := NetTopology{
		VSwitches: []VSwitchInfo{{Name: "vSwitch0"}},
		VMNICs:    []VMNICLink{{VMName: "fnOS", MAC: "00:0c:29:86:f2:a0"}},
	}
	gotJSON := stickyTopologyJSON(cur, true, mustJSON(prev))

	var got NetTopology
	if err := json.Unmarshal([]byte(gotJSON), &got); err != nil {
		t.Fatal(err)
	}
	if !got.LastFullSuccessAt.Equal(lastSuccess) {
		t.Fatalf("last full success should be preserved, got %s", got.LastFullSuccessAt.Format(time.RFC3339))
	}
}

func TestStickyPlatformJSONKeepsPreviousStaticSuccessAt(t *testing.T) {
	lastSuccess := time.Date(2026, 6, 12, 10, 0, 0, 0, time.Local)
	prev := PlatformInfo{Vendor: "Intel", Product: "NUC", StaticLastFullSuccessAt: lastSuccess}
	cur := PlatformInfo{Vendor: "Intel", Product: "NUC"}
	gotJSON := stickyPlatformJSON(cur, true, mustJSON(prev))

	var got PlatformInfo
	if err := json.Unmarshal([]byte(gotJSON), &got); err != nil {
		t.Fatal(err)
	}
	if !got.StaticLastFullSuccessAt.Equal(lastSuccess) {
		t.Fatalf("static success time should be preserved, got %s", got.StaticLastFullSuccessAt.Format(time.RFC3339))
	}
}

func TestStickyUSBJSONKeepsPreviousVMOwnedWhenUnknown(t *testing.T) {
	lastSuccess := time.Date(2026, 6, 12, 10, 0, 0, 0, time.Local)
	prev := USBState{
		VMOwned: []USBVMOwned{{
			VMID:   101,
			VMName: "fnOS",
			Label:  "USB device",
		}},
		VMOwnedLastFullSuccessAt: lastSuccess,
	}
	cur := USBState{
		ArbitratorRunning: true,
		arbitratorKnown:   true,
	}
	gotJSON := stickyUSBJSON(cur, mustJSON(prev))

	var got USBState
	if err := json.Unmarshal([]byte(gotJSON), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.VMOwned) != 1 || got.VMOwned[0].VMName != "fnOS" {
		t.Fatalf("previous vm owned should be preserved, got %#v", got.VMOwned)
	}
	if !got.VMOwnedLastFullSuccessAt.Equal(lastSuccess) {
		t.Fatalf("vm owned success time should be preserved, got %s", got.VMOwnedLastFullSuccessAt.Format(time.RFC3339))
	}
}

func TestSlowRefreshDueUsesLastFullSuccessAt(t *testing.T) {
	now := time.Date(2026, 6, 12, 11, 0, 0, 0, time.Local)
	interval := 30 * time.Minute
	if slowRefreshDue(time.Time{}, now, interval) != true {
		t.Fatal("empty success time should force slow collection")
	}
	if slowRefreshDue(now.Add(-29*time.Minute), now, interval) {
		t.Fatal("slow collection should be skipped before refresh interval")
	}
	if !slowRefreshDue(now.Add(-31*time.Minute), now, interval) {
		t.Fatal("slow collection should run after refresh interval")
	}
}

func TestCollectOptionsSkipsSlowModulesIndependently(t *testing.T) {
	now := time.Date(2026, 6, 12, 11, 0, 0, 0, time.Local)
	recent := now.Add(-10 * time.Minute)
	old := now.Add(-31 * time.Minute)
	prev := model.EsxiState{
		PlatformJSON:  mustJSON(PlatformInfo{Vendor: "Intel", StaticLastFullSuccessAt: recent}),
		CPUStaticJSON: mustJSON(CPUStatic{Brand: "Xeon", Cores: 8}),
		MemoryJSON:    mustJSON(MemoryInfo{TotalBytes: 1024}),
		VMJSON:        mustJSON([]VM{{ID: 101, Name: "fnOS", State: "powered_on", PowerStateLastFullSuccessAt: recent}}),
		USBJSON:       mustJSON(USBState{VMOwnedLastFullSuccessAt: recent}),
		DiskJSON:      mustJSON([]DiskHealth{{Device: "naa.disk", TempC: 35, SMARTLastFullSuccessAt: old}}),
		NICJSON:       mustJSON([]NIC{{Name: "vmnic0", StatsLastFullSuccessAt: recent}}),
		TopologyJSON:  mustJSON(NetTopology{LastFullSuccessAt: recent}),
	}

	opts := collectOptions(&prev, now, 30*time.Minute)
	if !opts.SkipStatic || !opts.SkipVMPower || !opts.SkipUSBVMOwned || !opts.SkipNICStats || !opts.SkipTopology {
		t.Fatalf("recent slow modules should be skipped: %+v", opts)
	}
	if opts.SkipDiskSMART {
		t.Fatal("old disk smart timestamp should force disk SMART collection")
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
