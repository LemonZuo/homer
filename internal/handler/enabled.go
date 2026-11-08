package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/LemonZuo/homer/internal/logx"
	"github.com/LemonZuo/homer/internal/model"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// setEnabledByID 只更新一条记录的 enabled 列（body: {"enabled": bool}）。
// row 传对应模型的零值指针，例如 &model.EventReminder{}。
func setEnabledByID(c *gin.Context, db *gorm.DB, row any) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var body struct {
		Enabled *bool `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.Enabled == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "enabled 必填"})
		return
	}
	if err := db.Select("id").First(row, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "record not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if err := db.Model(row).Update("enabled", model.BoolFlag(*body.Enabled)).Error; err != nil {
		logx.Error("set enabled failed", "id", id, "err", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
