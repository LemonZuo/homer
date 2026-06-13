package esximon

// ESXi 多次探测结果合并。

import (
	"strings"
)

func mergeHostMetrics(base, next HostMetrics) HostMetrics {
	staticNewer := next.Platform.StaticLastFullSuccessAt.After(base.Platform.StaticLastFullSuccessAt)
	if staticNewer {
		base.Platform = next.Platform
		base.CPU = next.CPU
		base.Memory = next.Memory
	} else if !platformUsable(base.Platform) && platformUsable(next.Platform) {
		base.Platform = next.Platform
	} else {
		base.Platform = mergePlatform(base.Platform, next.Platform)
	}
	if !staticNewer {
		if !cpuStaticUsable(base.CPU) && cpuStaticUsable(next.CPU) {
			base.CPU = next.CPU
		} else {
			base.CPU = mergeCPUStatic(base.CPU, next.CPU)
		}
		if !memoryUsable(base.Memory) && memoryUsable(next.Memory) {
			base.Memory = next.Memory
		} else {
			base.Memory = mergeMemory(base.Memory, next.Memory)
		}
	}
	if !runtimeUsable(base.Runtime) && runtimeUsable(next.Runtime) {
		base.Runtime = next.Runtime
	} else {
		base.Runtime = mergeRuntime(base.Runtime, next.Runtime)
	}
	if !cpuTempComplete(base) && cpuTempComplete(next) {
		base.CPUTemp = next.CPUTemp
	}
	if base.MCE.State == "" && next.MCE.State != "" {
		base.MCE = next.MCE
	}
	if !diskTempsComplete(base.Disks) && diskTempsComplete(next.Disks) {
		base.Disks = next.Disks
	} else {
		base.Disks = mergeDiskHealthList(base.Disks, next.Disks)
	}
	base.USB = mergeUSBState(base.USB, next.USB)
	if !vmStatesComplete(base.VMs) && vmStatesComplete(next.VMs) {
		base.VMs = next.VMs
	} else {
		base.VMs = mergeVMStates(base.VMs, next.VMs)
	}
	if base.Boot.UptimeSeconds < 0 && next.Boot.UptimeSeconds >= 0 {
		base.Boot = next.Boot
	}
	base.NICs = mergeNICList(base.NICs, next.NICs)
	base.Topology = mergeTopology(base.Topology, next.Topology)
	return base
}

// mergeTopology 两轮采集的拓扑做并集:vSwitch 按整组补,vm_nics 按 vm_name+mac 去重合并,
// vmk_ports 按 vmk 名称合并。
// `esxcli network vm list` 偶发截断会让单轮只抓到部分 VM,并集能把两轮各自抓到的拼全。
func mergeTopology(base, next NetTopology) NetTopology {
	if len(base.VSwitches) == 0 {
		base.VSwitches = next.VSwitches
	}
	idx := make(map[string]int, len(base.VMNICs))
	for i, l := range base.VMNICs {
		idx[vmNICKey(l)] = i
	}
	for _, l := range next.VMNICs {
		key := vmNICKey(l)
		if i, ok := idx[key]; ok {
			base.VMNICs[i] = mergeVMNICLink(base.VMNICs[i], l)
			continue
		}
		idx[key] = len(base.VMNICs)
		base.VMNICs = append(base.VMNICs, l)
	}
	if next.VMKCollected {
		base.VMKPorts = mergeVMKPorts(base.VMKPorts, next.VMKPorts)
		base.VMKCollected = true
	}
	base = mergeTopologyCollectionState(base, next)
	return base
}

func mergeTopologyCollectionState(base, next NetTopology) NetTopology {
	if next.LastFullSuccessAt.After(base.LastFullSuccessAt) {
		base.LastFullSuccessAt = next.LastFullSuccessAt
	}
	base.Skipped = base.Skipped && next.Skipped
	base.Collected = base.Collected || next.Collected
	base.VSwitchCollected = base.VSwitchCollected || next.VSwitchCollected
	base.VMNetCollected = base.VMNetCollected || next.VMNetCollected
	base.VMPortsCollected = base.VMPortsCollected || next.VMPortsCollected
	base.VMKCollected = base.VMKCollected || next.VMKCollected
	base.VMKFullCollected = base.VMKFullCollected || next.VMKFullCollected
	return base
}

func vmNICKey(l VMNICLink) string {
	return l.VMName + "|" + strings.ToLower(l.MAC)
}

func mergeVMNICLink(base, next VMNICLink) VMNICLink {
	if next.VMID != 0 {
		base.VMID = next.VMID
	}
	if next.VSwitch != "" {
		base.VSwitch = next.VSwitch
	}
	if next.Portgroup != "" {
		base.Portgroup = next.Portgroup
	}
	if next.MAC != "" {
		base.MAC = next.MAC
	}
	if next.IP != "" {
		base.IP = next.IP
	}
	if next.TeamUplink != "" {
		base.TeamUplink = next.TeamUplink
	}
	return base
}

func mergeVMKPorts(base, next []VMKPort) []VMKPort {
	idx := make(map[string]int, len(base))
	for i, p := range base {
		idx[p.Name] = i
	}
	for _, p := range next {
		if p.Name == "" {
			continue
		}
		if i, ok := idx[p.Name]; ok {
			base[i] = mergeVMKPort(base[i], p)
			continue
		}
		idx[p.Name] = len(base)
		base = append(base, p)
	}
	return base
}

func mergeVMKPort(base, next VMKPort) VMKPort {
	if next.VSwitch == "" {
		next.VSwitch = base.VSwitch
	}
	if next.Portgroup == "" {
		next.Portgroup = base.Portgroup
	}
	if next.MAC == "" {
		next.MAC = base.MAC
	}
	if next.IPv4 == "" {
		next.IPv4 = base.IPv4
	}
	return next
}

func platformUsable(p PlatformInfo) bool {
	return p.Vendor != "" || p.Product != "" || p.UUID != "" || p.ESXiVersion != ""
}

func mergePlatform(base, next PlatformInfo) PlatformInfo {
	if base.Vendor == "" {
		base.Vendor = next.Vendor
	}
	if base.Product == "" {
		base.Product = next.Product
	}
	if base.Serial == "" {
		base.Serial = next.Serial
	}
	if base.UUID == "" {
		base.UUID = next.UUID
	}
	if !base.IPMISupported {
		base.IPMISupported = next.IPMISupported
	}
	if base.ESXiVersion == "" {
		base.ESXiVersion = next.ESXiVersion
	}
	if base.ESXiBuild == 0 {
		base.ESXiBuild = next.ESXiBuild
	}
	return base
}

func cpuStaticUsable(c CPUStatic) bool {
	return c.Brand != "" || c.Cores > 0
}

func mergeCPUStatic(base, next CPUStatic) CPUStatic {
	if base.Brand == "" {
		base.Brand = next.Brand
	}
	if base.Family == 0 {
		base.Family = next.Family
	}
	if base.ModelID == 0 {
		base.ModelID = next.ModelID
	}
	if base.Stepping == 0 {
		base.Stepping = next.Stepping
	}
	if base.Cores == 0 {
		base.Cores = next.Cores
	}
	if base.FreqMHz == 0 {
		base.FreqMHz = next.FreqMHz
	}
	if base.L2KB == 0 {
		base.L2KB = next.L2KB
	}
	if base.L3KB == 0 {
		base.L3KB = next.L3KB
	}
	if base.TjMaxC <= 0 {
		base.TjMaxC = next.TjMaxC
	}
	return base
}

func memoryUsable(m MemoryInfo) bool {
	return m.TotalBytes > 0
}

func mergeMemory(base, next MemoryInfo) MemoryInfo {
	if base.TotalBytes <= 0 {
		base.TotalBytes = next.TotalBytes
	}
	if base.FreeBytes <= 0 {
		base.FreeBytes = next.FreeBytes
	}
	return base
}

func runtimeUsable(u RuntimeUsage) bool {
	return (u.CPUUsagePercent >= 0 && u.CPUCapacityMHz > 0) ||
		(u.MemoryUsagePercent >= 0 && u.MemoryTotalBytes > 0)
}

func mergeRuntime(base, next RuntimeUsage) RuntimeUsage {
	if base.CPUUsagePercent < 0 {
		base.CPUUsedMHz = next.CPUUsedMHz
		base.CPUCapacityMHz = next.CPUCapacityMHz
		base.CPUUsagePercent = next.CPUUsagePercent
	}
	if base.MemoryUsagePercent < 0 {
		base.MemoryUsedBytes = next.MemoryUsedBytes
		base.MemoryTotalBytes = next.MemoryTotalBytes
		base.MemoryUsagePercent = next.MemoryUsagePercent
	}
	return base
}

func mergeDiskHealthList(base, next []DiskHealth) []DiskHealth {
	if len(base) == 0 {
		return next
	}
	byDevice := map[string]DiskHealth{}
	for _, d := range next {
		byDevice[d.Device] = d
	}
	out := make([]DiskHealth, 0, len(base))
	for _, d := range base {
		if nd, ok := byDevice[d.Device]; ok {
			d = mergeDiskHealth(d, nd)
		}
		out = append(out, d)
	}
	seen := map[string]struct{}{}
	for _, d := range out {
		seen[d.Device] = struct{}{}
	}
	for _, d := range next {
		if _, ok := seen[d.Device]; !ok {
			out = append(out, d)
		}
	}
	return out
}

func mergeDiskHealth(base, next DiskHealth) DiskHealth {
	if next.SMARTLastFullSuccessAt.After(base.SMARTLastFullSuccessAt) {
		base.TempC = next.TempC
		base.ThresholdC = next.ThresholdC
		base.Status = next.Status
		base.HealthStatus = next.HealthStatus
		base.PowerOnHours = next.PowerOnHours
		base.PowerCycleCount = next.PowerCycleCount
		base.ReallocatedSectors = next.ReallocatedSectors
		base.UncorrectableErrors = next.UncorrectableErrors
		base.MediaWearoutValue = next.MediaWearoutValue
		base.ReadErrorCount = next.ReadErrorCount
		base.PendingSectorReallocation = next.PendingSectorReallocation
		base.SMARTLastFullSuccessAt = next.SMARTLastFullSuccessAt
	}
	if base.Model == "" {
		base.Model = next.Model
	}
	if base.Type == "" {
		base.Type = next.Type
	}
	if base.CapacityBytes <= 0 {
		base.CapacityBytes = next.CapacityBytes
	}
	if base.UsedBytes < 0 {
		base.UsedBytes = next.UsedBytes
	}
	if base.FreeBytes < 0 {
		base.FreeBytes = next.FreeBytes
	}
	if len(base.Datastores) == 0 {
		base.Datastores = next.Datastores
	}
	if base.TempC < 0 {
		base.TempC = next.TempC
	}
	if base.ThresholdC < 0 {
		base.ThresholdC = next.ThresholdC
	}
	if base.Status == "" || base.Status == "unknown" {
		base.Status = next.Status
	}
	if base.SMARTLastFullSuccessAt.IsZero() {
		base.SMARTLastFullSuccessAt = next.SMARTLastFullSuccessAt
	}
	return base
}

func mergeUSBState(base, next USBState) USBState {
	if !base.controllersKnown && next.controllersKnown {
		base.Controllers = next.Controllers
		base.controllersKnown = true
	}
	if !base.arbitratorKnown && next.arbitratorKnown {
		base.ArbitratorRunning = next.ArbitratorRunning
		base.arbitratorKnown = true
	}
	if !base.passthroughKnown && next.passthroughKnown {
		base.AvailableForPassthrough = next.AvailableForPassthrough
		base.passthroughKnown = true
	}
	if len(base.VMOwned) == 0 && len(next.VMOwned) > 0 {
		base.VMOwned = next.VMOwned
	}
	if next.VMOwnedLastFullSuccessAt.After(base.VMOwnedLastFullSuccessAt) {
		base.VMOwned = next.VMOwned
		base.VMOwnedLastFullSuccessAt = next.VMOwnedLastFullSuccessAt
		base.vmOwnedKnown = true
	} else if base.VMOwnedLastFullSuccessAt.IsZero() {
		base.VMOwnedLastFullSuccessAt = next.VMOwnedLastFullSuccessAt
	}
	return base
}

func mergeVMStates(base, next []VM) []VM {
	if base == nil {
		return next
	}
	if len(base) == 0 {
		return base
	}
	byID := map[int]VM{}
	for _, v := range next {
		byID[v.ID] = v
	}
	out := make([]VM, 0, len(base))
	for _, v := range base {
		if nv, ok := byID[v.ID]; ok {
			if nv.PowerStateLastFullSuccessAt.After(v.PowerStateLastFullSuccessAt) {
				v.State = nv.State
				v.PowerStateLastFullSuccessAt = nv.PowerStateLastFullSuccessAt
			}
			if v.Name == "" {
				v.Name = nv.Name
			}
			if v.GuestOS == "" {
				v.GuestOS = nv.GuestOS
			}
			if v.State == "" || v.State == "unknown" {
				v.State = nv.State
			}
			if v.PowerStateLastFullSuccessAt.IsZero() {
				v.PowerStateLastFullSuccessAt = nv.PowerStateLastFullSuccessAt
			}
		}
		out = append(out, v)
	}
	return out
}

func mergeNICList(base, next []NIC) []NIC {
	if len(base) == 0 {
		return next
	}
	if len(next) == 0 {
		return base
	}
	byName := map[string]NIC{}
	for _, n := range next {
		byName[n.Name] = n
	}
	out := make([]NIC, 0, len(base))
	for _, n := range base {
		if nn, ok := byName[n.Name]; ok {
			n = mergeNIC(n, nn)
		}
		out = append(out, n)
	}
	seen := map[string]struct{}{}
	for _, n := range out {
		seen[n.Name] = struct{}{}
	}
	for _, n := range next {
		if _, ok := seen[n.Name]; !ok {
			out = append(out, n)
		}
	}
	return out
}

func mergeNIC(base, next NIC) NIC {
	if next.Driver != "" {
		base.Driver = next.Driver
	}
	if next.MAC != "" {
		base.MAC = next.MAC
	}
	if next.MTU >= 0 {
		base.MTU = next.MTU
	}
	if next.Description != "" {
		base.Description = next.Description
	}
	if next.AdminStatus != "" {
		base.AdminStatus = next.AdminStatus
	}
	if next.LinkStatus != "" {
		base.LinkStatus = next.LinkStatus
	}
	if next.SpeedMbps >= 0 {
		base.SpeedMbps = next.SpeedMbps
	}
	if next.Duplex != "" {
		base.Duplex = next.Duplex
	}
	if next.StatsLastFullSuccessAt.After(base.StatsLastFullSuccessAt) {
		base.RxBytes = next.RxBytes
		base.TxBytes = next.TxBytes
		base.RxErrors = next.RxErrors
		base.TxErrors = next.TxErrors
		base.RxDropped = next.RxDropped
		base.TxDropped = next.TxDropped
		base.StatsLastFullSuccessAt = next.StatsLastFullSuccessAt
	} else if base.StatsLastFullSuccessAt.IsZero() {
		base.StatsLastFullSuccessAt = next.StatsLastFullSuccessAt
	}
	return base
}
