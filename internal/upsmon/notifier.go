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
//   - 已报过低电量后 LB 在阈值附近抖动(lb→battery→lb)不降级基线,不重复推送低电告警。
//
// 停电跟踪(outages):断电期间告警一直没送达(典型:光猫/路由器没接 UPS)就来电了,
// 基线仍停在 mains、"已恢复"无从谈起 —— 这时补发一条"期间曾断电"汇总,避免整段停电零通知。
//
// last_alert_at 记录最近一次成功告警时间。
type Notifier struct {
	out   notify.Notifier
	store *Store
	now   func() time.Time // 测试注入;nil 用 time.Now

	// 以下 map 只在 Service.RunSample(sampleMu 内)串行访问,不另加锁。
	reported map[string]string    // stateKey -> 已告知的供电状态(或 powerBaseline)
	retryAt  map[string]time.Time // stateKey -> 发送失败后的下次重试时间
	outages  map[string]*outage   // stateKey -> 进行中 / 待汇总的一段停电
}

// outage 一段停电的观测记录(按采样时刻,非精确起止)。
type outage struct {
	start      time.Time // 首次观测到非市电的时刻
	exact      bool      // start 是否是亲眼看到 mains→电池 的切换(重启/恢复监控时继承的停电为 false)
	end        time.Time // 首次观测到恢复市电的时刻;零值=仍在停电
	minBattery int       // 期间最低电量;-1=未知
	notified   bool      // 期间是否已有异常告警送达
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
	n.outages = nil
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
	if n.outages == nil {
		n.outages = map[string]*outage{}
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
	base := n.reported[key]
	o := n.trackOutage(key, base, curr)
	kind := classify(base, currSrc)
	if kind == "" && currSrc == model.UPSPowerMains && o != nil && !o.notified {
		kind = "outage_summary"
	}
	if kind == "" {
		// 已报过低电量:回到 battery 不降级基线,LB 抖动时不重复推送低电告警。
		if base != model.UPSPowerLowBattery || currSrc != model.UPSPowerBattery {
			n.reported[key] = currSrc
		}
		delete(n.retryAt, key)
		if currSrc == model.UPSPowerMains {
			delete(n.outages, key)
		}
		return
	}
	if at, ok := n.retryAt[key]; ok && n.clock().Before(at) {
		return
	}
	if !n.send(kind, curr, o) {
		n.retryAt[key] = n.clock().Add(alertRetryInterval)
		return
	}
	n.reported[key] = currSrc
	delete(n.retryAt, key)
	switch {
	case currSrc == model.UPSPowerMains:
		delete(n.outages, key)
	case o != nil:
		o.notified = true
	}
}

// trackOutage 按本轮供电状态维护停电记录。断电汇总尚未送达时又断电,合并为同一段。
func (n *Notifier) trackOutage(key, base string, curr model.UPSState) *outage {
	o := n.outages[key]
	switch curr.LastPowerSource {
	case model.UPSPowerBattery, model.UPSPowerLowBattery:
		if o == nil {
			o = &outage{start: n.clock(), exact: base == model.UPSPowerMains, minBattery: -1}
			n.outages[key] = o
		}
		o.end = time.Time{}
		if p := curr.LastBatteryPercent; p >= 0 && (o.minBattery < 0 || p < o.minBattery) {
			o.minBattery = p
		}
	case model.UPSPowerMains:
		if o != nil && o.end.IsZero() {
			o.end = n.clock()
		}
	}
	return o
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
func (n *Notifier) send(kind string, st model.UPSState, o *outage) bool {
	if !n.out.Enabled() {
		logx.Warn("ups alert: no channel", "host", st.HostName, "ups", st.UPSName, "kind", kind)
		return true
	}

	title, body := composeMessage(kind, st, o)
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

// composeMessage 生成告警标题与正文;o 为本段停电记录,可为 nil。
func composeMessage(kind string, st model.UPSState, o *outage) (string, string) {
	device := strings.TrimSpace(st.Mfr + " " + st.Model)
	header := fmt.Sprintf("%s 上的 %s", st.HostName, st.UPSName)
	if device != "" {
		header += ":(" + device + ")"
	}
	switch kind {
	case "current_battery":
		return "UPS 当前使用电池供电", fmt.Sprintf("%s\n恢复监控时处于电池供电\n剩余电量:%s\n预估续航:%s", header, fmtBattery(st.LastBatteryPercent), fmtRuntime(st.LastRuntimeMinutes))
	case "switched_to_battery":
		return "UPS 切到电池供电",
			fmt.Sprintf("%s\n剩余电量:%s\n预估续航:%s\n原始状态:%s",
				header, fmtBattery(st.LastBatteryPercent), fmtRuntime(st.LastRuntimeMinutes), st.LastRawStatus)
	case "low_battery":
		return "UPS 低电量告警",
			fmt.Sprintf("%s\n剩余电量:%s\n预估续航:%s\n原始状态:%s",
				header, fmtBattery(st.LastBatteryPercent), fmtRuntime(st.LastRuntimeMinutes), st.LastRawStatus)
	case "restored_mains":
		body := fmt.Sprintf("%s\n剩余电量:%s\n原始状态:%s", header, fmtBattery(st.LastBatteryPercent), st.LastRawStatus)
		if o != nil && o.exact && !o.end.IsZero() {
			body += "\n停电时长:约 " + fmtDuration(o.end.Sub(o.start))
		}
		return "UPS 已恢复市电", body
	case "outage_summary":
		var b strings.Builder
		b.WriteString(header)
		b.WriteString("\n期间的断电告警未能送达,现已恢复市电")
		if o != nil {
			end := o.end
			if end.IsZero() {
				end = o.start
			}
			fmt.Fprintf(&b, "\n断电时段(按采样观测):%s ~ %s,约 %s",
				o.start.Format("01-02 15:04:05"), end.Format("01-02 15:04:05"), fmtDuration(end.Sub(o.start)))
			if !o.exact {
				b.WriteString("(恢复监控前已在断电,实际开始更早)")
			}
			fmt.Fprintf(&b, "\n期间最低电量:%s", fmtBattery(o.minBattery))
		}
		fmt.Fprintf(&b, "\n当前电量:%s\n原始状态:%s", fmtBattery(st.LastBatteryPercent), st.LastRawStatus)
		return "UPS 期间曾断电", b.String()
	default:
		return "UPS 状态变化", st.LastRawStatus
	}
}

func fmtBattery(p int) string {
	if p < 0 {
		return "未知"
	}
	return fmt.Sprintf("%d%%", p)
}

func fmtDuration(d time.Duration) string {
	if d < time.Minute {
		return "不足 1 分钟"
	}
	return fmtRuntime(int(d / time.Minute))
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
