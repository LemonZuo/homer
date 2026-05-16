package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/LemonZuo/homer/internal/event"
	"github.com/LemonZuo/homer/internal/model"
	"github.com/LemonZuo/homer/internal/notify"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// EventNotify 手动触发：对指定 ID 的事项立刻推送一次，并刷新 last_notified_at。
func EventNotify(db *gorm.DB, notifier notify.Notifier) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
			return
		}
		var item model.EventReminder
		if err := db.Where("id = ?", id).First(&item).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "record not found"})
			return
		}
		if notifier == nil || !notifier.Enabled() {
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
		if err := notifier.Send(c.Request.Context(), notify.Message{Text: msg}); err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
			return
		}
		if err := db.Model(&model.EventReminder{}).Where("id = ?", item.ID).Update("last_notified_at", now).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true, "message": msg})
	}
}
