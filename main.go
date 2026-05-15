package main

import (
	"embed"
	"io/fs"
	"log"

	"github.com/LemonZuo/homer/internal/birthday"
	"github.com/LemonZuo/homer/internal/buildinfo"
	"github.com/LemonZuo/homer/internal/cas"
	"github.com/LemonZuo/homer/internal/cdn"
	"github.com/LemonZuo/homer/internal/config"
	"github.com/LemonZuo/homer/internal/db"
	"github.com/LemonZuo/homer/internal/handler"
	"github.com/LemonZuo/homer/internal/notify/wework"
	"github.com/LemonZuo/homer/internal/router"
	"github.com/LemonZuo/homer/internal/scheduler"
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

	notifier := wework.New(cfg.WeWorkCorpID, cfg.WeWorkAgentID, cfg.WeWorkSecret, cfg.WeWorkTagID)

	cdnSvc := cdn.NewService(cfg.AliyunCDNAccessKeyID, cfg.AliyunCDNAccessKeySecret)
	cdnHandler := handler.NewCDNHandler(cdnSvc)

	casSvc := cas.NewService(cfg.AliyunCASAccessKeyID, cfg.AliyunCASAccessKeySecret)
	casHandler := handler.NewCASHandler(casSvc, cdnSvc)

	sched := scheduler.New()
	if err := sched.Register("birthday", cfg.BirthdayRemindCron, func() {
		birthday.RunOnce(gormDB, notifier)
	}); err != nil {
		log.Fatalf("register birthday task: %v", err)
	}
	sched.Start()
	defer sched.Stop()

	dist, err := fs.Sub(frontendFS, "frontend/dist")
	if err != nil {
		log.Fatalf("sub frontend/dist: %v", err)
	}

	r := router.Setup(gormDB, notifier, cdnHandler, casHandler, dist)
	log.Printf("server listening on %s", cfg.ListenURL())
	if err := r.Run(cfg.ListenAddr()); err != nil {
		log.Fatalf("run server: %v", err)
	}
}
