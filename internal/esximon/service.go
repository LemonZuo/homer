package esximon

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/LemonZuo/homer/internal/logx"
	"github.com/LemonZuo/homer/internal/model"
	"github.com/LemonZuo/homer/internal/notify"
	"gorm.io/gorm"
)

// Service 串起一轮"扫描 → 采集 → 持久化 → SSE 广播"。
type Service struct {
	db        *gorm.DB
	sampler   *Sampler
	store     *Store
	hosts     *HostStore
	alertOut  notify.Notifier
	alertCfg  AlertConfig
	sse       *sseHub
	retention time.Duration
}

func NewService(db *gorm.DB, sampler *Sampler, store *Store, hosts *HostStore, alertOut notify.Notifier, alertCfg AlertConfig, retention time.Duration) *Service {
	if retention <= 0 {
		retention = 7 * 24 * time.Hour
	}
	return &Service{
		db:        db,
		sampler:   sampler,
		store:     store,
		hosts:     hosts,
		alertOut:  alertOut,
		alertCfg:  alertCfg.withDefaults(),
		sse:       newSSEHub(),
		retention: retention,
	}
}

// Subscribe 给 SSE handler 用:订阅采集完成后的 snapshot 广播。
func (s *Service) Subscribe() (<-chan []Snapshot, func()) { return s.sse.Subscribe() }

// RunSample cron 调用入口。整轮失败返回 error,单机失败封进 esxi_state.LastError。
func (s *Service) RunSample() error {
	results, err := s.sampler.Run()
	if err != nil {
		return err
	}
	if len(results) == 0 {
		return nil
	}

	// 先读一次上一轮的 state,后面 buildState 用来兜底单项失败的 JSON 列。
	// 单台 ESXi 一行 state,n 很小,这里直接全表扫即可。
	prevStates, _ := s.store.LoadStates()
	prevByHost := map[string]model.EsxiState{}
	for _, st := range prevStates {
		prevByHost[fmt.Sprintf("%s/%d", st.HostKind, st.HostID)] = st
	}

	now := time.Now()
	var samples []model.EsxiSample
	var states []model.EsxiState
	type alertSeed struct {
		prev *model.EsxiState
		cur  HostResult
	}
	var alerts []alertSeed
	reachableHosts := 0
	for _, r := range results {
		if r.OK {
			reachableHosts++
		}
		missing := sampleMissingMetrics(r.Metrics)
		sampleOK := r.OK && len(missing) == 0
		if r.OK && !sampleOK {
			r.Error = "采集不完整:" + strings.Join(missing, ",")
			logx.Warn("esxi sample skipped: incomplete metrics",
				"host", r.HostName, "host_id", r.HostID, "missing", strings.Join(missing, ","))
		}
		var prev *model.EsxiState
		if p, ok := prevByHost[fmt.Sprintf("%s/%d", r.HostKind, r.HostID)]; ok {
			prev = &p
		}
		st := buildState(r, prev, now, sampleOK)
		states = append(states, st)
		if sampleOK {
			samples = append(samples, buildSample(r, now))
		}
		alerts = append(alerts, alertSeed{prev: prev, cur: r})
	}

	if err := s.store.SaveSamples(samples); err != nil {
		return fmt.Errorf("写入 esxi_sample 失败:%w", err)
	}
	if err := s.store.UpsertState(states); err != nil {
		return fmt.Errorf("更新 esxi_state 失败:%w", err)
	}

	for _, a := range alerts {
		s.processThresholdAlerts(a.prev, a.cur)
	}

	logx.Info("esxi sample done",
		"hosts", len(results), "reachable", reachableHosts, "samples", len(samples))

	if snap, err := s.BuildSnapshot(); err == nil {
		s.sse.Publish(snap)
	} else {
		logx.Warn("esxi snapshot publish skipped", "err", err.Error())
	}
	return nil
}

// RunCleanup 清理过期 sample。
func (s *Service) RunCleanup() error {
	cutoff := time.Now().Add(-s.retention)
	n, err := s.store.PurgeOlderThan(cutoff)
	if err != nil {
		return err
	}
	if n > 0 {
		logx.Info("esxi sample purged", "cutoff", cutoff.Format(time.RFC3339), "rows", n)
	}
	return nil
}

// TriggerSample 手动触发(refresh 按钮)。
func (s *Service) TriggerSample() error { return s.RunSample() }

// --- Snapshot 接口 ---

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

// Series 暴露给 handler 的曲线接口。
func (s *Service) Series(hostKind string, hostID int64, window time.Duration) ([]SeriesPoint, error) {
	if window <= 0 {
		window = 24 * time.Hour
	}
	bucket := pickBucket(window)
	since := time.Now().Add(-window)
	return s.store.Series(hostKind, hostID, since, bucket)
}

// pickBucket 按窗口大小返回桶宽(秒)。控制画图点数 ≈ 300。
func pickBucket(window time.Duration) int {
	switch {
	case window <= 6*time.Hour:
		return 60 // 1min
	case window <= 24*time.Hour:
		return 5 * 60 // 5min
	case window <= 3*24*time.Hour:
		return 15 * 60 // 15min
	default:
		return 30 * 60 // 30min
	}
}

// --- 工具:把 sampler 结果转 sample / state ---

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

func buildSample(r HostResult, now time.Time) model.EsxiSample {
	m := r.Metrics
	cpu := makeSampleCPU(m.CPUTemp)
	disks := makeSampleDisks(m.Disks)
	vms := makeSampleVMs(m.VMs)

	return model.EsxiSample{
		HostKind:            r.HostKind,
		HostID:              r.HostID,
		HostName:            r.HostName,
		CPUMaxC:             cpu.MaxC,
		CPUAvgC:             cpu.AvgC,
		CPUTjMaxC:           cpu.TjMaxC,
		MCEState:            m.MCE.State,
		MCECorrectedTotal:   m.MCE.CorrectedTotal,
		MCEUncorrectedTotal: m.MCE.UncorrectedTotal,
		DiskMaxC:            disks.MaxC,
		VMTotal:             vms.Total,
		VMPoweredOn:         vms.PoweredOn,
		CPUTempPerCoreJSON:  cpu.JSON,
		DiskTempPerDiskJSON: disks.JSON,
		SampledAt:           now,
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
	st.PlatformJSON = stickyJSON(m.Platform, m.Platform.Vendor != "" || m.Platform.UUID != "" || m.Platform.Product != "", prevJSON(prev, func(p *model.EsxiState) string { return p.PlatformJSON }))
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
	st.TopologyJSON = stickyJSON(m.Topology, topologyComplete(m), prevJSON(prev, func(p *model.EsxiState) string { return p.TopologyJSON }))
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
	}
	if usbHasData(merged) || cur.controllersKnown || cur.arbitratorKnown || cur.passthroughKnown {
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

// ErrNoHosts 表示没有候选机器(handler 区分"未配机器"与"配了但全坏")。
var ErrNoHosts = errors.New("没有可监控的机器(请在「ESXi 机器」里添加要采集的主机)")
