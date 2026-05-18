package acme

import (
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

func (h *Handler) listTasks(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))
	status := c.Query("status")
	switch status {
	case "", "pending", "running", "success", "failed", "retrying":
	default:
		status = ""
	}
	items, total, err := h.svc.ListTasks(page, pageSize, status)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 10
	}
	c.JSON(http.StatusOK, gin.H{"data": items, "total": total, "page": page, "page_size": pageSize})
}

func (h *Handler) getTask(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 id"})
		return
	}
	t, err := h.svc.GetTask(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if t == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "任务不存在"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": t})
}

func (h *Handler) retryTask(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 id"})
		return
	}
	if err := h.svc.RetryDeployTaskNow(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"task_id": id}})
}

// streamTask SSE 推送任务日志。若任务已结束（FinishedAt 非空），
// 直接一次性发完 log_text 并关闭；运行中则订阅 hub。
func (h *Handler) streamTask(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 id"})
		return
	}
	t, err := h.svc.GetTask(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if t == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "任务不存在"})
		return
	}

	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no")
	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "ResponseWriter 不支持流式输出"})
		return
	}

	// 已结束：直接吐全文 + done
	if t.FinishedAt != nil {
		writeSSE(c.Writer, "log", t.LogText)
		writeSSE(c.Writer, "done", t.Status)
		flusher.Flush()
		return
	}

	ch, unsub := h.svc.Hub().Subscribe(id)
	defer unsub()

	// 先把已有的 log_text 当作回放发出
	if t.LogText != "" {
		writeSSE(c.Writer, "log", t.LogText)
		flusher.Flush()
	}

	notify := c.Request.Context().Done()
	for {
		select {
		case line, ok := <-ch:
			if !ok {
				// 任务结束：再查一次状态
				if final, err := h.svc.GetTask(id); err == nil && final != nil {
					writeSSE(c.Writer, "done", final.Status)
					flusher.Flush()
				}
				return
			}
			writeSSE(c.Writer, "log", line)
			flusher.Flush()
		case <-notify:
			return
		}
	}
}

// writeSSE 写一条 SSE 事件；多行 data 自动按行拆分（SSE 规范）。
func writeSSE(w io.Writer, event, data string) {
	fmt.Fprintf(w, "event: %s\n", event)
	// 按行拆分，每行一个 data:
	start := 0
	for i := 0; i < len(data); i++ {
		if data[i] == '\n' {
			fmt.Fprintf(w, "data: %s\n", data[start:i])
			start = i + 1
		}
	}
	if start < len(data) {
		fmt.Fprintf(w, "data: %s\n", data[start:])
	}
	if start == len(data) && len(data) == 0 {
		fmt.Fprint(w, "data: \n")
	}
	fmt.Fprint(w, "\n")
}
