package notify

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

type fakeNotifier struct {
	calls   int32
	failFor int   // 前 failFor 次返回错误,之后成功
	always  error // 非 nil 时永远失败
}

func (f *fakeNotifier) Name() string  { return "fake" }
func (f *fakeNotifier) Enabled() bool { return true }
func (f *fakeNotifier) Send(_ context.Context, _ Message) error {
	n := atomic.AddInt32(&f.calls, 1)
	if f.always != nil {
		return f.always
	}
	if int(n) <= f.failFor {
		return errors.New("transient")
	}
	return nil
}

func TestRetrySucceedsFirstTry(t *testing.T) {
	f := &fakeNotifier{}
	if err := Retry(3, time.Millisecond, f).Send(context.Background(), Message{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if f.calls != 1 {
		t.Fatalf("should call inner once, got %d", f.calls)
	}
}

func TestRetrySucceedsAfterFailures(t *testing.T) {
	f := &fakeNotifier{failFor: 2}
	if err := Retry(3, time.Millisecond, f).Send(context.Background(), Message{}); err != nil {
		t.Fatalf("expected eventual success, got %v", err)
	}
	if f.calls != 3 {
		t.Fatalf("expected 3 attempts, got %d", f.calls)
	}
}

func TestRetryReturnsLastErrorWhenExhausted(t *testing.T) {
	sentinel := errors.New("permanent")
	f := &fakeNotifier{always: sentinel}
	err := Retry(3, time.Millisecond, f).Send(context.Background(), Message{})
	if !errors.Is(err, sentinel) {
		t.Fatalf("expected sentinel error, got %v", err)
	}
	if f.calls != 3 {
		t.Fatalf("expected exactly 3 attempts, got %d", f.calls)
	}
}

func TestRetryNormalizesNonPositiveN(t *testing.T) {
	f := &fakeNotifier{always: errors.New("x")}
	_ = Retry(0, time.Millisecond, f).Send(context.Background(), Message{})
	if f.calls != 1 {
		t.Fatalf("n<1 must run exactly once, got %d", f.calls)
	}
}

func TestRetryAbortsOnContextCancelDuringBackoff(t *testing.T) {
	f := &fakeNotifier{always: errors.New("fail")}
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // 立即取消;第 1 次失败后进入退避等待时应直接返回 ctx.Err()
	err := Retry(5, time.Hour, f).Send(ctx, Message{})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
	if f.calls != 1 {
		t.Fatalf("should abort before second attempt, got %d calls", f.calls)
	}
}
