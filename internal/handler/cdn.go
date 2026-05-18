package handler

import (
	"errors"
	"net/http"

	"github.com/LemonZuo/homer/internal/cdnops"

	"github.com/gin-gonic/gin"
)

// CDNOpsHandler 加速域名运维接口（只读视图）。证书部署走证书库存 handler。
type CDNOpsHandler struct {
	svc *cdnops.Service
}

func NewCDNOpsHandler(svc *cdnops.Service) *CDNOpsHandler {
	return &CDNOpsHandler{svc: svc}
}

func (h *CDNOpsHandler) Register(rg *gin.RouterGroup) {
	g := rg.Group("/cdn")
	g.GET("/domains", h.domains)
}

func (h *CDNOpsHandler) writeErr(c *gin.Context, err error) {
	if errors.Is(err, cdnops.ErrNotConfigured) {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "阿里云 CDN 未配置"})
		return
	}
	c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
}

func (h *CDNOpsHandler) domains(c *gin.Context) {
	items, err := h.svc.ListDomains()
	if err != nil {
		h.writeErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": items})
}
