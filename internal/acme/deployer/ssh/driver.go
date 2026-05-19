// Package acmessh 是 ACME 证书的 SSH 部署 driver 实现。
// 与 safeline 子包平行：core（internal/acme）只持有 DeployDriver 接口与通用 store，
// 具体 driver 拆到各自子包，互不影响。
package acmessh

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/LemonZuo/homer/internal/acme"
	"github.com/LemonZuo/homer/internal/acme/deployer/sshlike"
	"github.com/LemonZuo/homer/internal/acme/deployer/sshx"
	"github.com/LemonZuo/homer/internal/model"
	"gorm.io/gorm"
)

// Driver 实现 acme.DeployDriver，把证书写到远端 SSH 机器并执行部署命令。
// db 用于跳板机模式下按 BastionTargetID 反查另一台 SSH 机器。
type Driver struct {
	credentials *acme.SSHCredentialStore
	db          *gorm.DB
}

// TargetAuth 是 acme_deploy_target.auth_json 在 SSH 场景下的结构。
// AuthSource:
//   - ""/"inline"：使用本结构里的 Username + 密码/私钥
//   - "credential"：忽略 inline 字段，运行时按 CredentialID 加载 ssh_credential
type TargetAuth = sshlike.TargetAuth

// AuthSourceCredential 表示按凭证 id 解析认证信息。
const AuthSourceCredential = sshlike.AuthSourceCredential

// TargetConfig 是 acme_deploy_target.config_json 在 SSH 场景下的结构。
// 目前只放跳板机字段，单跳；后续如要扩 keepalive、压缩等连接级参数也放这里。
type TargetConfig = sshlike.TargetConfig

func NewDriver(credentials *acme.SSHCredentialStore, db *gorm.DB) *Driver {
	return &Driver{credentials: credentials, db: db}
}

func (d *Driver) Kind() string  { return acme.DeployKindSSH }
func (d *Driver) Label() string { return "SSH 机器" }

func (d *Driver) ValidateTarget(target model.ACMEDeployTarget) error {
	t, err := targetFromDeployTarget(target)
	if err != nil {
		return err
	}
	if err := sshlike.ValidateTarget(*t, "SSH"); err != nil {
		return err
	}
	return sshlike.ValidateBastion(d.db, *t, "当前机器")
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
	conn, err := d.connFor(t)
	if err != nil {
		return err
	}
	client, cleanup, err := sshx.Dial(nil, conn)
	if err != nil {
		return err
	}
	defer cleanup()
	_ = client // 拨通即视为测试通过
	return nil
}

func (d *Driver) Deploy(_ context.Context, req acme.DeployRequest) (*acme.DeployResult, error) {
	target, err := targetFromDeployTarget(req.Target)
	if err != nil {
		return nil, err
	}
	if err := d.resolveCredential(target); err != nil {
		return nil, err
	}
	opts, err := optionsFromGenericConfig(req.Config, req.Domain.MainDomain)
	if err != nil {
		return nil, err
	}
	if req.Logf != nil {
		req.Logf("SSH 目标：%s", sshlike.Summary(*target))
	}
	if err := d.deployCert(req.Logf, target, req.Cert, opts); err != nil {
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

// targetFromDeployTarget 仅解析 auth_json/config_json，不解析凭证：
// 凭证模式下保留 AuthSource/CredentialID，inline 字段为空。
// 真正建连前（Deploy/TestTarget）再调 resolveCredential 把凭证字段填进去。
func targetFromDeployTarget(target model.ACMEDeployTarget) (*sshlike.Target, error) {
	return sshlike.ParseTarget(target, sshlike.Labels{Auth: "SSH", Config: "SSH", Host: "SSH"})
}

// resolveCredential 在凭证模式下把 ssh_credential 的认证信息覆盖到 target 上。
// inline 模式直接返回。
func (d *Driver) resolveCredential(t *sshlike.Target) error {
	return sshlike.ResolveCredential(d.credentials, t)
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

// connFor 把已解析的 SSH 目标（含凭证、跳板机）翻译成与 model 解耦的 sshx.Conn。
// 跳板机单跳，跳板机本身的 bastion 被忽略。
func (d *Driver) connFor(target *sshlike.Target) (*sshx.Conn, error) {
	return sshlike.ConnFor(target, sshlike.ConnOptions{
		Credentials: d.credentials,
		DB:          d.db,
	})
}

func (d *Driver) deployCert(logf func(string, ...any), target *sshlike.Target, cert model.ACMECert, opts DeployOptions) error {
	opts.normalize()
	if err := opts.validate(); err != nil {
		return err
	}
	conn, err := d.connFor(target)
	if err != nil {
		return err
	}
	client, cleanup, err := sshx.Dial(logf, conn)
	if err != nil {
		return err
	}
	defer cleanup()

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
		if err := sshx.WriteFile(client, f.path, []byte(f.data), f.mode); err != nil {
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
		out, err := sshx.Run(client, cmd, nil)
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
