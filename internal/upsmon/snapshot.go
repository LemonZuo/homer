package upsmon

// UPS 快照接口组装。

import (
	"fmt"
	"time"

	"github.com/LemonZuo/homer/internal/model"
)

// Snapshot 接口数据:每台机器 + 其下 UPS 当前状态。
type Snapshot struct {
	HostKind  string        `json:"host_kind"`
	HostID    int64         `json:"host_id"`
	HostName  string        `json:"host_name"`
	Endpoint  string        `json:"endpoint"`
	Reachable bool          `json:"reachable"`
	Error     string        `json:"error,omitempty"`
	UPSes     []SnapshotUPS `json:"upses"`
}

type SnapshotUPS struct {
	Name                  string    `json:"name"`
	Mfr                   string    `json:"mfr"`
	Model                 string    `json:"model"`
	PowerSource           string    `json:"power_source"`
	BatteryPercent        int       `json:"battery_percent"`
	RuntimeMinutes        int       `json:"runtime_minutes"`
	BatteryVoltage        float32   `json:"battery_voltage"`
	BatteryNominalVoltage float32   `json:"battery_nominal_voltage"`
	BatteryType           string    `json:"battery_type"`
	InputVoltage          float32   `json:"input_voltage"`
	OutputVoltage         float32   `json:"output_voltage"`
	LoadPercent           int       `json:"load_percent"`
	RealPower             int       `json:"real_power"`
	RawStatus             string    `json:"raw_status"`
	SampledAt             time.Time `json:"sampled_at"`
	// EnergyTodayWh 当日累计耗电(0 点至今,Wh,矩形积分)。-1 表示无法估算。
	EnergyTodayWh int `json:"energy_today_wh"`
}

// BuildSnapshot 不重新采样,基于 ups_state(最新)+ ups_host 表组装快照。
// 离线机器走 ups_sample 没法定位,直接读 ups_host 表保证 endpoint/name 总是有值。
func (s *Service) BuildSnapshot() ([]Snapshot, error) {
	targets, err := s.sampler.hosts.ListEnabled()
	if err != nil {
		return nil, err
	}
	if len(targets) == 0 {
		return nil, nil
	}
	states, err := s.store.LoadStates()
	if err != nil {
		return nil, err
	}
	stateByHost := map[string][]model.UPSState{}
	for _, st := range states {
		k := hostKey(st.HostKind, st.HostID)
		stateByHost[k] = append(stateByHost[k], st)
	}
	out := make([]Snapshot, 0, len(targets))
	for _, t := range targets {
		k := hostKey(model.UPSHostKind, t.ID)
		sn := Snapshot{
			HostKind: model.UPSHostKind,
			HostID:   t.ID,
			HostName: t.Name,
			Endpoint: t.Endpoint,
			UPSes:    []SnapshotUPS{},
		}
		// 今日累计 kWh:从本地当日 0 点起的矩形积分。失败回退 -1,前端隐藏该字段。
		// 注意:time.Truncate(24h) 在 UTC 维度截断,在东八区会落到当日 8 点;
		// 必须按 Local 的 Year/Month/Day 显式构造,才是"本地今日 0 点"。
		nowLocal := time.Now().Local()
		startOfDay := time.Date(nowLocal.Year(), nowLocal.Month(), nowLocal.Day(), 0, 0, 0, 0, nowLocal.Location())
		for _, st := range stateByHost[k] {
			energyWh := -1
			if kwh, samples, _, err := s.store.EnergyAccumulated(st.HostKind, st.HostID, st.UPSName, startOfDay, 300); err == nil && samples > 0 {
				energyWh = int(kwh*1000 + 0.5)
			}
			sn.UPSes = append(sn.UPSes, SnapshotUPS{
				Name:                  st.UPSName,
				Mfr:                   st.Mfr,
				Model:                 st.Model,
				PowerSource:           st.LastPowerSource,
				BatteryPercent:        st.LastBatteryPercent,
				RuntimeMinutes:        st.LastRuntimeMinutes,
				BatteryVoltage:        st.LastBatteryVoltage,
				BatteryNominalVoltage: st.LastBatteryNominalVoltage,
				BatteryType:           st.LastBatteryType,
				InputVoltage:          st.LastInputVoltage,
				OutputVoltage:         st.LastOutputVoltage,
				LoadPercent:           st.LastLoadPercent,
				RealPower:             st.LastRealPower,
				RawStatus:             st.LastRawStatus,
				SampledAt:             st.UpdatedAt,
				EnergyTodayWh:         energyWh,
			})
			sn.Reachable = true
		}
		out = append(out, sn)
	}
	return out, nil
}

func indexStates(states []model.UPSState) map[string]model.UPSState {
	m := make(map[string]model.UPSState, len(states))
	for _, st := range states {
		m[stateKey(st.HostKind, st.HostID, st.UPSName)] = st
	}
	return m
}

func stateKey(hostKind string, hostID int64, upsName string) string {
	return fmt.Sprintf("%s/%d/%s", hostKind, hostID, upsName)
}

func hostKey(hostKind string, hostID int64) string {
	return fmt.Sprintf("%s/%d", hostKind, hostID)
}
