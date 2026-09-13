package scheduler

import (
	"context"
	"errors"
	"testing"

	"github.com/LemonZuo/homer/internal/model"
	"github.com/LemonZuo/homer/internal/modulecontrol"
)

type moduleStore struct{}

func (moduleStore) Load(context.Context) ([]model.ModuleSetting, error)     { return nil, nil }
func (moduleStore) Save(context.Context, model.ModuleSetting, uint64) error { return nil }

func TestDisabledJobPreservesHistoryAndRejectsManualAndOldQueue(t *testing.T) {
	m, err := modulecontrol.New(context.Background(), moduleStore{})
	if err != nil {
		t.Fatal(err)
	}
	s := New()
	s.SetModules(m)
	runs := 0
	if err := s.Register("ups-sample", "", func() error { runs++; return errors.New("sample failed") }); err != nil {
		t.Fatal(err)
	}
	j := s.jobs["ups-sample"]
	s.run(j, "cron")
	oldTicket, _ := m.Ticket("ups")
	if _, err := m.Set(context.Background(), "ups", false, 1); err != nil {
		t.Fatal(err)
	}
	s.run(j, "cron")
	if err := s.Trigger("ups-sample"); !errors.Is(err, modulecontrol.ErrDisabled) {
		t.Fatal("manual trigger bypassed disable")
	}
	v := s.Jobs()[0]
	if !v.Paused || v.Next != nil || v.ConsecFails != 1 || len(v.History) != 1 {
		t.Fatalf("bad paused job: %+v", v)
	}
	if _, err := m.Set(context.Background(), "ups", true, 2); err != nil {
		t.Fatal(err)
	}
	s.runWithTicket(j, "manual", oldTicket)
	if runs != 1 {
		t.Fatal("old queue ran after reenable")
	}
	s.run(j, "cron")
	if runs != 2 || s.Jobs()[0].Paused {
		t.Fatal("job did not resume")
	}
}
