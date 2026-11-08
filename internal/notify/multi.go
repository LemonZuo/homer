package notify

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/LemonZuo/homer/internal/logx"
)

// Multi 扇出：把一条消息并行发给所有 Enabled() 的子通道。
//
// 送达语义：有任一路成功即视为已送达，返回 nil（失败路只记日志）；全部失败才返回各路错误的合并。
// 调用方把 error 当作「用户没收到」去重试 / 不推进基线，单个坏通道不能让正常通道被反复轰炸。
// 并行发送：外网通道超时不会耗尽共享 ctx 拖死后面的内网通道。无可用子通道时 Enabled()=false。
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
	var active []Notifier
	for _, n := range m.inner {
		if n != nil && n.Enabled() {
			active = append(active, n)
		}
	}
	if len(active) == 0 {
		return nil
	}
	errs := make([]error, len(active))
	var wg sync.WaitGroup
	for i, n := range active {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs[i] = n.Send(ctx, msg)
		}()
	}
	wg.Wait()

	delivered := 0
	var failed []error
	for i, err := range errs {
		if err == nil {
			delivered++
			continue
		}
		failed = append(failed, fmt.Errorf("%s: %w", active[i].Name(), err))
	}
	if delivered > 0 {
		for _, err := range failed {
			logx.Warn("notify channel failed (delivered via other channels)",
				"title", msg.Title, "delivered", delivered, "err", err)
		}
		return nil
	}
	return errors.Join(failed...)
}
