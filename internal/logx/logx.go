// Package logx 是项目统一日志门面：标准库 log/slog + 自定义可读 handler，
// 零第三方依赖，只输出到控制台（stderr），不落文件。
// 级别由 LOG_LEVEL 控制（debug|info|warn|error）。
//
// 输出格式：2026/05/19 - 00:30:57.450 | INFO  | msg | k=v k=v
package logx

import (
	"context"
	"io"
	"log"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"sync"
)

var base *slog.Logger

// Init 初始化全局 logger，并把标准库 log 的输出也桥接到 slog，
// 保证第三方库（如 gorm 默认 logger）经 log 打的日志格式一致。
func Init(level string) {
	lv := parseLevel(level)
	base = slog.New(newHandler(os.Stderr, lv))
	slog.SetDefault(base)

	// 标准库 log.Printf / log.Fatalf 的输出统一走 slog（Info 级）。
	log.SetFlags(0)
	log.SetPrefix("")
	log.SetOutput(stdlogBridge{})
}

func parseLevel(s string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// stdlogBridge 把标准库 log 的每行输出转成一条 slog.Info。
type stdlogBridge struct{}

func (stdlogBridge) Write(p []byte) (int, error) {
	msg := strings.TrimRight(string(p), "\n")
	logger().LogAttrs(context.Background(), slog.LevelInfo, msg)
	return len(p), nil
}

func logger() *slog.Logger {
	if base == nil {
		return slog.Default()
	}
	return base
}

// With 返回带固定字段的子 logger，常用于按模块打 component=xxx。
func With(args ...any) *slog.Logger { return logger().With(args...) }

func Debug(msg string, args ...any) { logger().Debug(msg, args...) }
func Info(msg string, args ...any)  { logger().Info(msg, args...) }
func Warn(msg string, args ...any)  { logger().Warn(msg, args...) }
func Error(msg string, args ...any) { logger().Error(msg, args...) }

// Fatal 打一条 error 日志后退出进程（替代 log.Fatalf）。
func Fatal(msg string, args ...any) {
	logger().Error(msg, args...)
	os.Exit(1)
}

// handler 自定义 slog.Handler：人类可读的「时间 | 级别 | msg | k=v」单行格式。
type handler struct {
	w      io.Writer
	level  slog.Level
	mu     *sync.Mutex
	attrs  []slog.Attr
	groups []string
}

func newHandler(w io.Writer, level slog.Level) *handler {
	return &handler{w: w, level: level, mu: &sync.Mutex{}}
}

func (h *handler) Enabled(_ context.Context, lv slog.Level) bool { return lv >= h.level }

func (h *handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	if len(attrs) == 0 {
		return h
	}
	cp := *h
	cp.attrs = append(append([]slog.Attr{}, h.attrs...), attrs...)
	return &cp
}

func (h *handler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	cp := *h
	cp.groups = append(append([]string{}, h.groups...), name)
	return &cp
}

func (h *handler) Handle(_ context.Context, r slog.Record) error {
	var b strings.Builder
	b.Grow(128)
	b.WriteString(r.Time.Format("2006/01/02 - 15:04:05.000"))
	b.WriteString(" | ")
	b.WriteString(levelLabel(r.Level))
	b.WriteString(" | ")
	b.WriteString(r.Message)

	prefix := strings.Join(h.groups, ".")
	first := true
	writeAttr := func(a slog.Attr) {
		if a.Equal(slog.Attr{}) {
			return
		}
		if first {
			b.WriteString(" | ")
			first = false
		} else {
			b.WriteByte(' ')
		}
		key := a.Key
		if prefix != "" {
			key = prefix + "." + key
		}
		b.WriteString(key)
		b.WriteByte('=')
		b.WriteString(formatValue(a.Value))
	}
	for _, a := range h.attrs {
		writeAttr(a)
	}
	r.Attrs(func(a slog.Attr) bool {
		writeAttr(a)
		return true
	})
	b.WriteByte('\n')

	h.mu.Lock()
	defer h.mu.Unlock()
	_, err := io.WriteString(h.w, b.String())
	return err
}

func levelLabel(l slog.Level) string {
	switch {
	case l >= slog.LevelError:
		return "ERROR"
	case l >= slog.LevelWarn:
		return "WARN "
	case l >= slog.LevelInfo:
		return "INFO "
	default:
		return "DEBUG"
	}
}

func formatValue(v slog.Value) string {
	v = v.Resolve()
	s := v.String()
	if needsQuoting(s) {
		return strconv.Quote(s)
	}
	return s
}

func needsQuoting(s string) bool {
	if s == "" {
		return true
	}
	for _, c := range s {
		if c <= ' ' || c == '"' || c == '\\' || c == '=' {
			return true
		}
	}
	return false
}
