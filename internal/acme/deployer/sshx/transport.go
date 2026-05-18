// Package sshx 是与 model/store 解耦的 SSH 传输层：拨号（含单跳跳板）、
// 写远端文件、执行远端命令。ssh / fnos 等 driver 复用它，避免重复实现连接细节。
package sshx

import (
	"bytes"
	"fmt"
	"net"
	"path"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

// Conn 是一次 SSH 连接的中立描述，不依赖业务 model。
// Bastion != nil 时先连跳板机再单跳到目标；跳板机自身的 Bastion 被忽略（不支持链）。
type Conn struct {
	Host    string
	Port    int
	User    string
	Auth    ssh.AuthMethod
	Bastion *Conn
}

func (c *Conn) addr() string { return net.JoinHostPort(c.Host, strconv.Itoa(c.Port)) }

// AuthMethod 按 authType（password | key）构造 ssh.AuthMethod。
func AuthMethod(authType, password, privateKey, passphrase string) (ssh.AuthMethod, error) {
	switch authType {
	case "password":
		return ssh.Password(password), nil
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
		return ssh.PublicKeys(signer), nil
	default:
		return nil, fmt.Errorf("未知 SSH 认证方式：%s", authType)
	}
}

// Dial 拨号到 conn，返回 client 与按 LIFO 关闭所有连接的 cleanup。
func Dial(logf func(string, ...any), conn *Conn) (*ssh.Client, func(), error) {
	cfg := &ssh.ClientConfig{
		User:            conn.User,
		Auth:            []ssh.AuthMethod{conn.Auth},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         15 * time.Second,
	}
	addr := conn.addr()

	if conn.Bastion == nil {
		if logf != nil {
			logf("连接 SSH：%s@%s", conn.User, addr)
		}
		client, err := ssh.Dial("tcp", addr, cfg)
		if err != nil {
			return nil, nil, fmt.Errorf("连接 SSH 失败：%w", err)
		}
		return client, func() { _ = client.Close() }, nil
	}

	b := conn.Bastion
	bCfg := &ssh.ClientConfig{
		User:            b.User,
		Auth:            []ssh.AuthMethod{b.Auth},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         15 * time.Second,
	}
	bAddr := b.addr()
	if logf != nil {
		logf("连接跳板机：%s@%s", b.User, bAddr)
	}
	bClient, err := ssh.Dial("tcp", bAddr, bCfg)
	if err != nil {
		return nil, nil, fmt.Errorf("连接跳板机失败：%w", err)
	}
	if logf != nil {
		logf("经跳板机连接 SSH：%s@%s", conn.User, addr)
	}
	tunnel, err := bClient.Dial("tcp", addr)
	if err != nil {
		_ = bClient.Close()
		return nil, nil, fmt.Errorf("从跳板机连接目标失败：%w", err)
	}
	nc, chans, reqs, err := ssh.NewClientConn(tunnel, addr, cfg)
	if err != nil {
		_ = tunnel.Close()
		_ = bClient.Close()
		return nil, nil, fmt.Errorf("通过跳板机握手 SSH 失败：%w", err)
	}
	client := ssh.NewClient(nc, chans, reqs)
	return client, func() {
		_ = client.Close()
		_ = bClient.Close()
	}, nil
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

	var out bytes.Buffer
	session.Stdout = &out
	session.Stderr = &out
	if stdin != nil {
		session.Stdin = bytes.NewReader(stdin)
	}
	err = session.Run(cmd)
	return out.String(), err
}

func ShellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'"
}
