package esximon

// ESXi CPU 温度和 MCE 采集。

import (
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/LemonZuo/homer/internal/logx"
	"golang.org/x/crypto/ssh"
)

// collectCPUTemp 通过单次 SSH session 拉所有核(0..15)的 MSR 0x1A2 / 0x19C,
// 远端用一段 shell 循环读取并按 `CORE=<n> TJ=<v> DRO=<v>` 一行一核打印,
// 失败核打印空 TJ/DRO,本地遇空即停(与逐核串行的旧行为保持一致)。
// 这样原本 N*2 次 ssh.NewSession 压缩到 1 次,典型 4 核机器省下 ~7 次 session 开销。
func collectCPUTemp(client *ssh.Client, fallbackTjMax, expectedCores int) CPUTemperature {
	res := CPUTemperature{TjMaxC: fallbackTjMax, MaxC: -1, AvgC: -1}
	script := `for i in 0 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15; do
  tj=$(vsish -e get /hardware/msr/pcpu/$i/addr/0x1A2 2>/dev/null)
  dro=$(vsish -e get /hardware/msr/pcpu/$i/addr/0x19C 2>/dev/null)
  printf 'CORE=%s TJ=%s DRO=%s\n' "$i" "$tj" "$dro"
  if [ -z "$tj" ] || [ -z "$dro" ]; then break; fi
done`
	// 16 核 × 2 次 vsish,本地 shell 循环,典型 < 2s;给 15s 留余量,
	// 防止默认 8s 在多核机器上偶发被截断。
	out, err := runEsxiRetry(client, "cpu temperature", script, 15*time.Second, 3, func(out string) bool {
		got := len(parseCPUTempOutput(out, fallbackTjMax).Cores)
		if expectedCores > 0 {
			return got >= expectedCores
		}
		return got > 0
	})
	if err != nil {
		logx.Warn("esxi cpu temperature failed", "err", err.Error())
		return res
	}
	return parseCPUTempOutput(out, fallbackTjMax)
}

func parseCPUTempOutput(out string, fallbackTjMax int) CPUTemperature {
	res := CPUTemperature{TjMaxC: fallbackTjMax, MaxC: -1, AvgC: -1}
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
