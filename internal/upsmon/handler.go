package upsmon

import (
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// Handler 暴露 UPS 监控的 HTTP 接口。
type Handler struct {
	svc     *Service
	sampler *Sampler
	hosts   *HostStore
	creds   *CredentialStore
}

func NewHandler(svc *Service, sampler *Sampler, hosts *HostStore, creds *CredentialStore) *Handler {
	return &Handler{svc: svc, sampler: sampler, hosts: hosts, creds: creds}
}

func (h *Handler) Register(rg *gin.RouterGroup) {
	g := rg.Group("/ups")
	g.GET("/snapshot", h.snapshot)
	g.GET("/stream", h.stream)
	g.GET("/series", h.series)
	g.POST("/refresh", h.refresh)

	g.GET("/hosts", h.listHosts)
	g.POST("/hosts", h.upsertHost)
	g.PUT("/hosts/:id", h.upsertHost)
	g.DELETE("/hosts/:id", h.deleteHost)
	g.POST("/hosts/:id/toggle", h.toggleHost)
	g.POST("/hosts/:id/test", h.testHost)

	g.GET("/credentials", h.listCredentials)
	g.POST("/credentials", h.upsertCredential)
	g.PUT("/credentials/:id", h.upsertCredential)
	g.DELETE("/credentials/:id", h.deleteCredential)
}

// parseRange 把人类时间窗映射到 time.Duration。未知值回落 24h。
func parseRange(s string) time.Duration {
	switch strings.TrimSpace(s) {
	case "1h":
		return 1 * time.Hour
	case "6h":
		return 6 * time.Hour
	case "12h":
		return 12 * time.Hour
	case "24h", "":
		return 24 * time.Hour
	case "3d":
		return 3 * 24 * time.Hour
	case "7d":
		return 7 * 24 * time.Hour
	default:
		return 24 * time.Hour
	}
}
