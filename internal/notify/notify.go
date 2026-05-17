// Package notify 把 wework / email / webhook 收敛到统一接口，
// 调用方只面向 Notifier，重试与失败日志在此集中处理。
package notify

import (
	"context"
	"time"

	"github.com/LemonZuo/homer/internal/logx"
)

// Message 通道无关的消息载体。Title 作邮件主题/webhook 标题；
// wework 仅用 Text；To 仅 email 用，为空时回退到通道默认收件人。
type Message struct {
	Title string
	Text  string
	To    string
}

type Notifier interface {
	Name() string
	Enabled() bool
	Send(ctx context.Context, m Message) error
}

// Retry 包装首次 + 最多 n-1 次重试，退避 base, 2base, 4base...
// 全部失败时统一记一条日志并返回最后一次错误。
func Retry(n int, base time.Duration, inner Notifier) Notifier {
	if n < 1 {
		n = 1
	}
	return &retry{n: n, base: base, inner: inner}
}

type retry struct {
	n     int
	base  time.Duration
	inner Notifier
}

func (r *retry) Name() string  { return r.inner.Name() }
func (r *retry) Enabled() bool { return r.inner.Enabled() }

func (r *retry) Send(ctx context.Context, m Message) error {
	var err error
	for i := 0; i < r.n; i++ {
		if i > 0 {
			t := time.NewTimer(r.base << (i - 1))
			select {
			case <-ctx.Done():
				t.Stop()
				return ctx.Err()
			case <-t.C:
			}
		}
		if err = r.inner.Send(ctx, m); err == nil {
			if i > 0 {
				logx.Info("notify sent after retry", "channel", r.inner.Name(), "attempts", i+1)
			}
			return nil
		}
		logx.Warn("notify send attempt failed", "channel", r.inner.Name(), "attempt", i+1, "err", err)
	}
	logx.Error("notify send failed", "channel", r.inner.Name(), "attempts", r.n, "err", err)
	return err
}
