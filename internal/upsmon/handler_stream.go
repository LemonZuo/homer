package upsmon

// UPS 快照、SSE、历史曲线和刷新接口。

import (
	"strconv"
	"strings"
	"time"

	"encoding/json"
	"github.com/gin-gonic/gin"
	"net/http"
)

// stream SSE 推送 snapshot,替代前端 30 秒一次的 HTTP 轮询。
// 协议:订阅时立即发一帧当前 snapshot(避免空白等下一轮),之后每轮采样完推一帧。
// 25 秒一次的注释行心跳(`: ping`)防止反代/浏览器把 idle 连接掐掉。
func (h *Handler) stream(c *gin.Context) {
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no")
	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "ResponseWriter 不支持流式输出"})
		return
	}

	send := func(snap []Snapshot) bool {
		buf, err := json.Marshal(snap)
		if err != nil {
			return false
		}
		if _, err := c.Writer.WriteString("event: snapshot\ndata: "); err != nil {
			return false
		}
		if _, err := c.Writer.Write(buf); err != nil {
			return false
		}
		if _, err := c.Writer.WriteString("\n\n"); err != nil {
			return false
		}
		flusher.Flush()
		return true
	}

	ch, unsub := h.svc.Subscribe()
	defer unsub()

	if snap, err := h.svc.BuildSnapshot(); err == nil {
		if !send(snap) {
			return
		}
	}

	ping := time.NewTicker(25 * time.Second)
	defer ping.Stop()
	ctxDone := c.Request.Context().Done()
	for {
		select {
		case snap, ok := <-ch:
			if !ok {
				return
			}
			if !send(snap) {
				return
			}
		case <-ping.C:
			if _, err := c.Writer.WriteString(": ping\n\n"); err != nil {
				return
			}
			flusher.Flush()
		case <-ctxDone:
			return
		}
	}
}

// snapshot 返回每台已订阅机器的最新 UPS 状态(不触发采样)。
func (h *Handler) snapshot(c *gin.Context) {
	data, err := h.svc.BuildSnapshot()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": data})
}

// series 返回指定 UPS 的历史曲线(已聚合)。
// 必填:host_kind / host_id / ups_name,可选 range=24h|7d(默认 24h)。
func (h *Handler) series(c *gin.Context) {
	hostKind := strings.TrimSpace(c.Query("host_kind"))
	upsName := strings.TrimSpace(c.Query("ups_name"))
	hostIDStr := strings.TrimSpace(c.Query("host_id"))
	if hostKind == "" || upsName == "" || hostIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "host_kind / host_id / ups_name 必填"})
		return
	}
	hostID, err := strconv.ParseInt(hostIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "host_id 无效"})
		return
	}
	window := parseRange(c.Query("range"))
	pts, err := h.svc.Series(hostKind, hostID, upsName, window)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"data":   pts,
		"range":  window.String(),
		"bucket": pickBucket(window),
	})
}

// refresh 手动触发一次采样(同步等待),供前端"立即刷新"按钮用。
func (h *Handler) refresh(c *gin.Context) {
	if err := h.svc.TriggerSample(); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	data, err := h.svc.BuildSnapshot()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": data})
}
