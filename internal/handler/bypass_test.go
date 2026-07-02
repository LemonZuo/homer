package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/LemonZuo/homer/internal/notify"
)

type captureNotifier struct {
	enabled bool
	msgs    []notify.Message
}

func (c *captureNotifier) Name() string  { return "capture" }
func (c *captureNotifier) Enabled() bool { return c.enabled }
func (c *captureNotifier) Send(_ context.Context, m notify.Message) error {
	c.msgs = append(c.msgs, m)
	return nil
}

func postBypass(t *testing.T, h *BypassHandler, body string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h.Register(r.Group("/api"))
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/byPass/receive", strings.NewReader(body))
	r.ServeHTTP(w, req)
	return w
}

func TestBypassReceiveForwards(t *testing.T) {
	n := &captureNotifier{enabled: true}
	// 12306Bypass 推送带首尾引号的 JSON 字符串,应剥掉引号
	w := postBypass(t, NewBypassHandler(n), `"车票候补成功"`)
	if w.Code != 200 {
		t.Fatalf("code = %d", w.Code)
	}
	if len(n.msgs) != 1 || n.msgs[0].Title != "分流通知" || n.msgs[0].Text != "车票候补成功" {
		t.Fatalf("msgs = %+v", n.msgs)
	}
}

func TestBypassReceiveEmptySkipped(t *testing.T) {
	n := &captureNotifier{enabled: true}
	w := postBypass(t, NewBypassHandler(n), `  ""  `)
	if w.Code != 200 || !strings.Contains(w.Body.String(), "skipped") {
		t.Fatalf("empty body = %d %q", w.Code, w.Body.String())
	}
	if len(n.msgs) != 0 {
		t.Fatalf("empty content must not forward: %+v", n.msgs)
	}
}

func TestBypassReceiveDisabledNotifier(t *testing.T) {
	n := &captureNotifier{enabled: false}
	if w := postBypass(t, NewBypassHandler(n), "hi"); w.Code != 200 {
		t.Fatalf("code = %d", w.Code)
	}
	if len(n.msgs) != 0 {
		t.Fatalf("disabled notifier must not send: %+v", n.msgs)
	}
	// nil Notifier 不 panic
	if w := postBypass(t, NewBypassHandler(nil), "hi"); w.Code != 200 {
		t.Fatalf("nil notifier code = %d", w.Code)
	}
}
