package esximon

// ESXi SMART 属性解析。

import (
	"strconv"
	"strings"
	"time"

	"github.com/LemonZuo/homer/internal/logx"
	"github.com/LemonZuo/homer/internal/sshx"
	"golang.org/x/crypto/ssh"
)

// parseSMARTAttrs 把 `esxcli storage core device smart get` 的输出解析成统一属性。
//
// 列布局实测三家差异(prompt 6_ESXI_SSH_MONITORING.md 16.4 + 16 节):
//
//	ATA SSD (Samsung 870):  Drive Temperature 66 0  49  34   → Raw=34°C 是真值
//	ATA HDD (WDC):          Drive Temperature 48 0  N/A 45   → Raw=45°C 是真值
//	NVMe (Samsung 990 PRO): Drive Temperature 46 82 N/A N/A  → Raw=N/A,Value=46 才是真值
//
// 取值统一:Raw 优先,Raw=N/A 时回退 Value(吃下 NVMe);否则返回 -1。
// 行结构:`<参数名 token1..tokenN> <Value> <Threshold> <Worst> <Raw>`,末尾固定 4 列。
// 参数名可能是 1~多 token,例如 "Pending Sector Reallocation Count"(4 个 token)。
func parseSMARTAttrs(out string) SMARTAttrs {
	attrs := newSMARTAttrs()
	for _, line := range strings.Split(out, "\n") {
		trim := strings.TrimSpace(line)
		if trim == "" {
			continue
		}
		fields := strings.Fields(trim)
		// 最少要有 1 个名字 token + 4 个数据列 = 5 列。
		if len(fields) < 5 {
			continue
		}
		// 跳过表头和分隔线。
		if fields[0] == "Parameter" || strings.HasPrefix(fields[0], "---") {
			continue
		}
		nameTokens := fields[:len(fields)-4]
		name := strings.Join(nameTokens, " ")
		valueCol := fields[len(fields)-4]
		thresholdCol := fields[len(fields)-3]
		rawCol := fields[len(fields)-1]

		switch name {
		case "Health Status":
			if valueCol != "" && valueCol != "N/A" {
				attrs.HealthStatus = valueCol
			}
		case "Power-on Hours":
			attrs.PowerOnHours = smartInt64Pick(rawCol, valueCol)
		case "Power Cycle Count":
			attrs.PowerCycleCount = smartInt64Pick(rawCol, valueCol)
		case "Reallocated Sector Count":
			attrs.ReallocatedSectors = smartInt64Pick(rawCol, valueCol)
		case "Uncorrectable Error Count", "Uncorrectable Sector Count":
			attrs.UncorrectableErrors = smartInt64Pick(rawCol, valueCol)
		case "Media Wearout Indicator":
			// SSD 独有。Value 是 normalized(100=新,0=磨损完),跨厂商可比较;Raw 各家含义不同,不存。
			attrs.MediaWearoutValue = parseIntDefault(valueCol, -1)
		case "Read Error Count":
			attrs.ReadErrorCount = smartInt64Pick(rawCol, valueCol)
		case "Pending Sector Reallocation Count":
			attrs.PendingSectorReallocation = smartInt64Pick(rawCol, valueCol)
		case "Drive Temperature":
			attrs.TempC = smartIntPick(rawCol, valueCol)
			attrs.ThresholdC = parseIntDefault(thresholdCol, -1)
		}
	}
	return attrs
}

// SMARTAttrs 是 parseSMARTAttrs 的中间产物,collectDisks 取需要的字段写到 DiskHealth。
type SMARTAttrs struct {
	HealthStatus              string
	PowerOnHours              int64
	PowerCycleCount           int64
	ReallocatedSectors        int64
	UncorrectableErrors       int64
	MediaWearoutValue         int
	ReadErrorCount            int64
	PendingSectorReallocation int64
	TempC                     int
	ThresholdC                int
}

func newSMARTAttrs() SMARTAttrs {
	return SMARTAttrs{
		PowerOnHours:              -1,
		PowerCycleCount:           -1,
		ReallocatedSectors:        -1,
		UncorrectableErrors:       -1,
		MediaWearoutValue:         -1,
		ReadErrorCount:            -1,
		PendingSectorReallocation: -1,
		TempC:                     -1,
		ThresholdC:                -1,
	}
}

func smartInt64Pick(raw, value string) int64 {
	if v := parseInt64Default(raw, -1); v >= 0 {
		return v
	}
	return parseInt64Default(value, -1)
}

func smartIntPick(raw, value string) int {
	if v := parseIntDefault(raw, -1); v >= 0 {
		return v
	}
	return parseIntDefault(value, -1)
}

// ataSMARTAttrs 是 ATA 盘从 vsish valuesBuffer 解析出的真实 6-byte raw 值。
// 作用:覆盖 esxcli 输出的 Raw 列(对 ATA 盘只暴露低 1 字节,Power-on Hours 等会被截到 0-255)。
// NVMe 不走这条路(vsish 该节点 Not supported),所有字段保持 -1。
type ataSMARTAttrs struct {
	PowerOnHours              int64
	PowerCycleCount           int64
	ReallocatedSectors        int64
	PendingSectorReallocation int64
	UncorrectableErrors       int64
}

func newATASMARTAttrs() ataSMARTAttrs {
	return ataSMARTAttrs{
		PowerOnHours:              -1,
		PowerCycleCount:           -1,
		ReallocatedSectors:        -1,
		PendingSectorReallocation: -1,
		UncorrectableErrors:       -1,
	}
}

// collectATASmartBuffers 对每块 ATA 盘合批跑 vsish,读 /storage/scsifw/devices/<id>/smart/valuesBuffer。
// vsish 返回的 512 字节就是 ATA SMART data 结构:bytes[0:2]=revision + 30 个 12 字节 attribute entry。
// NVMe 设备前缀以 "t10.NVMe" 起手,直接跳过(vsish 该路径返回 "Not supported")。
func collectATASmartBuffers(client *ssh.Client, ids []string) map[string]ataSMARTAttrs {
	var ataIDs []string
	for _, id := range ids {
		if strings.HasPrefix(id, "t10.ATA_") {
			ataIDs = append(ataIDs, id)
		}
	}
	if len(ataIDs) == 0 {
		return nil
	}
	var b strings.Builder
	for _, id := range ataIDs {
		b.WriteString(`printf '===DEV===%s\n' `)
		b.WriteString(sshx.ShellQuote(id))
		b.WriteString("; vsish -e get /storage/scsifw/devices/")
		b.WriteString(sshx.ShellQuote(id))
		b.WriteString("/smart/valuesBuffer; ")
	}
	out, err := runEsxiRetry(client, "disk vsish smart batch", b.String(), 20*time.Second, 2, func(s string) bool {
		return strings.Count(s, "===DEV===") >= len(ataIDs)
	})
	if err != nil && !strings.Contains(out, "===DEV===") {
		logx.Warn("esxi vsish smart batch failed", "err", err.Error(), "n", len(ataIDs))
		return nil
	}
	segs := splitSMARTOutput(out)
	res := map[string]ataSMARTAttrs{}
	for id, seg := range segs {
		if attrs, ok := parseATASMARTBuffer(seg); ok {
			res[id] = attrs
		}
	}
	return res
}

// parseATASMARTBuffer 把 vsish 输出(每行形如 `[N]: 0xXX`)还原成 512 字节 ATA SMART data,
// 然后按 12 字节为一组扫 30 个 attribute,取关注的 attribute id 的 6 字节 raw(little endian)。
// 不关心 value/worst 列(那些 esxcli 已经给到了)。
// attribute ID 含义(SATA 标准):5=Reallocated / 9=Power-on Hours / 12=Power Cycle / 197=Pending Realloc / 198=Offline Uncorr。
func parseATASMARTBuffer(text string) (ataSMARTAttrs, bool) {
	attrs := newATASMARTAttrs()
	var buf [512]byte
	got := 0
	for _, line := range strings.Split(text, "\n") {
		trim := strings.TrimSpace(line)
		if !strings.HasPrefix(trim, "[") {
			continue
		}
		rb := strings.IndexByte(trim, ']')
		if rb < 2 {
			continue
		}
		idx, err := strconv.Atoi(trim[1:rb])
		if err != nil || idx < 0 || idx >= 512 {
			continue
		}
		hx := strings.Index(trim, "0x")
		if hx < 0 {
			continue
		}
		end := hx + 2
		for end < len(trim) {
			c := trim[end]
			if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
				break
			}
			end++
		}
		n, err := strconv.ParseUint(trim[hx+2:end], 16, 8)
		if err != nil {
			continue
		}
		buf[idx] = byte(n)
		got++
	}
	// 解析过少说明 vsish 输出截断/格式异常,放弃覆盖,沿用 esxcli。
	if got < 50 {
		return attrs, false
	}
	for i := 0; i < 30; i++ {
		off := 2 + i*12
		aid := buf[off]
		if aid == 0 {
			continue
		}
		var raw int64
		for j := 0; j < 6; j++ {
			raw |= int64(buf[off+5+j]) << (8 * j)
		}
		switch aid {
		case 5:
			attrs.ReallocatedSectors = raw
		case 9:
			attrs.PowerOnHours = raw
		case 12:
			attrs.PowerCycleCount = raw
		case 197:
			attrs.PendingSectorReallocation = raw
		case 198:
			attrs.UncorrectableErrors = raw
		}
	}
	return attrs, true
}
