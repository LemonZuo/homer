// Package scheduler 在主进程内启动 cron 调度，集中管理所有后台周期任务。
// 单 binary 部署，不引入额外组件。除调度外还记录每个任务的执行历史，
// 供任务面板展示与手动触发。
package scheduler

import (
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
)

// 每个任务在内存里保留最近 N 次执行记录，单进程够用，重启即清空。
const historySize = 10

// Run 一次执行的结果。
type Run struct {
	Start   time.Time `json:"start"`
	End     time.Time `json:"end"`
	OK      bool      `json:"ok"`
	Err     string    `json:"err,omitempty"`
	Trigger string    `json:"trigger"` // cron | manual
}

// Observer 接收每次执行结果（用于持久化）与达到阈值的失败告警。
// 解耦在 scheduler 之外实现（依赖 DB / notify），scheduler 本身保持零外部依赖。
type Observer interface {
	Record(name string, r Run, consecFails int)
	Alert(name string, r Run, consecFails int)
}

type job struct {
	name string
	spec string
	fn   func() error

	entryID cron.EntryID

	mu      sync.Mutex
	running bool
	last    *Run
	history []Run
	consec  int // 连续失败次数，成功清零
}

type Scheduler struct {
	c   *cron.Cron
	mu  sync.RWMutex
	jobs  map[string]*job
	order []string

	obs           Observer
	failThreshold int
}

// New 创建调度器；启用秒级精度（cron 表达式 6 段：秒 分 时 日 月 周）。
func New() *Scheduler {
	return &Scheduler{
		c:             cron.New(cron.WithSeconds()),
		jobs:          make(map[string]*job),
		failThreshold: 1,
	}
}

// SetObserver 注入结果持久化 + 告警观察者；failThreshold 为「连续失败达 N 次才告警」，
// <1 视为 1（每次失败都告警）。须在 Start 前调用。
func (s *Scheduler) SetObserver(obs Observer, failThreshold int) {
	if failThreshold < 1 {
		failThreshold = 1
	}
	s.mu.Lock()
	s.obs = obs
	s.failThreshold = failThreshold
	s.mu.Unlock()
}

// Seed 用持久化的上次状态预热内存（重启后面板/healthz 仍可见）。须在 Start 前调用。
func (s *Scheduler) Seed(name string, last *Run, consec int) {
	s.mu.RLock()
	j, ok := s.jobs[name]
	s.mu.RUnlock()
	if !ok {
		return
	}
	j.mu.Lock()
	j.last = last
	j.consec = consec
	j.mu.Unlock()
}

// Register 添加一个任务。spec 为空时不进 cron，但仍登记为「仅手动」任务，
// 可在面板里点「立即执行」。fn 返回 error 作为该次执行的结果摘要；
// fn 内 panic 不会扩散到调度器，仅记录为失败。
func (s *Scheduler) Register(name, spec string, fn func() error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, dup := s.jobs[name]; dup {
		return fmt.Errorf("duplicate scheduler job: %s", name)
	}
	j := &job{name: name, spec: spec, fn: fn}
	if spec != "" {
		id, err := s.c.AddFunc(spec, func() { s.run(j, "cron") })
		if err != nil {
			return err
		}
		j.entryID = id
		log.Printf("scheduler[%s] registered with spec %q", name, spec)
	} else {
		log.Printf("scheduler[%s] manual-only (empty cron spec)", name)
	}
	s.jobs[name] = j
	s.order = append(s.order, name)
	return nil
}

func (s *Scheduler) run(j *job, trigger string) {
	j.mu.Lock()
	if j.running {
		j.mu.Unlock()
		log.Printf("scheduler[%s] skip %s: already running", j.name, trigger)
		return
	}
	j.running = true
	j.mu.Unlock()

	start := time.Now()
	var runErr error
	func() {
		defer func() {
			if r := recover(); r != nil {
				runErr = fmt.Errorf("panic: %v", r)
				log.Printf("scheduler[%s] panic: %v", j.name, r)
			}
		}()
		log.Printf("scheduler[%s] tick (%s)", j.name, trigger)
		runErr = j.fn()
	}()

	rec := Run{Start: start, End: time.Now(), OK: runErr == nil, Trigger: trigger}
	if runErr != nil {
		rec.Err = runErr.Error()
	}
	j.mu.Lock()
	j.running = false
	j.last = &rec
	if rec.OK {
		j.consec = 0
	} else {
		j.consec++
	}
	consec := j.consec
	j.history = append(j.history, rec)
	if len(j.history) > historySize {
		j.history = j.history[len(j.history)-historySize:]
	}
	j.mu.Unlock()

	s.mu.RLock()
	obs, threshold := s.obs, s.failThreshold
	s.mu.RUnlock()
	if obs != nil {
		obs.Record(j.name, rec, consec)
		if !rec.OK && consec >= threshold {
			obs.Alert(j.name, rec, consec)
		}
	}
}

// Trigger 立即手动执行一次（异步）；任务不存在或正在执行时返回错误。
func (s *Scheduler) Trigger(name string) error {
	s.mu.RLock()
	j, ok := s.jobs[name]
	s.mu.RUnlock()
	if !ok {
		return errors.New("任务不存在")
	}
	j.mu.Lock()
	if j.running {
		j.mu.Unlock()
		return errors.New("任务正在执行中")
	}
	j.mu.Unlock()
	go s.run(j, "manual")
	return nil
}

// JobView 面板用的任务视图，历史按时间倒序（最近在前）。
type JobView struct {
	Name       string     `json:"name"`
	Spec       string     `json:"spec"`
	ManualOnly bool       `json:"manual_only"`
	Next       *time.Time `json:"next,omitempty"`
	Running    bool       `json:"running"`
	Last       *Run       `json:"last,omitempty"`
	ConsecFails int       `json:"consec_fails"`
	History    []Run      `json:"history"`
}

func (s *Scheduler) Jobs() []JobView {
	s.mu.RLock()
	defer s.mu.RUnlock()
	views := make([]JobView, 0, len(s.order))
	for _, name := range s.order {
		j := s.jobs[name]
		j.mu.Lock()
		v := JobView{
			Name:       j.name,
			Spec:       j.spec,
			ManualOnly: j.spec == "",
			Running:     j.running,
			Last:        j.last,
			ConsecFails: j.consec,
		}
		// 倒序拷贝，避免把内部 slice 暴露出去
		v.History = make([]Run, 0, len(j.history))
		for i := len(j.history) - 1; i >= 0; i-- {
			v.History = append(v.History, j.history[i])
		}
		if j.entryID != 0 {
			if e := s.c.Entry(j.entryID); !e.Next.IsZero() {
				n := e.Next
				v.Next = &n
			}
		}
		j.mu.Unlock()
		views = append(views, v)
	}
	return views
}

func (s *Scheduler) Start() { s.c.Start() }
func (s *Scheduler) Stop()  { s.c.Stop() }
