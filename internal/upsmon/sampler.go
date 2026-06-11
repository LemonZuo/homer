package upsmon

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/LemonZuo/homer/internal/logx"
	"github.com/LemonZuo/homer/internal/model"
	"github.com/LemonZuo/homer/internal/sshlike"
	"github.com/LemonZuo/homer/internal/sshx"
	"gorm.io/gorm"
)

// HostResult 单台机器一轮采样的结果。HostKind 恒为 model.UPSHostKind = "ups"。
type HostResult struct {
	HostKind      string        `json:"host_kind"`
	HostID        int64         `json:"host_id"`
	HostName      string        `json:"host_name"`
	Endpoint      string        `json:"endpoint"`
	OK            bool          `json:"ok"`
	Error         string        `json:"error,omitempty"`
	HasUPS        bool          `json:"has_ups"`
	Diag          string        `json:"-"` // 探测连通但没拿到 UPS 时的诊断输出,供 test 接口展示
	UPSes         []upscReading `json:"-"` // 内部用,转 sample 时消费
	UPSNames      []string      `json:"-"` // upsc -l 拿到的 UPS 名(NUT 已知集合),用于"失联"判定;与 UPSes 不一定一一对应(driver 抖动会丢 reading 但 NUT 仍知道名字)
	UPSEnumerated bool          `json:"-"` // 本轮是否成功拿到 NUT 已知集合(false 时 UPSNames 不可信,告警侧应跳过本轮判定)
	StartAt       time.Time     `json:"-"`
}

// Sampler 负责一轮"扫所有 ups_host → 并发 SSH → 跑 upsc → 聚合结果"。
type Sampler struct {
	db          *gorm.DB
	hosts       *HostStore
	credentials sshlike.CredentialResolver
	timeout     time.Duration
}

func NewSampler(db *gorm.DB, hosts *HostStore, credentials sshlike.CredentialResolver, sshTimeout time.Duration) *Sampler {
	if sshTimeout <= 0 {
		sshTimeout = 5 * time.Second
	}
	return &Sampler{db: db, hosts: hosts, credentials: credentials, timeout: sshTimeout}
}

// Run 扫一轮所有启用的 ups_host 并发采样,聚合返回。
// 单机失败不影响其他;返回的 HostResult 顺序与 ups_host.id 升序一致。
func (s *Sampler) Run() ([]HostResult, error) {
	targets, err := s.hosts.ListEnabled()
	if err != nil {
		return nil, fmt.Errorf("列出 UPS 主机失败:%w", err)
	}
	if len(targets) == 0 {
		return nil, nil
	}

	results := make([]HostResult, len(targets))
	var wg sync.WaitGroup
	for i := range targets {
		wg.Add(1)
		go func(idx int, h model.UPSHost) {
			defer wg.Done()
			results[idx] = s.probeOne(h)
		}(i, targets[i])
	}
	wg.Wait()
	return results, nil
}

// probeOne 单台机器一轮:连 SSH → 跑 upsc → 关闭。任何一步失败封装到 HostResult.Error。
func (s *Sampler) probeOne(h model.UPSHost) HostResult {
	res := HostResult{
		HostKind: model.UPSHostKind,
		HostID:   h.ID,
		HostName: h.Name,
		Endpoint: h.Endpoint,
		StartAt:  time.Now(),
	}

	target, err := ParseUPSTarget(h)
	if err != nil {
		res.Error = err.Error()
		return res
	}
	conn, err := sshlike.ConnFor(target, sshlike.ConnOptions{
		Credentials:        s.credentials,
		LoadBastion:        func(id int64) (*sshlike.Target, error) { return LoadUPSBastion(s.db, id) },
		RejectBastionChain: true,
	})
	if err != nil {
		res.Error = err.Error()
		return res
	}

	// 串行超时门:Dial 自带 15s,但整轮上限交给上层 cron 间隔约束;
	// 这里通过 goroutine + 时限保证一台机器卡死不拖垮整轮。
	// 结果通过 buffered channel 传回(不共享变量),超时分支只写主协程本地,
	// 避免主/子协程对同一组变量的 data race。子协程即使超时后才完成,
	// send 不阻塞,cleanup 会在它内部 defer 里跑完。
	type probeOut struct {
		readings   []upscReading
		names      []string
		enumerated bool
		diag       string
		err        error
	}
	resultCh := make(chan probeOut, 1)
	go func() {
		var out probeOut
		client, cleanup, err := sshx.Dial(nil, conn)
		if err != nil {
			out.err = err
			resultCh <- out
			return
		}
		defer cleanup()
		out.readings, out.names, out.enumerated, out.diag, out.err = probeHost(client)
		resultCh <- out
	}()

	var (
		readings   []upscReading
		names      []string
		enumerated bool
		diag       string
		probeErr   error
	)
	select {
	case out := <-resultCh:
		readings = out.readings
		names = out.names
		enumerated = out.enumerated
		diag = out.diag
		probeErr = out.err
	case <-time.After(s.timeout + 15*time.Second): // sshx.Dial 自带 15s 拨号超时
		probeErr = errors.New("采样超时")
	}

	if probeErr != nil {
		res.Error = probeErr.Error()
		logx.Debug("ups probe failed", "host", h.Name, "err", probeErr)
		return res
	}
	res.OK = true
	res.HasUPS = len(readings) > 0
	res.UPSes = readings
	res.UPSNames = names
	res.UPSEnumerated = enumerated
	res.Diag = diag
	return res
}

// ProbeByHostID 给 handler 的 /ups/hosts/:id/test 用:按 id 拉一条立即探测。
// 返回 HostResult 即可,前端展示 OK/Error + 探到的 UPS 名列表。
func (s *Sampler) ProbeByHostID(id int64) (HostResult, error) {
	h, err := s.hosts.Get(id)
	if err != nil {
		return HostResult{}, err
	}
	return s.probeOne(*h), nil
}
