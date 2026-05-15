package handler

import (
	"errors"
	"net/http"

	"github.com/LemonZuo/homer/internal/cdn"

	"github.com/gin-gonic/gin"
)

// CDNHandler 加速域名管理接口（只读视图）。证书部署走 CAS handler。
type CDNHandler struct {
	svc *cdn.Service
}

func NewCDNHandler(svc *cdn.Service) *CDNHandler {
	return &CDNHandler{svc: svc}
}

func (h *CDNHandler) Register(rg *gin.RouterGroup) {
	g := rg.Group("/cdn")
	g.GET("/domains", h.domains)
}

func (h *CDNHandler) writeErr(c *gin.Context, err error) {
	if errors.Is(err, cdn.ErrNotConfigured) {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "阿里云 CDN 未配置"})
		return
	}
	c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
}

func (h *CDNHandler) domains(c *gin.Context) {
	items, err := h.svc.ListDomains()
	if err != nil {
		h.writeErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": items})
}
