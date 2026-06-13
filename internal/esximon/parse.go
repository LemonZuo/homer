package esximon

// ESXi 文本输出通用解析工具。

import (
	"regexp"
	"strconv"
	"strings"
)

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

func parseInt64Default(s string, def int64) int64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return def
	}
	if n, err := strconv.ParseInt(s, 10, 64); err == nil {
		return n
	}
	end := 0
	for end < len(s) && (s[end] >= '0' && s[end] <= '9') {
		end++
	}
	if end == 0 {
		return def
	}
	if n, err := strconv.ParseInt(s[:end], 10, 64); err == nil {
		return n
	}
	return def
}

func percentInt(used, total int64) int {
	if used < 0 || total <= 0 {
		return -1
	}
	return int((used*100 + total/2) / total)
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
		// 没有显式单位:ESXi 的 `Core Speed` 字段实际是 Hz(几十亿那种大整数)。
		// 合理 CPU 主频范围 < 100000 MHz,超过即视为 Hz。
		if f > 100000 {
			return int(f / 1_000_000)
		}
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

var quotedRe = regexp.MustCompile(`"([^"]*)"`)

func firstQuoted(s string) string {
	if m := quotedRe.FindStringSubmatch(s); len(m) == 2 {
		return strings.TrimSpace(m[1])
	}
	return ""
}

// splitCSV 拆 "a, b, c" 风格的列表,去空格、丢空段。
func splitCSV(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}
