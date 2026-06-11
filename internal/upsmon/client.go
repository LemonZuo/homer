// Package upsmon 通过 SSH 远程执行 NUT 的 upsc 命令,采集机器上挂接的 UPS 状态。
// 机器来源于 acme_deploy_target 表里 kind IN ('ssh','fnos') 的目标,复用 sshlike 凭证体系。
// 不持有任何 ssh.Client,每次采样建连即用即抛。
package upsmon

import (
	"strconv"
	"strings"
	"time"

	"github.com/LemonZuo/homer/internal/model"
	"github.com/LemonZuo/homer/internal/sshx"
	"golang.org/x/crypto/ssh"
)

// upscReading 是一个 UPS 解析后的中间结构,store 层会拆成 sample + state。
// 所有数值字段统一用 -1 表哨兵值(未读到),与 battery_percent 一致,
// 前端用 >=0 判断是否有数据,而不是用 0(0 在功率/负载场景下是有效值)。
type upscReading struct {
	Name                  string
	Mfr                   string
	Model                 string
	PowerSource           string  // mains | battery | low_battery | unknown
	BatteryPercent        int     // -1 未读到
	RuntimeMinutes        int     // -1 未读到
	BatteryVoltage        float32 // -1 未读到,单位 V(当前电池端电压)
	BatteryNominalVoltage float32 // -1 未读到,单位 V(电池组标称电压,如 12/24/48)
	BatteryType           string  // 空串未读到,如 PbAc(铅酸)
	InputVoltage          float32 // -1 未读到
	OutputVoltage         float32 // -1 未读到
	LoadPercent           int     // -1 未读到
	RealPower             int     // -1 未读到,单位 W
	RawStatus             string  // 原始 ups.status,前端 tooltip 用
}

// 非交互非登录 SSH session 的 PATH 经常缺 /usr/local/sbin、/usr/local/bin,
// 而 NUT 在 fnOS / 群晖 / 部分发行版上正是装在那里。这里在命令前面统一显式
// 注入一份常见的系统 PATH,让 upsc 一定可见,避免 cron 跑十轮才碰巧成功一次。
const upscPathPrefix = "export PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin:$PATH; "

// probeHost 在 client 上枚举本机所有 UPS 并读取状态。
// 第二返回值 diag 在无 UPS 时给上层(test 接口)展示诊断信息,采样链路不消费。
// 若主机未装 NUT 或没绑 UPS,返回 (nil, diag, nil) —— 让上层静默跳过。
// 真正的连接/命令错误才返回 error。
//
// 注意:upsc 2.8+ 启动时会往 stderr 打 "Init SSL without certificate database"
// 这行 banner。一旦合并进 stdout,会被当成 6 个伪 UPS 名(Init/SSL/without/...
// /database/真名),后续 upsc <伪名> 全部失败,真名的诊断信息也被覆盖掉。
// 所以正常路径用 2>/dev/null 严格只取 stdout,诊断信息走单独的 session。
func probeHost(client *ssh.Client) ([]upscReading, []string, string, error) {
	out, err := sshx.Run(client, upscPathPrefix+"upsc -l 2>/dev/null", nil)
	if err != nil {
		// 命令本身失败(找不到 upsc / 非零退出),用一次带 stderr 的额外探测拿诊断信息。
		return nil, nil, diagnose(client, ""), nil
	}
	names := splitUPSNames(out)
	if len(names) == 0 {
		return nil, nil, diagnose(client, ""), nil
	}
	readings := make([]upscReading, 0, len(names))
	var lastFailName string
	for _, name := range names {
		reading, ok := readOneUPS(client, name)
		if !ok {
			// upsd 在 driver pollfreq 重新枚举 USB 的瞬间会返回 exit=0 + 空白(NUT 已知行为)。
			// 单次失败不阻断,但也不写空值 — 保留上一轮 state,避免被零值覆盖。
			// names 仍然返回完整列表:NUT 知道这台 UPS 存在,只是本轮 reading 抖动。
			lastFailName = name
			continue
		}
		readings = append(readings, reading)
	}
	if len(readings) == 0 {
		return nil, names, diagnose(client, lastFailName), nil
	}
	return readings, names, "", nil
}

// readOneUPS 跑 `upsc <name>`,空白 / 解析失败时短延迟后再试两次。
// NUT 驱动每 pollfreq 秒(默认 30s)会重新枚举 USB,期间 upsd 对该 UPS 的查询
// 会返回 exit=0 + 空 stdout,从而和我们 30s cron 形成 50/50 共振。
// 200ms / 500ms 内驱动一般已经回到稳定态。
func readOneUPS(client *ssh.Client, name string) (upscReading, bool) {
	delays := []time.Duration{0, 200 * time.Millisecond, 500 * time.Millisecond}
	for _, d := range delays {
		if d > 0 {
			time.Sleep(d)
		}
		raw, err := sshx.Run(client, upscPathPrefix+"upsc "+sshx.ShellQuote(name)+" 2>/dev/null", nil)
		if err != nil {
			continue
		}
		if reading, ok := parseUPSCOutput(name, raw); ok {
			return reading, true
		}
	}
	return upscReading{}, false
}

// diagnose 在主探测拿不到 UPS 时,把对应 upsc 命令的 stderr 带回来给前端 toast 显示。
// failName 为空表示连 `upsc -l` 都没拿到名字,否则是某个 UPS 名读取失败。
func diagnose(client *ssh.Client, failName string) string {
	cmd := upscPathPrefix + "upsc -l 2>&1"
	if failName != "" {
		cmd = upscPathPrefix + "upsc " + sshx.ShellQuote(failName) + " 2>&1"
	}
	raw, _ := sshx.Run(client, cmd, nil)
	return strings.TrimSpace(raw)
}

// splitUPSNames 解析 `upsc -l` 的输出。
// 飞牛 / 标准 NUT 配置下每行一个 UPS 名,但有少数实现把列表打印在一行用空格分隔,两种都兜住。
func splitUPSNames(out string) []string {
	out = strings.TrimSpace(out)
	if out == "" {
		return nil
	}
	seen := map[string]struct{}{}
	var names []string
	for _, line := range strings.Split(out, "\n") {
		for _, name := range strings.Fields(line) {
			name = strings.TrimSpace(name)
			if name == "" {
				continue
			}
			if _, dup := seen[name]; dup {
				continue
			}
			seen[name] = struct{}{}
			names = append(names, name)
		}
	}
	return names
}

// parseUPSCOutput 把 `upsc <name>` 的 KV 输出映射成 upscReading。
// 关键字段(NUT 标准):
//   - device.mfr   / device.model       品牌型号(部分驱动用 ups.mfr / ups.model 兜底)
//   - ups.status                        OL / OB / LB / CHRG / DISCHRG / RB ...
//   - battery.charge                    剩余电量百分比 0~100
//   - battery.runtime                   预估秒数
//
// 实时功率(瓦)优先级:ups.realpower > ups.power > ups.realpower.nominal × ups.load / 100。
// CyberPower 等只暴露 nominal + load 时走第三档,展示用足够准。
//
// 返回 ok=false 表示输出为空或没匹配到任何 NUT 字段(upsd 瞬时不响应),
// 调用方应跳过本轮,避免用零值覆盖上一轮的有效 state。
func parseUPSCOutput(name, raw string) (upscReading, bool) {
	if strings.TrimSpace(raw) == "" {
		return upscReading{}, false
	}
	r := upscReading{
		Name:                  name,
		PowerSource:           model.UPSPowerUnknown,
		BatteryPercent:        -1,
		RuntimeMinutes:        -1,
		BatteryVoltage:        -1,
		BatteryNominalVoltage: -1,
		InputVoltage:          -1,
		OutputVoltage:         -1,
		LoadPercent:           -1,
		RealPower:             -1,
	}
	nominalReal := -1 // ups.realpower.nominal,仅用于在 realpower/power 都缺时推算
	matched := 0
	for _, line := range strings.Split(raw, "\n") {
		idx := strings.IndexByte(line, ':')
		if idx <= 0 {
			continue
		}
		key := strings.TrimSpace(line[:idx])
		val := strings.TrimSpace(line[idx+1:])
		switch key {
		case "device.mfr", "ups.mfr":
			if r.Mfr == "" {
				r.Mfr = val
				matched++
			}
		case "device.model", "ups.model":
			if r.Model == "" {
				r.Model = val
				matched++
			}
		case "ups.status":
			r.RawStatus = val
			r.PowerSource = mapPowerSource(val)
			matched++
		case "battery.charge":
			if n, err := strconv.ParseFloat(val, 64); err == nil {
				r.BatteryPercent = clampInt(int(n+0.5), 0, 100)
				matched++
			}
		case "battery.runtime":
			if n, err := strconv.ParseFloat(val, 64); err == nil && n >= 0 {
				r.RuntimeMinutes = int(n / 60)
				matched++
			}
		case "battery.voltage":
			if n, err := strconv.ParseFloat(val, 64); err == nil && n >= 0 {
				r.BatteryVoltage = float32(n)
				matched++
			}
		case "battery.voltage.nominal":
			if n, err := strconv.ParseFloat(val, 64); err == nil && n > 0 {
				r.BatteryNominalVoltage = float32(n)
				matched++
			}
		case "battery.type":
			if val != "" {
				r.BatteryType = val
				matched++
			}
		case "input.voltage":
			if n, err := strconv.ParseFloat(val, 64); err == nil && n >= 0 {
				r.InputVoltage = float32(n)
				matched++
			}
		case "output.voltage":
			if n, err := strconv.ParseFloat(val, 64); err == nil && n >= 0 {
				r.OutputVoltage = float32(n)
				matched++
			}
		case "ups.load":
			if n, err := strconv.ParseFloat(val, 64); err == nil {
				r.LoadPercent = clampInt(int(n+0.5), 0, 100)
				matched++
			}
		case "ups.realpower":
			if n, err := strconv.ParseFloat(val, 64); err == nil && n >= 0 {
				r.RealPower = int(n + 0.5)
				matched++
			}
		case "ups.power":
			// realpower 缺失时的回退(部分驱动只暴露视在功率,但作为展示已够)
			if r.RealPower < 0 {
				if n, err := strconv.ParseFloat(val, 64); err == nil && n >= 0 {
					r.RealPower = int(n + 0.5)
					matched++
				}
			}
		case "ups.realpower.nominal":
			// 不计入 matched(只是辅助推算,本身没有"实时"语义)
			if n, err := strconv.ParseFloat(val, 64); err == nil && n > 0 {
				nominalReal = int(n + 0.5)
			}
		}
	}
	// 兜底:CyberPower 等驱动只有 nominal + load,按 nominal × load% 估算实时功率
	if r.RealPower < 0 && nominalReal > 0 && r.LoadPercent >= 0 {
		r.RealPower = (nominalReal*r.LoadPercent + 50) / 100
	}
	if matched == 0 {
		return upscReading{}, false
	}
	return r, true
}

// mapPowerSource 把 NUT 的 ups.status 字段映射成 4 个枚举值。
// 状态字符串可能包含多个 token(如 "OB LB" / "OL CHRG"),按"最严重的"取:LB > OB > OL。
func mapPowerSource(status string) string {
	if status == "" {
		return model.UPSPowerUnknown
	}
	tokens := strings.Fields(strings.ToUpper(status))
	hasLB, hasOB, hasOL := false, false, false
	for _, t := range tokens {
		switch t {
		case "LB":
			hasLB = true
		case "OB", "DISCHRG":
			hasOB = true
		case "OL":
			hasOL = true
		}
	}
	switch {
	case hasLB:
		return model.UPSPowerLowBattery
	case hasOB:
		return model.UPSPowerBattery
	case hasOL:
		return model.UPSPowerMains
	default:
		return model.UPSPowerUnknown
	}
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
