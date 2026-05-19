package acme

import (
	"bytes"
	"fmt"
	"strings"
	"time"
)

// teeWriter 把 lego 日志同时写入 buffer（最终落库）和 SSE hub（实时推送）。
type teeWriter struct {
	buf    *bytes.Buffer
	hub    *SSEHub
	taskID int64
}

func (w *teeWriter) Write(p []byte) (int, error) {
	w.buf.Write(p)
	// 按行拆分推送
	for _, line := range strings.Split(strings.TrimRight(string(p), "\n"), "\n") {
		if line == "" {
			continue
		}
		w.hub.Publish(w.taskID, line)
	}
	return len(p), nil
}

func logf(w *teeWriter, format string, args ...any) {
	line := fmt.Sprintf("["+time.Now().Format("15:04:05")+"] "+format, args...)
	w.buf.WriteString(line)
	w.buf.WriteByte('\n')
	w.hub.Publish(w.taskID, line)
}
