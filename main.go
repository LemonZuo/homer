package main

import (
	"embed"
	"io/fs"
	"log"
	"time"

	"github.com/LemonZuo/homer/internal/buildinfo"
	"github.com/LemonZuo/homer/internal/cas"
	"github.com/LemonZuo/homer/internal/cdn"
	"github.com/LemonZuo/homer/internal/config"
	"github.com/LemonZuo/homer/internal/db"
	"github.com/LemonZuo/homer/internal/handler"
	"github.com/LemonZuo/homer/internal/model"
	"github.com/LemonZuo/homer/internal/notify"
	"github.com/LemonZuo/homer/internal/notify/email"
	"github.com/LemonZuo/homer/internal/notify/wework"
	"github.com/LemonZuo/homer/internal/router"
)

//go:embed all:frontend/dist
var frontendFS embed.FS

func main() {
	log.Printf("homer %s (commit %s) starting", buildinfo.Version, buildinfo.Commit)

	cfg := config.Load()

	gormDB, err := db.New(cfg)
	if err != nil {
		log.Fatalf("connect db: %v", err)
	}
	if err := gormDB.AutoMigrate(&model.SmsForwarder{}); err != nil {
		log.Fatalf("migrate sms_forwarder: %v", err)
	}

	notifier := notify.Retry(3, 2*time.Second, notify.WeWork(
		wework.New(cfg.WeWorkBirthdayCorpID, cfg.WeWorkBirthdayAgentID, cfg.WeWorkBirthdaySecret, cfg.WeWorkBirthdayTagID)))
	eventNotifier := notify.Retry(3, 2*time.Second, notify.WeWork(
		wework.New(cfg.WeWorkEventCorpID, cfg.WeWorkEventAgentID, cfg.WeWorkEventSecret, cfg.WeWorkEventTagID)))

	cdnSvc := cdn.NewService(cfg.AliyunCDNAccessKeyID, cfg.AliyunCDNAccessKeySecret)
	casSvc := cas.NewService(cfg.AliyunCASAccessKeyID, cfg.AliyunCASAccessKeySecret)

	cdnHandler := handler.NewCDNHandler(cdnSvc)
	casHandler := handler.NewCASHandler(casSvc, cdnSvc)
	acmeSvc := buildACMEService(gormDB, cfg, casSvc)
	acmeHandler := handler.NewACMEHandler(acmeSvc)

	bypassWeWork := notify.Retry(3, 2*time.Second, notify.WeWork(
		wework.New(cfg.WeWorkBypassCorpID, cfg.WeWorkBypassAgentID, cfg.WeWorkBypassSecret, cfg.WeWorkBypassTagID)))
	bypassEmail := notify.Retry(3, 2*time.Second, notify.Email(
		email.NewResend(cfg.ResendAPIKey, cfg.BypassEmailFrom), cfg.BypassEmailTo))
	bypassHandler := handler.NewBypassHandler(bypassWeWork, bypassEmail, cfg.BypassSubject)

	smsHandler := handler.NewSMSHandler(gormDB)

	sched := startScheduler(gormDB, cfg, notifier, eventNotifier, acmeSvc)
	defer sched.Stop()
	schedulerHandler := handler.NewSchedulerHandler(sched)

	dist, err := fs.Sub(frontendFS, "frontend/dist")
	if err != nil {
		log.Fatalf("sub frontend/dist: %v", err)
	}

	r := router.Setup(gormDB, notifier, eventNotifier, cdnHandler, casHandler, acmeHandler, bypassHandler, smsHandler, schedulerHandler, dist)
	log.Printf("server listening on %s", cfg.ListenURL())
	if err := r.Run(cfg.ListenAddr()); err != nil {
		log.Fatalf("run server: %v", err)
	}
}
