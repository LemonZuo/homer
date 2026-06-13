package esximon

// ESXi sample 行构建与完整性判定。

import (
	"time"

	"github.com/LemonZuo/homer/internal/model"
)

// CoreTempPoint / DiskTempPoint 是 esxi_sample 里两个 JSON 列的最小形态。
// 故意比 client.go 的 CPUCore / DiskHealth 精简,只留历史曲线需要的字段,
// 减小落库体积(单次采样可能 8 核 4 盘,字段越精炼 disk 累积越友好)。
type CoreTempPoint struct {
	ID    int `json:"id"`
	TempC int `json:"temp_c"`
}

type DiskTempPoint struct {
	Device string `json:"device"`
	TempC  int    `json:"temp_c"`
}

// DiskUsagePoint 是 esxi_sample.disk_usage_json 的最小形态,
// 只留历史曲线画「单盘已用容量」需要的字段,沿用 sample 表里 -1/0 哨兵约定。
type DiskUsagePoint struct {
	Device        string `json:"device"`
	UsedBytes     int64  `json:"used_bytes"`
	CapacityBytes int64  `json:"capacity_bytes"`
}

// MemoryUsagePoint 是 esxi_sample.memory_usage_json 的最小形态。
// 同时落 used/total/percent,后续历史图表要切换绝对值或比率都不需要重采样。
type MemoryUsagePoint struct {
	UsedBytes    int64 `json:"used_bytes"`
	TotalBytes   int64 `json:"total_bytes"`
	UsagePercent int   `json:"usage_percent"`
}

func buildSample(r HostResult, now time.Time) model.EsxiSample {
	m := r.Metrics
	cpu := makeSampleCPU(m.CPUTemp)
	disks := makeSampleDisks(m.Disks)
	vms := makeSampleVMs(m.VMs)
	usage := makeSampleDiskUsage(m.Disks)
	memory := makeSampleMemoryUsage(m.Runtime)

	return model.EsxiSample{
		HostKind:             r.HostKind,
		HostID:               r.HostID,
		HostName:             r.HostName,
		CPUMaxC:              cpu.MaxC,
		CPUAvgC:              cpu.AvgC,
		CPUTjMaxC:            cpu.TjMaxC,
		MCEState:             m.MCE.State,
		MCECorrectedTotal:    m.MCE.CorrectedTotal,
		MCEUncorrectedTotal:  m.MCE.UncorrectedTotal,
		DiskMaxC:             disks.MaxC,
		CPUUsagePercent:      m.Runtime.CPUUsagePercent,
		MemoryUsageJSON:      memory,
		VMTotal:              vms.Total,
		VMPoweredOn:          vms.PoweredOn,
		CPUTempPerCoreJSON:   cpu.JSON,
		DiskTempPerDiskJSON:  disks.JSON,
		DiskUsagePerDiskJSON: usage,
		SampledAt:            now,
	}
}

type sampleCPUResult struct {
	MaxC   int
	AvgC   int
	TjMaxC int
	JSON   string
}

type sampleDiskResult struct {
	MaxC int
	JSON string
}

type sampleVMResult struct {
	Total     int
	PoweredOn int
}

func makeSampleCPU(t CPUTemperature) sampleCPUResult {
	points := make([]CoreTempPoint, 0, len(t.Cores))
	sum := 0
	maxC := -1
	for _, c := range t.Cores {
		points = append(points, CoreTempPoint{ID: c.ID, TempC: c.TempC})
		sum += c.TempC
		if c.TempC > maxC {
			maxC = c.TempC
		}
	}
	avg := t.AvgC
	if avg < 0 && len(points) > 0 {
		avg = sum / len(points)
	}
	if t.MaxC >= 0 {
		maxC = t.MaxC
	}
	return sampleCPUResult{MaxC: maxC, AvgC: avg, TjMaxC: t.TjMaxC, JSON: mustJSON(points)}
}

func makeSampleDisks(disks []DiskHealth) sampleDiskResult {
	maxC := -1
	points := make([]DiskTempPoint, 0, len(disks))
	for _, d := range disks {
		if d.TempC < 0 {
			continue
		}
		points = append(points, DiskTempPoint{Device: d.Device, TempC: d.TempC})
		if d.TempC > maxC {
			maxC = d.TempC
		}
	}
	if len(points) == 0 {
		return sampleDiskResult{MaxC: -1}
	}
	return sampleDiskResult{MaxC: maxC, JSON: mustJSON(points)}
}

// makeSampleDiskUsage 把每块盘的 used/capacity 摘成精简 JSON。
// 任何一块盘只要 used/capacity 有效就纳入,全无数据则返回空串。
func makeSampleDiskUsage(disks []DiskHealth) string {
	points := make([]DiskUsagePoint, 0, len(disks))
	for _, d := range disks {
		if d.CapacityBytes <= 0 && d.UsedBytes <= 0 {
			continue
		}
		points = append(points, DiskUsagePoint{
			Device:        d.Device,
			UsedBytes:     d.UsedBytes,
			CapacityBytes: d.CapacityBytes,
		})
	}
	if len(points) == 0 {
		return ""
	}
	return mustJSON(points)
}

func makeSampleMemoryUsage(runtime RuntimeUsage) string {
	usage := MemoryUsagePoint{
		UsedBytes:    runtime.MemoryUsedBytes,
		TotalBytes:   runtime.MemoryTotalBytes,
		UsagePercent: runtime.MemoryUsagePercent,
	}
	if usage.UsagePercent < 0 && usage.UsedBytes >= 0 && usage.TotalBytes > 0 {
		usage.UsagePercent = percentInt(usage.UsedBytes, usage.TotalBytes)
	}
	if usage.UsedBytes < 0 && usage.TotalBytes <= 0 && usage.UsagePercent < 0 {
		return ""
	}
	return mustJSON(usage)
}

func makeSampleVMs(vms []VM) sampleVMResult {
	on := 0
	for _, v := range vms {
		if v.State == "powered_on" {
			on++
		}
	}
	return sampleVMResult{Total: len(vms), PoweredOn: on}
}

func sampleMissingMetrics(m HostMetrics) []string {
	missing := make([]string, 0, 4)
	if !cpuTempComplete(m) {
		missing = append(missing, "cpu_temperature")
	}
	if !diskTempsComplete(m.Disks) {
		missing = append(missing, "disk_health")
	}
	if m.MCE.State == "" {
		missing = append(missing, "mce_health")
	}
	if !vmStatesComplete(m.VMs) {
		missing = append(missing, "vm_power_state")
	}
	return missing
}

func cpuTempComplete(m HostMetrics) bool {
	if m.CPUTemp.MaxC < 0 || len(m.CPUTemp.Cores) == 0 {
		return false
	}
	return m.CPU.Cores <= 0 || len(m.CPUTemp.Cores) >= m.CPU.Cores
}

func diskTempsComplete(disks []DiskHealth) bool {
	if len(disks) == 0 {
		return false
	}
	for _, d := range disks {
		if d.TempC < 0 {
			return false
		}
	}
	return true
}

func vmStatesComplete(vms []VM) bool {
	if vms == nil {
		return false
	}
	if len(vms) == 0 {
		return true
	}
	for _, v := range vms {
		if v.State == "" || v.State == "unknown" {
			return false
		}
	}
	return true
}

// topologyComplete 判定拓扑是否抓全:vSwitch 列表非空,且每台开机 VM 都在 vm_nics 里。
// 假设所有开机 VM 都至少有一块 vNIC(esxcli network vm list 只列有网络端口的 VM,
// 个人环境的 VM 全都带网卡;若以后出现无网卡 VM 再放宽)。
func topologyComplete(m HostMetrics) bool {
	if len(m.Topology.VSwitches) == 0 {
		return false
	}
	inTopo := make(map[string]bool, len(m.Topology.VMNICs))
	for _, l := range m.Topology.VMNICs {
		inTopo[l.VMName] = true
	}
	for _, v := range m.VMs {
		if v.State == "powered_on" && !inTopo[v.Name] {
			return false
		}
	}
	return true
}

func topologyFullyCollected(m HostMetrics) bool {
	t := m.Topology
	return t.Collected &&
		t.VSwitchCollected &&
		t.VMNetCollected &&
		t.VMPortsCollected &&
		t.VMKCollected &&
		t.VMKFullCollected &&
		topologyComplete(m)
}
