package handler

import (
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/LemonZuo/homer/internal/notify"

	"github.com/gin-gonic/gin"
)

// BypassHandler 接收 12306Bypass 分流抢票助手推过来的 webhook 通知，
// 经扇出 Notifier 转发到配置的各通道；任意一路失败不影响另一路。
type BypassHandler struct {
	Notifier notify.Notifier
}

func NewBypassHandler(n notify.Notifier) *BypassHandler {
	return &BypassHandler{Notifier: n}
}

func (h *BypassHandler) Register(api *gin.RouterGroup) {
	// 路径与老 Java 项目保持一致：12306Bypass 桌面助手里配置的 webhook 直接复用
	api.POST("/byPass/receive", h.receive)
}

func (h *BypassHandler) receive(c *gin.Context) {
	raw, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "read body"})
		return
	}
	content := strings.TrimSpace(string(raw))
	// 12306Bypass 推送的是带首尾引号的 JSON 字符串，去掉以保持可读
	content = strings.Trim(content, "\"")
	if content == "" {
		c.JSON(http.StatusOK, gin.H{"ok": true, "skipped": true})
		return
	}
	log.Printf("bypass receive: %s", content)

	ctx := c.Request.Context()
	if h.Notifier != nil && h.Notifier.Enabled() {
		if err := h.Notifier.Send(ctx, notify.Message{Title: "分流通知", Text: content}); err != nil {
			log.Printf("bypass send: %v", err)
		}
	}

	c.JSON(http.StatusOK, gin.H{"ok": true})
}
