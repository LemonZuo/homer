package upsmon

// UPS 可达性、NUT 和设备失联告警。

import (
	"context"
	"fmt"
	"time"

	"github.com/LemonZuo/homer/internal/logx"
	"github.com/LemonZuo/homer/internal/notify"
)

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
		if prev.missCount >= s.nutOfflineThreshold && !prev.alertedDown {
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
			if t.missCount >= s.upsOfflineThreshold && !t.alertedDown {
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
