package upsmon

import (
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

	// 告警去抖阈值,见各自字段注释。从 .env 注入,不在 .env 配置时走 NewService 内默认值。
	// upsOfflineThreshold:单台 UPS 连续几轮 readings 缺失才报"UPS 设备失联"。
	// 30s cron + 默认 3 → 约 90s 窗口,readOneUPS 内部已有 3 次重试扛单次 pollfreq 共振,
	// 所以连续 3 轮还失败意味着 driver 真死了(USB 拔了 / 配置移除)。
	upsOfflineThreshold int
	// nutOfflineThreshold:主机 `upsc -l` 连续几轮失败才报"主机 NUT 不可用"。
	// 默认 5 → 约 150s 窗口,比 UPS 失联阈值宽,扛 homer/fnOS 间网络瞬抖、homer 自身
	// GC 卡顿之类的"两台机器同时哑一会"共因,这些通常 90s 内自愈,不该报警。
	nutOfflineThreshold int
}

// upsTrack 通用"计数 + 已告警"状态:用作单个 UPS 的失联去抖,也用作主机级 NUT 不可用去抖。
// missCount 是连续触发次数,alertedDown 防止跨阈值后每轮都重复发同一条告警。
type upsTrack struct {
	missCount   int
	alertedDown bool
}

func NewService(db *gorm.DB, sampler *Sampler, store *Store, hub *notify.Hub, retention time.Duration, upsOfflineThreshold, nutOfflineThreshold int) *Service {
	if retention <= 0 {
		retention = 7 * 24 * time.Hour
	}
	if upsOfflineThreshold <= 0 {
		upsOfflineThreshold = 3
	}
	if nutOfflineThreshold <= 0 {
		nutOfflineThreshold = 5
	}
	out := hub.For(notify.ModuleUPS)
	return &Service{
		db:                  db,
		sampler:             sampler,
		store:               store,
		notifier:            NewNotifier(out, store),
		sampleOut:           out,
		sse:                 newSSEHub(),
		retention:           retention,
		lastOK:              true, // 启动假设 OK,首轮失败才会发"开始失败"
		hostReach:           map[int64]bool{},
		hostUPSNames:        map[int64]map[string]upsTrack{},
		hostNUTState:        map[int64]upsTrack{},
		upsOfflineThreshold: upsOfflineThreshold,
		nutOfflineThreshold: nutOfflineThreshold,
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

// TriggerSample 暴露给 handler 的"立刻采一轮"接口。
// 不重试 / 不告警阈值,返回真实错误给前端。
func (s *Service) TriggerSample() error {
	if err := s.RunSample(); err != nil {
		return err
	}
	return nil
}

// ErrNoCandidates 表示没有候选机器(用于 handler 区分"没接 UPS"与"接了但全坏")。
var ErrNoCandidates = errors.New("没有可监控的机器(请在「UPS 机器」里添加要采样的主机)")
