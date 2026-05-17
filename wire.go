package main

import (
	"fmt"
	"io/fs"
	"log"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/LemonZuo/homer/internal/acme"
	acmesafeline "github.com/LemonZuo/homer/internal/acme/deployer/safeline"
	acmessh "github.com/LemonZuo/homer/internal/acme/deployer/ssh"
	"github.com/LemonZuo/homer/internal/birthday"
	"github.com/LemonZuo/homer/internal/cas"
	"github.com/LemonZuo/homer/internal/cdn"
	"github.com/LemonZuo/homer/internal/config"
	"github.com/LemonZuo/homer/internal/db"
	"github.com/LemonZuo/homer/internal/event"
	"github.com/LemonZuo/homer/internal/handler"
	"github.com/LemonZuo/homer/internal/jobmonitor"
	"github.com/LemonZuo/homer/internal/model"
	"github.com/LemonZuo/homer/internal/notify"
	"github.com/LemonZuo/homer/internal/router"
	"github.com/LemonZuo/homer/internal/scheduler"
)

// buildServer 组装全部依赖（DB / notify Hub / service / handler / scheduler / router），
// 返回可运行的 gin.Engine 与清理函数（停止调度器）。
func buildServer(cfg *config.Config, frontend fs.FS) (*gin.Engine, func(), error) {
	gormDB, err := db.New(cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("connect db: %w", err)
	}
	if err := gormDB.AutoMigrate(
		&model.BirthdayRemind{},
		&model.EventReminder{},
		&model.SmsForwarder{},
		&model.NotifyChannel{},
		&model.NotifyBinding{},
		&model.SchedulerJobState{},
		&model.ACMECredential{},
		&model.ACMEAccount{},
		&model.ACMEDomain{},
		&model.ACMECert{},
		&model.ACMEIssueTask{},
		&model.ACMEDeployTarget{},
		&model.ACMEDeployConfig{},
		&model.SSHCredential{},
	); err != nil {
		return nil, nil, fmt.Errorf("migrate: %w", err)
	}

	hub := notify.NewHub(gormDB)
	notifyStore := notify.NewStore(gormDB)

	birthdayNotifier := hub.For(notify.ModuleBirthday)
	eventNotifier := hub.For(notify.ModuleEvent)
	bypassNotifier := hub.For(notify.ModuleBypass)

	cdnSvc := cdn.NewService(cfg.AliyunCDNAccessKeyID, cfg.AliyunCDNAccessKeySecret)
	casSvc := cas.NewService(cfg.AliyunCASAccessKeyID, cfg.AliyunCASAccessKeySecret)
	acmeSvc := buildACMEService(gormDB, cfg, casSvc)

	sched := startScheduler(gormDB, cfg, birthdayNotifier, eventNotifier, acmeSvc, hub)

	cdnHandler := handler.NewCDNHandler(cdnSvc)
	casHandler := handler.NewCASHandler(casSvc, cdnSvc)
	acmeHandler := handler.NewACMEHandler(acmeSvc)
	bypassHandler := handler.NewBypassHandler(bypassNotifier)
	smsHandler := handler.NewSMSHandler(gormDB)
	schedulerHandler := handler.NewSchedulerHandler(sched)
	notifyHandler := handler.NewNotifyHandler(notifyStore)

	r := router.Setup(gormDB, birthdayNotifier, eventNotifier, cdnHandler, casHandler, acmeHandler, bypassHandler, smsHandler, schedulerHandler, notifyHandler, frontend)
	r.GET("/healthz", handler.Health(gormDB, sched))

	return r, sched.Stop, nil
}

// buildACMEService 组装 ACME 依赖图（store / registry / driver / manager / SSE / service）。
func buildACMEService(gormDB *gorm.DB, cfg *config.Config, casSvc *cas.Service) *acme.Service {
	sshCreds := acme.NewSSHCredentialStore(gormDB)
	registry := acme.NewDeployRegistry(acmessh.NewDriver(sshCreds, gormDB), acmesafeline.NewDriver())
	targets := acme.NewDeployTargetStore(gormDB, registry)
	configs := acme.NewDeployConfigStore(gormDB, targets, registry)
	return acme.NewService(
		gormDB,
		acme.NewManager(cfg.ACMEDataDir),
		acme.NewCredentialStore(gormDB),
		sshCreds,
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

// startScheduler 注册后台任务、挂上 jobmonitor（持久化 + 失败告警）并启动，
// 返回 Scheduler 供调用方 defer Stop。
func startScheduler(gormDB *gorm.DB, cfg *config.Config, notifier notify.Notifier, eventNotifier notify.Notifier, acmeSvc *acme.Service, hub *notify.Hub) *scheduler.Scheduler {
	sched := scheduler.New()

	if err := sched.Register("birthday", cfg.BirthdayRemindCron, func() error {
		birthday.RunOnce(gormDB, notifier)
		return nil
	}); err != nil {
		log.Fatalf("register birthday task: %v", err)
	}

	if err := sched.Register("event", cfg.EventRemindCron, func() error {
		event.RunOnce(gormDB, eventNotifier)
		return nil
	}); err != nil {
		log.Fatalf("register event task: %v", err)
	}

	if err := sched.Register("acme-renew", cfg.ACMERenewCron, func() error {
		ids, err := acmeSvc.RenewExpiring()
		if err != nil {
			return err
		}
		if len(ids) > 0 {
			log.Printf("acme renew 触发：%v", ids)
		}
		return nil
	}); err != nil {
		log.Fatalf("register acme-renew task: %v", err)
	}

	// 观察者：落库 + 连续失败达阈值经 Hub 告警。须在 Start 前注入并预热。
	mon := jobmonitor.New(gormDB, hub.For(notify.ModuleSchedAlrt))
	sched.SetObserver(mon, cfg.SchedulerAlertFailThreshold)
	mon.Hydrate(sched)

	sched.Start()
	return sched
}
