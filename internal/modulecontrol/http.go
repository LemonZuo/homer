package modulecontrol

import (
	"context"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// Guard 统一覆盖业务路由。流连接随本启用周期结束而关闭，其余已准入请求安全收尾。
func (m *Manager) Guard() gin.HandlerFunc {
	return func(c *gin.Context) {
		path := strings.TrimPrefix(c.Request.URL.Path, "/api/")
		key, _, _ := strings.Cut(path, "/")
		if key == "byPass" {
			key = "bypass"
		}
		if !Known(key) {
			c.Next()
			return
		}
		ticket, err := m.Ticket(key)
		if err != nil {
			disabledResponse(c, key)
			return
		}
		if strings.HasSuffix(path, "/stream") {
			ctx, cancel := context.WithCancel(c.Request.Context())
			defer cancel()
			go func() {
				select {
				case <-ticket.Done():
					cancel()
				case <-ctx.Done():
				}
			}()
			c.Request = c.Request.WithContext(ctx)
		} else {
			finish, err := ticket.Begin()
			if err != nil {
				disabledResponse(c, key)
				return
			}
			defer finish()
		}
		c.Next()
	}
}

func disabledResponse(c *gin.Context, key string) {
	c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"code": "MODULE_DISABLED", "module": key, "error": "该模块已禁用，请在模块管理中启用"})
}
