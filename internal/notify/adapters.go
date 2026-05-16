package notify

import (
	"context"

	"github.com/LemonZuo/homer/internal/notify/email"
	"github.com/LemonZuo/homer/internal/notify/wework"
)

// WeWork 把企业微信 client 适配成 Notifier；仅用 Message.Text，与原 SendText 行为一致。
func WeWork(c *wework.Client) Notifier { return &weworkNotifier{c: c} }

type weworkNotifier struct{ c *wework.Client }

func (w *weworkNotifier) Name() string  { return "wework" }
func (w *weworkNotifier) Enabled() bool { return w.c != nil && w.c.Enabled() }
func (w *weworkNotifier) Send(_ context.Context, m Message) error {
	return w.c.SendText(m.Text)
}

// Email 把 Resend client 适配成 Notifier；defaultTo 在 Message.To 为空时兜底。
// Enabled 要求有可用收件人，保持原 bypass「无收件人则跳过」的行为。
func Email(c *email.ResendClient, defaultTo string) Notifier {
	return &emailNotifier{c: c, to: defaultTo}
}

type emailNotifier struct {
	c  *email.ResendClient
	to string
}

func (e *emailNotifier) Name() string  { return "email" }
func (e *emailNotifier) Enabled() bool {
	return e.c != nil && e.c.Enabled() && e.to != ""
}
func (e *emailNotifier) Send(_ context.Context, m Message) error {
	to := m.To
	if to == "" {
		to = e.to
	}
	return e.c.SendText(to, m.Title, m.Text)
}
