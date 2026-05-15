package main

import (
	"embed"
	"io/fs"
	"log"

	"github.com/LemonZuo/homer/internal/buildinfo"
	"github.com/LemonZuo/homer/internal/config"
	"github.com/LemonZuo/homer/internal/db"
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

	dist, err := fs.Sub(frontendFS, "frontend/dist")
	if err != nil {
		log.Fatalf("sub frontend/dist: %v", err)
	}

	r := router.Setup(gormDB, dist)
	log.Printf("server listening on %s", cfg.ListenURL())
	if err := r.Run(cfg.ListenAddr()); err != nil {
		log.Fatalf("run server: %v", err)
	}
}
