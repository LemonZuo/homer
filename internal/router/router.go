package router

import (
	"io/fs"
	"strings"

	"github.com/LemonZuo/homer/internal/buildinfo"
	"github.com/LemonZuo/homer/internal/handler"
	acmehandler "github.com/LemonZuo/homer/internal/handler/acme"
	"github.com/LemonZuo/homer/internal/notify"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func Setup(db *gorm.DB, notifier notify.Notifier, eventNotifier notify.Notifier, cdnopsHandler *handler.CDNOpsHandler, certstoreHandler *handler.CertStoreHandler, acmeHandler *acmehandler.Handler, bypassHandler *handler.BypassHandler, smsHandler *handler.SMSHandler, schedulerHandler *handler.SchedulerHandler, notifyHandler *handler.NotifyHandler, frontend fs.FS) *gin.Engine {
	r := gin.New()
	r.Use(gin.LoggerWithConfig(gin.LoggerConfig{SkipPaths: []string{"/healthz"}}))
	r.Use(gin.Recovery())
	r.Use(cors.New(cors.Config{
		AllowAllOrigins: true,
		AllowMethods:    []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:    []string{"Origin", "Content-Type", "Accept", "Authorization"},
	}))

	api := r.Group("/api")

	api.GET("/version", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"version":  buildinfo.Version,
			"commit":   buildinfo.Commit,
			"build_id": buildinfo.BuildID,
		})
	})

	handler.NewBirthdayHandler(db, notifier).Register(api)
	handler.NewEventHandler(db, eventNotifier).Register(api)

	cdnopsHandler.Register(api)
	certstoreHandler.Register(api)
	acmeHandler.Register(api)
	bypassHandler.Register(api)
	smsHandler.Register(api)
	schedulerHandler.Register(api)
	notifyHandler.Register(api)

	// 前端单页：未命中 /api/* 的请求都交给 embed 出来的 dist
	spa := SPAHandler(frontend)
	r.NoRoute(func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/api/") {
			c.JSON(404, gin.H{"error": "not found"})
			return
		}
		spa(c)
	})

	return r
}
