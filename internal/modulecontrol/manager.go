package modulecontrol

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/LemonZuo/homer/internal/model"
)

var (
	ErrDisabled = errors.New("模块已禁用或执行已暂停")
	ErrUnknown  = errors.New("该模块不支持禁用")
	ErrConflict = errors.New("配置已被更新，请刷新后重试")
)

type View struct {
	Definition
	Enabled   bool      `json:"enabled"`
	Version   uint64    `json:"version"`
	UpdatedAt time.Time `json:"updated_at"`
	Active    int       `json:"active"`
}

type Snapshot struct {
	Instance string `json:"instance"`
	Revision uint64 `json:"revision"`
	Modules  []View `json:"modules"`
}

type state struct {
	setting model.ModuleSetting
	active  int
	done    chan struct{}
}

type Manager struct {
	mu       sync.Mutex // 配置提交与任务准入使用同一把锁，保存成功后不会再放行旧任务。
	store    Store
	states   map[string]*state
	instance string
	revision uint64
	subs     map[chan Snapshot]struct{}
}

func New(ctx context.Context, store Store) (*Manager, error) {
	rows, err := store.Load(ctx)
	if err != nil {
		return nil, fmt.Errorf("加载模块配置: %w", err)
	}
	id := make([]byte, 16)
	if _, err := rand.Read(id); err != nil {
		return nil, err
	}
	m := &Manager{store: store, states: map[string]*state{}, instance: hex.EncodeToString(id), revision: 1, subs: map[chan Snapshot]struct{}{}}
	for _, d := range definitions {
		m.states[d.Key] = &state{setting: model.ModuleSetting{Key: d.Key, Enabled: true, Version: 1}, done: make(chan struct{})}
	}
	for _, row := range rows {
		if s := m.states[row.Key]; s != nil {
			s.setting = row
			if !row.Enabled {
				close(s.done)
			}
		}
	}
	return m, nil
}

func (m *Manager) Enabled(key string) bool {
	if m == nil {
		return true
	} // 未注入时保留独立服务/单测的原行为。
	m.mu.Lock()
	defer m.mu.Unlock()
	s := m.states[key]
	return s != nil && s.setting.Enabled
}

func (m *Manager) Snapshot() Snapshot {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.snapshotLocked()
}

func (m *Manager) snapshotLocked() Snapshot {
	out := Snapshot{Instance: m.instance, Revision: m.revision, Modules: make([]View, 0, len(definitions))}
	for _, d := range definitions {
		s := m.states[d.Key]
		// 独立复制 slice，调用方不能修改注册表。
		d.Jobs = append([]string{}, d.Jobs...)
		out.Modules = append(out.Modules, View{Definition: d, Enabled: s.setting.Enabled, Version: s.setting.Version, UpdatedAt: s.setting.UpdatedAt, Active: s.active})
	}
	return out
}

func (m *Manager) Set(ctx context.Context, key string, enabled bool, expected uint64) (Snapshot, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s := m.states[key]
	if s == nil {
		return Snapshot{}, ErrUnknown
	}
	if s.setting.Version != expected {
		return Snapshot{}, ErrConflict
	}
	if s.setting.Enabled == enabled {
		return m.snapshotLocked(), nil
	}
	next := s.setting
	next.Enabled, next.Version, next.UpdatedAt = enabled, expected+1, time.Now()
	if err := m.store.Save(ctx, next, expected); err != nil {
		return Snapshot{}, err
	}
	s.setting = next
	if enabled {
		s.done = make(chan struct{})
	} else {
		close(s.done)
	}
	m.publishLocked()
	return m.snapshotLocked(), nil
}

// Version 返回模块当前的开关版本（每次启用/禁用 +1，持久化）；未知模块或 nil manager 返回 0。
func (m *Manager) Version(key string) uint64 {
	if m == nil {
		return 0
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if s := m.states[key]; s != nil {
		return s.setting.Version
	}
	return 0
}

// Ticket 标记提交时的启用周期；先禁用再启用也不会重新放行旧队列。
type Ticket struct {
	manager *Manager
	key     string
	version uint64
	done    <-chan struct{}
}

func (m *Manager) Ticket(key string) (Ticket, error) {
	if m == nil {
		return Ticket{}, nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	s := m.states[key]
	if s == nil {
		return Ticket{}, ErrUnknown
	}
	if !s.setting.Enabled {
		return Ticket{}, ErrDisabled
	}
	return Ticket{m, key, s.setting.Version, s.done}, nil
}

func (t Ticket) Done() <-chan struct{} { return t.done }
func (t Ticket) Version() uint64       { return t.version }
func (t Ticket) Valid() bool {
	if t.manager == nil {
		return true
	}
	select {
	case <-t.done:
		return false
	default:
		return true
	}
}

// Begin 必须放在真正执行处（例如等待互斥锁之后）。Finish 允许已开始的操作安全收尾。
func (t Ticket) Begin() (func(), error) {
	m := t.manager
	if m == nil {
		return func() {}, nil
	}
	m.mu.Lock()
	s := m.states[t.key]
	if !s.setting.Enabled || s.setting.Version != t.version {
		m.mu.Unlock()
		return nil, ErrDisabled
	}
	s.active++
	m.publishLocked()
	m.mu.Unlock()
	var once sync.Once
	return func() {
		once.Do(func() {
			m.mu.Lock()
			defer m.mu.Unlock()
			s.active--
			m.publishLocked()
		})
	}, nil
}

func (m *Manager) Begin(key string) (func(), error) {
	t, err := m.Ticket(key)
	if err != nil {
		return nil, err
	}
	return t.Begin()
}

func (m *Manager) Subscribe() (<-chan Snapshot, func()) {
	m.mu.Lock()
	defer m.mu.Unlock()
	ch := make(chan Snapshot, 1)
	m.subs[ch] = struct{}{}
	ch <- m.snapshotLocked()
	return ch, func() {
		m.mu.Lock()
		defer m.mu.Unlock()
		if _, ok := m.subs[ch]; ok {
			delete(m.subs, ch)
			close(ch)
		}
	}
}

func (m *Manager) publishLocked() {
	m.revision++
	if len(m.subs) == 0 {
		return
	}
	snap := m.snapshotLocked()
	for ch := range m.subs {
		select {
		case <-ch:
		default:
		}
		ch <- snap
	}
}
