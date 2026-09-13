package modulecontrol

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/LemonZuo/homer/internal/model"
	"github.com/gin-gonic/gin"
)

type memoryStore struct {
	rows map[string]model.ModuleSetting
	fail bool
}

func (s *memoryStore) Load(context.Context) ([]model.ModuleSetting, error) {
	if s.fail {
		return nil, errors.New("database unavailable")
	}
	var rows []model.ModuleSetting
	for _, row := range s.rows {
		rows = append(rows, row)
	}
	return rows, nil
}
func (s *memoryStore) Save(_ context.Context, row model.ModuleSetting, _ uint64) error {
	if s.fail {
		return errors.New("database unavailable")
	}
	if s.rows == nil {
		s.rows = map[string]model.ModuleSetting{}
	}
	s.rows[row.Key] = row
	return nil
}

func testManager(t *testing.T) (*Manager, *memoryStore) {
	t.Helper()
	store := &memoryStore{}
	m, err := New(context.Background(), store)
	if err != nil {
		t.Fatal(err)
	}
	return m, store
}

func setEnabled(t *testing.T, m *Manager, key string, enabled bool) {
	t.Helper()
	for _, v := range m.Snapshot().Modules {
		if v.Key == key {
			if _, err := m.Set(context.Background(), key, enabled, v.Version); err != nil {
				t.Fatal(err)
			}
			return
		}
	}
	t.Fatalf("module %s missing", key)
}

func TestPersistenceConflictAndSaveFailure(t *testing.T) {
	m, store := testManager(t)
	setEnabled(t, m, "ups", false)
	restarted, err := New(context.Background(), store)
	if err != nil || restarted.Enabled("ups") {
		t.Fatalf("disabled state lost after restart: %v", err)
	}
	if _, err := m.Set(context.Background(), "ups", true, 1); !errors.Is(err, ErrConflict) {
		t.Fatalf("stale update accepted: %v", err)
	}
	store.fail = true
	if _, err := m.Set(context.Background(), "ups", true, 2); err == nil {
		t.Fatal("save failure ignored")
	}
	if m.Enabled("ups") {
		t.Fatal("failed save changed runtime state")
	}
	if _, err := New(context.Background(), store); err == nil {
		t.Fatal("startup silently enabled modules on DB failure")
	}
}

func TestDisableDrainsRunningAndInvalidatesQueuedGeneration(t *testing.T) {
	m, _ := testManager(t)
	ticket, _ := m.Ticket("acme")
	finish, err := ticket.Begin()
	if err != nil {
		t.Fatal(err)
	}
	setEnabled(t, m, "acme", false)
	if ticket.Valid() {
		t.Fatal("disabled generation still valid")
	}
	if _, err := m.Begin("acme"); !errors.Is(err, ErrDisabled) {
		t.Fatal(err)
	}
	setEnabled(t, m, "acme", true)
	if _, err := ticket.Begin(); !errors.Is(err, ErrDisabled) {
		t.Fatal("old queued task survived disable-enable")
	}
	finish()
	finish() // cleanup is idempotent.
	for _, v := range m.Snapshot().Modules {
		if v.Active != 0 {
			t.Fatalf("active leaked: %+v", v)
		}
	}
}

func TestAllBusinessModulesCanBeDisabledButCoreCannot(t *testing.T) {
	m, _ := testManager(t)
	for _, d := range definitions {
		setEnabled(t, m, d.Key, false)
	}
	for _, v := range m.Snapshot().Modules {
		if v.Enabled {
			t.Fatal(v.Key)
		}
	}
	for _, key := range []string{"modules", "scheduler", "notify", "unknown"} {
		if _, err := m.Set(context.Background(), key, false, 1); !errors.Is(err, ErrUnknown) {
			t.Fatalf("core %s changed", key)
		}
	}
}

func TestSlowSubscriberReceivesLatestAndConcurrentReaders(t *testing.T) {
	m, _ := testManager(t)
	ch, unsubscribe := m.Subscribe()
	defer unsubscribe()
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				if finish, err := m.Begin("ups"); err == nil {
					finish()
				}
				_ = m.Snapshot()
			}
		}()
	}
	for i := 0; i < 20; i++ {
		setEnabled(t, m, "ups", i%2 == 0)
	}
	wg.Wait()
	latest := m.Snapshot()
	if snap := <-ch; snap.Revision != latest.Revision {
		t.Fatalf("stale snapshot: %d != %d", snap.Revision, latest.Revision)
	}
}

func TestGuardBlocksBusinessAndClosesExistingStream(t *testing.T) {
	gin.SetMode(gin.TestMode)
	m, _ := testManager(t)
	r := gin.New()
	r.Use(m.Guard())
	r.GET("/api/ups/snapshot", func(c *gin.Context) { c.Status(http.StatusOK) })
	r.GET("/api/system/modules", func(c *gin.Context) { c.Status(http.StatusOK) })
	r.POST("/api/byPass/receive", func(c *gin.Context) { c.Status(http.StatusOK) })
	started, stopped := make(chan struct{}), make(chan struct{})
	r.GET("/api/ups/stream", func(c *gin.Context) {
		close(started)
		<-c.Request.Context().Done()
		close(stopped)
	})
	go r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/ups/stream", nil))
	<-started
	setEnabled(t, m, "ups", false)
	setEnabled(t, m, "bypass", false)
	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("stream was not cancelled")
	}
	for _, path := range []string{"/api/ups/snapshot", "/api/byPass/receive"} {
		method := http.MethodGet
		if strings.Contains(path, "byPass") {
			method = http.MethodPost
		}
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(method, path, nil))
		if w.Code != http.StatusForbidden || !strings.Contains(w.Body.String(), "MODULE_DISABLED") {
			t.Fatalf("unguarded %s: %d %s", path, w.Code, w.Body.String())
		}
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/system/modules", nil))
	if w.Code != http.StatusOK {
		t.Fatal("recovery route blocked")
	}
}
