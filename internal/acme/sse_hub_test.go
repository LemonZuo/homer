package acme

import (
	"bytes"
	"strings"
	"testing"
)

func TestSSEHubSubscribePublish(t *testing.T) {
	h := NewSSEHub()
	ch, unsub := h.Subscribe(1)
	defer unsub()

	h.Publish(1, "line1")
	h.Publish(2, "other task") // 不同任务不串流
	select {
	case got := <-ch:
		if got != "line1" {
			t.Fatalf("got %q", got)
		}
	default:
		t.Fatal("expected buffered line")
	}
	select {
	case got := <-ch:
		t.Fatalf("unexpected extra line %q", got)
	default:
	}
}

func TestSSEHubCloseEndsSubscribers(t *testing.T) {
	h := NewSSEHub()
	ch, unsub := h.Subscribe(1)
	h.Close(1)
	if _, ok := <-ch; ok {
		t.Fatal("channel must be closed after Close")
	}
	// Close 后 unsubscribe 幂等,不 panic
	unsub()
	// Close 后 Publish 无订阅者,不 panic
	h.Publish(1, "x")
}

func TestSSEHubUnsubscribeIdempotent(t *testing.T) {
	h := NewSSEHub()
	ch, unsub := h.Subscribe(5)
	unsub()
	unsub()
	if _, ok := <-ch; ok {
		t.Fatal("channel must be closed after unsubscribe")
	}
}

func TestSSEHubFullChannelDoesNotBlock(t *testing.T) {
	h := NewSSEHub()
	_, unsub := h.Subscribe(1)
	defer unsub()
	// buffer 128,发 200 条不阻塞(满了丢弃)
	for i := 0; i < 200; i++ {
		h.Publish(1, "flood")
	}
}

func TestTeeWriter(t *testing.T) {
	h := NewSSEHub()
	ch, unsub := h.Subscribe(9)
	defer unsub()

	w := &teeWriter{buf: &bytes.Buffer{}, hub: h, taskID: 9}
	n, err := w.Write([]byte("a\nb\n"))
	if err != nil || n != 4 {
		t.Fatalf("n=%d err=%v", n, err)
	}
	if w.buf.String() != "a\nb\n" {
		t.Fatalf("buf = %q", w.buf.String())
	}
	// 按行推送,空行跳过
	if got := <-ch; got != "a" {
		t.Fatalf("first = %q", got)
	}
	if got := <-ch; got != "b" {
		t.Fatalf("second = %q", got)
	}

	logf(w, "issue %s", "example.com")
	line := <-ch
	if !strings.HasSuffix(line, "issue example.com") || !strings.HasPrefix(line, "[") {
		t.Fatalf("logf line = %q", line)
	}
	if !strings.Contains(w.buf.String(), "issue example.com") {
		t.Fatalf("buf missing logf line: %q", w.buf.String())
	}
}
