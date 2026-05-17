package main

import (
	"fmt"
	"io/fs"
	"log"
	"time"

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
	"github.com/LemonZuo/homer/internal/model"
	"github.com/LemonZuo/homer/internal/notify"
	"github.com/LemonZuo/homer/internal/notify/email"
	"github.com/LemonZuo/homer/internal/notify/wework"
	"github.com/LemonZuo/homer/internal/router"
	"github.com/LemonZuo/homer/internal/scheduler"
)

// retryWeWork 构造带统一重试（3 次 / 2s 指数退避）的企业微信 Notifier。
func retryWeWork(corpID, agentID, secret, tagID string) notify.Notifier {
	return notify.Retry(3, 2*time.Second, notify.WeWork(wework.New(corpID, agentID, secret, tagID)))
}

// retryEmail 构造带统一重试（3 次 / 2s 指数退避）的 Resend 邮件 Notifier。
func retryEmail(apiKey, from, to string) notify.Notifier {
	return notify.Retry(3, 2*time.Second, notify.Email(email.NewResend(apiKey, from), to))
}

// buildServer 组装全部依赖（DB / notifier / service / handler / scheduler / router），
// 返回可运行的 gin.Engine 与清理函数（停止调度器）。
func buildServer(cfg *config.Config, frontend fs.FS) (*gin.Engine, func(), error) {
	gormDB, err := db.New(cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("connect db: %w", err)
	}
	if err := gormDB.AutoMigrate(&model.SmsForwarder{}); err != nil {
		return nil, nil, fmt.Errorf("migrate sms_forwarder: %w", err)
	}

	notifier := retryWeWork(cfg.WeWorkBirthdayCorpID, cfg.WeWorkBirthdayAgentID, cfg.WeWorkBirthdaySecret, cfg.WeWorkBirthdayTagID)
	eventNotifier := retryWeWork(cfg.WeWorkEventCorpID, cfg.WeWorkEventAgentID, cfg.WeWorkEventSecret, cfg.WeWorkEventTagID)

	cdnSvc := cdn.NewService(cfg.AliyunCDNAccessKeyID, cfg.AliyunCDNAccessKeySecret)
	casSvc := cas.NewService(cfg.AliyunCASAccessKeyID, cfg.AliyunCASAccessKeySecret)
	acmeSvc := buildACMEService(gormDB, cfg, casSvc)

	bypassWeWork := retryWeWork(cfg.WeWorkBypassCorpID, cfg.WeWorkBypassAgentID, cfg.WeWorkBypassSecret, cfg.WeWorkBypassTagID)
	bypassEmail := retryEmail(cfg.ResendAPIKey, cfg.BypassEmailFrom, cfg.BypassEmailTo)

	sched := startScheduler(gormDB, cfg, notifier, eventNotifier, acmeSvc)

	cdnHandler := handler.NewCDNHandler(cdnSvc)
	casHandler := handler.NewCASHandler(casSvc, cdnSvc)
	acmeHandler := handler.NewACMEHandler(acmeSvc)
	bypassHandler := handler.NewBypassHandler(bypassWeWork, bypassEmail, cfg.BypassSubject)
	smsHandler := handler.NewSMSHandler(gormDB)
	schedulerHandler := handler.NewSchedulerHandler(sched)

	r := router.Setup(gormDB, notifier, eventNotifier, cdnHandler, casHandler, acmeHandler, bypassHandler, smsHandler, schedulerHandler, frontend)
	r.GET("/healthz", handler.Health(gormDB, sched))

	return r, sched.Stop, nil
}

// buildACMEService 组装 ACME 依赖图（store / registry / driver / manager / SSE / service）。
func buildACMEService(gormDB *gorm.DB, cfg *config.Config, casSvc *cas.Service) *acme.Service {
	sshCreds := acme.NewSSHCredentialStore(gormDB)
	registry := acme.NewDeployRegistry(acmessh.NewDriver(sshCreds), acmesafeline.NewDriver())
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

// startScheduler 注册并启动后台任务，返回 Scheduler 供调用方 defer Stop。
func startScheduler(gormDB *gorm.DB, cfg *config.Config, notifier notify.Notifier, eventNotifier notify.Notifier, acmeSvc *acme.Service) *scheduler.Scheduler {
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

	sched.Start()
	return sched
}
