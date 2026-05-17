package main

import (
	"embed"
	"io/fs"

	"github.com/LemonZuo/homer/internal/buildinfo"
	"github.com/LemonZuo/homer/internal/config"
	"github.com/LemonZuo/homer/internal/logx"
)

//go:embed all:frontend/dist
var frontendFS embed.FS

func main() {
	cfg := config.Load()
	logx.Init(cfg.LogLevel)

	logx.Info("homer starting",
		"version", buildinfo.Version,
		"commit", buildinfo.Commit,
		"build", buildinfo.BuildID,
		"log_level", cfg.LogLevel,
	)

	dist, err := fs.Sub(frontendFS, "frontend/dist")
	if err != nil {
		logx.Fatal("sub frontend/dist", "err", err)
	}

	srv, cleanup, err := buildServer(cfg, dist)
	if err != nil {
		logx.Fatal("build server", "err", err)
	}
	defer cleanup()

	logx.Info("server listening", "url", cfg.ListenURL())
	if err := srv.Run(cfg.ListenAddr()); err != nil {
		logx.Fatal("run server", "err", err)
	}
}
