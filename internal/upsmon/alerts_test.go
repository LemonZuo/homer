package upsmon

import (
	"context"
	"sync"
	"testing"

	"github.com/LemonZuo/homer/internal/model"
	"github.com/LemonZuo/homer/internal/notify"
)

// fakeOut 捕获告警发送,替代真实通知通道。
type fakeOut struct {
	mu   sync.Mutex
	sent []notify.Message
}

func (f *fakeOut) Name() string  { return "fake" }
func (f *fakeOut) Enabled() bool { return true }
func (f *fakeOut) Send(_ context.Context, m notify.Message) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sent = append(f.sent, m)
	return nil
}

func (f *fakeOut) titles() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]string, len(f.sent))
	for i, m := range f.sent {
		out[i] = m.Title
	}
	return out
}

func (f *fakeOut) reset() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sent = nil
}

// newAlertService 构造只测告警状态机所需的最小 Service。
func newAlertService(out notify.Notifier) *Service {
	return &Service{
		sampleOut:           out,
		lastOK:              true,
		hostReach:           map[int64]bool{},
		hostUPSNames:        map[int64]map[string]upsTrack{},
		hostNUTState:        map[int64]upsTrack{},
		upsOfflineThreshold: 3,
		nutOfflineThreshold: 5,
	}
}

func hostOK(id int64, name string, upses ...string) HostResult {
	h := HostResult{HostID: id, HostName: name, OK: true, UPSEnumerated: true}
	for _, u := range upses {
		h.UPSes = append(h.UPSes, upscReading{Name: u})
	}
	return h
}

func hostDown(id int64, name string) HostResult {
	return HostResult{HostID: id, HostName: name, OK: false}
}

func TestHostReachAlertsFirstSeenSilent(t *testing.T) {
	out := &fakeOut{}
	s := newAlertService(out)
	// 首轮:离线主机也不告警,只记录
	s.handleHostReachAlerts([]HostResult{hostDown(1, "nas")})
	if len(out.titles()) != 0 {
		t.Fatalf("first round must be silent, got %v", out.titles())
	}
	// 第二轮转为 OK → 恢复告警
	s.handleHostReachAlerts([]HostResult{hostOK(1, "nas", "ups")})
	if got := out.titles(); len(got) != 1 || got[0] != "UPS 主机已恢复" {
		t.Fatalf("got %v", got)
	}
}

func TestHostReachAlertsTransitionOnly(t *testing.T) {
	out := &fakeOut{}
	s := newAlertService(out)
	s.handleHostReachAlerts([]HostResult{hostOK(1, "nas", "ups")})
	// 持续 OK 不打扰
	s.handleHostReachAlerts([]HostResult{hostOK(1, "nas", "ups")})
	if len(out.titles()) != 0 {
		t.Fatalf("steady OK must be silent, got %v", out.titles())
	}
	// OK → down
	s.handleHostReachAlerts([]HostResult{hostDown(1, "nas")})
	if got := out.titles(); len(got) != 1 || got[0] != "UPS 主机离线" {
		t.Fatalf("got %v", got)
	}
	// 持续 down 不重复
	out.reset()
	s.handleHostReachAlerts([]HostResult{hostDown(1, "nas")})
	if len(out.titles()) != 0 {
		t.Fatalf("steady down must be silent, got %v", out.titles())
	}
}

func TestHostReachAlertsForgetsRemovedHost(t *testing.T) {
	out := &fakeOut{}
	s := newAlertService(out)
	s.handleHostReachAlerts([]HostResult{hostOK(1, "nas", "ups")})
	// 该 host 从配置中移除后,状态应被清理;重新加回视作首次见到 → 静默
	s.handleHostReachAlerts([]HostResult{})
	s.handleHostReachAlerts([]HostResult{hostDown(1, "nas")})
	if len(out.titles()) != 0 {
		t.Fatalf("re-added host must be silent on first round, got %v", out.titles())
	}
}

func TestNUTAlertsDebounceThreshold(t *testing.T) {
	out := &fakeOut{}
	s := newAlertService(out)
	nutDown := HostResult{HostID: 1, HostName: "nas", OK: true, UPSEnumerated: false}

	// 阈值 5:前 4 轮静默
	for i := 0; i < 4; i++ {
		s.handleNUTAlerts([]HostResult{nutDown})
	}
	if len(out.titles()) != 0 {
		t.Fatalf("below threshold must be silent, got %v", out.titles())
	}
	// 第 5 轮触发
	s.handleNUTAlerts([]HostResult{nutDown})
	if got := out.titles(); len(got) != 1 || got[0] != "UPS 主机 NUT 不可用" {
		t.Fatalf("got %v", got)
	}
	// 已告警后持续 down 不重复
	out.reset()
	s.handleNUTAlerts([]HostResult{nutDown})
	if len(out.titles()) != 0 {
		t.Fatalf("already alerted must not repeat, got %v", out.titles())
	}
	// 恢复 → 发恢复
	s.handleNUTAlerts([]HostResult{hostOK(1, "nas", "ups")})
	if got := out.titles(); len(got) != 1 || got[0] != "UPS 主机 NUT 已恢复" {
		t.Fatalf("got %v", got)
	}
}

func TestNUTAlertsRecoverWithoutPriorAlertSilent(t *testing.T) {
	out := &fakeOut{}
	s := newAlertService(out)
	nutDown := HostResult{HostID: 1, HostName: "nas", OK: true, UPSEnumerated: false}
	// 只抖了 2 轮(未过阈值)就恢复 → 全程静默
	s.handleNUTAlerts([]HostResult{nutDown})
	s.handleNUTAlerts([]HostResult{nutDown})
	s.handleNUTAlerts([]HostResult{hostOK(1, "nas", "ups")})
	if len(out.titles()) != 0 {
		t.Fatalf("transient NUT flap must be silent, got %v", out.titles())
	}
}

func TestNUTAlertsHostOfflineClearsTracking(t *testing.T) {
	out := &fakeOut{}
	s := newAlertService(out)
	nutDown := HostResult{HostID: 1, HostName: "nas", OK: true, UPSEnumerated: false}
	for i := 0; i < 4; i++ {
		s.handleNUTAlerts([]HostResult{nutDown})
	}
	// 主机整个离线 → NUT 计数清零(主机层告警另有通道)
	s.handleNUTAlerts([]HostResult{hostDown(1, "nas")})
	// 主机回来但 NUT 仍不可用:从 1 重新累计,不应立刻告警
	s.handleNUTAlerts([]HostResult{nutDown})
	if len(out.titles()) != 0 {
		t.Fatalf("counter must restart after host offline, got %v", out.titles())
	}
}

func TestUPSAvailabilityDebounceAndRecover(t *testing.T) {
	out := &fakeOut{}
	s := newAlertService(out)
	// 首轮:记录 ups1/ups2
	s.handleUPSAvailabilityAlerts([]HostResult{hostOK(1, "nas", "ups1", "ups2")})
	if len(out.titles()) != 0 {
		t.Fatalf("first round must be silent, got %v", out.titles())
	}
	// ups2 消失,阈值 3:前 2 轮静默
	s.handleUPSAvailabilityAlerts([]HostResult{hostOK(1, "nas", "ups1")})
	s.handleUPSAvailabilityAlerts([]HostResult{hostOK(1, "nas", "ups1")})
	if len(out.titles()) != 0 {
		t.Fatalf("below threshold must be silent, got %v", out.titles())
	}
	// 第 3 轮触发失联
	s.handleUPSAvailabilityAlerts([]HostResult{hostOK(1, "nas", "ups1")})
	if got := out.titles(); len(got) != 1 || got[0] != "UPS 设备失联" {
		t.Fatalf("got %v", got)
	}
	// 持续缺失不重复
	out.reset()
	s.handleUPSAvailabilityAlerts([]HostResult{hostOK(1, "nas", "ups1")})
	if len(out.titles()) != 0 {
		t.Fatalf("already alerted must not repeat, got %v", out.titles())
	}
	// ups2 回来 → 恢复告警
	s.handleUPSAvailabilityAlerts([]HostResult{hostOK(1, "nas", "ups1", "ups2")})
	if got := out.titles(); len(got) != 1 || got[0] != "UPS 设备已恢复" {
		t.Fatalf("got %v", got)
	}
}

func TestUPSAvailabilityTransientFlapSilent(t *testing.T) {
	out := &fakeOut{}
	s := newAlertService(out)
	s.handleUPSAvailabilityAlerts([]HostResult{hostOK(1, "nas", "ups1", "ups2")})
	// 只缺 1 轮就回来(driver pollfreq 共振自愈)→ 全程静默
	s.handleUPSAvailabilityAlerts([]HostResult{hostOK(1, "nas", "ups1")})
	s.handleUPSAvailabilityAlerts([]HostResult{hostOK(1, "nas", "ups1", "ups2")})
	if len(out.titles()) != 0 {
		t.Fatalf("transient flap must be silent, got %v", out.titles())
	}
}

func TestUPSAvailabilityNewUPSAnnounced(t *testing.T) {
	out := &fakeOut{}
	s := newAlertService(out)
	s.handleUPSAvailabilityAlerts([]HostResult{hostOK(1, "nas", "ups1")})
	// 新接一台 ups2 → "已上线"
	s.handleUPSAvailabilityAlerts([]HostResult{hostOK(1, "nas", "ups1", "ups2")})
	if got := out.titles(); len(got) != 1 || got[0] != "UPS 设备已恢复" {
		t.Fatalf("got %v", got)
	}
}

func TestUPSAvailabilitySkipsWhenNotEnumerated(t *testing.T) {
	out := &fakeOut{}
	s := newAlertService(out)
	s.handleUPSAvailabilityAlerts([]HostResult{hostOK(1, "nas", "ups1", "ups2")})
	// NUT 整轮失败(UPSEnumerated=false):跳过判定,不把 UPS 拽进 missCount
	nutFlap := HostResult{HostID: 1, HostName: "nas", OK: true, UPSEnumerated: false}
	for i := 0; i < 5; i++ {
		s.handleUPSAvailabilityAlerts([]HostResult{nutFlap})
	}
	if len(out.titles()) != 0 {
		t.Fatalf("enumeration failure must not trigger UPS-missing alerts, got %v", out.titles())
	}
	// NUT 恢复且 UPS 都在 → 依然静默(prev 保留,无变化)
	s.handleUPSAvailabilityAlerts([]HostResult{hostOK(1, "nas", "ups1", "ups2")})
	if len(out.titles()) != 0 {
		t.Fatalf("recovery with same set must be silent, got %v", out.titles())
	}
}

func TestReachAlertTransitions(t *testing.T) {
	out := &fakeOut{}
	s := newAlertService(out)
	// 启动假设 OK,首轮全挂 → "开始失败"
	s.handleReachAlert(false, 2, "dial tcp: timeout")
	if got := out.titles(); len(got) != 1 || got[0] != "UPS 采样开始失败" {
		t.Fatalf("got %v", got)
	}
	// 持续失败不重复
	out.reset()
	s.handleReachAlert(false, 2, "dial tcp: timeout")
	if len(out.titles()) != 0 {
		t.Fatalf("steady failure must be silent, got %v", out.titles())
	}
	// 恢复
	s.handleReachAlert(true, 2, "")
	if got := out.titles(); len(got) != 1 || got[0] != "UPS 采样已恢复" {
		t.Fatalf("got %v", got)
	}
}

func TestClassifyPowerTransitions(t *testing.T) {
	cases := []struct {
		prev, curr, want string
	}{
		{model.UPSPowerMains, model.UPSPowerBattery, "switched_to_battery"},
		{model.UPSPowerMains, model.UPSPowerLowBattery, "low_battery"},
		{model.UPSPowerBattery, model.UPSPowerLowBattery, "low_battery"},
		{model.UPSPowerBattery, model.UPSPowerMains, "restored_mains"},
		{model.UPSPowerLowBattery, model.UPSPowerMains, "restored_mains"},
		// 不该告警的转换
		{model.UPSPowerLowBattery, model.UPSPowerBattery, ""}, // 电量回升不是"恢复"
	}
	for _, c := range cases {
		if got := classify(c.prev, c.curr); got != c.want {
			t.Errorf("classify(%s→%s) = %q, want %q", c.prev, c.curr, got, c.want)
		}
	}
}
