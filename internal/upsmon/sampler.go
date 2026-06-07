package upsmon

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/LemonZuo/homer/internal/acme"
	"github.com/LemonZuo/homer/internal/acme/deployer/sshlike"
	"github.com/LemonZuo/homer/internal/acme/deployer/sshx"
	"github.com/LemonZuo/homer/internal/logx"
	"github.com/LemonZuo/homer/internal/model"
	"gorm.io/gorm"
)

// 支持的机器 kind(对应 acme_deploy_target.kind)。fnos 和 ssh 共用 sshlike 解析。
var supportedHostKinds = []string{acme.DeployKindSSH, acme.DeployKindFnOS}

// HostResult 单台机器一轮采样的结果。
type HostResult struct {
	HostKind string        `json:"host_kind"`
	HostID   int64         `json:"host_id"`
	HostName string        `json:"host_name"`
	Endpoint string        `json:"endpoint"`
	OK       bool          `json:"ok"`
	Error    string        `json:"error,omitempty"`
	HasUPS   bool          `json:"has_ups"`
	UPSes    []upscReading `json:"-"` // 内部用,转 sample 时消费
	StartAt  time.Time     `json:"-"`
}

// Sampler 负责一轮"扫所有目标 → 并发 SSH → 跑 upsc → 聚合结果"。
type Sampler struct {
	db          *gorm.DB
	credentials *acme.SSHCredentialStore
	timeout     time.Duration
}

func NewSampler(db *gorm.DB, credentials *acme.SSHCredentialStore, sshTimeout time.Duration) *Sampler {
	if sshTimeout <= 0 {
		sshTimeout = 5 * time.Second
	}
	return &Sampler{db: db, credentials: credentials, timeout: sshTimeout}
}

// Run 扫一轮所有候选机器并发采样,聚合返回。
// 单机失败不影响其他;返回的 HostResult 顺序与 acme_deploy_target.id 升序一致。
func (s *Sampler) Run() ([]HostResult, error) {
	targets, err := s.listTargets()
	if err != nil {
		return nil, err
	}
	if len(targets) == 0 {
		return nil, nil
	}

	results := make([]HostResult, len(targets))
	var wg sync.WaitGroup
	for i := range targets {
		wg.Add(1)
		go func(idx int, t model.ACMEDeployTarget) {
			defer wg.Done()
			results[idx] = s.probeOne(t)
		}(i, targets[i])
	}
	wg.Wait()
	return results, nil
}

// listTargets 读 acme_deploy_target,过滤 enabled=1 + kind in (ssh, fnos) + ups_monitor=1。
// ups_monitor 是显式订阅开关,用户没勾选的机器不会被采样打扰。
func (s *Sampler) listTargets() ([]model.ACMEDeployTarget, error) {
	var rows []model.ACMEDeployTarget
	err := s.db.
		Where("kind IN ?", supportedHostKinds).
		Where("enabled = ?", "1").
		Where("ups_monitor = ?", "1").
		Order("id").
		Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("列出主机失败:%w", err)
	}
	return rows, nil
}

// ListCandidates 给前端"机器订阅"区用:返回所有 ssh/fnos 目标(忽略 ups_monitor),
// 用户在 UI 上挨个勾选哪些纳入采样。
func (s *Sampler) ListCandidates() ([]model.ACMEDeployTarget, error) {
	var rows []model.ACMEDeployTarget
	err := s.db.
		Where("kind IN ?", supportedHostKinds).
		Where("enabled = ?", "1").
		Order("id").
		Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("列出候选主机失败:%w", err)
	}
	return rows, nil
}

// SetMonitor 切换某个目标的 ups_monitor 开关。
// hostKind 限制在 ssh/fnos —— 不让用户误改其他 driver 的目标。
func (s *Sampler) SetMonitor(hostID int64, enable bool) error {
	val := "0"
	if enable {
		val = "1"
	}
	res := s.db.Model(&model.ACMEDeployTarget{}).
		Where("id = ? AND kind IN ?", hostID, supportedHostKinds).
		Update("ups_monitor", val)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return fmt.Errorf("未找到 id=%d 的 ssh/fnos 目标", hostID)
	}
	return nil
}

// probeOne 单台机器一轮:连 SSH → 跑 upsc → 关闭。任何一步失败封装到 HostResult.Error。
func (s *Sampler) probeOne(t model.ACMEDeployTarget) HostResult {
	res := HostResult{
		HostKind: t.Kind,
		HostID:   t.ID,
		HostName: t.Name,
		Endpoint: t.Endpoint,
		StartAt:  time.Now(),
	}

	target, err := sshlike.ParseTarget(t, sshlike.Labels{Auth: "UPS", Config: "UPS", Host: "UPS"})
	if err != nil {
		res.Error = err.Error()
		return res
	}
	conn, err := sshlike.ConnFor(target, sshlike.ConnOptions{
		Credentials:        s.credentials,
		DB:                 s.db,
		RejectBastionChain: true,
	})
	if err != nil {
		res.Error = err.Error()
		return res
	}

	// 串行超时门:Dial 自带 15s,但整轮上限交给上层 cron 间隔约束;
	// 这里通过 goroutine + 时限保证一台机器卡死不拖垮整轮。
	done := make(chan struct{})
	var readings []upscReading
	var probeErr error
	go func() {
		defer close(done)
		client, cleanup, err := sshx.Dial(nil, conn)
		if err != nil {
			probeErr = err
			return
		}
		defer cleanup()
		readings, probeErr = probeHost(client)
	}()
	select {
	case <-done:
	case <-time.After(s.timeout + 15*time.Second): // sshx.Dial 自带 15s 拨号超时
		probeErr = errors.New("采样超时")
	}

	if probeErr != nil {
		res.Error = probeErr.Error()
		logx.Debug("ups probe failed", "host", t.Name, "kind", t.Kind, "err", probeErr)
		return res
	}
	res.OK = true
	res.HasUPS = len(readings) > 0
	res.UPSes = readings
	return res
}
