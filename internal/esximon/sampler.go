package esximon

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/LemonZuo/homer/internal/acme/deployer/sshx"
	"github.com/LemonZuo/homer/internal/esximon/sshhost"
	"github.com/LemonZuo/homer/internal/logx"
	"github.com/LemonZuo/homer/internal/model"
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
	db          *gorm.DB
	hosts       *HostStore
	credentials sshhost.CredentialResolver
	timeout     time.Duration
}

func NewSampler(db *gorm.DB, hosts *HostStore, credentials sshhost.CredentialResolver, sshTimeout time.Duration) *Sampler {
	if sshTimeout <= 0 {
		sshTimeout = 120 * time.Second
	}
	return &Sampler{db: db, hosts: hosts, credentials: credentials, timeout: sshTimeout}
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

	results := make([]HostResult, len(targets))
	var wg sync.WaitGroup
	for i := range targets {
		wg.Add(1)
		go func(idx int, h model.EsxiHost) {
			defer wg.Done()
			results[idx] = s.probeOne(h)
		}(i, targets[i])
	}
	wg.Wait()
	return results, nil
}

// probeOne 单台机器一轮:连 SSH → 跑全套命令 → 关闭。任何关键步骤失败封装到 HostResult.Error。
func (s *Sampler) probeOne(h model.EsxiHost) HostResult {
	res := HostResult{
		HostKind: model.EsxiHostKind,
		HostID:   h.ID,
		HostName: h.Name,
		Endpoint: h.Endpoint,
		StartAt:  time.Now(),
	}

	target, err := sshhost.ParseTarget(h)
	if err != nil {
		res.Error = err.Error()
		return res
	}
	conn, err := sshhost.ConnFor(target, sshhost.ConnOptions{
		Credentials: s.credentials,
		DB:          s.db,
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
		metrics = CollectAll(client)
		missing := sampleMissingMetrics(metrics)
		for attempt := 2; len(missing) > 0 && attempt <= 2; attempt++ {
			logx.Warn("esxi probe incomplete, retrying",
				"host", h.Name, "attempt", attempt, "missing", strings.Join(missing, ","))
			next := CollectAll(client)
			metrics = mergeHostMetrics(metrics, next)
			missing = sampleMissingMetrics(metrics)
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
		base.Disks = mergeDiskTemperatures(base.Disks, next.Disks)
	}
	base.USB = mergeUSBState(base.USB, next.USB)
	if !vmStatesComplete(base.VMs) && vmStatesComplete(next.VMs) {
		base.VMs = next.VMs
	} else {
		base.VMs = mergeVMStates(base.VMs, next.VMs)
	}
	return base
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

func mergeDiskTemperatures(base, next []DiskTemperature) []DiskTemperature {
	if len(base) == 0 {
		return next
	}
	byDevice := map[string]DiskTemperature{}
	for _, d := range next {
		byDevice[d.Device] = d
	}
	out := make([]DiskTemperature, 0, len(base))
	for _, d := range base {
		if nd, ok := byDevice[d.Device]; ok {
			d = mergeDiskTemperature(d, nd)
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

func mergeDiskTemperature(base, next DiskTemperature) DiskTemperature {
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
	return s.probeOne(*h), nil
}
