// Package sshx 是与 model/store 解耦的 SSH 传输层：拨号（含单跳跳板）、
// 写远端文件、执行远端命令。ssh / fnos 等 driver 复用它，避免重复实现连接细节。
package sshx

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
)

// Conn 是一次 SSH 连接的中立描述，不依赖业务 model。
// Bastion != nil 时先连跳板机再单跳到目标；跳板机自身的 Bastion 被忽略（不支持链）。
type Conn struct {
	Host    string
	Port    int
	User    string
	Auth    []ssh.AuthMethod
	Bastion *Conn
}

func (c *Conn) addr() string { return net.JoinHostPort(c.Host, strconv.Itoa(c.Port)) }

// AuthMethod 按 authType（password | key）构造 SSH 认证方法集。
// password 模式同时返回 password 与 keyboard-interactive 两种，兼容 ESXi
// 等只接受 keyboard-interactive 的 sshd。
func AuthMethod(authType, password, privateKey, passphrase string) ([]ssh.AuthMethod, error) {
	switch authType {
	case "password":
		ki := ssh.KeyboardInteractive(func(_, _ string, questions []string, _ []bool) ([]string, error) {
			answers := make([]string, len(questions))
			for i := range questions {
				answers[i] = password
			}
			return answers, nil
		})
		return []ssh.AuthMethod{ssh.Password(password), ki}, nil
	case "key":
		var signer ssh.Signer
		var err error
		key := []byte(privateKey)
		if strings.TrimSpace(passphrase) != "" {
			signer, err = ssh.ParsePrivateKeyWithPassphrase(key, []byte(passphrase))
		} else {
			signer, err = ssh.ParsePrivateKey(key)
		}
		if err != nil {
			return nil, fmt.Errorf("解析 SSH 私钥失败：%w", err)
		}
		return []ssh.AuthMethod{ssh.PublicKeys(signer)}, nil
	default:
		return nil, fmt.Errorf("未知 SSH 认证方式：%s", authType)
	}
}

const dialTimeout = 15 * time.Second

// handshakeTimeout 限制单次 SSH 握手+认证（以及经跳板机打开隧道）的时长。
// ssh.ClientConfig.Timeout 只管 TCP 建连，对端收下连接却不回包时握手会无限阻塞。
// 用变量便于测试缩短。
var handshakeTimeout = 30 * time.Second

// Dial 拨号到 conn，返回 client 与按 LIFO 关闭所有连接的 cleanup。
// 等价于 DialContext(context.Background(), ...)：握手有超时，但已建立的会话不受时限约束。
func Dial(logf func(string, ...any), conn *Conn) (*ssh.Client, func(), error) {
	return DialContext(context.Background(), logf, conn)
}

// DialContext 同 Dial，额外绑定 ctx：ctx 结束时（握手阶段或之后）主动关闭底层连接，
// 卡住的握手、Run/WriteFile 会因连接关闭而返回错误。调用 cleanup 会解除绑定并关闭连接。
func DialContext(ctx context.Context, logf func(string, ...any), conn *Conn) (*ssh.Client, func(), error) {
	if err := ctx.Err(); err != nil {
		return nil, nil, fmt.Errorf("连接 SSH 前已取消：%w", err)
	}
	cfg := clientConfig(conn)
	addr := conn.addr()

	if conn.Bastion == nil {
		if logf != nil {
			logf("连接 SSH：%s@%s", conn.User, addr)
		}
		client, err := dialClient(ctx, addr, cfg)
		if err != nil {
			return nil, nil, fmt.Errorf("连接 SSH 失败：%w", err)
		}
		return client, bindContext(ctx, client), nil
	}

	b := conn.Bastion
	bAddr := b.addr()
	if logf != nil {
		logf("连接跳板机：%s@%s", b.User, bAddr)
	}
	bClient, err := dialClient(ctx, bAddr, clientConfig(b))
	if err != nil {
		return nil, nil, fmt.Errorf("连接跳板机失败：%w", err)
	}
	if logf != nil {
		logf("经跳板机连接 SSH：%s@%s", conn.User, addr)
	}
	// 隧道是跳板机连接上的 channel，单独关闭它不一定能打断阻塞读；超时时直接关跳板机连接。
	var tunnel net.Conn
	if err := guard(ctx, bClient, func() error {
		var derr error
		tunnel, derr = bClient.Dial("tcp", addr)
		return derr
	}); err != nil {
		_ = bClient.Close()
		return nil, nil, fmt.Errorf("从跳板机连接目标失败：%w", err)
	}
	client, err := newClient(ctx, tunnel, addr, cfg, bClient)
	if err != nil {
		_ = tunnel.Close()
		_ = bClient.Close()
		return nil, nil, fmt.Errorf("通过跳板机握手 SSH 失败：%w", err)
	}
	return client, bindContext(ctx, client, bClient), nil
}

func clientConfig(c *Conn) *ssh.ClientConfig {
	return &ssh.ClientConfig{
		User:            c.User,
		Auth:            c.Auth,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         dialTimeout,
	}
}

// dialClient 建 TCP 连接（dialTimeout）后做带超时的 SSH 握手。
func dialClient(ctx context.Context, addr string, cfg *ssh.ClientConfig) (*ssh.Client, error) {
	d := net.Dialer{Timeout: dialTimeout}
	nc, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, err
	}
	client, err := newClient(ctx, nc, addr, cfg, nc)
	if err != nil {
		_ = nc.Close()
		return nil, err
	}
	return client, nil
}

// newClient 在 nc 上做 SSH 握手；超时或 ctx 结束时关闭 abort 以打断阻塞。
func newClient(ctx context.Context, nc net.Conn, addr string, cfg *ssh.ClientConfig, abort io.Closer) (*ssh.Client, error) {
	var client *ssh.Client
	err := guard(ctx, abort, func() error {
		c, chans, reqs, err := ssh.NewClientConn(nc, addr, cfg)
		if err != nil {
			return err
		}
		client = ssh.NewClient(c, chans, reqs)
		return nil
	})
	if err != nil {
		if client != nil {
			_ = client.Close()
		}
		return nil, err
	}
	return client, nil
}

// guard 执行 fn；若 handshakeTimeout 到期或 ctx 结束，关闭 c 打断 fn 内的阻塞 IO，并返回超时/取消原因。
func guard(ctx context.Context, c io.Closer, fn func() error) error {
	tctx, cancel := context.WithTimeout(ctx, handshakeTimeout)
	defer cancel()
	stop := context.AfterFunc(tctx, func() { _ = c.Close() })
	err := fn()
	if !stop() {
		// 已触发关闭：即使 fn 恰好成功，连接也已不可用。
		if perr := ctx.Err(); perr != nil {
			return fmt.Errorf("已取消：%w", perr)
		}
		return fmt.Errorf("超过 %s 未完成：%w", handshakeTimeout, context.DeadlineExceeded)
	}
	return err
}

// bindContext 让 ctx 结束时关闭 closers（按传入顺序），返回解除绑定并关闭的 cleanup。
func bindContext(ctx context.Context, closers ...io.Closer) func() {
	closeAll := func() {
		for _, c := range closers {
			_ = c.Close()
		}
	}
	stop := context.AfterFunc(ctx, closeAll)
	return func() {
		stop()
		closeAll()
	}
}

// WriteFile 把 data 写到远端 remotePath，先 mkdir -p 父目录再 chmod。
func WriteFile(client *ssh.Client, remotePath string, data []byte, mode string) error {
	dir := path.Dir(remotePath)
	if _, err := Run(client, "mkdir -p "+ShellQuote(dir), nil); err != nil {
		return err
	}
	cmd := "cat > " + ShellQuote(remotePath) + " && chmod " + ShellQuote(mode) + " " + ShellQuote(remotePath)
	_, err := Run(client, cmd, data)
	return err
}

// Run 在远端执行 cmd，stdin 非 nil 时作为标准输入；返回合并后的 stdout+stderr。
func Run(client *ssh.Client, cmd string, stdin []byte) (string, error) {
	session, err := client.NewSession()
	if err != nil {
		return "", err
	}
	defer session.Close()

	// stdout/stderr 由两个 copy 协程并发写入；x/crypto/ssh 不像 os/exec 会对同一 writer 串行化，需自带锁。
	var out syncBuffer
	session.Stdout = &out
	session.Stderr = &out
	if stdin != nil {
		session.Stdin = bytes.NewReader(stdin)
	}
	err = session.Run(cmd)
	return out.String(), err
}

type syncBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *syncBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

// RunStreams 在远端执行 cmd，分别返回 stdout 与 stderr。
func RunStreams(client *ssh.Client, cmd string, stdin []byte) (string, string, error) {
	session, err := client.NewSession()
	if err != nil {
		return "", "", err
	}
	defer session.Close()

	var stdout, stderr bytes.Buffer
	session.Stdout = &stdout
	session.Stderr = &stderr
	if stdin != nil {
		session.Stdin = bytes.NewReader(stdin)
	}
	err = session.Run(cmd)
	return stdout.String(), stderr.String(), err
}

func ShellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'"
}
