package upsmon

import (
	"errors"
	"fmt"
	"time"

	"github.com/LemonZuo/homer/internal/logx"
	"github.com/LemonZuo/homer/internal/model"
	"github.com/LemonZuo/homer/internal/notify"
	"gorm.io/gorm"
)

// Service 串起一轮"扫描 → 采样 → 持久化 → 告警 → SSE 广播"。
type Service struct {
	db        *gorm.DB
	sampler   *Sampler
	store     *Store
	notifier  *Notifier
	sse       *sseHub
	retention time.Duration
}

func NewService(db *gorm.DB, sampler *Sampler, store *Store, hub *notify.Hub, retention time.Duration) *Service {
	if retention <= 0 {
		retention = 7 * 24 * time.Hour
	}
	return &Service{
		db:        db,
		sampler:   sampler,
		store:     store,
		notifier:  NewNotifier(hub.For(notify.ModuleUPS), store),
		sse:       newSSEHub(),
		retention: retention,
	}
}

// Subscribe 给 SSE handler 用:订阅采样完成后的 snapshot 广播。
func (s *Service) Subscribe() (<-chan []Snapshot, func()) { return s.sse.Subscribe() }

// RunSample 是 cron 调用的入口。一轮全失败(且确实有候选机器)才回 error,
// 让 jobmonitor 记一笔失败。单机不可达不算 job 失败。
func (s *Service) RunSample() error {
	hosts, err := s.sampler.Run()
	if err != nil {
		return err
	}
	if len(hosts) == 0 {
		// 没候选机器,直接成功收工(常见于新装库 / 只用 safeline)
		return nil
	}

	// 读上一轮 state 做告警去抖比对
	prevStates, err := s.store.LoadStates()
	if err != nil {
		return fmt.Errorf("加载 ups_state 失败:%w", err)
	}
	prev := indexStates(prevStates)

	now := time.Now()
	var samples []model.UPSSample
	var states []model.UPSState
	type alertSeed struct {
		hostKind string
		hostID   int64
		ups      string
		prev     *model.UPSState
		curr     model.UPSState
	}
	var alerts []alertSeed

	reachableHosts := 0
	for _, h := range hosts {
		if h.OK {
			reachableHosts++
		}
		if !h.HasUPS {
			continue
		}
		for _, r := range h.UPSes {
			samples = append(samples, model.UPSSample{
				HostKind:       h.HostKind,
				HostID:         h.HostID,
				HostName:       h.HostName,
				UPSName:        r.Name,
				Mfr:            r.Mfr,
				Model:          r.Model,
				PowerSource:    r.PowerSource,
				BatteryPercent: r.BatteryPercent,
				RuntimeMinutes: r.RuntimeMinutes,
				InputVoltage:   r.InputVoltage,
				OutputVoltage:  r.OutputVoltage,
				LoadPercent:    r.LoadPercent,
				RealPower:      r.RealPower,
				RawStatus:      r.RawStatus,
				SampledAt:      now,
			})
			st := model.UPSState{
				HostKind:           h.HostKind,
				HostID:             h.HostID,
				HostName:           h.HostName,
				UPSName:            r.Name,
				Mfr:                r.Mfr,
				Model:              r.Model,
				LastPowerSource:    r.PowerSource,
				LastBatteryPercent: r.BatteryPercent,
				LastRuntimeMinutes: r.RuntimeMinutes,
				LastInputVoltage:   r.InputVoltage,
				LastOutputVoltage:  r.OutputVoltage,
				LastLoadPercent:    r.LoadPercent,
				LastRealPower:      r.RealPower,
				LastRawStatus:      r.RawStatus,
				UpdatedAt:          now,
			}
			// 保留上一轮的 last_alert_at(后续 notifier 决定要不要重置)
			key := stateKey(h.HostKind, h.HostID, r.Name)
			if pv, ok := prev[key]; ok {
				st.LastAlertAt = pv.LastAlertAt
			}
			states = append(states, st)
			var prevPtr *model.UPSState
			if pv, ok := prev[key]; ok {
				v := pv
				prevPtr = &v
			}
			alerts = append(alerts, alertSeed{
				hostKind: h.HostKind, hostID: h.HostID, ups: r.Name,
				prev: prevPtr, curr: st,
			})
		}
	}

	if err := s.store.SaveSamples(samples); err != nil {
		return fmt.Errorf("写入 ups_sample 失败:%w", err)
	}
	if err := s.store.UpsertState(states); err != nil {
		return fmt.Errorf("更新 ups_state 失败:%w", err)
	}

	for _, a := range alerts {
		s.notifier.Process(a.prev, a.curr)
	}

	// 整轮没有一台机器可达,判定为采样失败,触发 jobmonitor
	if reachableHosts == 0 && len(hosts) > 0 {
		var first string
		for _, h := range hosts {
			if h.Error != "" {
				first = h.Error
				break
			}
		}
		return fmt.Errorf("所有候选机器均不可达(%d 台);示例错误:%s", len(hosts), first)
	}
	// 广播最新快照给所有 SSE 订阅者。BuildSnapshot 失败只记日志,不阻断采样链路。
	if snap, err := s.BuildSnapshot(); err == nil {
		s.sse.Publish(snap)
	} else {
		logx.Warn("ups snapshot publish skipped", "err", err.Error())
	}
	logx.Info("ups sample done", "hosts", len(hosts), "reachable", reachableHosts, "samples", len(samples))
	return nil
}

// RunCleanup 清理过期 sample,返回删除行数(用于日志)。
func (s *Service) RunCleanup() error {
	cutoff := time.Now().Add(-s.retention)
	n, err := s.store.PurgeOlderThan(cutoff)
	if err != nil {
		return err
	}
	if n > 0 {
		logx.Info("ups sample purged", "cutoff", cutoff.Format(time.RFC3339), "rows", n)
	}
	return nil
}

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
	Name           string    `json:"name"`
	Mfr            string    `json:"mfr"`
	Model          string    `json:"model"`
	PowerSource    string    `json:"power_source"`
	BatteryPercent int       `json:"battery_percent"`
	RuntimeMinutes int       `json:"runtime_minutes"`
	InputVoltage   float32   `json:"input_voltage"`
	OutputVoltage  float32   `json:"output_voltage"`
	LoadPercent    int       `json:"load_percent"`
	RealPower      int       `json:"real_power"`
	RawStatus      string    `json:"raw_status"`
	SampledAt      time.Time `json:"sampled_at"`
}

// BuildSnapshot 不重新采样,基于 ups_state(最新)+ 候选机器表组装快照。
// 离线机器走 ups_sample 最旧的 host_name 没法定位,所以直接读 acme_deploy_target 表。
func (s *Service) BuildSnapshot() ([]Snapshot, error) {
	targets, err := s.sampler.listTargets()
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
		k := hostKey(t.Kind, t.ID)
		sn := Snapshot{
			HostKind: t.Kind,
			HostID:   t.ID,
			HostName: t.Name,
			Endpoint: t.Endpoint,
			UPSes:    []SnapshotUPS{},
		}
		for _, st := range stateByHost[k] {
			sn.UPSes = append(sn.UPSes, SnapshotUPS{
				Name:           st.UPSName,
				Mfr:            st.Mfr,
				Model:          st.Model,
				PowerSource:    st.LastPowerSource,
				BatteryPercent: st.LastBatteryPercent,
				RuntimeMinutes: st.LastRuntimeMinutes,
				InputVoltage:   st.LastInputVoltage,
				OutputVoltage:  st.LastOutputVoltage,
				LoadPercent:    st.LastLoadPercent,
				RealPower:      st.LastRealPower,
				RawStatus:      st.LastRawStatus,
				SampledAt:      st.UpdatedAt,
			})
			sn.Reachable = true
		}
		out = append(out, sn)
	}
	return out, nil
}

// Series 暴露给 handler 的曲线接口。range 是允许的人类时间窗,bucket 由它推导。
func (s *Service) Series(hostKind string, hostID int64, upsName string, window time.Duration) ([]SeriesPoint, error) {
	if window <= 0 {
		window = 24 * time.Hour
	}
	bucket := pickBucket(window)
	since := time.Now().Add(-window)
	return s.store.Series(hostKind, hostID, upsName, since, bucket)
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

// TriggerSample 暴露给 handler 的"立刻采一轮"接口。
// 不重试 / 不告警阈值,返回真实错误给前端。
func (s *Service) TriggerSample() error {
	if err := s.RunSample(); err != nil {
		return err
	}
	return nil
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

// ErrNoCandidates 表示没有候选机器(用于 handler 区分"没接 UPS"与"接了但全坏")。
var ErrNoCandidates = errors.New("没有可监控的机器(需要在 ACME 部署目标里添加 ssh/fnos 主机)")
