package handler

import (
	"net/http"
	"strconv"

	"github.com/LemonZuo/homer/internal/birthday"
	"github.com/LemonZuo/homer/internal/logx"
	"github.com/LemonZuo/homer/internal/model"
	"github.com/LemonZuo/homer/internal/notify"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// BirthdayHandler 负责生日提醒 CRUD，并在保存时回填农历生日/生肖。
type BirthdayHandler struct {
	db       *gorm.DB
	notifier notify.Notifier
}

func NewBirthdayHandler(db *gorm.DB, notifier notify.Notifier) *BirthdayHandler {
	return &BirthdayHandler{db: db, notifier: notifier}
}

func (h *BirthdayHandler) Register(api *gin.RouterGroup) {
	g := api.Group("/birthday")
	g.GET("", h.list)
	g.POST("", h.create)
	g.PUT("/:id", h.update)
	g.DELETE("/:id", h.delete)
	g.POST("/:id/notify", h.notify)
}

func (h *BirthdayHandler) list(c *gin.Context) {
	var items []model.BirthdayRemind
	if err := h.db.Order("id ASC").Find(&items).Error; err != nil {
		logx.Error("birthday list failed", "err", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": items})
}

func (h *BirthdayHandler) create(c *gin.Context) {
	var item model.BirthdayRemind
	if err := c.ShouldBindJSON(&item); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := birthday.FillDerivedFields(&item); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.db.Create(&item).Error; err != nil {
		logx.Error("birthday create failed", "err", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": item})
}

func (h *BirthdayHandler) update(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var item model.BirthdayRemind
	if err := c.ShouldBindJSON(&item); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	item.ID = id
	if err := birthday.FillDerivedFields(&item); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.db.Model(&model.BirthdayRemind{}).
		Where("id = ?", id).
		Select("*").
		Omit("id").
		Updates(item).Error; err != nil {
		logx.Error("birthday update failed", "id", id, "err", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": item})
}

func (h *BirthdayHandler) delete(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	if err := h.db.Where("id = ?", id).Delete(&model.BirthdayRemind{}).Error; err != nil {
		logx.Error("birthday delete failed", "id", id, "err", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// notify 手动触发：对指定 ID 的记录立刻推送一次企业微信通知。
// 与定时任务共用消息构造逻辑，措辞按距下次农历生日的天数自适应。
func (h *BirthdayHandler) notify(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var item model.BirthdayRemind
	if err := h.db.Where("id = ?", id).First(&item).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "record not found"})
		return
	}
	if h.notifier == nil || !h.notifier.Enabled() {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "企业微信未配置"})
		return
	}
	msg := birthday.BuildMessage(&item)
	if err := h.notifier.Send(c.Request.Context(), notify.Message{Text: msg}); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "message": msg})
}
