// Package esximon 通过 SSH 远程执行 ESXi 的 esxcli / vsish / vim-cmd 命令,
// 采集平台、CPU 温度、磁盘 SMART、MCE、USB、VM 等信息。机器来源 esxi_host 表,
// 凭证来源 esxi_ssh_credential,与 UPS / ACME 完全解耦。
//
// 数据采集策略:每轮 SSH 连接复用,**串行**地跑 N 条命令(esxcli/vsish 都很轻),
// 不在远端拼 JSON —— 直接把每条命令的纯文本拉回本地用 Go 解析,避开 busybox
// 转义陷阱。详见 prompt/6_ESXI_SSH_MONITORING.md。
package esximon

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/LemonZuo/homer/internal/acme/deployer/sshx"
	"github.com/LemonZuo/homer/internal/logx"
	"golang.org/x/crypto/ssh"
)

// --- 数据形态(对应 prompt 里的 JSON 范式) ---

// PlatformInfo 主机标识 + ESXi 版本(启动时一次就够,但我们每轮都顺手刷一次)。
type PlatformInfo struct {
	Vendor        string `json:"vendor"`
	Product       string `json:"product"`
	Serial        string `json:"serial"`
	UUID          string `json:"uuid"`
	IPMISupported bool   `json:"ipmi_supported"`
	ESXiVersion   string `json:"esxi_version"`
	ESXiBuild     int64  `json:"esxi_build"`
}

// CPUStatic 静态 CPU 信息。
type CPUStatic struct {
	Brand    string `json:"brand"`
	Family   int    `json:"family"`
	ModelID  int    `json:"model"`
	Stepping int    `json:"stepping"`
	Cores    int    `json:"cores"`
	FreqMHz  int    `json:"freq_mhz"`
	L2KB     int    `json:"l2_kb"`
	L3KB     int    `json:"l3_kb"`
	TjMaxC   int    `json:"tjmax_c"`
}

// MemoryInfo 内存。
type MemoryInfo struct {
	TotalBytes    int64 `json:"mem_total_bytes"`
	ReliableBytes int64 `json:"mem_reliable_bytes"`
	NUMANodes     int   `json:"mem_numa_nodes"`
}

// CPUCore 每核温度。
type CPUCore struct {
	ID        int `json:"id"`
	TempC     int `json:"temp_c"`
	HeadroomC int `json:"headroom_c"`
}

// CPUTemperature 全核温度聚合。
type CPUTemperature struct {
	TjMaxC int       `json:"tjmax_c"`
	Cores  []CPUCore `json:"cores"`
	MaxC   int       `json:"max_c"`
	AvgC   int       `json:"avg_c"`
}

// MCEHealth Machine Check Error 健康状态。
type MCEHealth struct {
	State            string `json:"state"` // Green / Yellow / Red
	CorrectedTotal   int64  `json:"corrected_total"`
	CorrectedEWMA    int64  `json:"corrected_ewma"`
	PeriodSeconds    int    `json:"period_seconds"`
	UncorrectedTotal int64  `json:"uncorrected_total"`
}

// DiskTemperature 单块盘温度。
type DiskTemperature struct {
	Device     string `json:"device"`
	Model      string `json:"model"`
	Type       string `json:"type"`        // SATA-SSD / SATA-HDD / NVMe / unknown
	TempC      int    `json:"temp_c"`      // -1 表示无数据
	ThresholdC int    `json:"threshold_c"` // -1 表示无数据
	Status     string `json:"status"`      // ok / warning / critical / unknown
}

// USBController USB 控制器(实测 TS80X 只有一个 xHCI)。
type USBController struct {
	PCIAddr string `json:"pci_addr"`
	Name    string `json:"name"`
}

// USBPassthroughDevice 可直通设备(未被 VM 持有)。
type USBPassthroughDevice struct {
	Bus     int    `json:"bus"`
	Dev     int    `json:"dev"`
	VID     string `json:"vid"`
	PID     string `json:"pid"`
	Name    string `json:"name"`
	Enabled bool   `json:"enabled"`
}

// USBVMOwned VM 持有的物理直通 USB 设备。
type USBVMOwned struct {
	VMID    int    `json:"vm_id"`
	VMName  string `json:"vm_name"`
	Label   string `json:"label"`
	Summary string `json:"summary"`
	Path    string `json:"path"`
}

// USBState USB 完整状态。
type USBState struct {
	Controllers             []USBController        `json:"controllers"`
	ArbitratorRunning       bool                   `json:"arbitrator_running"`
	AvailableForPassthrough []USBPassthroughDevice `json:"available_for_passthrough"`
	VMOwned                 []USBVMOwned           `json:"vm_owned"`
}

// VM 单台虚拟机。
type VM struct {
	ID      int    `json:"id"`
	Name    string `json:"name"`
	GuestOS string `json:"guest_os"`
	State   string `json:"state"` // powered_on / powered_off / suspended / unknown
}

// HostMetrics 一台 ESXi 一轮的完整采集结果。
// 任何一段子采集失败(esxcli/vsish 单点错误)都不会让整轮挂掉 —— 失败的部分留空,
// 上层 service 写入 esxi_state 时 reachable=true 但相应 JSON 列为空字符串。
type HostMetrics struct {
	Platform PlatformInfo      `json:"platform"`
	CPU      CPUStatic         `json:"cpu_static"`
	Memory   MemoryInfo        `json:"memory"`
	CPUTemp  CPUTemperature    `json:"cpu_temperature"`
	MCE      MCEHealth         `json:"mce_health"`
	Disks    []DiskTemperature `json:"disk_temperature"`
	USB      USBState          `json:"usb"`
	VMs      []VM              `json:"vms"`
}

// --- 主入口:在已建立的 ssh.Client 上跑一整轮 ---

// 非交互非登录 SSH session 经常缺路径,统一显式注入(ESXi 实际不需要,
// 但加上没坏处,并且如果走 bastion + Linux jump host 这里就能兜住)。
const esxiPathPrefix = "export PATH=/bin:/sbin:/usr/lib/vmware/bin:/usr/lib/vmware/vsan/bin:$PATH; "

// CollectAll 在 client 上跑所有 ESXi 数据采集命令并解析。
// 单条命令失败用空值兜底,不阻断后续。
func CollectAll(client *ssh.Client) HostMetrics {
	snap := HostMetrics{
		CPUTemp: CPUTemperature{TjMaxC: -1, MaxC: -1, AvgC: -1},
		MCE:     MCEHealth{State: ""},
		CPU:     CPUStatic{TjMaxC: -1},
		Memory:  MemoryInfo{NUMANodes: -1},
	}

	if out, err := runEsxi(client, "esxcli hardware platform get"); err == nil {
		snap.Platform = parsePlatform(out)
	}
	if out, err := runEsxi(client, "vmware -v; esxcli system version get"); err == nil {
		parseVersionInto(&snap.Platform, out)
	}
	if out, err := runEsxi(client, "esxcli hardware cpu list"); err == nil {
		snap.CPU = parseCPUStatic(out)
	}
	if out, err := runEsxi(client, "vsish -e get /hardware/msr/pcpu/0/addr/0x1A2"); err == nil {
		if tj := decodeTjMax(out); tj > 0 {
			snap.CPU.TjMaxC = tj
		}
	}
	if out, err := runEsxi(client, "esxcli hardware memory get"); err == nil {
		snap.Memory = parseMemory(out)
	}

	// CPU 温度:遍历 0..15 核,失败即停。
	snap.CPUTemp = collectCPUTemp(client, snap.CPU.TjMaxC)

	// MCE。
	if out, err := runEsxi(client, "vsish -e cat /hardware/health/mce"); err == nil {
		snap.MCE = parseMCE(out)
	}

	// 磁盘 SMART(逐盘遍历)。
	snap.Disks = collectDisks(client)

	// vim-cmd vmsvc/getallvms 跑一次,USB owned 和 VM 列表共用,
	// 省一次 session 也避免两边对 VM 列表看法不一致。
	var vmsShallow []VMShallow
	var guestOS map[int]string
	if out, err := runEsxi(client, "vim-cmd vmsvc/getallvms"); err == nil {
		vmsShallow = parseVMListShallow(out)
		guestOS = parseVMGuestOS(out)
	}

	// USB:控制器 + arbitrator + 可直通 + VM 持有。
	snap.USB = collectUSB(client, vmsShallow)

	// VM 列表 + 电源态。
	snap.VMs = collectVMs(client, vmsShallow, guestOS)

	return snap
}

// 单条命令默认超时:ESXi 上单条 esxcli/vsish 多数 < 2s,8s 已经留了很大裕量。
// 用来防"某条命令偶发 hang 拖死整轮",避免一台机器一轮采样里只有部分子项有值。
const defaultCmdTimeout = 8 * time.Second

// runEsxi 在 client 上跑一条 shell 命令,前面统一加 PATH 注入。
// stderr 一律丢弃 —— ESXi 的 vsish/esxcli 偶尔会往 stderr 打 banner,影响 stdout 解析。
// 单条命令带 8s 超时;超时后命令在远端可能还在跑,session 由 ssh.Client 关闭时统一回收。
func runEsxi(client *ssh.Client, cmd string) (string, error) {
	return runEsxiTimeout(client, cmd, defaultCmdTimeout)
}

// runEsxiTimeout 同 runEsxi,但允许调用方为合批命令指定更长的超时。
//
// 末尾 `; true`:合批里某条 esxcli 偶发非零退出(典型如 NVMe 盘的 smart get 字段格式不同)
// 不应该让 sshx.Run 因为退出码非零而把已收集的 stdout 全部丢弃 —— stdout 里
// `===DEV===` 分段大概率已经写入大半,调用方按 stdout 是否空判定才是真的失败信号。
func runEsxiTimeout(client *ssh.Client, cmd string, timeout time.Duration) (string, error) {
	full := esxiPathPrefix + "{ " + cmd + "; true; } 2>/dev/null"
	type result struct {
		out string
		err error
	}
	ch := make(chan result, 1)
	go func() {
		out, err := sshx.Run(client, full, nil)
		ch <- result{out, err}
	}()
	select {
	case r := <-ch:
		return r.out, r.err
	case <-time.After(timeout):
		return "", fmt.Errorf("esxi command timeout after %s", timeout)
	}
}

// --- 解析:平台 / 版本 ---

// parseKV 把 ESXi 一行 "Key: Value" 风格的输出拆成 map(小写 key、去空白 value)。
// 字段名规范:全小写,空格替换为下划线,与 prompt 里描述一致。
func parseKV(text string) map[string]string {
	m := map[string]string{}
	for _, line := range strings.Split(text, "\n") {
		idx := strings.IndexByte(line, ':')
		if idx <= 0 {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(line[:idx]))
		key = strings.ReplaceAll(key, " ", "_")
		val := strings.TrimSpace(line[idx+1:])
		if key == "" || val == "" {
			continue
		}
		m[key] = val
	}
	return m
}

func parsePlatform(out string) PlatformInfo {
	kv := parseKV(out)
	p := PlatformInfo{
		Vendor:  kv["vendor_name"],
		Product: kv["product_name"],
		Serial:  kv["serial_number"],
		UUID:    kv["uuid"],
	}
	if v, ok := kv["ipmi_supported"]; ok {
		p.IPMISupported = strings.EqualFold(v, "true")
	}
	// 部分版本字段名变体兜底
	if p.Vendor == "" {
		p.Vendor = kv["vendor"]
	}
	if p.Product == "" {
		p.Product = kv["product"]
	}
	return p
}

// vmware -v 形态:"VMware ESXi 7.0.3 build-24784741"
var versionRe = regexp.MustCompile(`ESXi\s+(\S+)\s+build-(\d+)`)

func parseVersionInto(p *PlatformInfo, out string) {
	if m := versionRe.FindStringSubmatch(out); len(m) == 3 {
		p.ESXiVersion = m[1]
		if b, err := strconv.ParseInt(m[2], 10, 64); err == nil {
			p.ESXiBuild = b
		}
	}
}

// --- 解析:CPU 静态 ---

// esxcli hardware cpu list 输出形如多块、每块多行 "  Key: Value",取第一块即代表平台 CPU。
func parseCPUStatic(out string) CPUStatic {
	c := CPUStatic{TjMaxC: -1}
	kv := parseKV(out)
	if v, ok := kv["brand"]; ok {
		c.Brand = v
	} else if v, ok := kv["vendor"]; ok {
		c.Brand = v
	}
	c.Family = parseIntDefault(kv["family"], 0)
	c.ModelID = parseIntDefault(kv["model"], 0)
	c.Stepping = parseIntDefault(kv["stepping"], 0)
	// "Number of CPU Cores" / "core_speed" 等键
	for k, v := range kv {
		switch {
		case strings.Contains(k, "core_speed"):
			c.FreqMHz = parseFreqMHz(v)
		case strings.Contains(k, "l2_cache") || k == "l2_cache_size":
			c.L2KB = parseSizeKB(v)
		case strings.Contains(k, "l3_cache") || k == "l3_cache_size":
			c.L3KB = parseSizeKB(v)
		case k == "cpu_cores" || strings.HasPrefix(k, "number_of_cpu_cores"):
			c.Cores = parseIntDefault(v, c.Cores)
		}
	}
	return c
}

// decodeTjMax 解 MSR_TEMPERATURE_TARGET(0x1A2)bits 23:16。
// 输入是 vsish 返回的 hex 或 dec 文本(忽略大小写、前后空白)。
func decodeTjMax(out string) int {
	s := strings.TrimSpace(out)
	if s == "" {
		return -1
	}
	n, err := parseUint64Auto(s)
	if err != nil {
		return -1
	}
	return int((n >> 16) & 0xFF)
}

// --- 解析:内存 ---

func parseMemory(out string) MemoryInfo {
	kv := parseKV(out)
	m := MemoryInfo{NUMANodes: -1}
	for k, v := range kv {
		switch {
		case strings.Contains(k, "physical_memory"):
			m.TotalBytes = parseBytes(v)
		case strings.Contains(k, "reliable_memory"):
			m.ReliableBytes = parseBytes(v)
		case strings.Contains(k, "numa_node"):
			m.NUMANodes = parseIntDefault(v, -1)
		}
	}
	return m
}

// --- 采集 + 解析:CPU 温度 ---

// collectCPUTemp 通过单次 SSH session 拉所有核(0..15)的 MSR 0x1A2 / 0x19C,
// 远端用一段 shell 循环读取并按 `CORE=<n> TJ=<v> DRO=<v>` 一行一核打印,
// 失败核打印空 TJ/DRO,本地遇空即停(与逐核串行的旧行为保持一致)。
// 这样原本 N*2 次 ssh.NewSession 压缩到 1 次,典型 4 核机器省下 ~7 次 session 开销。
func collectCPUTemp(client *ssh.Client, fallbackTjMax int) CPUTemperature {
	res := CPUTemperature{TjMaxC: fallbackTjMax, MaxC: -1, AvgC: -1}
	script := `for i in 0 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15; do
  tj=$(vsish -e get /hardware/msr/pcpu/$i/addr/0x1A2 2>/dev/null)
  dro=$(vsish -e get /hardware/msr/pcpu/$i/addr/0x19C 2>/dev/null)
  printf 'CORE=%s TJ=%s DRO=%s\n' "$i" "$tj" "$dro"
  if [ -z "$tj" ] || [ -z "$dro" ]; then break; fi
done`
	// 16 核 × 2 次 vsish,本地 shell 循环,典型 < 2s;给 15s 留余量,
	// 防止默认 8s 在多核机器上偶发被截断。
	out, err := runEsxiTimeout(client, script, 15*time.Second)
	if err != nil {
		return res
	}
	var sum int
	maxC := -1
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "CORE=") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 3 {
			break
		}
		idStr := strings.TrimPrefix(fields[0], "CORE=")
		tjStr := strings.TrimPrefix(fields[1], "TJ=")
		droStr := strings.TrimPrefix(fields[2], "DRO=")
		id, err := strconv.Atoi(idStr)
		if err != nil {
			break
		}
		if tjStr == "" || droStr == "" {
			break
		}
		t, err := parseUint64Auto(tjStr)
		if err != nil {
			break
		}
		ss, err := parseUint64Auto(droStr)
		if err != nil {
			break
		}
		tjmax := int((t >> 16) & 0xFF)
		dro := int((ss >> 16) & 0x7F)
		temp := tjmax - dro
		if res.TjMaxC <= 0 {
			res.TjMaxC = tjmax
		}
		res.Cores = append(res.Cores, CPUCore{ID: id, TempC: temp, HeadroomC: dro})
		if temp > maxC {
			maxC = temp
		}
		sum += temp
	}
	if len(res.Cores) > 0 {
		res.MaxC = maxC
		res.AvgC = sum / len(res.Cores)
	}
	return res
}

// --- 解析:MCE ---

// vsish -e cat /hardware/health/mce 输出形如:
//
//	Machine check error stats {
//	   Total corrected errors since boot: 0
//	   EWMA of corrected errors per period: 0
//	   Period in seconds: 120
//	   Total uncorrected errors since boot: 0
//	   Health state: 0 -> Green
//	}
var mceStateRe = regexp.MustCompile(`Health state:\s*\d+\s*->\s*(\w+)`)

func parseMCE(out string) MCEHealth {
	m := MCEHealth{State: ""}
	if mm := mceStateRe.FindStringSubmatch(out); len(mm) == 2 {
		m.State = mm[1]
	}
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "Total corrected errors"):
			m.CorrectedTotal = parseTrailingInt64(line)
		case strings.HasPrefix(line, "EWMA"):
			m.CorrectedEWMA = parseTrailingInt64(line)
		case strings.HasPrefix(line, "Period in seconds"):
			m.PeriodSeconds = int(parseTrailingInt64(line))
		case strings.HasPrefix(line, "Total uncorrected errors"):
			m.UncorrectedTotal = parseTrailingInt64(line)
		}
	}
	return m
}

// parseTrailingInt64 从 "Some prefix: 1234" / "...errors since boot: 0" 取尾部数字。
func parseTrailingInt64(line string) int64 {
	idx := strings.LastIndexByte(line, ':')
	if idx < 0 {
		return 0
	}
	tail := strings.TrimSpace(line[idx+1:])
	tail = strings.TrimSuffix(tail, "}")
	tail = strings.TrimSpace(tail)
	// 兜底:取首串数字
	for i := 0; i < len(tail); i++ {
		if tail[i] < '0' || tail[i] > '9' {
			tail = tail[:i]
			break
		}
	}
	n, _ := strconv.ParseInt(tail, 10, 64)
	return n
}

// --- 采集 + 解析:磁盘 ---

var deviceIDRe = regexp.MustCompile(`(?m)^(t10\.\S+|naa\.\S+|mpx\.\S+)`)

func collectDisks(client *ssh.Client) []DiskTemperature {
	// list 命令本身在多盘机器上偶发 5-10s(要走 SCSI inquiry),给 15s 留余量。
	listOut, err := runEsxi(client, "esxcli storage core device list")
	if err != nil {
		logx.Warn("esxi collectDisks: list failed", "err", err.Error())
		return nil
	}
	rawIDs := deviceIDRe.FindAllString(listOut, -1)
	if len(rawIDs) == 0 {
		// list 跑通但 regex 没匹配 —— 通常是设备前缀不在 t10./naa./mpx. 里(比如新版 ESXi 上的 eui.)。
		// 截短一下输出方便看,512 字节就够看到几行了。
		head := listOut
		if len(head) > 512 {
			head = head[:512]
		}
		logx.Warn("esxi collectDisks: list parsed 0 devices", "bytes", len(listOut), "head", head)
		return nil
	}
	devModel, devType := parseDeviceModels(listOut)

	// 去重保序
	seen := map[string]struct{}{}
	ids := make([]string, 0, len(rawIDs))
	for _, id := range rawIDs {
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}

	// 把所有盘的 SMART 合并到一次 SSH session,用 `===DEV===<id>` 行做分段标志。
	// 远端逐盘跑 esxcli smart get,stderr 已被外层 runEsxi 重定向丢掉。
	// SMART 单盘 1~2s,N 盘合批后总耗时随盘数线性增长(8 盘可能 12-15s),
	// 这里给到 25s 上限,远高于默认 8s 单命令超时,避免被截断成空结果。
	var b strings.Builder
	for _, id := range ids {
		b.WriteString(`printf '===DEV===%s\n' `)
		b.WriteString(sshx.ShellQuote(id))
		b.WriteString("; esxcli storage core device smart get -d ")
		b.WriteString(sshx.ShellQuote(id))
		b.WriteString("; ")
	}
	smartAll, err := runEsxiTimeout(client, b.String(), 25*time.Second)
	if err != nil {
		logx.Warn("esxi collectDisks: smart batch failed", "err", err.Error(), "n_dev", len(ids))
		return nil
	}
	smartByID := splitSMARTOutput(smartAll)
	logx.Debug("esxi collectDisks", "n_dev", len(ids), "smart_bytes", len(smartAll), "smart_segments", len(smartByID))

	var out []DiskTemperature
	for _, id := range ids {
		smart := smartByID[id]
		t, thr := parseSMARTTemp(smart)
		out = append(out, DiskTemperature{
			Device:     id,
			Model:      devModel[id],
			Type:       devType[id],
			TempC:      t,
			ThresholdC: thr,
			Status:     classifyDisk(devType[id], t),
		})
	}
	return out
}

// splitSMARTOutput 按 `===DEV===<id>` 标记切合批后的 SMART 输出,返回 id→该盘原始 SMART 文本。
func splitSMARTOutput(out string) map[string]string {
	res := map[string]string{}
	var currentID string
	var buf strings.Builder
	flush := func() {
		if currentID != "" {
			res[currentID] = buf.String()
		}
		buf.Reset()
	}
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "===DEV===") {
			flush()
			currentID = strings.TrimPrefix(line, "===DEV===")
			currentID = strings.TrimRight(currentID, "\r")
			continue
		}
		if currentID == "" {
			continue
		}
		buf.WriteString(line)
		buf.WriteByte('\n')
	}
	flush()
	return res
}

// parseDeviceModels 扫 `esxcli storage core device list` 输出,
// 把每个设备的 Model / Type 映射出来(同一段以设备 id 起首,后续若干行缩进键值)。
func parseDeviceModels(out string) (map[string]string, map[string]string) {
	models := map[string]string{}
	types := map[string]string{}
	var current string
	for _, line := range strings.Split(out, "\n") {
		trim := strings.TrimSpace(line)
		if trim == "" {
			continue
		}
		if strings.HasPrefix(trim, "t10.") || strings.HasPrefix(trim, "naa.") || strings.HasPrefix(trim, "mpx.") {
			current = strings.Fields(trim)[0]
			continue
		}
		if current == "" {
			continue
		}
		idx := strings.IndexByte(trim, ':')
		if idx <= 0 {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(trim[:idx]))
		val := strings.TrimSpace(trim[idx+1:])
		switch key {
		case "model":
			if val != "" {
				models[current] = val
			}
		case "device type":
			if val != "" {
				types[current] = val
			}
		}
	}
	return models, types
}

// parseSMARTTemp 从 `esxcli storage core device smart get` 输出里找 "Drive Temperature" 行。
// 列顺序:Parameter | Value | Threshold | Worst | Pass/Fail (具体宽度各版本不同)。
// 这里按 awk 思路按列号取 $3=current/$5=threshold(prompt 里实测的列布局)。
func parseSMARTTemp(out string) (int, int) {
	for _, line := range strings.Split(out, "\n") {
		trim := strings.TrimSpace(line)
		if !strings.HasPrefix(trim, "Drive Temperature") {
			continue
		}
		fields := strings.Fields(trim)
		// fields: ["Drive", "Temperature", "<Value>", "<Threshold>", "<Worst>", ...]
		if len(fields) >= 4 {
			cur := parseIntDefault(fields[2], -1)
			thr := parseIntDefault(fields[3], -1)
			return cur, thr
		}
	}
	return -1, -1
}

// classifyDisk 给磁盘按温度评 ok/warning/critical(基于 prompt 表格)。
func classifyDisk(devType string, temp int) string {
	if temp < 0 {
		return "unknown"
	}
	t := strings.ToUpper(devType)
	switch {
	case strings.Contains(t, "NVME"):
		switch {
		case temp >= 80:
			return "critical"
		case temp >= 70:
			return "warning"
		}
	case strings.Contains(t, "SSD"):
		switch {
		case temp >= 70:
			return "critical"
		case temp >= 60:
			return "warning"
		}
	case strings.Contains(t, "HDD"), strings.Contains(t, "ATA"):
		switch {
		case temp >= 55:
			return "critical"
		case temp >= 50:
			return "warning"
		}
	}
	return "ok"
}

// --- 采集 + 解析:USB ---

// collectUSB 拉控制器 / arbitrator / 可直通设备,VM 持有部分用调用方共享的 VMShallow 列表
// (避免在这里再跑一次 getallvms)。
func collectUSB(client *ssh.Client, vms []VMShallow) USBState {
	u := USBState{}
	if out, err := runEsxi(client, "lspci | grep -i usb"); err == nil {
		u.Controllers = parseUSBControllers(out)
	}
	if out, err := runEsxi(client, "/etc/init.d/usbarbitrator status"); err == nil {
		u.ArbitratorRunning = strings.Contains(strings.ToLower(out), "running")
	}
	if out, err := runEsxi(client, "localcli hardware usb passthrough device list"); err == nil {
		u.AvailableForPassthrough = parseUSBPassthrough(out)
	}
	u.VMOwned = collectUSBVMOwned(client, vms)
	return u
}

// parseUSBControllers 解 `lspci | grep -i usb` 一行一控制器,形如:
//
//	0000:00:14.0 USB controller: Intel Cannon Lake PCH USB 3.1 xHCI Host Controller
var lspciLineRe = regexp.MustCompile(`^(\S+)\s+USB\s+controller:\s+(.+)$`)

func parseUSBControllers(out string) []USBController {
	var list []USBController
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if m := lspciLineRe.FindStringSubmatch(line); len(m) == 3 {
			list = append(list, USBController{PCIAddr: m[1], Name: strings.TrimSpace(m[2])})
		}
	}
	return list
}

// parseUSBPassthrough 解 `localcli hardware usb passthrough device list` 表格。
// 第 1 行是表头(Bus  Dev  VendorId  ProductId  Enabled  Can Connect to VM  Name),
// 第 2 行是分隔线(全 -),之后每行一台设备。
func parseUSBPassthrough(out string) []USBPassthroughDevice {
	var list []USBPassthroughDevice
	lines := strings.Split(out, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "Bus") || strings.HasPrefix(line, "---") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 7 {
			continue
		}
		bus := parseIntDefault(fields[0], 0)
		dev := parseIntDefault(fields[1], 0)
		vid := fields[2]
		pid := fields[3]
		enabled := strings.EqualFold(fields[4], "true")
		// fields[5] = Can Connect (yes/no)
		name := strings.Join(fields[6:], " ")
		list = append(list, USBPassthroughDevice{
			Bus: bus, Dev: dev, VID: vid, PID: pid, Name: name, Enabled: enabled,
		})
	}
	return list
}

// collectUSBVMOwned 对每台 VM 取 device.getdevices,筛 VirtualUSB + USBBackingInfo path:。
// 这是 prompt 里强调的"物理直通"判据 —— `backing` 含 deviceName = "path:..."。
//
// 以前是「for VM 各起 1 个 SSH session」,14 VM 会被 ESXi sshd 的 MaxStartups 限速;
// 现在把所有 VM 的 device.getdevices 合到一次 session,用 `===VM===<id>` 行分段。
func collectUSBVMOwned(client *ssh.Client, vms []VMShallow) []USBVMOwned {
	if len(vms) == 0 {
		return nil
	}
	devByVM := batchVMDevices(client, vms)
	var out []USBVMOwned
	for _, v := range vms {
		devOut, ok := devByVM[v.ID]
		if !ok || devOut == "" {
			continue
		}
		out = append(out, extractVirtualUSB(v.ID, v.Name, devOut)...)
	}
	return out
}

// batchVMDevices 把所有 VM 的 vim-cmd vmsvc/device.getdevices 合到一次 session,
// 用 `===VM===<id>` 行做分段标志。device.getdevices 单次几百毫秒,N VM 串起来很贵。
// 输出可能很大(每 VM 几 KB),给 20s 超时留余量。
func batchVMDevices(client *ssh.Client, vms []VMShallow) map[int]string {
	res := map[int]string{}
	if len(vms) == 0 {
		return res
	}
	var b strings.Builder
	for _, v := range vms {
		b.WriteString("printf '===VM===%d\\n' ")
		b.WriteString(strconv.Itoa(v.ID))
		b.WriteString("; vim-cmd vmsvc/device.getdevices ")
		b.WriteString(strconv.Itoa(v.ID))
		b.WriteString("; ")
	}
	out, err := runEsxiTimeout(client, b.String(), 20*time.Second)
	if err != nil {
		return res
	}
	currentID := -1
	var buf strings.Builder
	flush := func() {
		if currentID >= 0 {
			res[currentID] = buf.String()
		}
		buf.Reset()
	}
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "===VM===") {
			flush()
			idStr := strings.TrimRight(strings.TrimPrefix(line, "===VM==="), "\r")
			if id, err := strconv.Atoi(idStr); err == nil {
				currentID = id
			} else {
				currentID = -1
			}
			continue
		}
		if currentID < 0 {
			continue
		}
		buf.WriteString(line)
		buf.WriteByte('\n')
	}
	flush()
	return res
}

// extractVirtualUSB 在 device.getdevices 输出里找 VirtualUSB 段:
//
//	(vim.vm.device.VirtualUSB) {
//	   key = 41000,
//	   deviceInfo = (vim.Description) {
//	      label = "USB 41001",
//	      summary = "Cyber Power System UT1050EGC"
//	   },
//	   backing = (vim.vm.device.VirtualUSB.USBBackingInfo) {
//	      deviceName = "path:0/1/6",
//	   }
//
// 只关心带 `deviceName = "path:..."` 的(物理直通,排除 xHCI controller 等)。
func extractVirtualUSB(vmID int, vmName, out string) []USBVMOwned {
	var list []USBUVMScan
	const marker = "(vim.vm.device.VirtualUSB)"
	idx := 0
	for {
		i := strings.Index(out[idx:], marker)
		if i < 0 {
			break
		}
		start := idx + i
		// 截到下一个段标志或结尾(取一个安全的近似:看下一处 ')' 同级)
		end := start + len(marker)
		// 简化:再向后 30 行
		end = advanceLines(out, end, 30)
		seg := out[start:end]
		list = append(list, scanVirtualUSBSegment(vmID, vmName, seg))
		idx = end
	}
	var owned []USBVMOwned
	for _, l := range list {
		if l.HasPath {
			owned = append(owned, USBVMOwned{
				VMID: vmID, VMName: vmName, Label: l.Label, Summary: l.Summary, Path: l.Path,
			})
		}
	}
	return owned
}

type USBUVMScan struct {
	Label   string
	Summary string
	Path    string
	HasPath bool
}

func advanceLines(s string, from, nLines int) int {
	if from >= len(s) {
		return len(s)
	}
	count := 0
	for i := from; i < len(s); i++ {
		if s[i] == '\n' {
			count++
			if count >= nLines {
				return i
			}
		}
	}
	return len(s)
}

var quotedRe = regexp.MustCompile(`"([^"]*)"`)

func scanVirtualUSBSegment(_ int, _, seg string) USBUVMScan {
	r := USBUVMScan{}
	for _, line := range strings.Split(seg, "\n") {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "label"):
			if m := quotedRe.FindStringSubmatch(line); len(m) == 2 {
				r.Label = m[1]
			}
		case strings.HasPrefix(line, "summary"):
			if m := quotedRe.FindStringSubmatch(line); len(m) == 2 {
				r.Summary = m[1]
			}
		case strings.HasPrefix(line, "deviceName"):
			if m := quotedRe.FindStringSubmatch(line); len(m) == 2 {
				v := m[1]
				if strings.HasPrefix(v, "path:") {
					r.Path = strings.TrimPrefix(v, "path:")
					r.HasPath = true
				}
			}
		}
	}
	return r
}

// --- 采集 + 解析:VM ---

// VMShallow 仅 (id, name) 给 USB 路径用。
type VMShallow struct {
	ID   int
	Name string
}

// collectVMs 用 CollectAll 已经拉过一次的 VMShallow 列表 + guestOS 映射,
// 自己只负责 power.getstate 合批查询和组装结果(避免重复跑 getallvms)。
//
// vim-cmd vmsvc/getallvms 输出:第一行表头,后续每行 5 列:
//
//	Vmid  Name  File  Guest OS  Version  Annotation
//	"Name" 可能含空格 —— 由 vim-cmd 用空格对齐,这里偷懒按"首数字 + 空格 + 名字 + 空格 + 路径开始[/vmfs/...]"切。
func collectVMs(client *ssh.Client, shallow []VMShallow, guestOS map[int]string) []VM {
	if shallow == nil {
		// 调用方拿到的 getallvms 失败 —— 返回 nil,buildSample 会写 vm_total/vm_powered_on=-1。
		return nil
	}
	powerByID := batchVMPowerState(client, shallow)
	vms := make([]VM, 0, len(shallow))
	for _, s := range shallow {
		state, ok := powerByID[s.ID]
		if !ok {
			state = "unknown"
		}
		vms = append(vms, VM{
			ID:      s.ID,
			Name:    s.Name,
			GuestOS: guestOS[s.ID],
			State:   state,
		})
	}
	return vms
}

// batchVMPowerState 把多 VM 的 power.getstate 合并到一次 SSH session,
// 远端用 `===VM===<id>` 行作分段标志。每次 vim-cmd 启动很慢(~1s),合批省得最明显。
func batchVMPowerState(client *ssh.Client, vms []VMShallow) map[int]string {
	res := map[int]string{}
	if len(vms) == 0 {
		return res
	}
	var b strings.Builder
	for _, v := range vms {
		b.WriteString("printf '===VM===%d\\n' ")
		b.WriteString(strconv.Itoa(v.ID))
		b.WriteString("; vim-cmd vmsvc/power.getstate ")
		b.WriteString(strconv.Itoa(v.ID))
		b.WriteString("; ")
	}
	// 14 VM × 每次 ~1s = 14s,远超默认 8s,这里给 20s。
	out, err := runEsxiTimeout(client, b.String(), 20*time.Second)
	if err != nil {
		return res
	}
	var currentID int = -1
	var buf strings.Builder
	flush := func() {
		if currentID >= 0 {
			res[currentID] = mapVMPowerState(buf.String())
		}
		buf.Reset()
	}
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "===VM===") {
			flush()
			idStr := strings.TrimRight(strings.TrimPrefix(line, "===VM==="), "\r")
			if id, err := strconv.Atoi(idStr); err == nil {
				currentID = id
			} else {
				currentID = -1
			}
			continue
		}
		if currentID < 0 {
			continue
		}
		buf.WriteString(line)
		buf.WriteByte('\n')
	}
	flush()
	return res
}

func parseVMListShallow(out string) []VMShallow {
	var list []VMShallow
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimRight(line, "\r")
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		id, err := strconv.Atoi(fields[0])
		if err != nil {
			continue
		}
		// 名字到下一个出现 "[" 之前的字段
		nameTokens := []string{}
		for _, f := range fields[1:] {
			if strings.HasPrefix(f, "[") {
				break
			}
			nameTokens = append(nameTokens, f)
		}
		if len(nameTokens) == 0 {
			continue
		}
		list = append(list, VMShallow{ID: id, Name: strings.Join(nameTokens, " ")})
	}
	return list
}

// parseVMGuestOS 从 vim-cmd 输出里抽 Guest OS 列:
// 输出每行 file 列形如 "[datastore] vmfolder/vm.vmx",紧随其后是 Guest OS、Version、Annotation。
// 偷懒:从 ".vmx" 之后再分词,前 1~3 个 token 是 Guest OS(可能含 "Other 5.x or later Linux (64-bit)" 之类空格)。
// 这里简化为:取 ".vmx]" 之后到 "vmx-NN" 之前的 token 串。
var vmGuestOSRe = regexp.MustCompile(`(?m)^(\d+)\s+.+\.vmx\]?\s+(.+?)\s+vmx-\d+`)

func parseVMGuestOS(out string) map[int]string {
	m := map[int]string{}
	for _, mm := range vmGuestOSRe.FindAllStringSubmatch(out, -1) {
		id, err := strconv.Atoi(mm[1])
		if err != nil {
			continue
		}
		m[id] = strings.TrimSpace(mm[2])
	}
	return m
}

func mapVMPowerState(out string) string {
	s := strings.ToLower(strings.TrimSpace(out))
	switch {
	case strings.Contains(s, "powered on"):
		return "powered_on"
	case strings.Contains(s, "powered off"):
		return "powered_off"
	case strings.Contains(s, "suspended"):
		return "suspended"
	default:
		return "unknown"
	}
}

// --- 小工具 ---

func parseIntDefault(s string, def int) int {
	s = strings.TrimSpace(s)
	if s == "" {
		return def
	}
	if n, err := strconv.Atoi(s); err == nil {
		return n
	}
	// 尝试取前导整数(如 "3504 MHz")
	end := 0
	for end < len(s) && (s[end] >= '0' && s[end] <= '9') {
		end++
	}
	if end == 0 {
		return def
	}
	if n, err := strconv.Atoi(s[:end]); err == nil {
		return n
	}
	return def
}

// parseUint64Auto 支持 "0x..." 与十进制。
func parseUint64Auto(s string) (uint64, error) {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "0x") || strings.HasPrefix(s, "0X") {
		return strconv.ParseUint(s[2:], 16, 64)
	}
	return strconv.ParseUint(s, 10, 64)
}

// parseFreqMHz 把 "3504 MHz" / "3504000000 Hz" / "3.5 GHz" 之类统一成 MHz。
func parseFreqMHz(s string) int {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	low := strings.ToLower(s)
	var f float64
	num := extractLeadingFloat(s)
	if num != "" {
		if v, err := strconv.ParseFloat(num, 64); err == nil {
			f = v
		}
	}
	switch {
	case strings.Contains(low, "ghz"):
		return int(f * 1000)
	case strings.Contains(low, "hz") && !strings.Contains(low, "mhz") && !strings.Contains(low, "khz"):
		return int(f / 1_000_000)
	default:
		return int(f)
	}
}

// parseSizeKB 把 "256 KB" / "8 MB" / "8192 KB" / "1 GiB" 统一成 KB。
func parseSizeKB(s string) int {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	low := strings.ToLower(s)
	num := extractLeadingFloat(s)
	var f float64
	if num != "" {
		if v, err := strconv.ParseFloat(num, 64); err == nil {
			f = v
		}
	}
	switch {
	case strings.Contains(low, "gb") || strings.Contains(low, "gib"):
		return int(f * 1024 * 1024)
	case strings.Contains(low, "mb") || strings.Contains(low, "mib"):
		return int(f * 1024)
	default:
		return int(f)
	}
}

// parseBytes 把 "137262243840 Bytes" / "131072 MB" 等统一成 bytes。
func parseBytes(s string) int64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	low := strings.ToLower(s)
	num := extractLeadingFloat(s)
	var f float64
	if num != "" {
		if v, err := strconv.ParseFloat(num, 64); err == nil {
			f = v
		}
	}
	switch {
	case strings.Contains(low, "gb") || strings.Contains(low, "gib"):
		return int64(f * 1024 * 1024 * 1024)
	case strings.Contains(low, "mb") || strings.Contains(low, "mib"):
		return int64(f * 1024 * 1024)
	case strings.Contains(low, "kb") || strings.Contains(low, "kib"):
		return int64(f * 1024)
	default:
		return int64(f)
	}
}

// extractLeadingFloat 取字符串前导可解析为 float 的子串(允许小数点)。
func extractLeadingFloat(s string) string {
	end := 0
	dot := false
	for end < len(s) {
		c := s[end]
		if c >= '0' && c <= '9' {
			end++
			continue
		}
		if c == '.' && !dot {
			dot = true
			end++
			continue
		}
		break
	}
	return s[:end]
}
