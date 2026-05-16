package main

import (
	"embed"
	"io/fs"
	"log"

	"github.com/LemonZuo/homer/internal/buildinfo"
	"github.com/LemonZuo/homer/internal/cas"
	"github.com/LemonZuo/homer/internal/cdn"
	"github.com/LemonZuo/homer/internal/config"
	"github.com/LemonZuo/homer/internal/db"
	"github.com/LemonZuo/homer/internal/handler"
	"github.com/LemonZuo/homer/internal/model"
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

	notifier := wework.New(cfg.WeWorkBirthdayCorpID, cfg.WeWorkBirthdayAgentID, cfg.WeWorkBirthdaySecret, cfg.WeWorkBirthdayTagID)

	cdnSvc := cdn.NewService(cfg.AliyunCDNAccessKeyID, cfg.AliyunCDNAccessKeySecret)
	casSvc := cas.NewService(cfg.AliyunCASAccessKeyID, cfg.AliyunCASAccessKeySecret)

	cdnHandler := handler.NewCDNHandler(cdnSvc)
	casHandler := handler.NewCASHandler(casSvc, cdnSvc)
	acmeSvc := buildACMEService(gormDB, cfg, casSvc)
	acmeHandler := handler.NewACMEHandler(acmeSvc)

	bypassWeWork := wework.New(cfg.WeWorkBypassCorpID, cfg.WeWorkBypassAgentID, cfg.WeWorkBypassSecret, cfg.WeWorkBypassTagID)
	bypassEmail := email.NewResend(cfg.ResendAPIKey, cfg.BypassEmailFrom)
	bypassHandler := handler.NewBypassHandler(bypassWeWork, bypassEmail, cfg.BypassEmailTo, cfg.BypassSubject)

	smsHandler := handler.NewSMSHandler(gormDB)

	sched := startScheduler(gormDB, cfg, notifier, acmeSvc)
	defer sched.Stop()

	dist, err := fs.Sub(frontendFS, "frontend/dist")
	if err != nil {
		log.Fatalf("sub frontend/dist: %v", err)
	}

	r := router.Setup(gormDB, notifier, cdnHandler, casHandler, acmeHandler, bypassHandler, smsHandler, dist)
	log.Printf("server listening on %s", cfg.ListenURL())
	if err := r.Run(cfg.ListenAddr()); err != nil {
		log.Fatalf("run server: %v", err)
	}
}
