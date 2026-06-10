package esximon

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/LemonZuo/homer/internal/logx"
	"github.com/LemonZuo/homer/internal/model"
	"github.com/LemonZuo/homer/internal/notify"
)

type AlertConfig struct {
	CPUTempC           int
	CPUUsagePercent    int
	MemoryUsagePercent int
	DiskTempC          int
	DiskUsagePercent   int
}

func (c AlertConfig) withDefaults() AlertConfig {
	if c.CPUTempC <= 0 {
		c.CPUTempC = 85
	}
	if c.CPUUsagePercent <= 0 {
		c.CPUUsagePercent = 90
	}
	if c.MemoryUsagePercent <= 0 {
		c.MemoryUsagePercent = 90
	}
	if c.DiskTempC <= 0 {
		c.DiskTempC = 55
	}
	if c.DiskUsagePercent <= 0 {
		c.DiskUsagePercent = 90
	}
	return c
}

type thresholdAlertItem struct {
	Key       string
	Metric    string
	Target    string
	Value     string
	Threshold string
}

func (s *Service) processThresholdAlerts(prev *model.EsxiState, cur HostResult) {
	if s == nil || !cur.OK {
		return
	}
	cfg := s.alertCfg.withDefaults()
	currItems := thresholdAlertItems(cur.Metrics, cfg)
	if len(currItems) == 0 {
		return
	}
	prevItems := thresholdAlertItems(metricsFromState(prev), cfg)
	prevSet := map[string]struct{}{}
	for _, item := range prevItems {
		prevSet[item.Key] = struct{}{}
	}

	newItems := make([]thresholdAlertItem, 0, len(currItems))
	for _, item := range currItems {
		if _, ok := prevSet[item.Key]; !ok {
			newItems = append(newItems, item)
		}
	}
	if len(newItems) == 0 {
		if prev == nil || prev.LastAlertAt != nil {
			return
		}
		// 功能首次上线时,上一轮 state 可能已经超阈值但从未发送过 ESXi 阈值告警。
		// last_alert_at 为空时补发一次当前活跃告警,之后同一状态不再重复。
		newItems = currItems
	}
	sort.Slice(newItems, func(i, j int) bool { return newItems[i].Key < newItems[j].Key })

	if s.alertOut == nil || !s.alertOut.Enabled() {
		logx.Warn("esxi threshold alert skipped: no channel",
			"host", cur.HostName, "host_id", cur.HostID, "items", len(newItems))
		return
	}

	title, body := composeThresholdAlert(cur, newItems)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := s.alertOut.Send(ctx, notify.Message{Title: title, Text: body}); err != nil {
		logx.Error("esxi threshold alert send failed", "host", cur.HostName, "host_id", cur.HostID, "err", err)
		return
	}
	logx.Warn("esxi threshold alert sent", "host", cur.HostName, "host_id", cur.HostID, "items", len(newItems))
	_ = s.store.MarkAlerted(cur.HostKind, cur.HostID, time.Now())
}

func composeThresholdAlert(cur HostResult, items []thresholdAlertItem) (string, string) {
	var b strings.Builder
	b.WriteString("机器:")
	b.WriteString(cur.HostName)
	if cur.Endpoint != "" {
		b.WriteString("\n地址:")
		b.WriteString(cur.Endpoint)
	}
	b.WriteString("\n新增超阈值:")
	for _, item := range items {
		b.WriteString("\n- ")
		b.WriteString(item.Metric)
		if item.Target != "" {
			b.WriteString("(")
			b.WriteString(item.Target)
			b.WriteString(")")
		}
		b.WriteString(": ")
		b.WriteString(item.Value)
		b.WriteString(" >= ")
		b.WriteString(item.Threshold)
	}
	return "ESXi 阈值告警", b.String()
}

func metricsFromState(st *model.EsxiState) HostMetrics {
	m := HostMetrics{
		CPUTemp: CPUTemperature{TjMaxC: -1, MaxC: -1, AvgC: -1},
		Runtime: RuntimeUsage{CPUUsagePercent: -1, MemoryUsagePercent: -1},
		CPU:     CPUStatic{TjMaxC: -1},
	}
	if st == nil {
		return m
	}
	if st.CPUStaticJSON != "" {
		_ = json.Unmarshal([]byte(st.CPUStaticJSON), &m.CPU)
	}
	if st.MemoryJSON != "" {
		_ = json.Unmarshal([]byte(st.MemoryJSON), &m.Memory)
	}
	if st.RuntimeJSON != "" {
		_ = json.Unmarshal([]byte(st.RuntimeJSON), &m.Runtime)
	}
	if st.CPUTempJSON != "" {
		_ = json.Unmarshal([]byte(st.CPUTempJSON), &m.CPUTemp)
	}
	if st.DiskJSON != "" {
		_ = json.Unmarshal([]byte(st.DiskJSON), &m.Disks)
	}
	return m
}

func thresholdAlertItems(m HostMetrics, cfg AlertConfig) []thresholdAlertItem {
	cfg = cfg.withDefaults()
	var items []thresholdAlertItem
	if m.CPUTemp.MaxC >= cfg.CPUTempC {
		items = append(items, thresholdAlertItem{
			Key:       "cpu_temp",
			Metric:    "CPU 温度",
			Value:     fmt.Sprintf("%d°C", m.CPUTemp.MaxC),
			Threshold: fmt.Sprintf("%d°C", cfg.CPUTempC),
		})
	}
	if cpuPct := cpuUsagePercent(m); cpuPct >= cfg.CPUUsagePercent {
		items = append(items, thresholdAlertItem{
			Key:       "cpu_usage",
			Metric:    "CPU 使用率",
			Value:     fmt.Sprintf("%d%%", cpuPct),
			Threshold: fmt.Sprintf("%d%%", cfg.CPUUsagePercent),
		})
	}
	if memPct := memoryUsagePercent(m); memPct >= cfg.MemoryUsagePercent {
		items = append(items, thresholdAlertItem{
			Key:       "memory_usage",
			Metric:    "内存使用率",
			Value:     fmt.Sprintf("%d%%", memPct),
			Threshold: fmt.Sprintf("%d%%", cfg.MemoryUsagePercent),
		})
	}
	for _, d := range m.Disks {
		name := diskAlertName(d)
		if d.TempC >= cfg.DiskTempC {
			items = append(items, thresholdAlertItem{
				Key:       "disk_temp:" + d.Device,
				Metric:    "磁盘温度",
				Target:    name,
				Value:     fmt.Sprintf("%d°C", d.TempC),
				Threshold: fmt.Sprintf("%d°C", cfg.DiskTempC),
			})
		}
		if pct := diskUsagePercent(d); pct >= cfg.DiskUsagePercent {
			items = append(items, thresholdAlertItem{
				Key:       "disk_usage:" + d.Device,
				Metric:    "磁盘使用率",
				Target:    name,
				Value:     fmt.Sprintf("%d%%", pct),
				Threshold: fmt.Sprintf("%d%%", cfg.DiskUsagePercent),
			})
		}
	}
	return items
}

func cpuUsagePercent(m HostMetrics) int {
	return m.Runtime.CPUUsagePercent
}

func memoryUsagePercent(m HostMetrics) int {
	if m.Runtime.MemoryUsagePercent >= 0 {
		return m.Runtime.MemoryUsagePercent
	}
	if m.Memory.TotalBytes > 0 && m.Memory.FreeBytes >= 0 {
		used := m.Memory.TotalBytes - m.Memory.FreeBytes
		if used < 0 {
			used = 0
		}
		return percentInt(used, m.Memory.TotalBytes)
	}
	return -1
}

func diskUsagePercent(d DiskTemperature) int {
	if d.UsedBytes < 0 {
		return -1
	}
	total := d.CapacityBytes
	if total <= 0 && d.FreeBytes >= 0 {
		total = d.UsedBytes + d.FreeBytes
	}
	return percentInt(d.UsedBytes, total)
}

func diskAlertName(d DiskTemperature) string {
	switch {
	case d.Model != "":
		return d.Model
	case len(d.Datastores) > 0:
		return strings.Join(d.Datastores, ",")
	case d.Device != "":
		return shortDeviceID(d.Device)
	default:
		return "unknown"
	}
}

func shortDeviceID(s string) string {
	if len(s) <= 18 {
		return s
	}
	return s[:8] + "..." + s[len(s)-7:]
}
