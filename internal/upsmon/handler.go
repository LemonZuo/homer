package upsmon

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// Handler 暴露 UPS 监控的 HTTP 接口。
type Handler struct {
	svc     *Service
	sampler *Sampler
}

func NewHandler(svc *Service, sampler *Sampler) *Handler {
	return &Handler{svc: svc, sampler: sampler}
}

func (h *Handler) Register(rg *gin.RouterGroup) {
	g := rg.Group("/ups")
	g.GET("/snapshot", h.snapshot)
	g.GET("/series", h.series)
	g.POST("/refresh", h.refresh)
	g.GET("/candidates", h.candidates)
	g.POST("/candidates/:id/toggle", h.toggle)
}

// snapshot 返回每台已订阅机器的最新 UPS 状态(不触发采样)。
func (h *Handler) snapshot(c *gin.Context) {
	data, err := h.svc.BuildSnapshot()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": data})
}

// series 返回指定 UPS 的历史曲线(已聚合)。
// 必填:host_kind / host_id / ups_name,可选 range=24h|7d(默认 24h)。
func (h *Handler) series(c *gin.Context) {
	hostKind := strings.TrimSpace(c.Query("host_kind"))
	upsName := strings.TrimSpace(c.Query("ups_name"))
	hostIDStr := strings.TrimSpace(c.Query("host_id"))
	if hostKind == "" || upsName == "" || hostIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "host_kind / host_id / ups_name 必填"})
		return
	}
	hostID, err := strconv.ParseInt(hostIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "host_id 无效"})
		return
	}
	window := parseRange(c.Query("range"))
	pts, err := h.svc.Series(hostKind, hostID, upsName, window)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"data":   pts,
		"range":  window.String(),
		"bucket": pickBucket(window),
	})
}

// refresh 手动触发一次采样(同步等待),供前端"立即刷新"按钮用。
func (h *Handler) refresh(c *gin.Context) {
	if err := h.svc.TriggerSample(); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	data, err := h.svc.BuildSnapshot()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": data})
}

// candidates 列出所有 ssh/fnos 目标(忽略 ups_monitor),供"订阅管理"页用。
func (h *Handler) candidates(c *gin.Context) {
	rows, err := h.sampler.ListCandidates()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	type item struct {
		ID         int64  `json:"id"`
		Name       string `json:"name"`
		Kind       string `json:"kind"`
		Endpoint   string `json:"endpoint"`
		UPSMonitor bool   `json:"ups_monitor"`
	}
	out := make([]item, 0, len(rows))
	for _, r := range rows {
		out = append(out, item{
			ID:         r.ID,
			Name:       r.Name,
			Kind:       r.Kind,
			Endpoint:   r.Endpoint,
			UPSMonitor: bool(r.UPSMonitor),
		})
	}
	c.JSON(http.StatusOK, gin.H{"data": out})
}

// toggle 切换某台机器的 ups_monitor 开关。Body: {"enable": true|false}。
func (h *Handler) toggle(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id 无效"})
		return
	}
	var body struct {
		Enable bool `json:"enable"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求体无效"})
		return
	}
	if err := h.sampler.SetMonitor(id, body.Enable); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"id": id, "ups_monitor": body.Enable}})
}

// parseRange 把人类时间窗映射到 time.Duration。未知值回落 24h。
func parseRange(s string) time.Duration {
	switch strings.TrimSpace(s) {
	case "1h":
		return 1 * time.Hour
	case "6h":
		return 6 * time.Hour
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
