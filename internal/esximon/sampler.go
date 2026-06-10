package esximon

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/LemonZuo/homer/internal/acme/deployer/sshx"
	"github.com/LemonZuo/homer/internal/esximon/sshhost"
	"github.com/LemonZuo/homer/internal/logx"
	"github.com/LemonZuo/homer/internal/model"
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
	db          *gorm.DB
	hosts       *HostStore
	credentials sshhost.CredentialResolver
	timeout     time.Duration
}

func NewSampler(db *gorm.DB, hosts *HostStore, credentials sshhost.CredentialResolver, sshTimeout time.Duration) *Sampler {
	if sshTimeout <= 0 {
		sshTimeout = 20 * time.Second
	}
	return &Sampler{db: db, hosts: hosts, credentials: credentials, timeout: sshTimeout}
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

	results := make([]HostResult, len(targets))
	var wg sync.WaitGroup
	for i := range targets {
		wg.Add(1)
		go func(idx int, h model.EsxiHost) {
			defer wg.Done()
			results[idx] = s.probeOne(h)
		}(i, targets[i])
	}
	wg.Wait()
	return results, nil
}

// probeOne 单台机器一轮:连 SSH → 跑全套命令 → 关闭。任何关键步骤失败封装到 HostResult.Error。
func (s *Sampler) probeOne(h model.EsxiHost) HostResult {
	res := HostResult{
		HostKind: model.EsxiHostKind,
		HostID:   h.ID,
		HostName: h.Name,
		Endpoint: h.Endpoint,
		StartAt:  time.Now(),
	}

	target, err := sshhost.ParseTarget(h)
	if err != nil {
		res.Error = err.Error()
		return res
	}
	conn, err := sshhost.ConnFor(target, sshhost.ConnOptions{
		Credentials: s.credentials,
		DB:          s.db,
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
		metrics = CollectAll(client)
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

// ProbeByHostID 给 handler 的 /esxi/hosts/:id/test 用:按 id 拉一条立即探测。
func (s *Sampler) ProbeByHostID(id int64) (HostResult, error) {
	h, err := s.hosts.Get(id)
	if err != nil {
		return HostResult{}, err
	}
	return s.probeOne(*h), nil
}
