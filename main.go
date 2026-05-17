package main

import (
	"embed"
	"io/fs"
	"log"

	"github.com/LemonZuo/homer/internal/buildinfo"
	"github.com/LemonZuo/homer/internal/config"
)

//go:embed all:frontend/dist
var frontendFS embed.FS

func main() {
	log.Printf("homer %s (commit %s, build %s) starting", buildinfo.Version, buildinfo.Commit, buildinfo.BuildID)

	cfg := config.Load()

	dist, err := fs.Sub(frontendFS, "frontend/dist")
	if err != nil {
		log.Fatalf("sub frontend/dist: %v", err)
	}

	srv, cleanup, err := buildServer(cfg, dist)
	if err != nil {
		log.Fatalf("build server: %v", err)
	}
	defer cleanup()

	log.Printf("server listening on %s", cfg.ListenURL())
	if err := srv.Run(cfg.ListenAddr()); err != nil {
		log.Fatalf("run server: %v", err)
	}
}
