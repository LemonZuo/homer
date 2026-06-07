package upsmon

import (
	"context"
	"fmt"
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
// last_alert_at 用于"切回 mains"时清零,以及后续做"持续 N 分钟未恢复二次提醒"的接入点。
type Notifier struct {
	out   notify.Notifier
	store *Store
}

func NewNotifier(out notify.Notifier, store *Store) *Notifier {
	return &Notifier{out: out, store: store}
}

// Process 处理一个 UPS 的状态变化。prev=nil 表示这是首次看到该 UPS。
func (n *Notifier) Process(prev *model.UPSState, curr model.UPSState) {
	if n == nil || n.out == nil {
		return
	}
	prevSrc := model.UPSPowerUnknown
	if prev != nil {
		prevSrc = prev.LastPowerSource
	}
	currSrc := curr.LastPowerSource

	// 同状态 / 未知态进出不发(避免抖动)。
	if prevSrc == currSrc {
		return
	}
	if currSrc == model.UPSPowerUnknown || prevSrc == model.UPSPowerUnknown {
		return
	}

	kind := classify(prevSrc, currSrc)
	if kind == "" {
		return
	}
	n.send(kind, curr)
}

func classify(prev, curr string) string {
	switch {
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

func (n *Notifier) send(kind string, st model.UPSState) {
	if !n.out.Enabled() {
		logx.Warn("ups alert: no channel", "host", st.HostName, "ups", st.UPSName, "kind", kind)
		return
	}

	title, body := composeMessage(kind, st)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := n.out.Send(ctx, notify.Message{Title: title, Text: body}); err != nil {
		logx.Error("ups alert send failed", "host", st.HostName, "ups", st.UPSName, "kind", kind, "err", err)
		return
	}
	logx.Warn("ups alert sent", "host", st.HostName, "ups", st.UPSName, "kind", kind, "status", st.LastRawStatus)

	now := time.Now()
	if kind == "restored_mains" {
		// 清零最近告警时间(可选),目前直接重写为 now,让告警频率指标可见。
		_ = n.store.MarkAlerted(st.HostKind, st.HostID, st.UPSName, now)
		return
	}
	_ = n.store.MarkAlerted(st.HostKind, st.HostID, st.UPSName, now)
}

func composeMessage(kind string, st model.UPSState) (string, string) {
	device := st.Mfr + " " + st.Model
	if device == " " {
		device = st.UPSName
	}
	switch kind {
	case "switched_to_battery":
		return "UPS 切到电池供电",
			fmt.Sprintf("机器:%s\nUPS:%s(%s)\n剩余电量:%d%%\n预估续航:%s\n原始状态:%s",
				st.HostName, st.UPSName, device,
				st.LastBatteryPercent, fmtRuntime(st.LastRuntimeMinutes), st.LastRawStatus)
	case "low_battery":
		return "UPS 低电量告警",
			fmt.Sprintf("机器:%s\nUPS:%s(%s)\n剩余电量:%d%%\n预估续航:%s\n原始状态:%s",
				st.HostName, st.UPSName, device,
				st.LastBatteryPercent, fmtRuntime(st.LastRuntimeMinutes), st.LastRawStatus)
	case "restored_mains":
		return "UPS 已恢复市电",
			fmt.Sprintf("机器:%s\nUPS:%s(%s)\n剩余电量:%d%%\n原始状态:%s",
				st.HostName, st.UPSName, device,
				st.LastBatteryPercent, st.LastRawStatus)
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
