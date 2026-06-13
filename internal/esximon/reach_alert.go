package esximon

// ESXi 主机可达性告警。

import (
	"context"
	"fmt"
	"time"

	"github.com/LemonZuo/homer/internal/logx"
	"github.com/LemonZuo/homer/internal/notify"
)

// handleHostReachAlerts 处理"单台机器可达性"状态转换告警。
// 首次见到的 host 仅记录不告警(首轮 / 新增 host),之后只在 OK ↔ 不可达 转换时各发一条。
// 阈值告警走 processThresholdAlerts,两者互不冲突:host 离线时只发这条,不会再叠一堆阈值告警。
func (s *Service) handleHostReachAlerts(results []HostResult) {
	type change struct {
		name string
		ok   bool
		err  string
	}
	var changes []change

	s.reachMu.Lock()
	seen := make(map[int64]struct{}, len(results))
	for _, r := range results {
		seen[r.HostID] = struct{}{}
		prev, known := s.hostReach[r.HostID]
		s.hostReach[r.HostID] = r.OK
		if !known || prev == r.OK {
			continue
		}
		changes = append(changes, change{name: r.HostName, ok: r.OK, err: r.Error})
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
	if s.alertOut == nil || !s.alertOut.Enabled() {
		return
	}
	for _, c := range changes {
		var title, body string
		if !c.ok {
			title = "ESXi 主机离线"
			if c.err != "" {
				body = fmt.Sprintf("%s 已离线\n错误:%s", c.name, c.err)
			} else {
				body = fmt.Sprintf("%s 已离线", c.name)
			}
		} else {
			title = "ESXi 主机已恢复"
			body = fmt.Sprintf("%s 已恢复采样", c.name)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		if err := s.alertOut.Send(ctx, notify.Message{Title: title, Text: body}); err != nil {
			logx.Error("esxi host reach alert send failed", "host", c.name, "ok", c.ok, "err", err)
		} else {
			logx.Warn("esxi host reach alert sent", "host", c.name, "ok", c.ok)
		}
		cancel()
	}
}
