package router

import (
	"io/fs"
	"strings"

	"github.com/LemonZuo/homer/internal/buildinfo"
	"github.com/LemonZuo/homer/internal/handler"
	"github.com/LemonZuo/homer/internal/model"
	"github.com/LemonZuo/homer/internal/notify/wework"
	"github.com/LemonZuo/homer/internal/web"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// TableMeta 描述前端展示和路由信息。
type TableMeta struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	Path  string `json:"path"`
}

var Tables = []TableMeta{
	{Key: "birthday", Label: "生日提醒", Path: "birthday"},
}

func Setup(db *gorm.DB, notifier *wework.Client, cdnHandler *handler.CDNHandler, frontend fs.FS) *gin.Engine {
	r := gin.Default()
	r.Use(cors.New(cors.Config{
		AllowAllOrigins: true,
		AllowMethods:    []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:    []string{"Origin", "Content-Type", "Accept", "Authorization"},
	}))

	api := r.Group("/api")

	api.GET("/tables", func(c *gin.Context) {
		c.JSON(200, gin.H{"data": Tables})
	})

	api.GET("/version", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"version": buildinfo.Version,
			"commit":  buildinfo.Commit,
		})
	})

	handler.NewCRUD[model.BirthdayRemind](db).Register(api, "/birthday")
	api.POST("/birthday/:id/notify", handler.BirthdayNotify(db, notifier))

	cdnHandler.Register(api)

	// 前端单页：未命中 /api/* 的请求都交给 embed 出来的 dist
	spa := web.SPAHandler(frontend)
	r.NoRoute(func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/api/") {
			c.JSON(404, gin.H{"error": "not found"})
			return
		}
		spa(c)
	})

	return r
}
