package upsmon

import "sync"

// sseHub 把每轮采样结束后的全量 snapshot 广播给所有 SSE 订阅者。
// 设计上和 acme.SSEHub 类似但 key 维度不同:UPS 是单一全局广播流(没有 per-task),
// 所有订阅者收同一份 snapshot,所以用一个 map[chan]struct{} 即可。
type sseHub struct {
	mu   sync.Mutex
	subs map[chan []Snapshot]struct{}
}

func newSSEHub() *sseHub {
	return &sseHub{subs: map[chan []Snapshot]struct{}{}}
}

// Subscribe 注册一个订阅者。返回的 unsub 必须在 SSE handler 退出时调用,
// 否则慢客户端 channel 永久堆在 subs 里。channel buffer=1,Publish 用非阻塞写,
// 慢消费者会丢中间帧但拿得到最新一帧(snapshot 是全量,可以容忍丢中间帧)。
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

// Publish 广播最新 snapshot。绝不阻塞采样路径,缓冲满直接丢弃旧帧再塞新帧。
func (h *sseHub) Publish(snap []Snapshot) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for ch := range h.subs {
		select {
		case ch <- snap:
		default:
			// 慢消费者:把通道里那条旧帧吃掉再塞新帧,保证下一次读到最新
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
