package main

import (
	"log"

	"gorm.io/gorm"

	"github.com/LemonZuo/homer/internal/acme"
	acmesafeline "github.com/LemonZuo/homer/internal/acme/deployer/safeline"
	acmessh "github.com/LemonZuo/homer/internal/acme/deployer/ssh"
	"github.com/LemonZuo/homer/internal/birthday"
	"github.com/LemonZuo/homer/internal/cas"
	"github.com/LemonZuo/homer/internal/config"
	"github.com/LemonZuo/homer/internal/notify/wework"
	"github.com/LemonZuo/homer/internal/scheduler"
)

// buildACMEService 组装 ACME 依赖图（store / registry / driver / manager / SSE / service）。
func buildACMEService(gormDB *gorm.DB, cfg *config.Config, casSvc *cas.Service) *acme.Service {
	registry := acme.NewDeployRegistry(acmessh.NewDriver(), acmesafeline.NewDriver())
	targets := acme.NewDeployTargetStore(gormDB, registry)
	configs := acme.NewDeployConfigStore(gormDB, targets, registry)
	return acme.NewService(
		gormDB,
		acme.NewManager(cfg.ACMEDataDir),
		acme.NewCredentialStore(gormDB),
		acme.NewAccountStore(gormDB),
		targets,
		configs,
		registry,
		casSvc,
		acme.NewSSEHub(),
		cfg.ACMEDataDir,
		cfg.ACMERenewBeforeDays,
	)
}

// startScheduler 注册并启动后台任务，返回 Scheduler 供调用方 defer Stop。
func startScheduler(gormDB *gorm.DB, cfg *config.Config, notifier *wework.Client, acmeSvc *acme.Service) *scheduler.Scheduler {
	sched := scheduler.New()

	if err := sched.Register("birthday", cfg.BirthdayRemindCron, func() {
		birthday.RunOnce(gormDB, notifier)
	}); err != nil {
		log.Fatalf("register birthday task: %v", err)
	}

	if err := sched.Register("acme-renew", cfg.ACMERenewCron, func() {
		ids, err := acmeSvc.RenewExpiring()
		if err != nil {
			log.Printf("acme renew scan: %v", err)
			return
		}
		if len(ids) > 0 {
			log.Printf("acme renew 触发：%v", ids)
		}
	}); err != nil {
		log.Fatalf("register acme-renew task: %v", err)
	}

	sched.Start()
	return sched
}
