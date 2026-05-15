// Package scheduler 在主进程内启动 cron 调度，集中管理所有后台周期任务。
// 单 binary 部署，不引入额外组件。
package scheduler

import (
	"log"

	"github.com/robfig/cron/v3"
)

type Scheduler struct {
	c *cron.Cron
}

// New 创建调度器；启用秒级精度（cron 表达式 6 段：秒 分 时 日 月 周）。
func New() *Scheduler {
	return &Scheduler{c: cron.New(cron.WithSeconds())}
}

// Register 添加一个周期任务。spec 为空时跳过注册（用于通过 .env 关闭某个任务）。
// fn 内 panic 不会扩散到调度器，仅记日志。
func (s *Scheduler) Register(name, spec string, fn func()) error {
	if spec == "" {
		log.Printf("scheduler[%s] disabled (empty cron spec)", name)
		return nil
	}
	_, err := s.c.AddFunc(spec, func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("scheduler[%s] panic: %v", name, r)
			}
		}()
		log.Printf("scheduler[%s] tick", name)
		fn()
	})
	if err != nil {
		return err
	}
	log.Printf("scheduler[%s] registered with spec %q", name, spec)
	return nil
}

func (s *Scheduler) Start() { s.c.Start() }
func (s *Scheduler) Stop()  { s.c.Stop() }
