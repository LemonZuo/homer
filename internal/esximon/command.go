package esximon

// ESXi SSH 命令执行与诊断。

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/LemonZuo/homer/internal/logx"
	"github.com/LemonZuo/homer/internal/sshx"
	"golang.org/x/crypto/ssh"
	"sync/atomic"
)

// 非交互非登录 SSH session 经常缺路径,统一显式注入(ESXi 实际不需要,
// 但加上没坏处,并且如果走 bastion + Linux jump host 这里就能兜住)。
const esxiPathPrefix = "export PATH=/bin:/sbin:/usr/lib/vmware/bin:/usr/lib/vmware/vsan/bin:$PATH; "

var commandSlowLogThresholdMS atomic.Int64

func init() {
	setCommandSlowLogThreshold(1500 * time.Millisecond)
}

func setCommandSlowLogThreshold(threshold time.Duration) {
	if threshold < 0 {
		threshold = 0
	}
	commandSlowLogThresholdMS.Store(threshold.Milliseconds())
}

func commandSlowLogThreshold() time.Duration {
	ms := commandSlowLogThresholdMS.Load()
	if ms <= 0 {
		return 0
	}
	return time.Duration(ms) * time.Millisecond
}

// 单条命令默认超时:ESXi 上单条 esxcli/vsish 多数 < 2s,8s 已经留了很大裕量。
// 用来防"某条命令偶发 hang 拖死整轮",避免一台机器一轮采样里只有部分子项有值。
const defaultCmdTimeout = 8 * time.Second

// runEsxi 在 client 上跑一条 shell 命令,前面统一加 PATH 注入。
// stderr 只用于诊断日志,不混入 stdout,避免 ESXi 的 vsish/esxcli banner 影响解析。
// 单条命令带 8s 超时;超时后命令在远端可能还在跑,session 由 ssh.Client 关闭时统一回收。
func runEsxi(client *ssh.Client, cmd string) (string, error) {
	return runEsxiTimeout(client, cmd, defaultCmdTimeout)
}

type esxiCommandResult struct {
	stdout              string
	stderr              string
	err                 error
	duration            time.Duration
	timedOut            bool
	remoteStarted       bool
	remoteFinished      bool
	remoteExitCode      int
	remoteExitCodeKnown bool
}

const (
	esxiDiagBegin     = "__HOMER_ESXI_BEGIN__"
	esxiDiagEndPrefix = "__HOMER_ESXI_END__ rc="
)

// runEsxiTimeout 同 runEsxi,但允许调用方为合批命令指定更长的超时。
//
// 末尾 `; true`:合批里某条 esxcli 偶发非零退出(典型如 NVMe 盘的 smart get 字段格式不同)
// 不应该让 sshx.Run 因为退出码非零而把已收集的 stdout 全部丢弃 —— stdout 里
// `===DEV===` 分段大概率已经写入大半,调用方按 stdout 是否空判定才是真的失败信号。
func runEsxiTimeout(client *ssh.Client, cmd string, timeout time.Duration) (string, error) {
	res := runEsxiTimeoutDetailed(client, cmd, timeout)
	return res.stdout, res.err
}

func runEsxiTimeoutDetailed(client *ssh.Client, cmd string, timeout time.Duration) esxiCommandResult {
	// 调用方拼合批命令时常在末尾留 `; `(方便循环里追加),
	// 但如果 cmd 末尾正好是 `;`,再附加 `; true` 会生成 `;;` —— POSIX/BusyBox sh
	// 对 `;;` 在非 case 上下文直接报语法错误,整段 group 不执行,stdout 为空,
	// 远端进程以 status 2 退出 —— 这正是之前看到 `smart batch failed err=Process exited with status 2`
	// 但 stdout 全空、partial 兜底打不上的根因。
	trimmed := strings.TrimRight(cmd, " \t\n;")
	full := esxiPathPrefix +
		"printf '" + esxiDiagBegin + "\\n' >&2; " +
		"{ " + trimmed + "; }; " +
		"__homer_rc=$?; " +
		"printf '" + esxiDiagEndPrefix + "%s\\n' \"$__homer_rc\" >&2; " +
		"true"
	start := time.Now()
	ch := make(chan esxiCommandResult, 1)
	go func() {
		stdout, stderr, err := sshx.RunStreams(client, full, nil)
		cleanStderr, diag := parseEsxiCommandDiagnostics(stderr)
		ch <- esxiCommandResult{
			stdout:              stdout,
			stderr:              cleanStderr,
			err:                 err,
			duration:            time.Since(start),
			remoteStarted:       diag.started,
			remoteFinished:      diag.finished,
			remoteExitCode:      diag.exitCode,
			remoteExitCodeKnown: diag.exitCodeKnown,
		}
	}()
	select {
	case r := <-ch:
		return r
	case <-time.After(timeout):
		return esxiCommandResult{
			err:      fmt.Errorf("esxi command timeout after %s", timeout),
			duration: time.Since(start),
			timedOut: true,
		}
	}
}

func runEsxiRetry(client *ssh.Client, name, cmd string, timeout time.Duration, attempts int, ok func(string) bool) (string, error) {
	if attempts <= 0 {
		attempts = 1
	}
	var lastOut string
	var lastErr error
	var lastRes esxiCommandResult
	for i := 1; i <= attempts; i++ {
		res := runEsxiTimeoutDetailed(client, cmd, timeout)
		out, err := res.stdout, res.err
		lastRes = res
		lastOut = out
		if err == nil && (ok == nil || ok(out)) {
			logEsxiCommandSlow(name, timeout, res)
			return out, nil
		}
		if err != nil {
			lastErr = err
		} else {
			lastErr = fmt.Errorf("validator failed")
		}
		if i < attempts {
			logx.Warn("esxi command retry", esxiCommandLogAttrs(name, i, timeout, lastErr, res)...)
			time.Sleep(time.Duration(i) * 150 * time.Millisecond)
		}
	}
	logx.Warn("esxi command failed", esxiCommandLogAttrs(name, attempts, timeout, lastErr, lastRes)...)
	return lastOut, fmt.Errorf("%s failed after %d attempts: %w", name, attempts, lastErr)
}

func logEsxiCommandSlow(name string, timeout time.Duration, res esxiCommandResult) {
	threshold := commandSlowLogThreshold()
	if threshold <= 0 || res.duration < threshold {
		return
	}
	attrs := []any{
		"name", name,
		"duration", res.duration.String(),
		"threshold", threshold.String(),
		"stdout_bytes", len(res.stdout),
		"stderr_bytes", len(res.stderr),
		"timeout", timeout.String(),
		"timed_out", res.timedOut,
		"remote_started", res.remoteStarted,
		"remote_finished", res.remoteFinished,
	}
	if res.remoteExitCodeKnown {
		attrs = append(attrs, "remote_exit_code", res.remoteExitCode)
	}
	logx.Info("esxi command slow", attrs...)
}

func esxiCommandLogAttrs(name string, attempt int, timeout time.Duration, err error, res esxiCommandResult) []any {
	attrs := []any{
		"name", name,
		"attempt", attempt,
		"err", err.Error(),
		"stdout_bytes", len(res.stdout),
		"stderr_bytes", len(res.stderr),
		"duration", res.duration.String(),
		"timeout", timeout.String(),
		"timed_out", res.timedOut,
		"remote_started", res.remoteStarted,
		"remote_finished", res.remoteFinished,
	}
	if res.remoteExitCodeKnown {
		attrs = append(attrs, "remote_exit_code", res.remoteExitCode)
	}
	if sample := compactLogSample(res.stderr, 240); sample != "" {
		attrs = append(attrs, "stderr", sample)
	}
	if err.Error() == "validator failed" {
		if sample := compactLogSample(res.stdout, 240); sample != "" {
			attrs = append(attrs, "stdout", sample)
		}
	}
	return attrs
}

type esxiCommandDiagnostics struct {
	started       bool
	finished      bool
	exitCode      int
	exitCodeKnown bool
}

func parseEsxiCommandDiagnostics(stderr string) (string, esxiCommandDiagnostics) {
	var diag esxiCommandDiagnostics
	var kept []string
	for _, line := range strings.Split(stderr, "\n") {
		trimmed := strings.TrimSpace(line)
		switch {
		case trimmed == "":
			if len(kept) > 0 {
				kept = append(kept, line)
			}
		case trimmed == esxiDiagBegin:
			diag.started = true
		case strings.HasPrefix(trimmed, esxiDiagEndPrefix):
			diag.finished = true
			raw := strings.TrimSpace(strings.TrimPrefix(trimmed, esxiDiagEndPrefix))
			if code, err := strconv.Atoi(raw); err == nil {
				diag.exitCode = code
				diag.exitCodeKnown = true
			}
		default:
			kept = append(kept, line)
		}
	}
	return strings.TrimSpace(strings.Join(kept, "\n")), diag
}

func compactLogSample(s string, limit int) string {
	s = strings.TrimSpace(s)
	if s == "" || limit <= 0 {
		return ""
	}
	s = strings.Join(strings.Fields(s), " ")
	if len(s) <= limit {
		return s
	}
	if limit <= 3 {
		return s[:limit]
	}
	return s[:limit-3] + "..."
}
