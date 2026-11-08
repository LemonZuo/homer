package acme

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/LemonZuo/homer/internal/model"
	"github.com/LemonZuo/homer/internal/notify"
)

type fakeAlertOut struct {
	enabled bool
	msgs    chan notify.Message
}

func newFakeAlertOut(enabled bool) *fakeAlertOut {
	return &fakeAlertOut{enabled: enabled, msgs: make(chan notify.Message, 8)}
}

func (f *fakeAlertOut) Name() string  { return "fake" }
func (f *fakeAlertOut) Enabled() bool { return f.enabled }
func (f *fakeAlertOut) Send(_ context.Context, m notify.Message) error {
	f.msgs <- m
	return nil
}

func (f *fakeAlertOut) wait(t *testing.T) notify.Message {
	t.Helper()
	select {
	case m := <-f.msgs:
		return m
	case <-time.After(3 * time.Second):
		t.Fatal("alert not sent")
		return notify.Message{}
	}
}

func (f *fakeAlertOut) none(t *testing.T) {
	t.Helper()
	select {
	case m := <-f.msgs:
		t.Fatalf("unexpected alert: %+v", m)
	case <-time.After(100 * time.Millisecond):
	}
}

func TestAlertIssueFailed(t *testing.T) {
	out := newFakeAlertOut(true)
	s := &Service{notifier: out}
	d := model.ACMEDomain{MainDomain: "a.example"}

	s.alertIssueFailed(7, "renew", d, errors.New("dns: bad token"))
	m := out.wait(t)
	if m.Title != "证书续期失败" {
		t.Fatalf("title = %q", m.Title)
	}
	for _, want := range []string{"a.example", "#7（续期）", "dns: bad token"} {
		if !strings.Contains(m.Text, want) {
			t.Fatalf("body missing %q:\n%s", want, m.Text)
		}
	}

	s.alertIssueFailed(8, "issue", d, errors.New("x"))
	if m := out.wait(t); m.Title != "证书签发失败" || !strings.Contains(m.Text, "#8（签发）") {
		t.Fatalf("issue alert = %+v", m)
	}
}

func TestAlertDeployFailedAndSkipped(t *testing.T) {
	out := newFakeAlertOut(true)
	s := &Service{notifier: out}

	s.alertDeployFailed(9, "a.example", "nas", "fullchain", "已尝试 3/3 次，重试用尽", errors.New("ssh: timeout"))
	m := out.wait(t)
	for _, want := range []string{"a.example", "目标：nas", "配置：fullchain", "#9（已尝试 3/3 次，重试用尽）", "ssh: timeout"} {
		if m.Title != "证书部署失败" || !strings.Contains(m.Text, want) {
			t.Fatalf("deploy alert missing %q: %+v", want, m)
		}
	}

	s.alertAutoDeploySkipped(model.ACMEDomain{MainDomain: "a.example"}, nil)
	out.none(t)
	s.alertAutoDeploySkipped(model.ACMEDomain{MainDomain: "a.example"}, []string{"cfg1：目标类型不匹配"})
	if m := out.wait(t); m.Title != "证书自动部署未提交" || !strings.Contains(m.Text, "- cfg1：目标类型不匹配") {
		t.Fatalf("skipped alert = %+v", m)
	}
}

func TestAlertNoChannelOrNotifier(t *testing.T) {
	out := newFakeAlertOut(false)
	(&Service{notifier: out}).alertFailure("t", "b")
	out.none(t)
	(&Service{}).alertFailure("t", "b") // nil notifier 不 panic
}
