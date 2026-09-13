package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/LemonZuo/homer/internal/logx"
	"github.com/LemonZuo/homer/internal/modulecontrol"
	"github.com/gin-gonic/gin"
)

type ModulesHandler struct{ modules *modulecontrol.Manager }

func NewModulesHandler(m *modulecontrol.Manager) *ModulesHandler { return &ModulesHandler{m} }

func (h *ModulesHandler) Register(api *gin.RouterGroup) {
	api.GET("/system/modules", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"data": h.modules.Snapshot()}) })
	api.PUT("/system/modules/:key", h.update)
	api.GET("/system/modules/stream", h.stream)
}

func (h *ModulesHandler) update(c *gin.Context) {
	var req struct {
		Enabled *bool  `json:"enabled" binding:"required"`
		Version uint64 `json:"version" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请提供 enabled 和当前 version"})
		return
	}
	snap, err := h.modules.Set(c.Request.Context(), c.Param("key"), *req.Enabled, req.Version)
	if err != nil {
		code := http.StatusInternalServerError
		if errors.Is(err, modulecontrol.ErrConflict) {
			code = http.StatusConflict
		}
		if errors.Is(err, modulecontrol.ErrUnknown) {
			code = http.StatusBadRequest
		}
		c.JSON(code, gin.H{"error": err.Error()})
		return
	}
	logx.Info("module setting updated", "module", c.Param("key"), "enabled", *req.Enabled, "client_ip", c.ClientIP())
	c.JSON(http.StatusOK, gin.H{"data": snap})
}

func (h *ModulesHandler) stream(c *gin.Context) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("X-Accel-Buffering", "no")
	ch, unsub := h.modules.Subscribe()
	defer unsub()
	ping := time.NewTicker(25 * time.Second)
	defer ping.Stop()
	for {
		select {
		case snap := <-ch:
			data, err := json.Marshal(snap)
			if err != nil {
				return
			}
			if _, err := fmt.Fprintf(c.Writer, "event: modules\ndata: %s\n\n", data); err != nil {
				return
			}
			c.Writer.Flush()
		case <-ping.C:
			if _, err := c.Writer.WriteString(": ping\n\n"); err != nil {
				return
			}
			c.Writer.Flush()
		case <-c.Request.Context().Done():
			return
		}
	}
}
