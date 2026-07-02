package upsmon

import (
	"testing"
)

func TestSSEHubSubscribeReceivesPublish(t *testing.T) {
	h := newSSEHub()
	ch, unsub := h.Subscribe()
	defer unsub()

	h.Publish([]Snapshot{{HostName: "nas"}})
	select {
	case snap := <-ch:
		if len(snap) != 1 || snap[0].HostName != "nas" {
			t.Fatalf("snap = %+v", snap)
		}
	default:
		t.Fatal("expected buffered snapshot")
	}
}

func TestSSEHubSlowConsumerGetsLatestFrame(t *testing.T) {
	h := newSSEHub()
	ch, unsub := h.Subscribe()
	defer unsub()

	// 订阅者不消费,连续发 3 帧:中间帧允许丢,必须拿到最新一帧
	h.Publish([]Snapshot{{HostName: "frame1"}})
	h.Publish([]Snapshot{{HostName: "frame2"}})
	h.Publish([]Snapshot{{HostName: "frame3"}})

	select {
	case snap := <-ch:
		if snap[0].HostName != "frame3" {
			t.Fatalf("slow consumer must get latest frame, got %q", snap[0].HostName)
		}
	default:
		t.Fatal("expected latest snapshot in buffer")
	}
}

func TestSSEHubUnsubStopsDelivery(t *testing.T) {
	h := newSSEHub()
	ch, unsub := h.Subscribe()
	unsub()

	// unsub 后 channel 已关闭,读到零值 ok=false
	if _, ok := <-ch; ok {
		t.Fatal("channel must be closed after unsub")
	}
	// unsub 后 Publish 不应 panic(订阅者已移除)
	h.Publish([]Snapshot{{HostName: "x"}})
	// 重复 unsub 幂等
	unsub()
}

func TestSSEHubMultipleSubscribers(t *testing.T) {
	h := newSSEHub()
	ch1, unsub1 := h.Subscribe()
	ch2, unsub2 := h.Subscribe()
	defer unsub1()
	defer unsub2()

	h.Publish([]Snapshot{{HostName: "broadcast"}})
	for i, ch := range []<-chan []Snapshot{ch1, ch2} {
		select {
		case snap := <-ch:
			if snap[0].HostName != "broadcast" {
				t.Fatalf("subscriber %d got %q", i+1, snap[0].HostName)
			}
		default:
			t.Fatalf("subscriber %d missed broadcast", i+1)
		}
	}
}
