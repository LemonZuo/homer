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
// 转发到企业微信 + Resend 邮件。任意一路失败不影响另一路。
type BypassHandler struct {
	WeWork  notify.Notifier
	Email   notify.Notifier
	Subject string
}

func NewBypassHandler(w, e notify.Notifier, subject string) *BypassHandler {
	return &BypassHandler{WeWork: w, Email: e, Subject: subject}
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

	subject := h.Subject
	if subject == "" {
		subject = "分流通知"
	}

	ctx := c.Request.Context()
	if h.WeWork != nil && h.WeWork.Enabled() {
		if err := h.WeWork.Send(ctx, notify.Message{Text: content}); err != nil {
			log.Printf("bypass wework send: %v", err)
		}
	}
	if h.Email != nil && h.Email.Enabled() {
		if err := h.Email.Send(ctx, notify.Message{Title: subject, Text: content}); err != nil {
			log.Printf("bypass email send: %v", err)
		}
	}

	c.JSON(http.StatusOK, gin.H{"ok": true})
}
