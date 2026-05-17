package handler

import (
	"net/http"
	"strconv"

	"github.com/LemonZuo/homer/internal/model"
	"github.com/LemonZuo/homer/internal/notify"

	"github.com/gin-gonic/gin"
)

// NotifyHandler 通知渠道配置中心：通道 CRUD + 模块绑定 + 测试。
type NotifyHandler struct{ store *notify.Store }

func NewNotifyHandler(store *notify.Store) *NotifyHandler { return &NotifyHandler{store: store} }

func (h *NotifyHandler) Register(api *gin.RouterGroup) {
	g := api.Group("/notify")
	g.GET("/meta", h.meta)
	g.GET("/channels", h.listChannels)
	g.POST("/channels", h.upsertChannel)
	g.PUT("/channels/:id", h.upsertChannel)
	g.DELETE("/channels/:id", h.deleteChannel)
	g.POST("/channels/:id/test", h.testChannel)
	g.GET("/bindings", h.bindings)
	g.PUT("/bindings/:module", h.setBinding)
}

func (h *NotifyHandler) meta(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"data": gin.H{
		"modules": notify.Modules,
		"types":   notify.ChannelTypes,
	}})
}

func (h *NotifyHandler) listChannels(c *gin.Context) {
	rows, err := h.store.ListChannels()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": rows})
}

func (h *NotifyHandler) upsertChannel(c *gin.Context) {
	var ch model.NotifyChannel
	if err := c.ShouldBindJSON(&ch); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}
	if idStr := c.Param("id"); idStr != "" {
		id, _ := strconv.ParseInt(idStr, 10, 64)
		ch.ID = id
	}
	row, err := h.store.UpsertChannel(&ch)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": row})
}

func (h *NotifyHandler) deleteChannel(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 id"})
		return
	}
	if err := h.store.DeleteChannel(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "已删除"})
}

func (h *NotifyHandler) testChannel(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 id"})
		return
	}
	if err := h.store.Test(id); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "已发送测试消息"})
}

func (h *NotifyHandler) bindings(c *gin.Context) {
	m, err := h.store.Bindings()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": m})
}

func (h *NotifyHandler) setBinding(c *gin.Context) {
	module := c.Param("module")
	var body struct {
		ChannelIDs []int64 `json:"channel_ids"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}
	if err := h.store.SetBinding(module, body.ChannelIDs); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "已保存"})
}
