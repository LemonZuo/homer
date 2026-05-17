// Package logx 是项目统一日志门面：标准库 log/slog，零第三方依赖，只输出到
// 控制台（stderr），不落文件。级别由 LOG_LEVEL 控制（debug|info|warn|error）。
package logx

import (
	"context"
	"log"
	"log/slog"
	"os"
	"strings"
)

var base *slog.Logger

// Init 初始化全局 logger，并把标准库 log 的输出也桥接到 slog，
// 保证第三方库（如 gorm 默认 logger）经 log 打的日志格式一致。
func Init(level string) {
	lv := parseLevel(level)
	h := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: lv,
		ReplaceAttr: func(_ []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey {
				a.Value = slog.StringValue(a.Value.Time().Format("2006-01-02 15:04:05.000"))
			}
			return a
		},
	})
	base = slog.New(h)
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
