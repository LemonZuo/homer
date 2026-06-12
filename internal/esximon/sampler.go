package esximon

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/LemonZuo/homer/internal/logx"
	"github.com/LemonZuo/homer/internal/model"
	"github.com/LemonZuo/homer/internal/sshlike"
	"github.com/LemonZuo/homer/internal/sshx"
	"gorm.io/gorm"
)

// HostResult 单台机器一轮采集的结果。HostKind 恒为 model.EsxiHostKind = "esxi"。
type HostResult struct {
	HostKind string      `json:"host_kind"`
	HostID   int64       `json:"host_id"`
	HostName string      `json:"host_name"`
	Endpoint string      `json:"endpoint"`
	OK       bool        `json:"ok"`
	Error    string      `json:"error,omitempty"`
	Metrics  HostMetrics `json:"-"`
	StartAt  time.Time   `json:"-"`
}

// Sampler 负责一轮"扫所有 esxi_host → 并发 SSH → 跑 esxcli/vsish → 聚合结果"。
type Sampler struct {
	db                      *gorm.DB
	hosts                   *HostStore
	credentials             sshlike.CredentialResolver
	timeout                 time.Duration
	topologyRefreshInterval time.Duration
}

func NewSampler(db *gorm.DB, hosts *HostStore, credentials sshlike.CredentialResolver, sshTimeout, topologyRefreshInterval time.Duration) *Sampler {
	if sshTimeout <= 0 {
		sshTimeout = 120 * time.Second
	}
	if topologyRefreshInterval <= 0 {
		topologyRefreshInterval = 30 * time.Minute
	}
	return &Sampler{db: db, hosts: hosts, credentials: credentials, timeout: sshTimeout, topologyRefreshInterval: topologyRefreshInterval}
}

// Run 扫一轮所有启用的 esxi_host 并发采集,单机失败不影响其他。
func (s *Sampler) Run() ([]HostResult, error) {
	targets, err := s.hosts.ListEnabled()
	if err != nil {
		return nil, fmt.Errorf("列出 ESXi 主机失败:%w", err)
	}
	if len(targets) == 0 {
		return nil, nil
	}
	prevStates := s.loadPreviousStatesByHostID()

	results := make([]HostResult, len(targets))
	var wg sync.WaitGroup
	for i := range targets {
		wg.Add(1)
		go func(idx int, h model.EsxiHost) {
			defer wg.Done()
			var prev *model.EsxiState
			if p, ok := prevStates[h.ID]; ok {
				prev = &p
			}
			results[idx] = s.probeOne(h, prev)
		}(i, targets[i])
	}
	wg.Wait()
	return results, nil
}

// probeOne 单台机器一轮:连 SSH → 跑全套命令 → 关闭。任何关键步骤失败封装到 HostResult.Error。
func (s *Sampler) probeOne(h model.EsxiHost, prev *model.EsxiState) HostResult {
	res := HostResult{
		HostKind: model.EsxiHostKind,
		HostID:   h.ID,
		HostName: h.Name,
		Endpoint: h.Endpoint,
		StartAt:  time.Now(),
	}

	target, err := ParseEsxiTarget(h)
	if err != nil {
		res.Error = err.Error()
		return res
	}
	conn, err := sshlike.ConnFor(target, sshlike.ConnOptions{
		Credentials:        s.credentials,
		LoadBastion:        func(id int64) (*sshlike.Target, error) { return LoadEsxiBastion(s.db, id) },
		RejectBastionChain: true,
	})
	if err != nil {
		res.Error = err.Error()
		return res
	}

	// 给整轮采集套一个总超时(sshx.Dial 自带 15s 拨号超时,这里只罩跑命令阶段)。
	done := make(chan struct{})
	var metrics HostMetrics
	var probeErr error
	go func() {
		defer close(done)
		client, cleanup, derr := sshx.Dial(nil, conn)
		if derr != nil {
			probeErr = derr
			return
		}
		defer cleanup()
		opts := topologyCollectOptions(prev, res.StartAt, s.topologyRefreshInterval)
		if opts.SkipTopology {
			logx.Debug("esxi topology collection skipped",
				"host", h.Name,
				"last_full_success_at", opts.PreviousTopology.LastFullSuccessAt.Format(time.RFC3339),
				"interval", s.topologyRefreshInterval.String())
		}
		metrics = CollectAllWithOptions(client, opts)
		missing := probeMissing(metrics)
		for attempt := 2; len(missing) > 0 && attempt <= 2; attempt++ {
			logx.Warn("esxi probe incomplete, retrying",
				"host", h.Name, "attempt", attempt, "missing", strings.Join(missing, ","))
			next := CollectAllWithOptions(client, opts)
			metrics = mergeHostMetrics(metrics, next)
			missing = probeMissing(metrics)
		}
		if topologyFullyCollected(metrics) && metrics.Topology.LastFullSuccessAt.IsZero() {
			metrics.Topology.LastFullSuccessAt = time.Now()
		}
		if len(missing) > 0 {
			logx.Warn("esxi probe incomplete after retries", "host", h.Name, "missing", strings.Join(missing, ","))
		}
	}()
	select {
	case <-done:
	case <-time.After(s.timeout + 15*time.Second):
		probeErr = errors.New("采集超时")
	}

	if probeErr != nil {
		res.Error = probeErr.Error()
		logx.Debug("esxi probe failed", "host", h.Name, "err", probeErr)
		return res
	}
	res.OK = true
	res.Metrics = metrics
	return res
}

func (s *Sampler) loadPreviousStatesByHostID() map[int64]model.EsxiState {
	var rows []model.EsxiState
	if err := s.db.Find(&rows).Error; err != nil {
		logx.Warn("esxi previous state load failed", "err", err.Error())
		return nil
	}
	out := make(map[int64]model.EsxiState, len(rows))
	for _, row := range rows {
		if row.HostKind == model.EsxiHostKind {
			out[row.HostID] = row
		}
	}
	return out
}

func topologyCollectOptions(prev *model.EsxiState, now time.Time, interval time.Duration) CollectOptions {
	if prev == nil || prev.TopologyJSON == "" {
		return CollectOptions{}
	}
	topo, ok := parsePrevTopology(prev.TopologyJSON)
	if !ok {
		return CollectOptions{}
	}
	if topologyRefreshDue(topo, now, interval) {
		return CollectOptions{PreviousTopology: topo}
	}
	return CollectOptions{SkipTopology: true, PreviousTopology: topo}
}

func topologyRefreshDue(topo NetTopology, now time.Time, interval time.Duration) bool {
	if interval <= 0 || topo.LastFullSuccessAt.IsZero() {
		return true
	}
	return now.Sub(topo.LastFullSuccessAt) > interval
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

func mergeHostMetrics(base, next HostMetrics) HostMetrics {
	if !platformUsable(base.Platform) && platformUsable(next.Platform) {
		base.Platform = next.Platform
	} else {
		base.Platform = mergePlatform(base.Platform, next.Platform)
	}
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
	if len(base.NICs) == 0 && len(next.NICs) > 0 {
		base.NICs = next.NICs
	}
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
			if v.Name == "" {
				v.Name = nv.Name
			}
			if v.GuestOS == "" {
				v.GuestOS = nv.GuestOS
			}
			if v.State == "" || v.State == "unknown" {
				v.State = nv.State
			}
		}
		out = append(out, v)
	}
	return out
}

// ProbeByHostID 给 handler 的 /esxi/hosts/:id/test 用:按 id 拉一条立即探测。
func (s *Sampler) ProbeByHostID(id int64) (HostResult, error) {
	h, err := s.hosts.Get(id)
	if err != nil {
		return HostResult{}, err
	}
	var prev *model.EsxiState
	if row, ok := s.loadPreviousStatesByHostID()[h.ID]; ok {
		prev = &row
	}
	return s.probeOne(*h, prev), nil
}
