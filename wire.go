package main

import (
	"fmt"
	"io/fs"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/LemonZuo/homer/internal/acme"
	acmealicas "github.com/LemonZuo/homer/internal/acme/deployer/alicas"
	acmefnos "github.com/LemonZuo/homer/internal/acme/deployer/fnos"
	acmesafeline "github.com/LemonZuo/homer/internal/acme/deployer/safeline"
	acmessh "github.com/LemonZuo/homer/internal/acme/deployer/ssh"
	"github.com/LemonZuo/homer/internal/birthday"
	"github.com/LemonZuo/homer/internal/cdnops"
	"github.com/LemonZuo/homer/internal/certstore"
	"github.com/LemonZuo/homer/internal/config"
	"github.com/LemonZuo/homer/internal/db"
	"github.com/LemonZuo/homer/internal/event"
	"github.com/LemonZuo/homer/internal/handler"
	acmehandler "github.com/LemonZuo/homer/internal/handler/acme"
	"github.com/LemonZuo/homer/internal/jobmonitor"
	"github.com/LemonZuo/homer/internal/logx"
	"github.com/LemonZuo/homer/internal/model"
	"github.com/LemonZuo/homer/internal/notify"
	"github.com/LemonZuo/homer/internal/router"
	"github.com/LemonZuo/homer/internal/scheduler"
	"github.com/LemonZuo/homer/internal/upsmon"
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
		&model.UPSSample{},
		&model.UPSState{},
		&model.UPSHost{},
		&model.UPSSSHCredential{},
	); err != nil {
		return nil, nil, fmt.Errorf("migrate: %w", err)
	}
	logx.Info("db automigrate done")

	hub := notify.NewHub(gormDB)
	notifyStore := notify.NewStore(gormDB)

	birthdayNotifier := hub.For(notify.ModuleBirthday)
	eventNotifier := hub.For(notify.ModuleEvent)
	bypassNotifier := hub.For(notify.ModuleBypass)

	cdnopsSvc := cdnops.NewService(cfg.AliyunCDNAccessKeyID, cfg.AliyunCDNAccessKeySecret)
	certstoreSvc := certstore.NewService(cfg.AliyunCASAccessKeyID, cfg.AliyunCASAccessKeySecret)
	acmeSvc := buildACMEService(gormDB, cfg)
	upsSvc, upsSampler, upsHosts, upsCreds := buildUPSService(gormDB, cfg, hub)

	sched := startScheduler(gormDB, cfg, birthdayNotifier, eventNotifier, acmeSvc, upsSvc, hub)

	cdnopsHandler := handler.NewCDNOpsHandler(cdnopsSvc)
	certstoreHandler := handler.NewCertStoreHandler(certstoreSvc, cdnopsSvc)
	acmeHandler := acmehandler.New(acmeSvc)
	bypassHandler := handler.NewBypassHandler(bypassNotifier)
	smsHandler := handler.NewSMSHandler(gormDB)
	schedulerHandler := handler.NewSchedulerHandler(sched)
	notifyHandler := handler.NewNotifyHandler(notifyStore)
	upsHandler := upsmon.NewHandler(upsSvc, upsSampler, upsHosts, upsCreds)

	r := router.Setup(gormDB, birthdayNotifier, eventNotifier, cdnopsHandler, certstoreHandler, acmeHandler, bypassHandler, smsHandler, schedulerHandler, notifyHandler, upsHandler, frontend)
	r.GET("/healthz", handler.Health(gormDB, sched))

	return r, sched.Stop, nil
}

// buildACMEService 组装 ACME 依赖图（store / registry / driver / manager / SSE / service）。
func buildACMEService(gormDB *gorm.DB, cfg *config.Config) *acme.Service {
	sshCreds := acme.NewSSHCredentialStore(gormDB)
	registry := acme.NewDeployRegistry(
		acmessh.NewDriver(sshCreds, gormDB),
		acmesafeline.NewDriver(),
		acmealicas.NewDriver(),
		acmefnos.NewDriver(sshCreds, gormDB),
	)
	targets := acme.NewDeployTargetStore(gormDB, registry)
	configs := acme.NewDeployConfigStore(gormDB, targets, registry)
	return acme.NewService(
		gormDB,
		acme.NewManager(cfg.ACMEDataDir, cfg.ACMEKeyType),
		acme.NewCredentialStore(gormDB),
		sshCreds,
		acme.NewAccountStore(gormDB),
		targets,
		configs,
		registry,
		acme.NewSSEHub(),
		cfg.ACMEDataDir,
		cfg.ACMERenewBeforeDays,
		cfg.ACMEDeployRetry,
		time.Duration(cfg.ACMEDeployRetryBackoffSec)*time.Second,
	)
}

// buildUPSService 组装 UPS 监控:用 UPS 自带的 HostStore / CredentialStore(完全独立的 ups_host /
// ups_ssh_credential 表),与 ACME 解耦。sampler 通过 CredentialResolver 接口拿凭证,handler 持有
// service+sampler+hosts+creds 暴露快照、订阅管理与 CRUD。
func buildUPSService(gormDB *gorm.DB, cfg *config.Config, hub *notify.Hub) (*upsmon.Service, *upsmon.Sampler, *upsmon.HostStore, *upsmon.CredentialStore) {
	hosts := upsmon.NewHostStore(gormDB)
	creds := upsmon.NewCredentialStore(gormDB, hosts)
	sampler := upsmon.NewSampler(gormDB, hosts, creds, time.Duration(cfg.UPSSSHTimeoutSec)*time.Second)
	store := upsmon.NewStore(gormDB)
	svc := upsmon.NewService(gormDB, sampler, store, hub, time.Duration(cfg.UPSRetentionDays)*24*time.Hour)
	return svc, sampler, hosts, creds
}

// startScheduler 注册后台任务、挂上 jobmonitor（持久化 + 失败告警）并启动，
// 返回 Scheduler 供调用方 defer Stop。
func startScheduler(gormDB *gorm.DB, cfg *config.Config, notifier notify.Notifier, eventNotifier notify.Notifier, acmeSvc *acme.Service, upsSvc *upsmon.Service, hub *notify.Hub) *scheduler.Scheduler {
	sched := scheduler.New()

	if err := sched.Register("birthday", cfg.BirthdayRemindCron, func() error {
		birthday.RunOnce(gormDB, notifier)
		return nil
	}); err != nil {
		logx.Fatal("register birthday task", "err", err)
	}

	if err := sched.Register("event", cfg.EventRemindCron, func() error {
		event.RunOnce(gormDB, eventNotifier)
		return nil
	}); err != nil {
		logx.Fatal("register event task", "err", err)
	}

	if err := sched.Register("acme-renew", cfg.ACMERenewCron, func() error {
		ids, err := acmeSvc.RenewExpiring()
		if err != nil {
			return err
		}
		if len(ids) > 0 {
			logx.Info("acme renew triggered", "task_ids", ids)
		}
		return nil
	}); err != nil {
		logx.Fatal("register acme-renew task", "err", err)
	}

	if err := sched.Register("acme-deploy-retry", cfg.ACMEDeployRetryCron, func() error {
		n, err := acmeSvc.RetryDeployTasks()
		if err != nil {
			return err
		}
		if n > 0 {
			logx.Info("acme deploy retry pulled", "count", n)
		}
		return nil
	}); err != nil {
		logx.Fatal("register acme-deploy-retry task", "err", err)
	}

	if err := sched.Register("ups-sample", cfg.UPSSampleCron, upsSvc.RunSample); err != nil {
		logx.Fatal("register ups-sample task", "err", err)
	}

	if err := sched.Register("ups-cleanup", cfg.UPSCleanupCron, upsSvc.RunCleanup); err != nil {
		logx.Fatal("register ups-cleanup task", "err", err)
	}

	// 观察者：落库 + 连续失败达阈值经 Hub 告警。须在 Start 前注入并预热。
	mon := jobmonitor.New(gormDB, hub.For(notify.ModuleSchedAlrt))
	sched.SetObserver(mon, cfg.SchedulerAlertFailThreshold)
	mon.Hydrate(sched)

	sched.Start()
	return sched
}
