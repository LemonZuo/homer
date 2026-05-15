package acme

import (
	"sync"
)

// SSEHub 把 ACME 任务运行期间的日志增量推送给监听该任务的 SSE 订阅者。
// 每个任务有 0..N 个订阅者；任务结束后 Hub 关闭对应 channel。
type SSEHub struct {
	mu   sync.Mutex
	subs map[int64]map[chan string]struct{}
}

func NewSSEHub() *SSEHub {
	return &SSEHub{subs: map[int64]map[chan string]struct{}{}}
}

// Subscribe 为任务 taskID 注册一个订阅 channel；返回的 unsubscribe 在 SSE 连接关闭时调用。
func (h *SSEHub) Subscribe(taskID int64) (<-chan string, func()) {
	ch := make(chan string, 128)
	h.mu.Lock()
	if _, ok := h.subs[taskID]; !ok {
		h.subs[taskID] = map[chan string]struct{}{}
	}
	h.subs[taskID][ch] = struct{}{}
	h.mu.Unlock()
	return ch, func() { h.unsubscribe(taskID, ch) }
}

func (h *SSEHub) unsubscribe(taskID int64, ch chan string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if set, ok := h.subs[taskID]; ok {
		if _, ok := set[ch]; ok {
			delete(set, ch)
			close(ch)
		}
		if len(set) == 0 {
			delete(h.subs, taskID)
		}
	}
}

// Publish 给所有订阅者发送一行日志；channel 满则丢弃（不阻塞业务）。
func (h *SSEHub) Publish(taskID int64, line string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for ch := range h.subs[taskID] {
		select {
		case ch <- line:
		default:
		}
	}
}

// Close 任务结束时关闭所有订阅 channel（SSE handler 据此结束流）。
func (h *SSEHub) Close(taskID int64) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for ch := range h.subs[taskID] {
		close(ch)
	}
	delete(h.subs, taskID)
}
