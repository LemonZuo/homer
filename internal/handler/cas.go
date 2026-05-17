package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/LemonZuo/homer/internal/cas"
	"github.com/LemonZuo/homer/internal/cdn"
	"github.com/LemonZuo/homer/internal/logx"

	"github.com/gin-gonic/gin"
)

// CASHandler 数字证书管理接口；「部署到 CDN」复用 cdn.Service。
type CASHandler struct {
	svc *cas.Service
	cdn *cdn.Service
}

func NewCASHandler(svc *cas.Service, cdnSvc *cdn.Service) *CASHandler {
	return &CASHandler{svc: svc, cdn: cdnSvc}
}

func (h *CASHandler) Register(rg *gin.RouterGroup) {
	g := rg.Group("/cas")
	g.GET("/certificates", h.certificates)
	g.DELETE("/certificates/:id", h.delete)
	g.POST("/deploy", h.deploy)
}

func (h *CASHandler) writeErr(c *gin.Context, err error) {
	if errors.Is(err, cas.ErrNotConfigured) {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "阿里云 CAS 未配置"})
		return
	}
	c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
}

func (h *CASHandler) certificates(c *gin.Context) {
	items, err := h.svc.ListCertificates()
	if err != nil {
		h.writeErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": items})
}

func (h *CASHandler) delete(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的证书 ID"})
		return
	}
	if err := h.svc.DeleteCertificate(id); err != nil {
		h.writeErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "已删除"})
}

// deploy 复刻老 Java 行为：异步执行部署，接口立即返回任务已提交。
func (h *CASHandler) deploy(c *gin.Context) {
	var body struct {
		CertName string `json:"certName"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.CertName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "certName 不能为空"})
		return
	}
	certName := body.CertName
	go func() {
		logx.Info("cas deploy to cdn start", "cert_name", certName)
		msg, err := h.cdn.DeployCertificate(certName)
		if err != nil {
			logx.Error("cas deploy to cdn failed", "cert_name", certName, "err", err)
			return
		}
		logx.Info("cas deploy to cdn done", "cert_name", certName, "result", msg)
	}()
	c.JSON(http.StatusOK, gin.H{"message": "证书部署任务已提交，结果见服务端日志"})
}
