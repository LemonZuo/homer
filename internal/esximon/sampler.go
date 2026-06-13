package esximon

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/LemonZuo/homer/internal/logx"
	"github.com/LemonZuo/homer/internal/model"
	"github.com/LemonZuo/homer/internal/sshlike"
	"github.com/LemonZuo/homer/internal/sshx"
	"gorm.io/gorm"
)

// HostResult 单台机器一轮采集的结果。HostKind 恒为 model.EsxiHostKind = "esxi"。
type HostResult struct {
	HostKind string      `json:"host_kind"`
	HostID   int64       `json:"host_id"`
	HostName string      `json:"host_name"`
	Endpoint string      `json:"endpoint"`
	OK       bool        `json:"ok"`
	Error    string      `json:"error,omitempty"`
	Metrics  HostMetrics `json:"-"`
	StartAt  time.Time   `json:"-"`
}

// Sampler 负责一轮"扫所有 esxi_host → 并发 SSH → 跑 esxcli/vsish → 聚合结果"。
type Sampler struct {
	db                  *gorm.DB
	hosts               *HostStore
	credentials         sshlike.CredentialResolver
	timeout             time.Duration
	slowRefreshInterval time.Duration
	commandSlowLog      time.Duration
}

func NewSampler(db *gorm.DB, hosts *HostStore, credentials sshlike.CredentialResolver, sshTimeout, slowRefreshInterval, commandSlowLog time.Duration) *Sampler {
	if sshTimeout <= 0 {
		sshTimeout = 120 * time.Second
	}
	if slowRefreshInterval <= 0 {
		slowRefreshInterval = 30 * time.Minute
	}
	setCommandSlowLogThreshold(commandSlowLog)
	return &Sampler{db: db, hosts: hosts, credentials: credentials, timeout: sshTimeout, slowRefreshInterval: slowRefreshInterval, commandSlowLog: commandSlowLog}
}

// Run 扫一轮所有启用的 esxi_host 并发采集,单机失败不影响其他。
func (s *Sampler) Run() ([]HostResult, error) {
	targets, err := s.hosts.ListEnabled()
	if err != nil {
		return nil, fmt.Errorf("列出 ESXi 主机失败:%w", err)
	}
	if len(targets) == 0 {
		return nil, nil
	}
	prevStates := s.loadPreviousStatesByHostID()

	results := make([]HostResult, len(targets))
	var wg sync.WaitGroup
	for i := range targets {
		wg.Add(1)
		go func(idx int, h model.EsxiHost) {
			defer wg.Done()
			var prev *model.EsxiState
			if p, ok := prevStates[h.ID]; ok {
				prev = &p
			}
			results[idx] = s.probeOne(h, prev)
		}(i, targets[i])
	}
	wg.Wait()
	return results, nil
}

// probeOne 单台机器一轮:连 SSH → 跑全套命令 → 关闭。任何关键步骤失败封装到 HostResult.Error。
func (s *Sampler) probeOne(h model.EsxiHost, prev *model.EsxiState) HostResult {
	res := HostResult{
		HostKind: model.EsxiHostKind,
		HostID:   h.ID,
		HostName: h.Name,
		Endpoint: h.Endpoint,
		StartAt:  time.Now(),
	}

	target, err := ParseEsxiTarget(h)
	if err != nil {
		res.Error = err.Error()
		return res
	}
	conn, err := sshlike.ConnFor(target, sshlike.ConnOptions{
		Credentials:        s.credentials,
		LoadBastion:        func(id int64) (*sshlike.Target, error) { return LoadEsxiBastion(s.db, id) },
		RejectBastionChain: true,
	})
	if err != nil {
		res.Error = err.Error()
		return res
	}

	// 给整轮采集套一个总超时(sshx.Dial 自带 15s 拨号超时,这里只罩跑命令阶段)。
	done := make(chan struct{})
	var metrics HostMetrics
	var probeErr error
	go func() {
		defer close(done)
		client, cleanup, derr := sshx.Dial(nil, conn)
		if derr != nil {
			probeErr = derr
			return
		}
		defer cleanup()
		opts := collectOptions(prev, res.StartAt, s.slowRefreshInterval)
		if opts.SkipTopology {
			logx.Debug("esxi topology collection skipped",
				"host", h.Name,
				"last_full_success_at", opts.PreviousTopology.LastFullSuccessAt.Format(time.RFC3339),
				"interval", s.slowRefreshInterval.String())
		}
		metrics = CollectAllWithOptions(client, opts)
		missing := probeMissing(metrics)
		for attempt := 2; len(missing) > 0 && attempt <= 2; attempt++ {
			logx.Warn("esxi probe incomplete, retrying",
				"host", h.Name, "attempt", attempt, "missing", strings.Join(missing, ","))
			next := CollectAllWithOptions(client, opts)
			metrics = mergeHostMetrics(metrics, next)
			missing = probeMissing(metrics)
		}
		if topologyFullyCollected(metrics) && metrics.Topology.LastFullSuccessAt.IsZero() {
			metrics.Topology.LastFullSuccessAt = time.Now()
		}
		if len(missing) > 0 {
			logx.Warn("esxi probe incomplete after retries", "host", h.Name, "missing", strings.Join(missing, ","))
		}
	}()
	select {
	case <-done:
	case <-time.After(s.timeout + 15*time.Second):
		probeErr = errors.New("采集超时")
	}

	if probeErr != nil {
		res.Error = probeErr.Error()
		logx.Debug("esxi probe failed", "host", h.Name, "err", probeErr)
		return res
	}
	res.OK = true
	res.Metrics = metrics
	return res
}

func (s *Sampler) loadPreviousStatesByHostID() map[int64]model.EsxiState {
	var rows []model.EsxiState
	if err := s.db.Find(&rows).Error; err != nil {
		logx.Warn("esxi previous state load failed", "err", err.Error())
		return nil
	}
	out := make(map[int64]model.EsxiState, len(rows))
	for _, row := range rows {
		if row.HostKind == model.EsxiHostKind {
			out[row.HostID] = row
		}
	}
	return out
}

// ProbeByHostID 给 handler 的 /esxi/hosts/:id/test 用:按 id 拉一条立即探测。
func (s *Sampler) ProbeByHostID(id int64) (HostResult, error) {
	h, err := s.hosts.Get(id)
	if err != nil {
		return HostResult{}, err
	}
	var prev *model.EsxiState
	if row, ok := s.loadPreviousStatesByHostID()[h.ID]; ok {
		prev = &row
	}
	return s.probeOne(*h, prev), nil
}
