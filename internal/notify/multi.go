package notify

import (
	"context"
	"errors"
)

// Multi 扇出：把一条消息发给所有 Enabled() 的子通道；单路失败不影响其他路，
// 全部失败时返回各路错误的合并。无可用子通道时 Enabled()=false。
func Multi(inner ...Notifier) Notifier {
	return &multi{inner: inner}
}

type multi struct{ inner []Notifier }

func (m *multi) Name() string { return "multi" }

func (m *multi) Enabled() bool {
	for _, n := range m.inner {
		if n != nil && n.Enabled() {
			return true
		}
	}
	return false
}

func (m *multi) Send(ctx context.Context, msg Message) error {
	var errs []error
	for _, n := range m.inner {
		if n == nil || !n.Enabled() {
			continue
		}
		if err := n.Send(ctx, msg); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}
