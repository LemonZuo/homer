package esximon

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/LemonZuo/homer/internal/logx"
	"github.com/LemonZuo/homer/internal/model"
	"github.com/LemonZuo/homer/internal/notify"
	"gorm.io/gorm"
)

// Service 串起一轮"扫描 → 采集 → 持久化 → SSE 广播"。
type Service struct {
	db        *gorm.DB
	sampler   *Sampler
	store     *Store
	hosts     *HostStore
	alertOut  notify.Notifier
	alertCfg  AlertConfig
	sse       *sseHub
	retention time.Duration

	// 单机可达性状态机:host_id -> 上次是否可达。首次见到的 host 仅记录,不告警,
	// 防止刚启动 / 刚加机器时刷一片"已离线/已恢复"。
	reachMu   sync.Mutex
	hostReach map[int64]bool
}

func NewService(db *gorm.DB, sampler *Sampler, store *Store, hosts *HostStore, alertOut notify.Notifier, alertCfg AlertConfig, retention time.Duration) *Service {
	if retention <= 0 {
		retention = 7 * 24 * time.Hour
	}
	return &Service{
		db:        db,
		sampler:   sampler,
		store:     store,
		hosts:     hosts,
		alertOut:  alertOut,
		alertCfg:  alertCfg.withDefaults(),
		sse:       newSSEHub(),
		retention: retention,
		hostReach: map[int64]bool{},
	}
}

// Subscribe 给 SSE handler 用:订阅采集完成后的 snapshot 广播。
func (s *Service) Subscribe() (<-chan []Snapshot, func()) { return s.sse.Subscribe() }

// RunSample cron 调用入口。整轮失败返回 error,单机失败封进 esxi_state.LastError。
func (s *Service) RunSample() error {
	results, err := s.sampler.Run()
	if err != nil {
		return err
	}
	if len(results) == 0 {
		return nil
	}

	// 先读一次上一轮的 state,后面 buildState 用来兜底单项失败的 JSON 列。
	// 单台 ESXi 一行 state,n 很小,这里直接全表扫即可。
	prevStates, _ := s.store.LoadStates()
	prevByHost := map[string]model.EsxiState{}
	for _, st := range prevStates {
		prevByHost[fmt.Sprintf("%s/%d", st.HostKind, st.HostID)] = st
	}

	now := time.Now()
	var samples []model.EsxiSample
	var states []model.EsxiState
	type alertSeed struct {
		prev *model.EsxiState
		cur  HostResult
	}
	var alerts []alertSeed
	reachableHosts := 0
	for _, r := range results {
		if r.OK {
			reachableHosts++
		}
		missing := sampleMissingMetrics(r.Metrics)
		sampleOK := r.OK && len(missing) == 0
		if r.OK && !sampleOK {
			r.Error = "采集不完整:" + strings.Join(missing, ",")
			logx.Warn("esxi sample skipped: incomplete metrics",
				"host", r.HostName, "host_id", r.HostID, "missing", strings.Join(missing, ","))
		}
		var prev *model.EsxiState
		if p, ok := prevByHost[fmt.Sprintf("%s/%d", r.HostKind, r.HostID)]; ok {
			prev = &p
		}
		st := buildState(r, prev, now, sampleOK)
		states = append(states, st)
		if sampleOK {
			samples = append(samples, buildSample(r, now))
		}
		alerts = append(alerts, alertSeed{prev: prev, cur: r})
	}

	if err := s.store.SaveSamples(samples); err != nil {
		return fmt.Errorf("写入 esxi_sample 失败:%w", err)
	}
	if err := s.store.UpsertState(states); err != nil {
		return fmt.Errorf("更新 esxi_state 失败:%w", err)
	}

	s.handleHostReachAlerts(results)
	for _, a := range alerts {
		s.processThresholdAlerts(a.prev, a.cur)
	}

	logx.Info("esxi sample done",
		"hosts", len(results), "reachable", reachableHosts, "samples", len(samples))

	if snap, err := s.BuildSnapshot(); err == nil {
		s.sse.Publish(snap)
	} else {
		logx.Warn("esxi snapshot publish skipped", "err", err.Error())
	}
	return nil
}

// RunCleanup 清理过期 sample。
func (s *Service) RunCleanup() error {
	cutoff := time.Now().Add(-s.retention)
	n, err := s.store.PurgeOlderThan(cutoff)
	if err != nil {
		return err
	}
	if n > 0 {
		logx.Info("esxi sample purged", "cutoff", cutoff.Format(time.RFC3339), "rows", n)
	}
	return nil
}

// TriggerSample 手动触发(refresh 按钮)。
func (s *Service) TriggerSample() error { return s.RunSample() }

// --- Snapshot 接口 ---

// ErrNoHosts 表示没有候选机器(handler 区分"未配机器"与"配了但全坏")。
var ErrNoHosts = errors.New("没有可监控的机器(请在「ESXi 机器」里添加要采集的主机)")
