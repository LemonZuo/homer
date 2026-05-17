package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/LemonZuo/homer/internal/scheduler"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// Health 轻量健康检查：探活 DB 连接与调度器状态。
// 挂在根路径 /healthz（不经 /api，便于反代/探针直连）。
// DB 不通返回 503，其余 200；调度信息仅作展示不影响存活判定。
func Health(db *gorm.DB, sched *scheduler.Scheduler) gin.HandlerFunc {
	return func(c *gin.Context) {
		resp := gin.H{"status": "ok", "db": "ok"}
		code := http.StatusOK

		if sqlDB, err := db.DB(); err != nil {
			resp["status"], resp["db"], code = "degraded", err.Error(), http.StatusServiceUnavailable
		} else {
			ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
			defer cancel()
			if err := sqlDB.PingContext(ctx); err != nil {
				resp["status"], resp["db"], code = "degraded", err.Error(), http.StatusServiceUnavailable
			}
		}

		jobs := sched.Jobs()
		running, failing := 0, 0
		for _, j := range jobs {
			if j.Running {
				running++
			}
			if j.Last != nil && !j.Last.OK {
				failing++
			}
		}
		resp["scheduler"] = gin.H{"jobs": len(jobs), "running": running, "failing": failing}

		c.JSON(code, resp)
	}
}
