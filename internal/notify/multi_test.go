package notify

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestMultiPartialFailureCountsAsDelivered(t *testing.T) {
	bad := &fakeNotifier{always: errors.New("bark key invalid")}
	good := &fakeNotifier{}
	if err := Multi(bad, good).Send(context.Background(), Message{Title: "t"}); err != nil {
		t.Fatalf("one channel delivered, want nil, got %v", err)
	}
	if bad.calls != 1 || good.calls != 1 {
		t.Fatalf("each channel should be tried once, got bad=%d good=%d", bad.calls, good.calls)
	}
}

func TestMultiAllFailedReturnsJoinedError(t *testing.T) {
	e1, e2 := errors.New("e1"), errors.New("e2")
	err := Multi(&fakeNotifier{always: e1}, &fakeNotifier{always: e2}).Send(context.Background(), Message{})
	if !errors.Is(err, e1) || !errors.Is(err, e2) {
		t.Fatalf("want joined e1+e2, got %v", err)
	}
}

func TestMultiNoActiveChannel(t *testing.T) {
	m := Multi(nil)
	if m.Enabled() {
		t.Fatal("no channel must be disabled")
	}
	if err := m.Send(context.Background(), Message{}); err != nil {
		t.Fatalf("no channel send = %v", err)
	}
}

// blockingNotifier 一直阻塞到 ctx 结束，模拟连不上的外网通道。
type blockingNotifier struct{ calls int32 }

func (b *blockingNotifier) Name() string  { return "blocking" }
func (b *blockingNotifier) Enabled() bool { return true }
func (b *blockingNotifier) Send(ctx context.Context, _ Message) error {
	atomic.AddInt32(&b.calls, 1)
	<-ctx.Done()
	return ctx.Err()
}

// 慢通道排在前面也不能拖死后面的通道：并行发送，后面的通道在 ctx 到期前已送达。
func TestMultiSlowChannelDoesNotStarveOthers(t *testing.T) {
	slow := &blockingNotifier{}
	fast := &fakeNotifier{}
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	start := time.Now()
	if err := Multi(slow, fast).Send(ctx, Message{}); err != nil {
		t.Fatalf("fast channel delivered, want nil, got %v", err)
	}
	if atomic.LoadInt32(&fast.calls) != 1 {
		t.Fatalf("fast channel calls = %d", fast.calls)
	}
	if d := time.Since(start); d > 2*time.Second {
		t.Fatalf("send took %s", d)
	}
}
