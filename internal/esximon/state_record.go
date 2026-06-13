package esximon

// ESXi state 行构建和 JSON 粘滞回退。

import (
	"time"

	"encoding/json"
	"github.com/LemonZuo/homer/internal/model"
)

// buildState 把单台机器一轮采集结果转成 esxi_state 行。
// 关键约定:当某个子项采集失败(整轮 SSH 挂 / 单个命令空返回)时,该 JSON 列不会被
// "新的空值"覆盖,而是回退到 prev 的同名 JSON,让前端看到的快照保持上一次已知值。
// 不完整采集不会推进 SampledAt;它只表示最近一次写入 esxi_sample 的完整采样时间。
func buildState(r HostResult, prev *model.EsxiState, now time.Time, sampleComplete bool) model.EsxiState {
	st := model.EsxiState{
		HostKind:  r.HostKind,
		HostID:    r.HostID,
		HostName:  r.HostName,
		Reachable: model.BoolFlag(r.OK),
		LastError: r.Error,
		UpdatedAt: now,
	}
	if !r.OK {
		// 整轮 SSH 失败:reachable=false + last_error 写入,但 JSON 列全部从 prev 继承,
		// 让前端依然能展示"最后一次成功"的硬件/温度/VM 信息。SampledAt 也保留 prev 的。
		if prev != nil {
			st.SampledAt = prev.SampledAt
			st.PlatformJSON = prev.PlatformJSON
			st.CPUStaticJSON = prev.CPUStaticJSON
			st.MemoryJSON = prev.MemoryJSON
			st.RuntimeJSON = prev.RuntimeJSON
			st.CPUTempJSON = prev.CPUTempJSON
			st.MCEJSON = prev.MCEJSON
			st.DiskJSON = prev.DiskJSON
			st.USBJSON = prev.USBJSON
			st.VMJSON = prev.VMJSON
			st.BootJSON = prev.BootJSON
			st.NICJSON = prev.NICJSON
			st.TopologyJSON = prev.TopologyJSON
		}
		return st
	}
	if sampleComplete {
		t := now
		st.SampledAt = &t
	} else if prev != nil {
		st.SampledAt = prev.SampledAt
	}

	m := r.Metrics
	// 各子项判 "是否拿到有效数据";没拿到就 fallback prev 同名 JSON。
	st.PlatformJSON = stickyPlatformJSON(m.Platform, m.Platform.Vendor != "" || m.Platform.UUID != "" || m.Platform.Product != "", prevJSON(prev, func(p *model.EsxiState) string { return p.PlatformJSON }))
	st.CPUStaticJSON = stickyJSON(m.CPU, m.CPU.Brand != "" || m.CPU.Cores > 0, prevJSON(prev, func(p *model.EsxiState) string { return p.CPUStaticJSON }))
	st.MemoryJSON = stickyJSON(m.Memory, m.Memory.TotalBytes > 0, prevJSON(prev, func(p *model.EsxiState) string { return p.MemoryJSON }))
	st.RuntimeJSON = stickyJSON(m.Runtime, runtimeUsable(m.Runtime), prevJSON(prev, func(p *model.EsxiState) string { return p.RuntimeJSON }))
	st.CPUTempJSON = stickyJSON(m.CPUTemp, cpuTempComplete(m), prevJSON(prev, func(p *model.EsxiState) string { return p.CPUTempJSON }))
	st.MCEJSON = stickyJSON(m.MCE, m.MCE.State != "", prevJSON(prev, func(p *model.EsxiState) string { return p.MCEJSON }))
	st.DiskJSON = stickyJSON(m.Disks, diskTempsComplete(m.Disks), prevJSON(prev, func(p *model.EsxiState) string { return p.DiskJSON }))
	st.USBJSON = stickyUSBJSON(m.USB, prevJSON(prev, func(p *model.EsxiState) string { return p.USBJSON }))
	st.VMJSON = stickyJSON(m.VMs, vmStatesComplete(m.VMs), prevJSON(prev, func(p *model.EsxiState) string { return p.VMJSON }))
	st.BootJSON = stickyJSON(m.Boot, m.Boot.UptimeSeconds >= 0, prevJSON(prev, func(p *model.EsxiState) string { return p.BootJSON }))
	st.NICJSON = stickyJSON(m.NICs, len(m.NICs) > 0, prevJSON(prev, func(p *model.EsxiState) string { return p.NICJSON }))
	st.TopologyJSON = stickyTopologyJSON(m.Topology, topologyComplete(m), prevJSON(prev, func(p *model.EsxiState) string { return p.TopologyJSON }))
	return st
}

// stickyJSON:本轮采到了就用新值序列化;没采到且有上次值就保留上次,否则写新值的零形态。
func stickyJSON(v any, ok bool, prevJSON string) string {
	if ok {
		return mustJSON(v)
	}
	if prevJSON != "" {
		return prevJSON
	}
	return mustJSON(v)
}

func stickyPlatformJSON(cur PlatformInfo, ok bool, prevJSON string) string {
	if ok {
		if cur.StaticLastFullSuccessAt.IsZero() && prevJSON != "" {
			if prev, hasPrev := parsePrevPlatform(prevJSON); hasPrev {
				cur.StaticLastFullSuccessAt = prev.StaticLastFullSuccessAt
			}
		}
		return mustJSON(cur)
	}
	if prevJSON != "" {
		return prevJSON
	}
	return mustJSON(cur)
}

func stickyTopologyJSON(cur NetTopology, ok bool, prevJSON string) string {
	if ok {
		if prevJSON != "" {
			if prev, hasPrev := parsePrevTopology(prevJSON); hasPrev {
				cur = fillVMNICIPsFromPrev(cur, prev)
				if cur.LastFullSuccessAt.IsZero() {
					cur.LastFullSuccessAt = prev.LastFullSuccessAt
				}
				if !cur.VMKCollected && len(prev.VMKPorts) > 0 {
					cur.VMKPorts = prev.VMKPorts
				}
			}
		}
		return mustJSON(cur)
	}
	if prevJSON != "" {
		return prevJSON
	}
	return mustJSON(cur)
}

func fillVMNICIPsFromPrev(cur, prev NetTopology) NetTopology {
	ipByKey := make(map[string]string, len(prev.VMNICs))
	for _, link := range prev.VMNICs {
		if link.IP != "" {
			ipByKey[vmNICKey(link)] = link.IP
		}
	}
	for i := range cur.VMNICs {
		if cur.VMNICs[i].IP != "" {
			continue
		}
		if ip := ipByKey[vmNICKey(cur.VMNICs[i])]; ip != "" {
			cur.VMNICs[i].IP = ip
		}
	}
	return cur
}

func parsePrevTopology(s string) (NetTopology, bool) {
	if s == "" {
		return NetTopology{}, false
	}
	var prev NetTopology
	if json.Unmarshal([]byte(s), &prev) != nil {
		return NetTopology{}, false
	}
	return prev, true
}

func parsePrevPlatform(s string) (PlatformInfo, bool) {
	if s == "" {
		return PlatformInfo{}, false
	}
	var prev PlatformInfo
	if json.Unmarshal([]byte(s), &prev) != nil {
		return PlatformInfo{}, false
	}
	return prev, true
}

func parsePrevCPU(s string) (CPUStatic, bool) {
	if s == "" {
		return CPUStatic{}, false
	}
	var prev CPUStatic
	if json.Unmarshal([]byte(s), &prev) != nil {
		return CPUStatic{}, false
	}
	return prev, true
}

func parsePrevMemory(s string) (MemoryInfo, bool) {
	if s == "" {
		return MemoryInfo{}, false
	}
	var prev MemoryInfo
	if json.Unmarshal([]byte(s), &prev) != nil {
		return MemoryInfo{}, false
	}
	return prev, true
}

func parsePrevVMs(s string) ([]VM, bool) {
	if s == "" {
		return nil, false
	}
	var prev []VM
	if json.Unmarshal([]byte(s), &prev) != nil {
		return nil, false
	}
	return prev, true
}

func parsePrevDisks(s string) ([]DiskHealth, bool) {
	if s == "" {
		return nil, false
	}
	var prev []DiskHealth
	if json.Unmarshal([]byte(s), &prev) != nil {
		return nil, false
	}
	return prev, true
}

func parsePrevNICs(s string) ([]NIC, bool) {
	if s == "" {
		return nil, false
	}
	var prev []NIC
	if json.Unmarshal([]byte(s), &prev) != nil {
		return nil, false
	}
	return prev, true
}

func stickyUSBJSON(cur USBState, prevJSON string) string {
	prev, hasPrev := parsePrevUSB(prevJSON)
	merged := cur
	if hasPrev {
		if !cur.controllersKnown && len(cur.Controllers) == 0 && len(prev.Controllers) > 0 {
			merged.Controllers = prev.Controllers
		}
		if !cur.arbitratorKnown {
			merged.ArbitratorRunning = prev.ArbitratorRunning
		}
		if !cur.passthroughKnown && len(cur.AvailableForPassthrough) == 0 && len(prev.AvailableForPassthrough) > 0 {
			merged.AvailableForPassthrough = prev.AvailableForPassthrough
		}
		if !cur.vmOwnedKnown {
			merged.VMOwned = prev.VMOwned
			merged.VMOwnedLastFullSuccessAt = prev.VMOwnedLastFullSuccessAt
		}
	}
	if usbHasData(merged) || cur.controllersKnown || cur.arbitratorKnown || cur.passthroughKnown || cur.vmOwnedKnown {
		return mustJSON(merged)
	}
	if prevJSON != "" {
		return prevJSON
	}
	return mustJSON(merged)
}

func parsePrevUSB(s string) (USBState, bool) {
	if s == "" {
		return USBState{}, false
	}
	var prev USBState
	if json.Unmarshal([]byte(s), &prev) != nil {
		return USBState{}, false
	}
	return prev, true
}

func usbHasData(u USBState) bool {
	return len(u.Controllers) > 0 ||
		u.ArbitratorRunning ||
		len(u.AvailableForPassthrough) > 0 ||
		len(u.VMOwned) > 0
}

func prevJSON(prev *model.EsxiState, sel func(*model.EsxiState) string) string {
	if prev == nil {
		return ""
	}
	return sel(prev)
}

func mustJSON(v any) string {
	buf, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return string(buf)
}
