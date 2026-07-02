package esximon

import "testing"

func TestSplitSMARTOutput(t *testing.T) {
	out := "===DEV===t10.ATA_____Samsung_SSD_870\nHealth Status: OK\nDrive Temperature: 35\n===DEV===t10.NVMe____990PRO\r\nDrive Temperature: 47\n"
	m := splitSMARTOutput(out)
	if len(m) != 2 {
		t.Fatalf("len = %d: %v", len(m), m)
	}
	if m["t10.ATA_____Samsung_SSD_870"] != "Health Status: OK\nDrive Temperature: 35\n" {
		t.Fatalf("first block = %q", m["t10.ATA_____Samsung_SSD_870"])
	}
	// \r 结尾的标记行要被去掉
	if _, ok := m["t10.NVMe____990PRO"]; !ok {
		t.Fatalf("crlf marker not trimmed: %v", m)
	}
	// 标记之前的内容丢弃
	m2 := splitSMARTOutput("garbage before\n===DEV===id1\ndata")
	if len(m2) != 1 || m2["id1"] != "data\n" {
		t.Fatalf("m2 = %v", m2)
	}
	// 空输入
	if len(splitSMARTOutput("")) != 0 {
		t.Fatal("empty input must yield empty map")
	}
}

func TestClassifyDisk(t *testing.T) {
	cases := []struct {
		devType string
		temp    int
		want    string
	}{
		{"NVMe", -1, "unknown"},
		{"NVMe", 69, "ok"},
		{"NVMe", 70, "warning"},
		{"NVMe", 80, "critical"},
		{"SATA-SSD", 59, "ok"},
		{"SATA-SSD", 60, "warning"},
		{"SATA-SSD", 70, "critical"},
		{"SATA-HDD", 49, "ok"},
		{"SATA-HDD", 50, "warning"},
		{"SATA-HDD", 55, "critical"},
		{"unknown", 99, "ok"}, // 未知类型无阈值表
	}
	for _, c := range cases {
		if got := classifyDisk(c.devType, c.temp); got != c.want {
			t.Errorf("classifyDisk(%q, %d) = %q, want %q", c.devType, c.temp, got, c.want)
		}
	}
}

func TestParseESXiDeviceSize(t *testing.T) {
	// 裸数字单位是 MiB;带单位交给 parseBytes
	cases := []struct {
		in   string
		want int64
	}{
		{"953869", 953869 << 20},
		{"500 GB", 500 << 30},
		{"", 0},
		{"n/a", 0},
	}
	for _, c := range cases {
		if got := parseESXiDeviceSize(c.in); got != c.want {
			t.Errorf("parseESXiDeviceSize(%q) = %d, want %d", c.in, got, c.want)
		}
	}
}

func TestDiskHealthByDevice(t *testing.T) {
	m := diskHealthByDevice([]DiskHealth{
		{Device: "t10.a", TempC: 35},
		{Device: "", TempC: 99}, // 无 device 跳过
		{Device: "t10.b", TempC: 47},
	})
	if len(m) != 2 || m["t10.a"].TempC != 35 || m["t10.b"].TempC != 47 {
		t.Fatalf("m = %v", m)
	}
}

func TestDiskSMARTComplete(t *testing.T) {
	// SMART 输出是 5 列表格(Parameter/Value/Threshold/Worst/Raw)
	good := "Drive Temperature    35    70    35    35"
	if diskSMARTComplete(nil, nil) {
		t.Fatal("no ids must be incomplete")
	}
	if diskSMARTComplete([]string{"a"}, map[string]string{"a": ""}) {
		t.Fatal("empty smart must be incomplete")
	}
	if !diskSMARTComplete([]string{"a"}, map[string]string{"a": good}) {
		t.Fatal("temp present must be complete")
	}
	if diskSMARTComplete([]string{"a", "b"}, map[string]string{"a": good}) {
		t.Fatal("missing one disk must be incomplete")
	}
}

func TestFillMissingDiskSMARTFromPrev(t *testing.T) {
	prev := DiskHealth{
		TempC: 35, ThresholdC: 70, Status: "ok", HealthStatus: "OK",
		PowerOnHours: 100, PowerCycleCount: 50, ReallocatedSectors: 0,
		UncorrectableErrors: 0, MediaWearoutValue: 99, ReadErrorCount: 0,
		PendingSectorReallocation: 0,
	}
	// 全缺失 → 全部回填
	d := DiskHealth{
		TempC: -1, ThresholdC: -1, Status: "unknown", HealthStatus: "",
		PowerOnHours: -1, PowerCycleCount: -1, ReallocatedSectors: -1,
		UncorrectableErrors: -1, MediaWearoutValue: -1, ReadErrorCount: -1,
		PendingSectorReallocation: -1,
	}
	fillMissingDiskSMARTFromPrev(&d, prev)
	if d.TempC != 35 || d.Status != "ok" || d.PowerOnHours != 100 || d.MediaWearoutValue != 99 {
		t.Fatalf("filled = %+v", d)
	}
	// 已有新值 → 不被旧值覆盖
	d2 := DiskHealth{TempC: 40, ThresholdC: 80, Status: "warning", HealthStatus: "OK",
		PowerOnHours: 101, PowerCycleCount: 51, ReallocatedSectors: 1,
		UncorrectableErrors: 1, MediaWearoutValue: 98, ReadErrorCount: 1,
		PendingSectorReallocation: 1}
	fillMissingDiskSMARTFromPrev(&d2, prev)
	if d2.TempC != 40 || d2.Status != "warning" || d2.PowerOnHours != 101 {
		t.Fatalf("overwritten = %+v", d2)
	}
}

func TestAppendUniqueString(t *testing.T) {
	list := appendUniqueString(nil, "a")
	list = appendUniqueString(list, "a") // 重复不加
	list = appendUniqueString(list, "")  // 空串不加
	list = appendUniqueString(list, "b")
	if len(list) != 2 || list[0] != "a" || list[1] != "b" {
		t.Fatalf("list = %v", list)
	}
}
