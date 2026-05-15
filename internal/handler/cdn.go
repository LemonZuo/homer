package handler

import (
	"errors"
	"net/http"

	"github.com/LemonZuo/homer/internal/cdn"

	"github.com/gin-gonic/gin"
)

// CDNHandler 加速域名管理接口（只读视图 + 证书部署）。
type CDNHandler struct {
	svc *cdn.Service
}

func NewCDNHandler(svc *cdn.Service) *CDNHandler {
	return &CDNHandler{svc: svc}
}

func (h *CDNHandler) Register(rg *gin.RouterGroup) {
	g := rg.Group("/cdn")
	g.GET("/domains", h.domains)
	g.GET("/certificates", h.certificates)
	g.POST("/deploy", h.deploy)
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

func (h *CDNHandler) certificates(c *gin.Context) {
	items, err := h.svc.ListCertificates()
	if err != nil {
		h.writeErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": items})
}

func (h *CDNHandler) deploy(c *gin.Context) {
	var body struct {
		CertName string `json:"certName"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}
	msg, err := h.svc.DeployCertificate(body.CertName)
	if err != nil {
		h.writeErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": msg})
}
