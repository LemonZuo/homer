package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/LemonZuo/homer/internal/event"
	"github.com/LemonZuo/homer/internal/logx"
	"github.com/LemonZuo/homer/internal/model"
	"github.com/LemonZuo/homer/internal/notify"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// EventHandler 负责一次性事项提醒 CRUD，并支持手动触发推送。
type EventHandler struct {
	db       *gorm.DB
	notifier notify.Notifier
}

func NewEventHandler(db *gorm.DB, notifier notify.Notifier) *EventHandler {
	return &EventHandler{db: db, notifier: notifier}
}

func (h *EventHandler) Register(api *gin.RouterGroup) {
	g := api.Group("/event")
	g.GET("", h.list)
	g.POST("", h.create)
	g.PUT("/:id", h.update)
	g.DELETE("/:id", h.delete)
	g.POST("/:id/notify", h.notify)
}

func (h *EventHandler) list(c *gin.Context) {
	var items []model.EventReminder
	if err := h.db.Order("id ASC").Find(&items).Error; err != nil {
		logx.Error("event list failed", "err", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": items})
}

func (h *EventHandler) create(c *gin.Context) {
	var item model.EventReminder
	if err := c.ShouldBindJSON(&item); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.db.Create(&item).Error; err != nil {
		logx.Error("event create failed", "err", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": item})
}

func (h *EventHandler) update(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var item model.EventReminder
	if err := c.ShouldBindJSON(&item); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.db.Model(&model.EventReminder{}).
		Where("id = ?", id).
		Select("*").
		Omit("id").
		Updates(item).Error; err != nil {
		logx.Error("event update failed", "id", id, "err", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": item})
}

func (h *EventHandler) delete(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	if err := h.db.Where("id = ?", id).Delete(&model.EventReminder{}).Error; err != nil {
		logx.Error("event delete failed", "id", id, "err", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// notify 手动触发：对指定 ID 的事项立刻推送一次，并刷新 last_notified_at。
func (h *EventHandler) notify(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var item model.EventReminder
	if err := h.db.Where("id = ?", id).First(&item).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "record not found"})
		return
	}
	if h.notifier == nil || !h.notifier.Enabled() {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "企业微信未配置"})
		return
	}
	now := time.Now()
	target, err := time.ParseInLocation("2006-01-02", item.EventDate, now.Location())
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "事项日期格式错误"})
		return
	}
	msg := event.BuildMessage(&item, target, now)
	if err := h.notifier.Send(c.Request.Context(), notify.Message{Text: msg}); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	if err := h.db.Model(&model.EventReminder{}).Where("id = ?", item.ID).Update("last_notified_at", now).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "message": msg})
}
