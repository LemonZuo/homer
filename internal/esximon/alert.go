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
	ConsecutiveSamples int
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
	if c.ConsecutiveSamples <= 0 {
		c.ConsecutiveSamples = 5
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
	if s == nil {
		return
	}
	cfg := s.alertCfg.withDefaults()
	if !cur.OK {
		s.persistThresholdAlertState(cur.HostKind, cur.HostID, thresholdAlertState{})
		return
	}

	currItems := thresholdAlertItems(cur.Metrics, cfg)
	prevState := parseThresholdAlertState(prev)
	nextState, dueItems := advanceThresholdAlertState(prevState, currItems, cfg.ConsecutiveSamples)
	if len(dueItems) == 0 {
		s.persistThresholdAlertState(cur.HostKind, cur.HostID, nextState)
		return
	}
	sort.Slice(dueItems, func(i, j int) bool { return dueItems[i].Key < dueItems[j].Key })

	if s.alertOut == nil || !s.alertOut.Enabled() {
		logx.Warn("esxi threshold alert skipped: no channel",
			"host", cur.HostName, "host_id", cur.HostID, "items", len(dueItems))
		s.persistThresholdAlertState(cur.HostKind, cur.HostID, nextState)
		return
	}

	title, body := composeThresholdAlert(cur, dueItems, cfg.ConsecutiveSamples)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := s.alertOut.Send(ctx, notify.Message{Title: title, Text: body}); err != nil {
		logx.Error("esxi threshold alert send failed", "host", cur.HostName, "host_id", cur.HostID, "err", err)
		s.persistThresholdAlertState(cur.HostKind, cur.HostID, nextState)
		return
	}
	logx.Warn("esxi threshold alert sent", "host", cur.HostName, "host_id", cur.HostID, "items", len(dueItems))
	for _, item := range dueItems {
		record := nextState.Items[item.Key]
		record.Notified = true
		nextState.Items[item.Key] = record
	}
	s.persistThresholdAlertState(cur.HostKind, cur.HostID, nextState, time.Now())
}

func composeThresholdAlert(cur HostResult, items []thresholdAlertItem, consecutiveSamples int) (string, string) {
	var b strings.Builder
	b.WriteString("机器:")
	b.WriteString(cur.HostName)
	if cur.Endpoint != "" {
		b.WriteString("\n地址:")
		b.WriteString(cur.Endpoint)
	}
	b.WriteString("\n连续超阈值:")
	b.WriteString(fmt.Sprintf("%d 次", consecutiveSamples))
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

type thresholdAlertState struct {
	Items map[string]thresholdAlertRecord `json:"items,omitempty"`
}

type thresholdAlertRecord struct {
	Count    int  `json:"count"`
	Notified bool `json:"notified"`
}

func parseThresholdAlertState(st *model.EsxiState) thresholdAlertState {
	if st == nil || st.AlertStateJSON == "" {
		return thresholdAlertState{}
	}
	var state thresholdAlertState
	if json.Unmarshal([]byte(st.AlertStateJSON), &state) != nil {
		return thresholdAlertState{}
	}
	if len(state.Items) == 0 {
		return thresholdAlertState{}
	}
	return state
}

func advanceThresholdAlertState(prev thresholdAlertState, curr []thresholdAlertItem, consecutiveSamples int) (thresholdAlertState, []thresholdAlertItem) {
	if consecutiveSamples <= 0 {
		consecutiveSamples = 5
	}
	if len(curr) == 0 {
		return thresholdAlertState{}, nil
	}
	next := thresholdAlertState{Items: make(map[string]thresholdAlertRecord, len(curr))}
	due := make([]thresholdAlertItem, 0)
	for _, item := range curr {
		record := thresholdAlertRecord{}
		if prev.Items != nil {
			record = prev.Items[item.Key]
		}
		record.Count++
		if record.Count >= consecutiveSamples && !record.Notified {
			due = append(due, item)
		}
		next.Items[item.Key] = record
	}
	return next, due
}

func (s *Service) persistThresholdAlertState(hostKind string, hostID int64, state thresholdAlertState, alertedAt ...time.Time) {
	if s == nil || s.store == nil || hostKind == "" || hostID <= 0 {
		return
	}
	stateJSON := ""
	if len(state.Items) > 0 {
		stateJSON = mustJSON(state)
	}
	var at *time.Time
	if len(alertedAt) > 0 {
		t := alertedAt[0]
		at = &t
	}
	if err := s.store.UpdateAlertState(hostKind, hostID, stateJSON, at); err != nil {
		logx.Warn("esxi threshold alert state update failed", "host_id", hostID, "err", err.Error())
	}
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

func diskUsagePercent(d DiskHealth) int {
	if d.UsedBytes < 0 {
		return -1
	}
	total := d.CapacityBytes
	if total <= 0 && d.FreeBytes >= 0 {
		total = d.UsedBytes + d.FreeBytes
	}
	return percentInt(d.UsedBytes, total)
}

func diskAlertName(d DiskHealth) string {
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
