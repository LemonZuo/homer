package handler

import (
	"net/http"
	"strconv"

	"github.com/LemonZuo/homer/internal/birthday"
	"github.com/LemonZuo/homer/internal/model"
	"github.com/LemonZuo/homer/internal/notify/wework"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// BirthdayNotify 手动触发：对指定 ID 的记录立刻推送一次企业微信通知。
// 与定时任务共用消息构造逻辑，措辞按距下次农历生日的天数自适应。
func BirthdayNotify(db *gorm.DB, notifier *wework.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
			return
		}
		var item model.BirthdayRemind
		if err := db.Where("remind_id = ?", id).First(&item).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "record not found"})
			return
		}
		if notifier == nil || !notifier.Enabled() {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "企业微信未配置"})
			return
		}
		msg := birthday.BuildMessage(&item)
		if err := notifier.SendText(msg); err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true, "message": msg})
	}
}
