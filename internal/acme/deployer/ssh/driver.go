// Package acmessh 是 ACME 证书的 SSH 部署 driver 实现。
// 与 safeline 子包平行：core（internal/acme）只持有 DeployDriver 接口与通用 store，
// 具体 driver 拆到各自子包，互不影响。
package acmessh

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/LemonZuo/homer/internal/acme"
	"github.com/LemonZuo/homer/internal/model"
	"golang.org/x/crypto/ssh"
)

// Driver 实现 acme.DeployDriver，把证书写到远端 SSH 机器并执行部署命令。
type Driver struct{}

// TargetAuth 是 acme_deploy_target.auth_json 在 SSH 场景下的结构。
type TargetAuth struct {
	Username   string `json:"username"`
	AuthType   string `json:"auth_type"`
	Password   string `json:"password"`
	PrivateKey string `json:"private_key"`
	Passphrase string `json:"passphrase"`
}

func NewDriver() *Driver { return &Driver{} }

func (d *Driver) Kind() string  { return acme.DeployKindSSH }
func (d *Driver) Label() string { return "SSH 机器" }

func (d *Driver) ValidateTarget(target model.ACMEDeployTarget) error {
	t, err := targetFromDeployTarget(target)
	if err != nil {
		return err
	}
	return validateTarget(*t)
}

func (d *Driver) ValidateConfig(_ model.ACMEDeployTarget, cfg model.ACMEDeployConfig) error {
	opts, err := optionsFromGenericConfig(cfg, "")
	if err != nil {
		return err
	}
	return opts.validate()
}

func (d *Driver) TestTarget(_ context.Context, target model.ACMEDeployTarget) error {
	t, err := targetFromDeployTarget(target)
	if err != nil {
		return err
	}
	auth, err := sshAuth(*t)
	if err != nil {
		return err
	}
	cfg := &ssh.ClientConfig{
		User:            t.Username,
		Auth:            []ssh.AuthMethod{auth},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         15 * time.Second,
	}
	addr := fmt.Sprintf("%s:%d", t.Host, t.Port)
	client, err := ssh.Dial("tcp", addr, cfg)
	if err != nil {
		return fmt.Errorf("连接 SSH 失败：%w", err)
	}
	return client.Close()
}

func (d *Driver) Deploy(_ context.Context, req acme.DeployRequest) (*acme.DeployResult, error) {
	target, err := targetFromDeployTarget(req.Target)
	if err != nil {
		return nil, err
	}
	opts, err := optionsFromGenericConfig(req.Config, req.Domain.MainDomain)
	if err != nil {
		return nil, err
	}
	if req.Logf != nil {
		req.Logf("SSH 目标：%s", targetSummary(*target))
	}
	if err := deployCert(req.Logf, *target, req.Cert, opts); err != nil {
		return nil, err
	}
	return &acme.DeployResult{}, nil
}

// DeployOptions 描述一次部署的远端路径和部署后命令。
// 支持在路径和命令里使用 {domain} 占位符。
type DeployOptions struct {
	Domain        string `json:"-"`
	CertPath      string `json:"cert_path"`
	KeyPath       string `json:"key_path"`
	ChainPath     string `json:"chain_path"`
	FullchainPath string `json:"fullchain_path"`
	DeployCommand string `json:"deploy_command"`
}

func (o *DeployOptions) normalize() {
	o.CertPath = strings.TrimSpace(o.CertPath)
	o.KeyPath = strings.TrimSpace(o.KeyPath)
	o.ChainPath = strings.TrimSpace(o.ChainPath)
	o.FullchainPath = strings.TrimSpace(o.FullchainPath)
	o.DeployCommand = strings.TrimSpace(o.DeployCommand)
}

func (o DeployOptions) validate() error {
	if strings.TrimSpace(o.KeyPath) == "" {
		return errors.New("远端 key.pem 路径不能为空")
	}
	if strings.TrimSpace(o.CertPath) == "" && strings.TrimSpace(o.FullchainPath) == "" {
		return errors.New("cert.pem 路径和 fullchain.pem 路径至少填写一个")
	}
	return nil
}

func targetFromDeployTarget(target model.ACMEDeployTarget) (*model.ACMESSHTarget, error) {
	auth := TargetAuth{}
	if err := acme.JSONUnmarshal([]byte(acme.EmptyJSON(target.AuthJSON)), &auth); err != nil {
		return nil, fmt.Errorf("解析 SSH 认证配置失败：%w", err)
	}
	host, port, err := splitEndpoint(target.Endpoint)
	if err != nil {
		return nil, err
	}
	out := &model.ACMESSHTarget{
		ID:         target.ID,
		Name:       target.Name,
		Host:       host,
		Port:       port,
		Username:   auth.Username,
		AuthType:   auth.AuthType,
		Password:   auth.Password,
		PrivateKey: auth.PrivateKey,
		Passphrase: auth.Passphrase,
		Enabled:    target.Enabled,
		CreatedAt:  target.CreatedAt,
		UpdatedAt:  target.UpdatedAt,
	}
	normalizeTarget(out)
	return out, nil
}

func deployTargetFromTarget(t model.ACMESSHTarget) model.ACMEDeployTarget {
	normalizeTarget(&t)
	return model.ACMEDeployTarget{
		ID:       t.ID,
		Name:     t.Name,
		Kind:     acme.DeployKindSSH,
		Endpoint: net.JoinHostPort(t.Host, strconv.Itoa(t.Port)),
		AuthJSON: acme.MustJSON(TargetAuth{
			Username:   t.Username,
			AuthType:   t.AuthType,
			Password:   t.Password,
			PrivateKey: t.PrivateKey,
			Passphrase: t.Passphrase,
		}),
		ConfigJSON: "{}",
		Enabled:    t.Enabled,
		CreatedAt:  t.CreatedAt,
		UpdatedAt:  t.UpdatedAt,
	}
}

func optionsFromGenericConfig(cfg model.ACMEDeployConfig, domain string) (DeployOptions, error) {
	var opts DeployOptions
	if err := acme.JSONUnmarshal([]byte(acme.EmptyJSON(cfg.ConfigJSON)), &opts); err != nil {
		return opts, fmt.Errorf("解析 SSH 部署配置失败：%w", err)
	}
	opts.Domain = domain
	opts.normalize()
	return opts, nil
}

func genericConfigFromDeployConfig(cfg model.ACMESSHDeployConfig) model.ACMEDeployConfig {
	normalizeDeployConfig(&cfg)
	return model.ACMEDeployConfig{
		ID:       cfg.ID,
		DomainID: cfg.DomainID,
		TargetID: cfg.TargetID,
		Kind:     acme.DeployKindSSH,
		Name:     cfg.Name,
		ConfigJSON: acme.MustJSON(DeployOptions{
			CertPath:      cfg.CertPath,
			KeyPath:       cfg.KeyPath,
			ChainPath:     cfg.ChainPath,
			FullchainPath: cfg.FullchainPath,
			DeployCommand: cfg.DeployCommand,
		}),
		StateJSON:  "{}",
		AutoDeploy: cfg.AutoDeploy,
		Enabled:    cfg.Enabled,
		CreatedAt:  cfg.CreatedAt,
		UpdatedAt:  cfg.UpdatedAt,
	}
}

func deployConfigFromGenericConfig(cfg model.ACMEDeployConfig) model.ACMESSHDeployConfig {
	opts, _ := optionsFromGenericConfig(cfg, "")
	return model.ACMESSHDeployConfig{
		ID:            cfg.ID,
		DomainID:      cfg.DomainID,
		TargetID:      cfg.TargetID,
		Name:          cfg.Name,
		CertPath:      opts.CertPath,
		KeyPath:       opts.KeyPath,
		ChainPath:     opts.ChainPath,
		FullchainPath: opts.FullchainPath,
		DeployCommand: opts.DeployCommand,
		AutoDeploy:    cfg.AutoDeploy,
		Enabled:       cfg.Enabled,
		CreatedAt:     cfg.CreatedAt,
		UpdatedAt:     cfg.UpdatedAt,
	}
}

func splitEndpoint(endpoint string) (string, int, error) {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		return "", 0, errors.New("SSH 主机不能为空")
	}
	host, portText, err := net.SplitHostPort(endpoint)
	if err == nil {
		port, err := strconv.Atoi(portText)
		if err != nil {
			return "", 0, errors.New("SSH 端口无效")
		}
		return host, port, nil
	}
	if strings.Count(endpoint, ":") == 1 {
		parts := strings.Split(endpoint, ":")
		port, err := strconv.Atoi(parts[1])
		if err != nil {
			return "", 0, errors.New("SSH 端口无效")
		}
		return parts[0], port, nil
	}
	return endpoint, 22, nil
}

func targetSummary(t model.ACMESSHTarget) string {
	return fmt.Sprintf("%s@%s:%d", t.Username, t.Host, t.Port)
}

func deployCert(logf func(string, ...any), target model.ACMESSHTarget, cert model.ACMECert, opts DeployOptions) error {
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
	if logf != nil {
		logf("连接 SSH：%s@%s", target.Username, addr)
	}
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
		{label: "cert.pem", path: renderTemplate(opts.CertPath, opts.Domain), data: cert.CertPEM, mode: "0644"},
		{label: "chain.pem", path: renderTemplate(opts.ChainPath, opts.Domain), data: cert.ChainPEM, mode: "0644"},
		{label: "fullchain.pem", path: renderTemplate(opts.FullchainPath, opts.Domain), data: cert.FullchainPEM, mode: "0644"},
		{label: "key.pem", path: renderTemplate(opts.KeyPath, opts.Domain), data: cert.KeyPEM, mode: "0600"},
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
		if logf != nil {
			logf("已写入远端文件：%s", f.path)
		}
	}

	cmd := renderTemplate(opts.DeployCommand, opts.Domain)
	if strings.TrimSpace(cmd) != "" {
		if logf != nil {
			logf("执行部署命令：%s", cmd)
		}
		out, err := runRemoteCommand(client, cmd, nil)
		if logf != nil && strings.TrimSpace(out) != "" {
			logf("命令输出：\n%s", strings.TrimSpace(out))
		}
		if err != nil {
			return fmt.Errorf("部署命令执行失败：%w", err)
		}
	}
	return nil
}

func renderTemplate(s, domain string) string {
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

func normalizeTarget(t *model.ACMESSHTarget) {
	t.Name = strings.TrimSpace(t.Name)
	t.Host = strings.TrimSpace(t.Host)
	t.Username = strings.TrimSpace(t.Username)
	t.AuthType = strings.ToLower(strings.TrimSpace(t.AuthType))
	t.Password = strings.TrimSpace(t.Password)
	t.PrivateKey = strings.TrimSpace(t.PrivateKey)
	t.Passphrase = strings.TrimSpace(t.Passphrase)
	if t.Port <= 0 {
		t.Port = 22
	}
	if t.AuthType == "" {
		t.AuthType = "password"
	}
}

func validateTarget(t model.ACMESSHTarget) error {
	if t.Name == "" {
		return errors.New("目标名称不能为空")
	}
	if t.Host == "" {
		return errors.New("SSH 主机不能为空")
	}
	if t.Username == "" {
		return errors.New("SSH 用户名不能为空")
	}
	if t.Port <= 0 || t.Port > 65535 {
		return errors.New("SSH 端口无效")
	}
	switch t.AuthType {
	case "password":
		if t.Password == "" {
			return errors.New("密码认证需要填写密码")
		}
	case "key":
		if t.PrivateKey == "" {
			return errors.New("证书模式需要填写私钥")
		}
	default:
		return errors.New("认证方式仅支持 password / key")
	}
	return nil
}

func normalizeDeployConfig(c *model.ACMESSHDeployConfig) {
	c.Name = strings.TrimSpace(c.Name)
	c.CertPath = strings.TrimSpace(c.CertPath)
	c.KeyPath = strings.TrimSpace(c.KeyPath)
	c.ChainPath = strings.TrimSpace(c.ChainPath)
	c.FullchainPath = strings.TrimSpace(c.FullchainPath)
	c.DeployCommand = strings.TrimSpace(c.DeployCommand)
}
