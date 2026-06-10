package esximon

import "sync"

// sseHub 把每轮采集结束后的全量 snapshot 广播给所有 SSE 订阅者。
// 同 upsmon.sseHub 一样:单一全局广播流,channel buffer=1,慢消费者只丢中间帧。
type sseHub struct {
	mu   sync.Mutex
	subs map[chan []Snapshot]struct{}
}

func newSSEHub() *sseHub {
	return &sseHub{subs: map[chan []Snapshot]struct{}{}}
}

func (h *sseHub) Subscribe() (<-chan []Snapshot, func()) {
	ch := make(chan []Snapshot, 1)
	h.mu.Lock()
	h.subs[ch] = struct{}{}
	h.mu.Unlock()
	return ch, func() {
		h.mu.Lock()
		if _, ok := h.subs[ch]; ok {
			delete(h.subs, ch)
			close(ch)
		}
		h.mu.Unlock()
	}
}

func (h *sseHub) Publish(snap []Snapshot) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for ch := range h.subs {
		select {
		case ch <- snap:
		default:
			select {
			case <-ch:
			default:
			}
			select {
			case ch <- snap:
			default:
			}
		}
	}
}
