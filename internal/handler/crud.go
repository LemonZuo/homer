package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// CRUD 为带自增 ID 主键的表生成通用 handler。
type CRUD[T any] struct {
	DB *gorm.DB
}

func NewCRUD[T any](db *gorm.DB) *CRUD[T] {
	return &CRUD[T]{DB: db}
}

func (c *CRUD[T]) Register(rg *gin.RouterGroup, path string) {
	g := rg.Group(path)
	g.GET("", c.List)
	g.POST("", c.Create)
	g.PUT("/:id", c.Update)
	g.DELETE("/:id", c.Delete)
}

func (c *CRUD[T]) List(ctx *gin.Context) {
	var items []T
	q := c.DB.Order("id ASC")
	if kw := ctx.Query("kw"); kw != "" {
		// 简单的全列模糊搜索：交给前端各表自己传，这里只是预留位
		_ = kw
	}
	if err := q.Find(&items).Error; err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"data": items})
}

func (c *CRUD[T]) Create(ctx *gin.Context) {
	var item T
	if err := ctx.ShouldBindJSON(&item); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := c.DB.Create(&item).Error; err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"data": item})
}

func (c *CRUD[T]) Update(ctx *gin.Context) {
	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var item T
	if err := ctx.ShouldBindJSON(&item); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	// 用 map 更新会丢字段；用结构体 + Select("*") 全字段覆盖
	if err := c.DB.Model(&item).Where("id = ?", id).Select("*").Omit("id").Updates(item).Error; err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"data": item})
}

func (c *CRUD[T]) Delete(ctx *gin.Context) {
	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var item T
	if err := c.DB.Where("id = ?", id).Delete(&item).Error; err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"ok": true})
}
