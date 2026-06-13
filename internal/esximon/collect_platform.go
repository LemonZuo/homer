package esximon

// ESXi 平台、CPU 静态信息、内存和启动信息采集。

import (
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/LemonZuo/homer/internal/logx"
	"golang.org/x/crypto/ssh"
)

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
//
// 注意 ESXi 7.0 之前 `Brand` 字段返回的是 vendor("GenuineIntel"),不是 model 名。
// CPU 真实型号、Current Speed、Core Count 由 fillFromSmbios 从 smbiosDump 覆盖。
//
// L2/L3 Cache Size 字段单位是 byte(实测 262144/8388608),
// 不能用 parseSizeKB(那是按 KB/MB/GB 文本单位推断),也不能用 Contains 匹配,
// 否则会被 `L2 Cache Line Size: 64` 或 `L2 Cache CPU Count: 1` 这类同前缀字段误覆盖
// (parseKV 把多个 CPU 块合并后 map 遍历顺序不定,最后赋值的赢)。
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
	if v, ok := kv["core_speed"]; ok {
		c.FreqMHz = parseFreqMHz(v)
	}
	if v, ok := kv["l2_cache_size"]; ok {
		if n, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64); err == nil {
			c.L2KB = int(n / 1024)
		}
	}
	if v, ok := kv["l3_cache_size"]; ok {
		if n, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64); err == nil {
			c.L3KB = int(n / 1024)
		}
	}
	if v, ok := kv["number_of_cpu_cores"]; ok {
		c.Cores = parseIntDefault(v, 0)
	}
	return c
}

// fillCoresFromGlobal 从 `esxcli hardware cpu global get` 补核心数。
// 输出:CPU Packages: 1 / CPU Cores: 4 / CPU Threads: 4 / Hyperthreading ...
func fillCoresFromGlobal(c *CPUStatic, out string) {
	kv := parseKV(out)
	if v, ok := kv["cpu_cores"]; ok {
		if n := parseIntDefault(v, 0); n > 0 {
			c.Cores = n
		}
	}
}

// fillFromSmbios 解 `smbiosDump` 输出里第一个 Processor Info (Type 4) 块,
// 取真实 CPU 型号、当前频率、核心数。优先级高于 esxcli cpu list / global get。
//
// 块结构示例(2 空格缩进):
//
//	Processor Info (Type 4): #74
//	  Socket: "U3E1"
//	  ...
//	  Version: "Intel(R) Xeon(R) E-2224G CPU @ 3.50GHz"
//	  ...
//	  Current Speed: 3500 MHz
//	  Core Count: 4
//
// 第一个顶格行(下一个 Type X)即块结束。
func fillFromSmbios(c *CPUStatic, out string) {
	inBlock := false
	matched := false
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, "Processor Info") && strings.Contains(line, "Type 4") {
			inBlock = true
			matched = true
			continue
		}
		if !inBlock {
			continue
		}
		// 块内行必须以空格起首;空行/顶格行视为块结束。
		if line == "" || !strings.HasPrefix(line, " ") {
			return
		}
		trim := strings.TrimSpace(line)
		idx := strings.Index(trim, ":")
		if idx <= 0 {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(trim[:idx]))
		val := strings.Trim(strings.TrimSpace(trim[idx+1:]), "\"")
		switch key {
		case "version":
			if val != "" && !strings.EqualFold(val, "To Be Filled By O.E.M.") {
				c.Brand = val
				logx.Debug("esxi smbios brand", "brand", val)
			}
		case "current speed":
			if mhz := parseFreqMHz(val); mhz > 0 {
				c.FreqMHz = mhz
			}
		case "core count":
			if n := parseIntDefault(val, 0); n > 0 {
				c.Cores = n
			}
		}
	}
	if !matched {
		logx.Warn("esxi smbios: Type 4 block not found", "bytes", len(out))
	}
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
	var m MemoryInfo
	for k, v := range kv {
		if strings.Contains(k, "physical_memory") {
			m.TotalBytes = parseBytes(v)
		}
	}
	return m
}

// fillMemoryFromVsish 解 `vsish -e cat /memory/comprehensive`:
//
//	Comprehensive {
//	   Physical memory estimate:134045160 KB
//	   ...
//	   Free:75142516 KB
//	}
//
// 取 Free 作为可用内存;Physical 作为总内存兜底(esxcli 没拿到时)。
func fillMemoryFromVsish(m *MemoryInfo, out string) {
	for _, line := range strings.Split(out, "\n") {
		trim := strings.TrimSpace(line)
		idx := strings.Index(trim, ":")
		if idx <= 0 {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(trim[:idx]))
		val := strings.TrimSpace(trim[idx+1:])
		switch key {
		case "free":
			m.FreeBytes = parseBytes(val)
		case "physical memory estimate":
			if m.TotalBytes <= 0 {
				m.TotalBytes = parseBytes(val)
			}
		}
	}
}

// parseRuntimeUsage 解 `vim-cmd hostsvc/hostsummary` 中 quickStats/hardware 的运行时用量:
// overallCpuUsage(MHz), overallMemoryUsage(MiB), cpuMhz, numCpuCores, memorySize(bytes)。
func parseRuntimeUsage(out string, cpu CPUStatic, mem MemoryInfo) RuntimeUsage {
	u := RuntimeUsage{CPUUsagePercent: -1, MemoryUsagePercent: -1}
	cpuUsedMHz := -1
	memUsedMiB := int64(-1)
	cpuMHz := 0
	cpuCores := 0
	memTotalBytes := int64(0)

	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(strings.TrimSuffix(line, ","))
		idx := strings.Index(line, "=")
		if idx <= 0 {
			continue
		}
		key := strings.TrimSpace(line[:idx])
		val := strings.TrimSpace(line[idx+1:])
		val = strings.TrimSuffix(val, ",")
		switch key {
		case "overallCpuUsage":
			cpuUsedMHz = parseIntDefault(val, -1)
		case "overallMemoryUsage":
			memUsedMiB = parseInt64Default(val, -1)
		case "cpuMhz":
			cpuMHz = parseIntDefault(val, 0)
		case "numCpuCores":
			cpuCores = parseIntDefault(val, 0)
		case "memorySize":
			memTotalBytes = parseInt64Default(val, 0)
		}
	}

	if cpuMHz <= 0 {
		cpuMHz = cpu.FreqMHz
	}
	if cpuCores <= 0 {
		cpuCores = cpu.Cores
	}
	if cpuUsedMHz >= 0 {
		u.CPUUsedMHz = cpuUsedMHz
		if cpuMHz > 0 && cpuCores > 0 {
			u.CPUCapacityMHz = cpuMHz * cpuCores
			u.CPUUsagePercent = percentInt(int64(cpuUsedMHz), int64(u.CPUCapacityMHz))
		}
	}

	if memTotalBytes <= 0 {
		memTotalBytes = mem.TotalBytes
	}
	if memUsedMiB >= 0 {
		u.MemoryUsedBytes = memUsedMiB * 1024 * 1024
		if memTotalBytes > 0 {
			u.MemoryTotalBytes = memTotalBytes
			u.MemoryUsagePercent = percentInt(u.MemoryUsedBytes, memTotalBytes)
		}
	}
	return u
}

// --- 采集 + 解析:CPU 温度 ---

// collectHostBoot 一气呵成抓 uptime + 当前 epoch + zdump 数,
// 后端用 (now - uptime) 算 booted_at,避免依赖远端时区。
// 输出格式(以行为单位的 KV)便于解析,无 zdump 时 LATEST 留空。
func collectHostBoot(client *ssh.Client) HostBoot {
	boot := newHostBoot()
	cmd := strings.Join([]string{
		`u=$(esxcli system stats uptime get 2>/dev/null)`,
		`n=$(date +%s)`,
		`c=$(ls /var/core/vmkernel-zdump.* 2>/dev/null | wc -l)`,
		`m=$(ls /var/core/vmkernel-zdump.* 2>/dev/null | xargs -n1 stat -c '%Y' 2>/dev/null | sort -n | tail -1)`,
		`printf 'UPTIME_US=%s\nNOW_EPOCH=%s\nZDUMP_COUNT=%s\nZDUMP_LATEST=%s\n' "$u" "$n" "$c" "$m"`,
	}, "; ")
	out, err := runEsxiRetry(client, "host boot", cmd, defaultCmdTimeout, 2, func(s string) bool {
		return strings.Contains(s, "UPTIME_US=") && strings.Contains(s, "NOW_EPOCH=")
	})
	if err != nil {
		logx.Warn("esxi host boot fetch failed", "err", err.Error())
		return boot
	}
	return parseHostBoot(out)
}

// parseHostBoot 解析 collectHostBoot 的 KV 输出。
// uptime 单位是微秒(esxcli system stats uptime get 实测),换算成秒。
func parseHostBoot(out string) HostBoot {
	boot := newHostBoot()
	kv := map[string]string{}
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if i := strings.Index(line, "="); i > 0 {
			kv[line[:i]] = line[i+1:]
		}
	}
	uptimeUS, _ := strconv.ParseInt(kv["UPTIME_US"], 10, 64)
	nowEpoch, _ := strconv.ParseInt(kv["NOW_EPOCH"], 10, 64)
	if uptimeUS > 0 {
		boot.UptimeSeconds = uptimeUS / 1_000_000
		if nowEpoch > 0 {
			boot.BootedAt = time.Unix(nowEpoch-boot.UptimeSeconds, 0).UTC()
		}
	}
	if c, err := strconv.Atoi(kv["ZDUMP_COUNT"]); err == nil {
		boot.CrashDumpCount = c
	}
	if m, _ := strconv.ParseInt(kv["ZDUMP_LATEST"], 10, 64); m > 0 {
		boot.LastCrashAt = time.Unix(m, 0).UTC()
	}
	return boot
}
