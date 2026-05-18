package acme

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// parseID 取路径参数 :id 并校验为正整数；非法时已写好 400 响应，ok=false 时调用方直接 return。
func parseID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 id"})
		return 0, false
	}
	return id, true
}

// bindJSON 反序列化请求体；失败时已写好 400 响应，ok=false 时调用方直接 return。
func bindJSON(c *gin.Context, v any) bool {
	if err := c.ShouldBindJSON(v); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return false
	}
	return true
}

// badReq 若 err 非空，写 400 并返回 true（调用方应 return）；err==nil 时返回 false 继续。
func badReq(c *gin.Context, err error) bool {
	if err == nil {
		return false
	}
	c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	return true
}

// serverErr 若 err 非空，写 500 并返回 true（调用方应 return）；err==nil 时返回 false 继续。
func serverErr(c *gin.Context, err error) bool {
	if err == nil {
		return false
	}
	c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	return true
}
