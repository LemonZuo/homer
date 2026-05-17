package logx

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// GormLogger 把 GORM 日志接到 slog。错误（除 record not found）→error，
// 慢查询→warn，普通 SQL 仅在 LOG_LEVEL=debug 时输出。
type GormLogger struct {
	slowThreshold time.Duration
	debug         bool
}

// NewGormLogger debug 为 true 时打印每条 SQL（对应 LOG_LEVEL=debug）。
func NewGormLogger(debug bool) GormLogger {
	return GormLogger{slowThreshold: 300 * time.Millisecond, debug: debug}
}

func (g GormLogger) LogMode(gormlogger.LogLevel) gormlogger.Interface { return g }

func (GormLogger) Info(_ context.Context, msg string, data ...any) {
	With("component", "gorm").Debug(msg, "data", data)
}

func (GormLogger) Warn(_ context.Context, msg string, data ...any) {
	With("component", "gorm").Warn(msg, "data", data)
}

func (GormLogger) Error(_ context.Context, msg string, data ...any) {
	With("component", "gorm").Error(msg, "data", data)
}

func (g GormLogger) Trace(_ context.Context, begin time.Time, fc func() (string, int64), err error) {
	elapsed := time.Since(begin)
	l := With("component", "gorm")
	switch {
	case err != nil && !errors.Is(err, gorm.ErrRecordNotFound):
		sql, rows := fc()
		l.Error("sql error", "err", err, "elapsed", elapsed.Round(time.Millisecond).String(), "rows", rows, "sql", sql)
	case elapsed > g.slowThreshold:
		sql, rows := fc()
		l.Warn("slow sql", "elapsed", elapsed.Round(time.Millisecond).String(), "rows", rows, "sql", sql)
	case g.debug:
		sql, rows := fc()
		l.Debug("sql", "elapsed", elapsed.Round(time.Millisecond).String(), "rows", rows, "sql", sql)
	}
}
