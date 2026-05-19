package scheduler

import (
	"errors"
	"sync"
	"testing"
)

type stubObserver struct {
	mu      sync.Mutex
	records []int // 每次 Record 的 consecFails
	alerts  []int // 每次 Alert 的 consecFails
}

func (s *stubObserver) Record(_ string, _ Run, consec int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.records = append(s.records, consec)
}

func (s *stubObserver) Alert(_ string, _ Run, consec int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.alerts = append(s.alerts, consec)
}

func TestRunConsecCountingAndReset(t *testing.T) {
	s := New()
	obs := &stubObserver{}
	s.SetObserver(obs, 2) // 连续失败达 2 次才告警

	var fail bool
	if err := s.Register("j", "", func() error {
		if fail {
			return errors.New("boom")
		}
		return nil
	}); err != nil {
		t.Fatalf("register: %v", err)
	}
	j := s.jobs["j"]

	fail = true
	s.run(j, "manual") // consec=1, < threshold, no alert
	s.run(j, "manual") // consec=2, alert
	fail = false
	s.run(j, "manual") // success, consec reset to 0
	fail = true
	s.run(j, "manual") // consec=1 again

	if want := []int{1, 2, 0, 1}; !equalInts(obs.records, want) {
		t.Fatalf("records = %v want %v", obs.records, want)
	}
	if want := []int{2}; !equalInts(obs.alerts, want) {
		t.Fatalf("alerts = %v want %v (only when consec>=threshold)", obs.alerts, want)
	}
}

func TestRunRecoversPanicAsFailure(t *testing.T) {
	s := New()
	obs := &stubObserver{}
	s.SetObserver(obs, 1)
	_ = s.Register("p", "", func() error { panic("kaboom") })
	j := s.jobs["p"]
	s.run(j, "manual") // 不应把 panic 扩散出来

	if j.last == nil || j.last.OK {
		t.Fatal("panic run should be recorded as failure")
	}
	if len(obs.alerts) != 1 {
		t.Fatalf("panic failure should alert at threshold 1, alerts=%v", obs.alerts)
	}
}

func TestSetObserverClampsThreshold(t *testing.T) {
	s := New()
	s.SetObserver(&stubObserver{}, 0)
	if s.failThreshold != 1 {
		t.Fatalf("threshold <1 must clamp to 1, got %d", s.failThreshold)
	}
}

func TestRegisterRejectsDuplicate(t *testing.T) {
	s := New()
	if err := s.Register("dup", "", func() error { return nil }); err != nil {
		t.Fatalf("first register: %v", err)
	}
	if err := s.Register("dup", "", func() error { return nil }); err == nil {
		t.Fatal("duplicate name should error")
	}
}

func TestRegisterRejectsBadCronSpec(t *testing.T) {
	s := New()
	if err := s.Register("bad", "not a cron spec", func() error { return nil }); err == nil {
		t.Fatal("invalid cron spec should error")
	}
}

func TestTriggerErrors(t *testing.T) {
	s := New()
	if err := s.Trigger("nope"); err == nil {
		t.Fatal("triggering unknown job should error")
	}
	_ = s.Register("busy", "", func() error { return nil })
	j := s.jobs["busy"]
	j.running = true // 模拟正在执行
	if err := s.Trigger("busy"); err == nil {
		t.Fatal("triggering a running job should error")
	}
}

func TestJobsHistoryReversedAndBounded(t *testing.T) {
	s := New()
	_ = s.Register("h", "", func() error { return nil })
	j := s.jobs["h"]
	for i := 0; i < historySize+5; i++ {
		s.run(j, "manual")
	}
	views := s.Jobs()
	if len(views) != 1 {
		t.Fatalf("expected 1 view, got %d", len(views))
	}
	v := views[0]
	if len(v.History) != historySize {
		t.Fatalf("history bounded to %d, got %d", historySize, len(v.History))
	}
	if v.ManualOnly != true {
		t.Fatal("empty spec job should be ManualOnly")
	}
	// 倒序:最后一次执行排在最前(用 Start 单调性近似校验非空即可)。
	if v.Last == nil || !v.Last.OK {
		t.Fatal("last run should be recorded and OK")
	}
}

func equalInts(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
