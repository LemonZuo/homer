package acme

import (
	"strings"
	"testing"
)

func TestWriteSSE(t *testing.T) {
	var b strings.Builder
	writeSSE(&b, "log", "hello")
	if got := b.String(); got != "event: log\ndata: hello\n\n" {
		t.Fatalf("single line = %q", got)
	}

	// 多行 data 按 SSE 规范逐行拆成多个 data:
	b.Reset()
	writeSSE(&b, "log", "line1\nline2")
	if got := b.String(); got != "event: log\ndata: line1\ndata: line2\n\n" {
		t.Fatalf("multi line = %q", got)
	}

	// 空 data 也要有一条 data: 保证事件完整
	b.Reset()
	writeSSE(&b, "done", "")
	if got := b.String(); got != "event: done\ndata: \n\n" {
		t.Fatalf("empty data = %q", got)
	}

	// 尾部换行不产生多余空 data 行
	b.Reset()
	writeSSE(&b, "log", "tail\n")
	if got := b.String(); got != "event: log\ndata: tail\n\n" {
		t.Fatalf("trailing newline = %q", got)
	}
}
