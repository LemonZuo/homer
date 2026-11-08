package upsmon

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/LemonZuo/homer/internal/logx"
	"github.com/LemonZuo/homer/internal/model"
	"github.com/LemonZuo/homer/internal/notify"
)

// Notifier 基于"前一轮 state vs 本轮 reading"判断告警。
// 状态机(只发"转变"告警,不持续打扰):
//   - mains       -> battery        发"切到电池供电"
//   - battery     -> low_battery    发"低电量告警"
//   - mains       -> low_battery    发"低电量告警"(罕见,通常会经过 battery)
//   - battery/lb  -> mains          发"已恢复市电"
//   - 同状态                          不发
//
// 判定基线是"已成功告知用户的供电状态"(reported),而不是上一轮 DB 状态:
//   - 发送失败时基线不前进,下一轮自动重试(间隔 alertRetryInterval,通道故障时不每轮卡满超时);
//   - 未知态既不触发告警也不覆盖基线,中间夹一轮未知不会吞掉真实切换;
//   - 进程重启后以持久化的上一轮状态为基线,停机期间恢复市电仍会补发"已恢复"。
//
// last_alert_at 记录最近一次成功告警时间。
type Notifier struct {
	out   notify.Notifier
	store *Store
	now   func() time.Time // 测试注入;nil 用 time.Now

	// 以下 map 只在 Service.RunSample(sampleMu 内)串行访问,不另加锁。
	reported map[string]string    // stateKey -> 已告知的供电状态(或 powerBaseline)
	retryAt  map[string]time.Time // stateKey -> 发送失败后的下次重试时间
}

// alertRetryInterval 告警发送失败后的重试间隔。
const alertRetryInterval = time.Minute

// powerBaseline 是重新启用监控后的伪基线:当前异常供电按"恢复监控时处于…"口径报一次,
// 当前市电则直接接受为基线,不把停用期间的变化误报为恢复市电。
const powerBaseline = "baseline"

func NewNotifier(out notify.Notifier, store *Store) *Notifier {
	return &Notifier{out: out, store: store}
}

// Reset 模块重新启用(rebase)时清空内存基线与重试计划。
func (n *Notifier) Reset() {
	if n == nil {
		return
	}
	n.reported = nil
	n.retryAt = nil
}

// ProcessBaseline 恢复监控后的首轮:只报告当前的异常供电。发送失败会在后续 Process 中重试。
func (n *Notifier) ProcessBaseline(curr model.UPSState) {
	if n == nil || n.out == nil {
		return
	}
	n.init()
	key := stateKey(curr.HostKind, curr.HostID, curr.UPSName)
	n.reported[key] = powerBaseline
	delete(n.retryAt, key)
	n.advance(key, curr)
}

// Process 处理一个 UPS 的状态变化。prev 是上一轮持久化状态,nil 表示首次看到该 UPS。
func (n *Notifier) Process(prev *model.UPSState, curr model.UPSState) {
	if n == nil || n.out == nil {
		return
	}
	n.init()
	key := stateKey(curr.HostKind, curr.HostID, curr.UPSName)
	if _, ok := n.reported[key]; !ok {
		// 本进程首次处理这台 UPS:以上一轮持久化状态为基线;
		// 首次见到该 UPS / 上一轮未知 → 只记录当前状态,不告警。
		if prev == nil || prev.LastPowerSource == model.UPSPowerUnknown {
			if curr.LastPowerSource != model.UPSPowerUnknown {
				n.reported[key] = curr.LastPowerSource
			}
			return
		}
		n.reported[key] = prev.LastPowerSource
	}
	n.advance(key, curr)
}

func (n *Notifier) init() {
	if n.reported == nil {
		n.reported = map[string]string{}
	}
	if n.retryAt == nil {
		n.retryAt = map[string]time.Time{}
	}
}

func (n *Notifier) clock() time.Time {
	if n.now != nil {
		return n.now()
	}
	return time.Now()
}

// advance 用 reported 基线判定本轮是否要告警;发送成功才推进基线。
func (n *Notifier) advance(key string, curr model.UPSState) {
	currSrc := curr.LastPowerSource
	if currSrc == model.UPSPowerUnknown {
		return
	}
	kind := classify(n.reported[key], currSrc)
	if kind == "" {
		n.reported[key] = currSrc
		delete(n.retryAt, key)
		return
	}
	if at, ok := n.retryAt[key]; ok && n.clock().Before(at) {
		return
	}
	if !n.send(kind, curr) {
		n.retryAt[key] = n.clock().Add(alertRetryInterval)
		return
	}
	n.reported[key] = currSrc
	delete(n.retryAt, key)
}

func classify(prev, curr string) string {
	switch {
	case prev == powerBaseline && curr == model.UPSPowerBattery:
		return "current_battery"
	case prev == powerBaseline && curr == model.UPSPowerLowBattery:
		return "low_battery"
	case prev == model.UPSPowerMains && curr == model.UPSPowerBattery:
		return "switched_to_battery"
	case prev == model.UPSPowerMains && curr == model.UPSPowerLowBattery:
		return "low_battery"
	case prev == model.UPSPowerBattery && curr == model.UPSPowerLowBattery:
		return "low_battery"
	case (prev == model.UPSPowerBattery || prev == model.UPSPowerLowBattery) && curr == model.UPSPowerMains:
		return "restored_mains"
	default:
		return ""
	}
}

// send 发送一条告警,返回是否算"已处理"。没有可用通道视为已处理(不重试,
// 免得之后配好通道时补发一堆过时告警);只有真正发送失败才返回 false 交给重试。
func (n *Notifier) send(kind string, st model.UPSState) bool {
	if !n.out.Enabled() {
		logx.Warn("ups alert: no channel", "host", st.HostName, "ups", st.UPSName, "kind", kind)
		return true
	}

	title, body := composeMessage(kind, st)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := n.out.Send(ctx, notify.Message{Title: title, Text: body}); err != nil {
		logx.Error("ups alert send failed, will retry", "host", st.HostName, "ups", st.UPSName,
			"kind", kind, "retry_in", alertRetryInterval.String(), "err", err)
		return false
	}
	logx.Warn("ups alert sent", "host", st.HostName, "ups", st.UPSName, "kind", kind, "status", st.LastRawStatus)
	if n.store != nil {
		if err := n.store.MarkAlerted(st.HostKind, st.HostID, st.UPSName, n.clock()); err != nil {
			logx.Warn("ups mark alerted failed", "host", st.HostName, "ups", st.UPSName, "err", err)
		}
	}
	return true
}

func composeMessage(kind string, st model.UPSState) (string, string) {
	device := strings.TrimSpace(st.Mfr + " " + st.Model)
	header := fmt.Sprintf("%s 上的 %s", st.HostName, st.UPSName)
	if device != "" {
		header += ":(" + device + ")"
	}
	switch kind {
	case "current_battery":
		return "UPS 当前使用电池供电", fmt.Sprintf("%s\n恢复监控时处于电池供电\n剩余电量:%d%%\n预估续航:%s", header, st.LastBatteryPercent, fmtRuntime(st.LastRuntimeMinutes))
	case "switched_to_battery":
		return "UPS 切到电池供电",
			fmt.Sprintf("%s\n剩余电量:%d%%\n预估续航:%s\n原始状态:%s",
				header, st.LastBatteryPercent, fmtRuntime(st.LastRuntimeMinutes), st.LastRawStatus)
	case "low_battery":
		return "UPS 低电量告警",
			fmt.Sprintf("%s\n剩余电量:%d%%\n预估续航:%s\n原始状态:%s",
				header, st.LastBatteryPercent, fmtRuntime(st.LastRuntimeMinutes), st.LastRawStatus)
	case "restored_mains":
		return "UPS 已恢复市电",
			fmt.Sprintf("%s\n剩余电量:%d%%\n原始状态:%s",
				header, st.LastBatteryPercent, st.LastRawStatus)
	default:
		return "UPS 状态变化", st.LastRawStatus
	}
}

func fmtRuntime(min int) string {
	if min < 0 {
		return "未知"
	}
	if min < 60 {
		return fmt.Sprintf("%d 分钟", min)
	}
	return fmt.Sprintf("%d 小时 %d 分", min/60, min%60)
}
