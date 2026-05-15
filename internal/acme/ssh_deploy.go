package acme

import (
	"bytes"
	"errors"
	"fmt"
	"path"
	"strings"
	"time"

	"github.com/LemonZuo/homer/internal/model"
	"golang.org/x/crypto/ssh"
)

// SSHDeployOptions 描述一次部署的远端路径和部署后命令。
// 支持在路径和命令里使用 {domain} 占位符。
type SSHDeployOptions struct {
	Domain        string `json:"-"`
	CertPath      string `json:"cert_path"`
	KeyPath       string `json:"key_path"`
	ChainPath     string `json:"chain_path"`
	FullchainPath string `json:"fullchain_path"`
	DeployCommand string `json:"deploy_command"`
}

func (o *SSHDeployOptions) normalize() {
	o.CertPath = strings.TrimSpace(o.CertPath)
	o.KeyPath = strings.TrimSpace(o.KeyPath)
	o.ChainPath = strings.TrimSpace(o.ChainPath)
	o.FullchainPath = strings.TrimSpace(o.FullchainPath)
	o.DeployCommand = strings.TrimSpace(o.DeployCommand)
}

func (o SSHDeployOptions) validate() error {
	if strings.TrimSpace(o.KeyPath) == "" {
		return errors.New("远端 key.pem 路径不能为空")
	}
	if strings.TrimSpace(o.CertPath) == "" && strings.TrimSpace(o.FullchainPath) == "" {
		return errors.New("cert.pem 路径和 fullchain.pem 路径至少填写一个")
	}
	return nil
}

func deployCertViaSSH(logw *teeWriter, target model.ACMESSHTarget, cert model.ACMECert, opts SSHDeployOptions) error {
	opts.normalize()
	if err := opts.validate(); err != nil {
		return err
	}
	auth, err := sshAuth(target)
	if err != nil {
		return err
	}
	cfg := &ssh.ClientConfig{
		User:            target.Username,
		Auth:            []ssh.AuthMethod{auth},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         15 * time.Second,
	}
	addr := fmt.Sprintf("%s:%d", target.Host, target.Port)
	logf(logw, "连接 SSH：%s@%s", target.Username, addr)
	client, err := ssh.Dial("tcp", addr, cfg)
	if err != nil {
		return fmt.Errorf("连接 SSH 失败：%w", err)
	}
	defer client.Close()

	files := []struct {
		label string
		path  string
		data  string
		mode  string
	}{
		{label: "cert.pem", path: renderSSHDeployTemplate(opts.CertPath, opts.Domain), data: cert.CertPEM, mode: "0644"},
		{label: "chain.pem", path: renderSSHDeployTemplate(opts.ChainPath, opts.Domain), data: cert.ChainPEM, mode: "0644"},
		{label: "fullchain.pem", path: renderSSHDeployTemplate(opts.FullchainPath, opts.Domain), data: cert.FullchainPEM, mode: "0644"},
		{label: "key.pem", path: renderSSHDeployTemplate(opts.KeyPath, opts.Domain), data: cert.KeyPEM, mode: "0600"},
	}
	for _, f := range files {
		if strings.TrimSpace(f.path) == "" {
			continue
		}
		if strings.TrimSpace(f.data) == "" {
			return fmt.Errorf("%s 内容为空，无法写入 %s", f.label, f.path)
		}
		if err := writeRemoteFile(client, f.path, []byte(f.data), f.mode); err != nil {
			return fmt.Errorf("写入远端 %s 失败：%w", f.path, err)
		}
		logf(logw, "已写入远端文件：%s", f.path)
	}

	cmd := renderSSHDeployTemplate(opts.DeployCommand, opts.Domain)
	if strings.TrimSpace(cmd) != "" {
		logf(logw, "执行部署命令：%s", cmd)
		out, err := runRemoteCommand(client, cmd, nil)
		if strings.TrimSpace(out) != "" {
			logf(logw, "命令输出：\n%s", strings.TrimSpace(out))
		}
		if err != nil {
			return fmt.Errorf("部署命令执行失败：%w", err)
		}
	}
	return nil
}

func renderSSHDeployTemplate(s, domain string) string {
	return strings.ReplaceAll(s, "{domain}", domain)
}

func sshAuth(target model.ACMESSHTarget) (ssh.AuthMethod, error) {
	switch target.AuthType {
	case "password":
		return ssh.Password(target.Password), nil
	case "key":
		var signer ssh.Signer
		var err error
		key := []byte(target.PrivateKey)
		if strings.TrimSpace(target.Passphrase) != "" {
			signer, err = ssh.ParsePrivateKeyWithPassphrase(key, []byte(target.Passphrase))
		} else {
			signer, err = ssh.ParsePrivateKey(key)
		}
		if err != nil {
			return nil, fmt.Errorf("解析 SSH 私钥失败：%w", err)
		}
		return ssh.PublicKeys(signer), nil
	default:
		return nil, fmt.Errorf("未知 SSH 认证方式：%s", target.AuthType)
	}
}

func writeRemoteFile(client *ssh.Client, remotePath string, data []byte, mode string) error {
	dir := path.Dir(remotePath)
	if _, err := runRemoteCommand(client, "mkdir -p "+shellQuote(dir), nil); err != nil {
		return err
	}
	cmd := "cat > " + shellQuote(remotePath) + " && chmod " + shellQuote(mode) + " " + shellQuote(remotePath)
	_, err := runRemoteCommand(client, cmd, data)
	return err
}

func runRemoteCommand(client *ssh.Client, cmd string, stdin []byte) (string, error) {
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

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'"
}
