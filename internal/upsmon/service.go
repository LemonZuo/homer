package upsmon

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/LemonZuo/homer/internal/logx"
	"github.com/LemonZuo/homer/internal/model"
	"github.com/LemonZuo/homer/internal/notify"
	"gorm.io/gorm"
)

// Service 串起一轮"扫描 → 采样 → 持久化 → 告警 → SSE 广播"。
type Service struct {
	db        *gorm.DB
	sampler   *Sampler
	store     *Store
	notifier  *Notifier
	sampleOut notify.Notifier // 走 ModuleUPS 通道发"采样整体可达性"状态转换告警
	sse       *sseHub
	retention time.Duration

	// 采样整体可达性状态机:OK ↔ 全部不可达。只在转换的那一轮发一条告警,
	// 持续不可达不重复打扰(jobmonitor 那条路径已经不走了,不会重复)。
	reachMu sync.Mutex
	lastOK  bool
	// 单机可达性状态机:host_id -> 上次是否可达。首次见到的 host 仅记录,不告警,
	// 防止刚启动 / 刚加机器时刷一片"已离线/已恢复"。
	hostReach map[int64]bool
	// UPS 可用性状态机:host_id -> ups_name -> 跟踪状态(missCount + alertedDown)。
	// 覆盖"SSH 通但 upsc 拿不到数据"(NUT 挂了 / USB 拔了 / driver not connected)
	// 与多 UPS 场景下"拔一台 / 加一台"。
	// 判定信号用 h.UPSes 名集合(`upsc <name>` 成功拿到 reading 的子集)——`upsc -l`
	// 只是 ups.conf 配置清单,USB 拔了 NUT 仍会列出,用它判定会漏报真失联。
	// 双层去抖:
	//  1. UPSEnumerated=false 时跳过本轮(`upsc -l` 整体失败 / 输出空白),避免 NUT
	//     服务整体抖动把同机所有 UPS 一起拽进 missCount。
	//  2. 单个 UPS 连续 upsOfflineThreshold 轮没出现在 readings 里才报失联,扛住
	//     readOneUPS 三次重试也救不回来的 driver pollfreq 共振(通常 1-2 轮自愈)。
	// 只在 OK 主机上跟踪,主机离线时清掉记录,主机恢复时进入"首次见到不告警"分支。
	hostUPSNames map[int64]map[string]upsTrack
	// NUT 服务级失联状态机:host_id -> 跟踪状态(unavailCount + alertedDown)。
	// 触发场景:NUT 被 systemctl stop / upsd socket 错 / SSH 通但根本没装 upsc。
	// 跟 UPS 失联告警互补:NUT 整轮失败时 handleUPSAvailabilityAlerts 跳过本轮,
	// 单 UPS 永远不会被报失联;但用户应当被告知"主机的 NUT 服务整体不可用"。
	// 主机离线时清记录(交给 handleHostReachAlerts 报),复用 upsTrack 结构语义。
	hostNUTState map[int64]upsTrack
}

// 连续 upsOfflineThreshold 轮 readings 没出现某 UPS,才认为它真失联。
// 30s cron + threshold=3 → 约 90s 去抖窗口。readOneUPS 内部已有 3 次重试扛单次
// pollfreq 共振,所以连续 3 轮还失败意味着 driver 真死了(USB 拔了 / 配置移除)。
const upsOfflineThreshold = 3

// 连续 nutOfflineThreshold 轮 `upsc -l` 失败,才认为 NUT 服务整体挂掉。
// 阈值跟 UPS 失联一致(3 轮 ≈ 90s),刚好吃下拔接 USB 时 NUT 整体抖 1-2 轮。
const nutOfflineThreshold = 3

// upsTrack 通用"计数 + 已告警"状态:用作单个 UPS 的失联去抖,也用作主机级 NUT 不可用去抖。
// missCount 是连续触发次数,alertedDown 防止跨阈值后每轮都重复发同一条告警。
type upsTrack struct {
	missCount   int
	alertedDown bool
}

func NewService(db *gorm.DB, sampler *Sampler, store *Store, hub *notify.Hub, retention time.Duration) *Service {
	if retention <= 0 {
		retention = 7 * 24 * time.Hour
	}
	out := hub.For(notify.ModuleUPS)
	return &Service{
		db:           db,
		sampler:      sampler,
		store:        store,
		notifier:     NewNotifier(out, store),
		sampleOut:    out,
		sse:          newSSEHub(),
		retention:    retention,
		lastOK:       true, // 启动假设 OK,首轮失败才会发"开始失败"
		hostReach:    map[int64]bool{},
		hostUPSNames: map[int64]map[string]upsTrack{},
		hostNUTState: map[int64]upsTrack{},
	}
}

// Subscribe 给 SSE handler 用:订阅采样完成后的 snapshot 广播。
func (s *Service) Subscribe() (<-chan []Snapshot, func()) { return s.sse.Subscribe() }

// RunSample 是 cron 调用的入口。永远返回 nil:整体不可达走 UPS 内部状态转换告警
// (见 handleReachAlert),不再让 jobmonitor 每轮都重复打扰。单机不可达由 notifier
// 走 UPS 通道发"单机不可达"的状态转换告警。
func (s *Service) RunSample() error {
	hosts, err := s.sampler.Run()
	if err != nil {
		return err
	}
	if len(hosts) == 0 {
		// 没候选机器,直接成功收工(常见于新装库 / 只用 safeline)
		return nil
	}

	// 读上一轮 state 做告警去抖比对
	prevStates, err := s.store.LoadStates()
	if err != nil {
		return fmt.Errorf("加载 ups_state 失败:%w", err)
	}
	prev := indexStates(prevStates)

	now := time.Now()
	var samples []model.UPSSample
	var states []model.UPSState
	type alertSeed struct {
		hostKind string
		hostID   int64
		ups      string
		prev     *model.UPSState
		curr     model.UPSState
	}
	var alerts []alertSeed

	reachableHosts := 0
	for _, h := range hosts {
		if h.OK {
			reachableHosts++
		}
		if !h.HasUPS {
			continue
		}
		for _, r := range h.UPSes {
			samples = append(samples, model.UPSSample{
				HostKind:              h.HostKind,
				HostID:                h.HostID,
				HostName:              h.HostName,
				UPSName:               r.Name,
				Mfr:                   r.Mfr,
				Model:                 r.Model,
				PowerSource:           r.PowerSource,
				BatteryPercent:        r.BatteryPercent,
				RuntimeMinutes:        r.RuntimeMinutes,
				BatteryVoltage:        r.BatteryVoltage,
				BatteryNominalVoltage: r.BatteryNominalVoltage,
				BatteryType:           r.BatteryType,
				InputVoltage:          r.InputVoltage,
				OutputVoltage:         r.OutputVoltage,
				LoadPercent:           r.LoadPercent,
				RealPower:             r.RealPower,
				RawStatus:             r.RawStatus,
				SampledAt:             now,
			})
			st := model.UPSState{
				HostKind:                  h.HostKind,
				HostID:                    h.HostID,
				HostName:                  h.HostName,
				UPSName:                   r.Name,
				Mfr:                       r.Mfr,
				Model:                     r.Model,
				LastPowerSource:           r.PowerSource,
				LastBatteryPercent:        r.BatteryPercent,
				LastRuntimeMinutes:        r.RuntimeMinutes,
				LastBatteryVoltage:        r.BatteryVoltage,
				LastBatteryNominalVoltage: r.BatteryNominalVoltage,
				LastBatteryType:           r.BatteryType,
				LastInputVoltage:          r.InputVoltage,
				LastOutputVoltage:         r.OutputVoltage,
				LastLoadPercent:           r.LoadPercent,
				LastRealPower:             r.RealPower,
				LastRawStatus:             r.RawStatus,
				UpdatedAt:                 now,
			}
			// 保留上一轮的 last_alert_at(后续 notifier 决定要不要重置)
			key := stateKey(h.HostKind, h.HostID, r.Name)
			if pv, ok := prev[key]; ok {
				st.LastAlertAt = pv.LastAlertAt
			}
			states = append(states, st)
			var prevPtr *model.UPSState
			if pv, ok := prev[key]; ok {
				v := pv
				prevPtr = &v
			}
			alerts = append(alerts, alertSeed{
				hostKind: h.HostKind, hostID: h.HostID, ups: r.Name,
				prev: prevPtr, curr: st,
			})
		}
	}

	if err := s.store.SaveSamples(samples); err != nil {
		return fmt.Errorf("写入 ups_sample 失败:%w", err)
	}
	if err := s.store.UpsertState(states); err != nil {
		return fmt.Errorf("更新 ups_state 失败:%w", err)
	}

	for _, a := range alerts {
		s.notifier.Process(a.prev, a.curr)
	}

	// 整体可达性状态机:只在 OK ↔ 全不可达 的转换瞬间各发一条,避免持续打扰。
	sampleOK := !(reachableHosts == 0 && len(hosts) > 0)
	var firstErr string
	if !sampleOK {
		for _, h := range hosts {
			if h.Error != "" {
				firstErr = h.Error
				break
			}
		}
		logx.Warn("ups sample all unreachable", "hosts", len(hosts), "first_error", firstErr)
	} else {
		logx.Info("ups sample done", "hosts", len(hosts), "reachable", reachableHosts, "samples", len(samples))
	}
	s.handleHostReachAlerts(hosts)
	s.handleNUTAlerts(hosts)
	s.handleUPSAvailabilityAlerts(hosts)
	s.handleReachAlert(sampleOK, len(hosts), firstErr)

	// 广播最新快照给所有 SSE 订阅者。BuildSnapshot 失败只记日志,不阻断采样链路。
	if snap, err := s.BuildSnapshot(); err == nil {
		s.sse.Publish(snap)
	} else {
		logx.Warn("ups snapshot publish skipped", "err", err.Error())
	}
	return nil
}

// handleHostReachAlerts 处理"单台机器可达性"状态转换告警。
// 首次见到的 host 仅记录不告警(首轮 / 新增 host),之后只在 OK ↔ 不可达 转换时各发一条。
// 与整体可达性告警互不冲突:全挂时整体 + 单机各发自己的。
func (s *Service) handleHostReachAlerts(hosts []HostResult) {
	type change struct {
		name string
		ok   bool
	}
	var changes []change

	s.reachMu.Lock()
	seen := make(map[int64]struct{}, len(hosts))
	for _, h := range hosts {
		seen[h.HostID] = struct{}{}
		prev, known := s.hostReach[h.HostID]
		s.hostReach[h.HostID] = h.OK
		if !known || prev == h.OK {
			continue
		}
		changes = append(changes, change{name: h.HostName, ok: h.OK})
	}
	for id := range s.hostReach {
		if _, ok := seen[id]; !ok {
			delete(s.hostReach, id)
		}
	}
	s.reachMu.Unlock()

	if len(changes) == 0 {
		return
	}
	if s.sampleOut == nil || !s.sampleOut.Enabled() {
		return
	}
	for _, c := range changes {
		var title, body string
		if !c.ok {
			title = "UPS 主机离线"
			body = fmt.Sprintf("%s 已离线", c.name)
		} else {
			title = "UPS 主机已恢复"
			body = fmt.Sprintf("%s 已上线", c.name)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		if err := s.sampleOut.Send(ctx, notify.Message{Title: title, Text: body}); err != nil {
			logx.Error("ups host reach alert send failed", "host", c.name, "ok", c.ok, "err", err)
		} else {
			logx.Warn("ups host reach alert sent", "host", c.name, "ok", c.ok)
		}
		cancel()
	}
}

// handleNUTAlerts 处理"主机在线但 NUT 服务整体不响应"的转换告警。
// 触发场景:NUT 被 stop / upsd socket 异常 / 远程主机根本没装 upsc 命令。
// 信号:连续 nutOfflineThreshold 轮 UPSEnumerated=false。
// 主机离线时清记录(主机层告警走 handleHostReachAlerts,这里不重叠);
// 首次见到该主机就 NUT 不可用时,从 1 开始累计而不是直接告警,保持去抖一致性。
func (s *Service) handleNUTAlerts(hosts []HostResult) {
	type change struct {
		host string
		ok   bool // true = NUT 已恢复,false = NUT 不可用
	}
	var changes []change

	s.reachMu.Lock()
	seen := make(map[int64]struct{}, len(hosts))
	for _, h := range hosts {
		seen[h.HostID] = struct{}{}
		if !h.OK {
			delete(s.hostNUTState, h.HostID)
			continue
		}
		prev, known := s.hostNUTState[h.HostID]
		if h.UPSEnumerated {
			// NUT 正常:之前如果报过不可用,现在报"已恢复",并重置计数。
			if known && prev.alertedDown {
				changes = append(changes, change{host: h.HostName, ok: true})
			}
			s.hostNUTState[h.HostID] = upsTrack{}
			continue
		}
		// UPSEnumerated=false:累计 missCount。
		if !known {
			s.hostNUTState[h.HostID] = upsTrack{missCount: 1}
			continue
		}
		prev.missCount++
		if prev.missCount >= nutOfflineThreshold && !prev.alertedDown {
			changes = append(changes, change{host: h.HostName, ok: false})
			prev.alertedDown = true
		}
		s.hostNUTState[h.HostID] = prev
	}
	for id := range s.hostNUTState {
		if _, ok := seen[id]; !ok {
			delete(s.hostNUTState, id)
		}
	}
	s.reachMu.Unlock()

	if len(changes) == 0 {
		return
	}
	if s.sampleOut == nil || !s.sampleOut.Enabled() {
		return
	}
	for _, c := range changes {
		var title, body string
		if !c.ok {
			title = "UPS 主机 NUT 不可用"
			body = fmt.Sprintf("%s 的 NUT 服务无法响应", c.host)
		} else {
			title = "UPS 主机 NUT 已恢复"
			body = fmt.Sprintf("%s 的 NUT 服务已恢复响应", c.host)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		if err := s.sampleOut.Send(ctx, notify.Message{Title: title, Text: body}); err != nil {
			logx.Error("nut alert send failed", "host", c.host, "ok", c.ok, "err", err)
		} else {
			logx.Warn("nut alert sent", "host", c.host, "ok", c.ok)
		}
		cancel()
	}
}

// handleUPSAvailabilityAlerts 处理"主机在线但具体 UPS 失联 / 上线"的转换告警。
// 触发场景:NUT 挂了 / USB 拔了 / driver not connected / 多 UPS 场景里拔接其中一台。
// 集合判定用 h.UPSes 名集合("本轮 upsc <name> 成功"的子集),不用 h.UPSNames ——
// 后者是 `upsc -l` 输出,本质是 ups.conf 配置清单,USB 拔了 NUT 仍会列出,用它
// 判定会漏报"USB 实际离线但配置还在"的情况(用户拔 USB 测试的真实场景)。
// 双层去抖见 upsOfflineThreshold 的注释。
// 只在 h.OK=true 时跟踪;主机离线时清掉记录,主机恢复时进入"首次见到不告警"分支。
func (s *Service) handleUPSAvailabilityAlerts(hosts []HostResult) {
	type change struct {
		host string
		ups  string
		has  bool
	}
	var changes []change

	s.reachMu.Lock()
	seen := make(map[int64]struct{}, len(hosts))
	for _, h := range hosts {
		seen[h.HostID] = struct{}{}
		if !h.OK {
			delete(s.hostUPSNames, h.HostID)
			continue
		}
		if !h.UPSEnumerated {
			// `upsc -l` 整轮失败,本轮无法判定 NUT 是否在工作,跳过(保留 prev)。
			// 避免 NUT 服务整体抖动时同机所有 UPS 一起被算成消失。
			continue
		}
		prev, known := s.hostUPSNames[h.HostID]
		curr := make(map[string]struct{}, len(h.UPSes))
		for _, r := range h.UPSes {
			curr[r.Name] = struct{}{}
		}
		if !known {
			next := make(map[string]upsTrack, len(curr))
			for name := range curr {
				next[name] = upsTrack{}
			}
			s.hostUPSNames[h.HostID] = next
			continue
		}
		next := make(map[string]upsTrack, len(prev)+len(curr))
		for name := range curr {
			t, was := prev[name]
			if !was {
				changes = append(changes, change{host: h.HostName, ups: name, has: true})
			} else if t.alertedDown {
				changes = append(changes, change{host: h.HostName, ups: name, has: true})
			}
			next[name] = upsTrack{}
		}
		for name, t := range prev {
			if _, stillIn := curr[name]; stillIn {
				continue
			}
			t.missCount++
			if t.missCount >= upsOfflineThreshold && !t.alertedDown {
				changes = append(changes, change{host: h.HostName, ups: name, has: false})
				t.alertedDown = true
			}
			next[name] = t
		}
		s.hostUPSNames[h.HostID] = next
	}
	for id := range s.hostUPSNames {
		if _, ok := seen[id]; !ok {
			delete(s.hostUPSNames, id)
		}
	}
	s.reachMu.Unlock()

	if len(changes) == 0 {
		return
	}
	if s.sampleOut == nil || !s.sampleOut.Enabled() {
		return
	}
	for _, c := range changes {
		var title, body string
		if !c.has {
			title = "UPS 设备失联"
			body = fmt.Sprintf("%s 上的 %s 已失联", c.host, c.ups)
		} else {
			title = "UPS 设备已恢复"
			body = fmt.Sprintf("%s 上的 %s 已上线", c.host, c.ups)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		if err := s.sampleOut.Send(ctx, notify.Message{Title: title, Text: body}); err != nil {
			logx.Error("ups availability alert send failed", "host", c.host, "ups", c.ups, "has", c.has, "err", err)
		} else {
			logx.Warn("ups availability alert sent", "host", c.host, "ups", c.ups, "has", c.has)
		}
		cancel()
	}
}

// handleReachAlert 处理"整体可达性"状态转换告警。
// 只在 prevOK != curOK 时发一条;持续不可达 / 持续 OK 都不打扰。
func (s *Service) handleReachAlert(curOK bool, hosts int, firstErr string) {
	s.reachMu.Lock()
	prevOK := s.lastOK
	s.lastOK = curOK
	s.reachMu.Unlock()
	if prevOK == curOK {
		return
	}
	if s.sampleOut == nil || !s.sampleOut.Enabled() {
		logx.Warn("ups sample alert skipped: no channel", "ok", curOK)
		return
	}
	var title, body string
	if !curOK {
		title = "UPS 采样开始失败"
		body = fmt.Sprintf("候选机器:%d 台\n全部不可达", hosts)
	} else {
		title = "UPS 采样已恢复"
		body = fmt.Sprintf("候选机器:%d 台\n至少 1 台可达,采样恢复", hosts)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := s.sampleOut.Send(ctx, notify.Message{Title: title, Text: body}); err != nil {
		logx.Error("ups sample alert send failed", "ok", curOK, "err", err)
	} else {
		logx.Warn("ups sample alert sent", "ok", curOK)
	}
}

// RunCleanup 清理过期 sample,返回删除行数(用于日志)。
func (s *Service) RunCleanup() error {
	cutoff := time.Now().Add(-s.retention)
	n, err := s.store.PurgeOlderThan(cutoff)
	if err != nil {
		return err
	}
	if n > 0 {
		logx.Info("ups sample purged", "cutoff", cutoff.Format(time.RFC3339), "rows", n)
	}
	return nil
}

// Snapshot 接口数据:每台机器 + 其下 UPS 当前状态。
type Snapshot struct {
	HostKind  string        `json:"host_kind"`
	HostID    int64         `json:"host_id"`
	HostName  string        `json:"host_name"`
	Endpoint  string        `json:"endpoint"`
	Reachable bool          `json:"reachable"`
	Error     string        `json:"error,omitempty"`
	UPSes     []SnapshotUPS `json:"upses"`
}

type SnapshotUPS struct {
	Name                  string    `json:"name"`
	Mfr                   string    `json:"mfr"`
	Model                 string    `json:"model"`
	PowerSource           string    `json:"power_source"`
	BatteryPercent        int       `json:"battery_percent"`
	RuntimeMinutes        int       `json:"runtime_minutes"`
	BatteryVoltage        float32   `json:"battery_voltage"`
	BatteryNominalVoltage float32   `json:"battery_nominal_voltage"`
	BatteryType           string    `json:"battery_type"`
	InputVoltage          float32   `json:"input_voltage"`
	OutputVoltage         float32   `json:"output_voltage"`
	LoadPercent           int       `json:"load_percent"`
	RealPower             int       `json:"real_power"`
	RawStatus             string    `json:"raw_status"`
	SampledAt             time.Time `json:"sampled_at"`
}

// BuildSnapshot 不重新采样,基于 ups_state(最新)+ ups_host 表组装快照。
// 离线机器走 ups_sample 没法定位,直接读 ups_host 表保证 endpoint/name 总是有值。
func (s *Service) BuildSnapshot() ([]Snapshot, error) {
	targets, err := s.sampler.hosts.ListEnabled()
	if err != nil {
		return nil, err
	}
	if len(targets) == 0 {
		return nil, nil
	}
	states, err := s.store.LoadStates()
	if err != nil {
		return nil, err
	}
	stateByHost := map[string][]model.UPSState{}
	for _, st := range states {
		k := hostKey(st.HostKind, st.HostID)
		stateByHost[k] = append(stateByHost[k], st)
	}
	out := make([]Snapshot, 0, len(targets))
	for _, t := range targets {
		k := hostKey(model.UPSHostKind, t.ID)
		sn := Snapshot{
			HostKind: model.UPSHostKind,
			HostID:   t.ID,
			HostName: t.Name,
			Endpoint: t.Endpoint,
			UPSes:    []SnapshotUPS{},
		}
		for _, st := range stateByHost[k] {
			sn.UPSes = append(sn.UPSes, SnapshotUPS{
				Name:                  st.UPSName,
				Mfr:                   st.Mfr,
				Model:                 st.Model,
				PowerSource:           st.LastPowerSource,
				BatteryPercent:        st.LastBatteryPercent,
				RuntimeMinutes:        st.LastRuntimeMinutes,
				BatteryVoltage:        st.LastBatteryVoltage,
				BatteryNominalVoltage: st.LastBatteryNominalVoltage,
				BatteryType:           st.LastBatteryType,
				InputVoltage:          st.LastInputVoltage,
				OutputVoltage:         st.LastOutputVoltage,
				LoadPercent:           st.LastLoadPercent,
				RealPower:             st.LastRealPower,
				RawStatus:             st.LastRawStatus,
				SampledAt:             st.UpdatedAt,
			})
			sn.Reachable = true
		}
		out = append(out, sn)
	}
	return out, nil
}

// Series 暴露给 handler 的曲线接口。range 是允许的人类时间窗,bucket 由它推导。
func (s *Service) Series(hostKind string, hostID int64, upsName string, window time.Duration) ([]SeriesPoint, error) {
	if window <= 0 {
		window = 24 * time.Hour
	}
	bucket := pickBucket(window)
	since := time.Now().Add(-window)
	return s.store.Series(hostKind, hostID, upsName, since, bucket)
}

// pickBucket 按窗口大小返回桶宽(秒)。控制画图点数 ≈ 300。
func pickBucket(window time.Duration) int {
	switch {
	case window <= 6*time.Hour:
		return 60 // 1min
	case window <= 24*time.Hour:
		return 5 * 60 // 5min
	case window <= 3*24*time.Hour:
		return 15 * 60 // 15min
	default:
		return 30 * 60 // 30min
	}
}

// TriggerSample 暴露给 handler 的"立刻采一轮"接口。
// 不重试 / 不告警阈值,返回真实错误给前端。
func (s *Service) TriggerSample() error {
	if err := s.RunSample(); err != nil {
		return err
	}
	return nil
}

func indexStates(states []model.UPSState) map[string]model.UPSState {
	m := make(map[string]model.UPSState, len(states))
	for _, st := range states {
		m[stateKey(st.HostKind, st.HostID, st.UPSName)] = st
	}
	return m
}

func stateKey(hostKind string, hostID int64, upsName string) string {
	return fmt.Sprintf("%s/%d/%s", hostKind, hostID, upsName)
}

func hostKey(hostKind string, hostID int64) string {
	return fmt.Sprintf("%s/%d", hostKind, hostID)
}

// ErrNoCandidates 表示没有候选机器(用于 handler 区分"没接 UPS"与"接了但全坏")。
var ErrNoCandidates = errors.New("没有可监控的机器(请在「UPS 机器」里添加要采样的主机)")
