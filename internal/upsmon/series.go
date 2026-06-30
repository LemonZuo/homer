package upsmon

// UPS 历史曲线查询。

import (
	"time"
)

// Series 暴露给 handler 的曲线接口。range 是允许的人类时间窗,bucket 由它推导。
func (s *Service) Series(hostKind string, hostID int64, upsName string, window time.Duration) ([]SeriesPoint, error) {
	if window <= 0 {
		window = 24 * time.Hour
	}
	bucket := pickBucket(window)
	since := time.Now().Add(-window)
	return s.store.Series(hostKind, hostID, upsName, since, bucket)
}

// EnergySummary 当前窗口的能耗汇总:累计 kWh、覆盖秒数、平均功率(W)。
// 平均功率按"实际有采样覆盖的时间"计,避免被掉机断档稀释成虚低值。
type EnergySummary struct {
	EnergyWh    int `json:"energy_wh"`
	AvgPowerW   int `json:"avg_power_w"`
	CoveredSec  int `json:"covered_sec"`
	WindowSec   int `json:"window_sec"`
	SampleCount int `json:"sample_count"`
}

// EnergyWindow 返回 window 内的能耗汇总。无采样时返回零值 + nil error。
func (s *Service) EnergyWindow(hostKind string, hostID int64, upsName string, window time.Duration) (EnergySummary, error) {
	if window <= 0 {
		window = 24 * time.Hour
	}
	since := time.Now().Add(-window)
	kwh, samples, coveredSec, err := s.store.EnergyAccumulated(hostKind, hostID, upsName, since, 300)
	if err != nil {
		return EnergySummary{}, err
	}
	sum := EnergySummary{
		EnergyWh:    int(kwh*1000 + 0.5),
		CoveredSec:  coveredSec,
		WindowSec:   int(window / time.Second),
		SampleCount: samples,
	}
	if coveredSec > 0 {
		sum.AvgPowerW = int(kwh*1000*3600/float64(coveredSec) + 0.5)
	}
	return sum, nil
}

// pickBucket 按窗口大小返回桶宽(秒)。控制画图点数 ≈ 300。
func pickBucket(window time.Duration) int {
	switch {
	case window <= 6*time.Hour:
		return 60 // 1min
	case window <= 24*time.Hour:
		return 5 * 60 // 5min
	case window <= 3*24*time.Hour:
		return 15 * 60 // 15min
	default:
		return 30 * 60 // 30min
	}
}
