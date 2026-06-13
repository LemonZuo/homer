package esximon

// ESXi 慢刷新策略和重试缺项判定。

import (
	"time"

	"github.com/LemonZuo/homer/internal/model"
)

func collectOptions(prev *model.EsxiState, now time.Time, interval time.Duration) CollectOptions {
	opts := CollectOptions{}
	if prev == nil {
		return opts
	}
	var hasPlatform bool
	if p, ok := parsePrevPlatform(prev.PlatformJSON); ok {
		opts.PreviousPlatform = p
		hasPlatform = true
	}
	if c, ok := parsePrevCPU(prev.CPUStaticJSON); ok {
		opts.PreviousCPU = c
	}
	if m, ok := parsePrevMemory(prev.MemoryJSON); ok {
		opts.PreviousMemory = m
	}
	if hasPlatform && platformUsable(opts.PreviousPlatform) && cpuStaticUsable(opts.PreviousCPU) && memoryUsable(opts.PreviousMemory) {
		opts.SkipStatic = !slowRefreshDue(opts.PreviousPlatform.StaticLastFullSuccessAt, now, interval)
	}
	if vms, ok := parsePrevVMs(prev.VMJSON); ok {
		opts.PreviousVMs = vms
		if vmStatesComplete(vms) {
			opts.SkipVMPower = !slowRefreshDue(vmsPowerLastFullSuccessAt(vms), now, interval)
		}
	}
	if usb, ok := parsePrevUSB(prev.USBJSON); ok {
		opts.PreviousUSB = usb
		opts.SkipUSBVMOwned = !slowRefreshDue(usb.VMOwnedLastFullSuccessAt, now, interval)
	}
	if disks, ok := parsePrevDisks(prev.DiskJSON); ok {
		opts.PreviousDisks = disks
		if diskTempsComplete(disks) {
			opts.SkipDiskSMART = !slowRefreshDue(disksSMARTLastFullSuccessAt(disks), now, interval)
		}
	}
	if nics, ok := parsePrevNICs(prev.NICJSON); ok {
		opts.PreviousNICs = nics
		if len(nics) > 0 {
			opts.SkipNICStats = !slowRefreshDue(nicsStatsLastFullSuccessAt(nics), now, interval)
		}
	}
	if topo, ok := parsePrevTopology(prev.TopologyJSON); ok {
		opts.PreviousTopology = topo
		opts.SkipTopology = !slowRefreshDue(topo.LastFullSuccessAt, now, interval)
	}
	return opts
}

func slowRefreshDue(last time.Time, now time.Time, interval time.Duration) bool {
	if interval <= 0 || last.IsZero() {
		return true
	}
	return now.Sub(last) > interval
}

func vmsPowerLastFullSuccessAt(vms []VM) time.Time {
	if len(vms) == 0 {
		return time.Time{}
	}
	last := vms[0].PowerStateLastFullSuccessAt
	for _, vm := range vms {
		if vm.PowerStateLastFullSuccessAt.IsZero() {
			return time.Time{}
		}
		if vm.PowerStateLastFullSuccessAt.Before(last) {
			last = vm.PowerStateLastFullSuccessAt
		}
	}
	return last
}

func disksSMARTLastFullSuccessAt(disks []DiskHealth) time.Time {
	if len(disks) == 0 {
		return time.Time{}
	}
	last := disks[0].SMARTLastFullSuccessAt
	for _, disk := range disks {
		if disk.SMARTLastFullSuccessAt.IsZero() {
			return time.Time{}
		}
		if disk.SMARTLastFullSuccessAt.Before(last) {
			last = disk.SMARTLastFullSuccessAt
		}
	}
	return last
}

func nicsStatsLastFullSuccessAt(nics []NIC) time.Time {
	if len(nics) == 0 {
		return time.Time{}
	}
	last := nics[0].StatsLastFullSuccessAt
	for _, nic := range nics {
		if nic.StatsLastFullSuccessAt.IsZero() {
			return time.Time{}
		}
		if nic.StatsLastFullSuccessAt.Before(last) {
			last = nic.StatsLastFullSuccessAt
		}
	}
	return last
}

// probeMissing 在 sampleMissingMetrics 基础上追加拓扑完整性,驱动同一轮内的整轮重试。
// 拓扑不写 esxi_sample,所以 sampleMissingMetrics 不含它(不挡时序入库),
// 这里加上只为了重试时把缺的 VM 边补全;最终完整性兜底在 buildState 的 prev 回退。
func probeMissing(m HostMetrics) []string {
	missing := sampleMissingMetrics(m)
	if !m.Topology.Skipped && !topologyComplete(m) {
		missing = append(missing, "net_topology")
	}
	return missing
}
