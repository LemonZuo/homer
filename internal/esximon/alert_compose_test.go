package esximon

import (
	"strings"
	"testing"

	"github.com/LemonZuo/homer/internal/model"
)

func TestComposeThresholdAlert(t *testing.T) {
	cur := HostResult{HostName: "esxi01", Endpoint: "192.168.8.138:22"}
	items := []thresholdAlertItem{
		{Key: "cpu_temp", Metric: "CPU 温度", Value: "85°C", Threshold: "80°C"},
		{Key: "disk_temp/t10.a", Metric: "磁盘温度", Target: "990 PRO", Value: "72°C", Threshold: "70°C"},
	}
	title, body := composeThresholdAlert(cur, items, 3)
	if title != "ESXi 阈值告警" {
		t.Fatalf("title = %q", title)
	}
	for _, want := range []string{"机器:esxi01", "地址:192.168.8.138:22", "连续超阈值:3 次", "CPU 温度: 85°C >= 80°C", "磁盘温度(990 PRO): 72°C >= 70°C"} {
		if !strings.Contains(body, want) {
			t.Fatalf("body missing %q:\n%s", want, body)
		}
	}
	// 无 Endpoint 时不渲染地址行
	_, body2 := composeThresholdAlert(HostResult{HostName: "h"}, items[:1], 1)
	if strings.Contains(body2, "地址") {
		t.Fatalf("empty endpoint must omit address line:\n%s", body2)
	}
}

func TestParseThresholdAlertState(t *testing.T) {
	// nil / 空串 / 非法 JSON / 空 items → 全部零值
	for _, st := range []*model.EsxiState{
		nil,
		{},
		{AlertStateJSON: "{bad json"},
		{AlertStateJSON: `{"items":{}}`},
	} {
		if got := parseThresholdAlertState(st); len(got.Items) != 0 {
			t.Fatalf("expected empty state for %+v, got %+v", st, got)
		}
	}
	st := &model.EsxiState{AlertStateJSON: `{"items":{"cpu_temp":{"count":2,"notified":true}}}`}
	got := parseThresholdAlertState(st)
	if r, ok := got.Items["cpu_temp"]; !ok || r.Count != 2 || !r.Notified {
		t.Fatalf("state = %+v", got)
	}
}

func TestShortDeviceID(t *testing.T) {
	if got := shortDeviceID("t10.short"); got != "t10.short" {
		t.Fatalf("short id = %q", got)
	}
	long := "t10.NVMe____Samsung_SSD_990_PRO_1TB_________0025384541408657"
	got := shortDeviceID(long)
	if got != "t10.NVMe...1408657" {
		t.Fatalf("long id = %q", got)
	}
}

func TestDiskAlertName(t *testing.T) {
	if got := diskAlertName(DiskHealth{Model: "870 EVO", Device: "t10.a"}); got != "870 EVO" {
		t.Fatalf("model priority = %q", got)
	}
	if got := diskAlertName(DiskHealth{Datastores: []string{"ds1", "ds2"}}); got != "ds1,ds2" {
		t.Fatalf("datastore fallback = %q", got)
	}
	if got := diskAlertName(DiskHealth{Device: "t10.a"}); got != "t10.a" {
		t.Fatalf("device fallback = %q", got)
	}
	if got := diskAlertName(DiskHealth{}); got != "unknown" {
		t.Fatalf("empty = %q", got)
	}
}
