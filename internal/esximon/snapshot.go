package esximon

// ESXi 快照接口组装。

import (
	"time"

	"encoding/json"
	"github.com/LemonZuo/homer/internal/model"
)

// Snapshot 每台机器一行,前端拿来渲染卡片。
// 各 JSON 块以"结构化对象"形式直接返回(不是字符串),便于前端 TS 类型化。
type Snapshot struct {
	HostKind  string          `json:"host_kind"`
	HostID    int64           `json:"host_id"`
	HostName  string          `json:"host_name"`
	Endpoint  string          `json:"endpoint"`
	Reachable bool            `json:"reachable"`
	Error     string          `json:"error,omitempty"`
	SampledAt *time.Time      `json:"sampled_at,omitempty"`
	Platform  *PlatformInfo   `json:"platform,omitempty"`
	CPU       *CPUStatic      `json:"cpu_static,omitempty"`
	Memory    *MemoryInfo     `json:"memory,omitempty"`
	Runtime   *RuntimeUsage   `json:"runtime_usage,omitempty"`
	CPUTemp   *CPUTemperature `json:"cpu_temperature,omitempty"`
	MCE       *MCEHealth      `json:"mce_health,omitempty"`
	Disks     []DiskHealth    `json:"disk_health,omitempty"`
	USB       *USBState       `json:"usb,omitempty"`
	VMs       []VM            `json:"vms,omitempty"`
	Boot      *HostBoot       `json:"boot,omitempty"`
	NICs      []NIC           `json:"nics,omitempty"`
	Topology  *NetTopology    `json:"net_topology,omitempty"`
}

// BuildSnapshot 基于 esxi_host(主) + esxi_state(JSON 列) 组装快照。
// 离线 / 从未采过的机器仍出现在结果里(reachable=false),保证前端卡片不消失。
func (s *Service) BuildSnapshot() ([]Snapshot, error) {
	hosts, err := s.hosts.List()
	if err != nil {
		return nil, err
	}
	if len(hosts) == 0 {
		return nil, nil
	}
	states, err := s.store.LoadStates()
	if err != nil {
		return nil, err
	}
	stateByID := map[int64]model.EsxiState{}
	for _, st := range states {
		stateByID[st.HostID] = st
	}

	out := make([]Snapshot, 0, len(hosts))
	for _, h := range hosts {
		if !bool(h.Enabled) {
			continue
		}
		sn := Snapshot{
			HostKind: model.EsxiHostKind,
			HostID:   h.ID,
			HostName: h.Name,
			Endpoint: h.Endpoint,
		}
		st, ok := stateByID[h.ID]
		if !ok {
			out = append(out, sn)
			continue
		}
		sn.Reachable = bool(st.Reachable)
		sn.Error = st.LastError
		sn.SampledAt = st.SampledAt
		hydrate(&sn, st)
		out = append(out, sn)
	}
	return out, nil
}

func hydrate(sn *Snapshot, st model.EsxiState) {
	if st.PlatformJSON != "" {
		var v PlatformInfo
		if json.Unmarshal([]byte(st.PlatformJSON), &v) == nil {
			sn.Platform = &v
		}
	}
	if st.CPUStaticJSON != "" {
		var v CPUStatic
		if json.Unmarshal([]byte(st.CPUStaticJSON), &v) == nil {
			sn.CPU = &v
		}
	}
	if st.MemoryJSON != "" {
		var v MemoryInfo
		if json.Unmarshal([]byte(st.MemoryJSON), &v) == nil {
			sn.Memory = &v
		}
	}
	if st.RuntimeJSON != "" {
		var v RuntimeUsage
		if json.Unmarshal([]byte(st.RuntimeJSON), &v) == nil {
			sn.Runtime = &v
		}
	}
	if st.CPUTempJSON != "" {
		var v CPUTemperature
		if json.Unmarshal([]byte(st.CPUTempJSON), &v) == nil {
			sn.CPUTemp = &v
		}
	}
	if st.MCEJSON != "" {
		var v MCEHealth
		if json.Unmarshal([]byte(st.MCEJSON), &v) == nil {
			sn.MCE = &v
		}
	}
	if st.DiskJSON != "" {
		var v []DiskHealth
		if json.Unmarshal([]byte(st.DiskJSON), &v) == nil {
			sn.Disks = v
		}
	}
	if st.USBJSON != "" {
		var v USBState
		if json.Unmarshal([]byte(st.USBJSON), &v) == nil {
			sn.USB = &v
		}
	}
	if st.VMJSON != "" {
		var v []VM
		if json.Unmarshal([]byte(st.VMJSON), &v) == nil {
			sn.VMs = v
		}
	}
	if st.BootJSON != "" {
		var v HostBoot
		if json.Unmarshal([]byte(st.BootJSON), &v) == nil {
			sn.Boot = &v
		}
	}
	if st.NICJSON != "" {
		var v []NIC
		if json.Unmarshal([]byte(st.NICJSON), &v) == nil {
			sn.NICs = v
		}
	}
	if st.TopologyJSON != "" {
		var v NetTopology
		if json.Unmarshal([]byte(st.TopologyJSON), &v) == nil {
			sn.Topology = &v
		}
	}
}
