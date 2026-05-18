// Package acmessh 是 ACME 证书的 SSH 部署 driver 实现。
// 与 safeline 子包平行：core（internal/acme）只持有 DeployDriver 接口与通用 store，
// 具体 driver 拆到各自子包，互不影响。
package acmessh

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/LemonZuo/homer/internal/acme"
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
type TargetAuth struct {
	AuthSource   string `json:"auth_source,omitempty"`
	CredentialID int64  `json:"credential_id,omitempty"`
	Username     string `json:"username,omitempty"`
	AuthType     string `json:"auth_type,omitempty"`
	Password     string `json:"password,omitempty"`
	PrivateKey   string `json:"private_key,omitempty"`
	Passphrase   string `json:"passphrase,omitempty"`
}

// AuthSourceCredential 表示按凭证 id 解析认证信息。
const AuthSourceCredential = "credential"

// TargetConfig 是 acme_deploy_target.config_json 在 SSH 场景下的结构。
// 目前只放跳板机字段，单跳；后续如要扩 keepalive、压缩等连接级参数也放这里。
type TargetConfig struct {
	BastionTargetID int64 `json:"bastion_target_id,omitempty"`
}

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
	if err := validateTarget(*t); err != nil {
		return err
	}
	if t.BastionTargetID > 0 {
		if t.BastionTargetID == t.ID {
			return errors.New("跳板机不能是自己")
		}
		// 上游：已被别人当跳板的机器，自身不能再有跳板，否则单跳被绕成链
		if t.ID > 0 {
			if name, ok, err := d.findUpstreamRef(t.ID); err != nil {
				return err
			} else if ok {
				return fmt.Errorf("当前机器已被 %s 设为跳板机，不能再为自己设置跳板机", name)
			}
		}
		// 下游：所选跳板自身不能再有跳板
		b, err := d.loadBastion(t.BastionTargetID)
		if err != nil {
			return err
		}
		if b.BastionTargetID > 0 {
			return errors.New("所选跳板机已经设置了自己的跳板机，单跳模式不支持跳板机链")
		}
	}
	return nil
}

// findUpstreamRef 反查是否有任何 SSH/fnOS target 把 id 当作 bastion 在用。
// 返回第一个引用者的 name，用于错误提示。
func (d *Driver) findUpstreamRef(id int64) (string, bool, error) {
	if d.db == nil {
		return "", false, errors.New("跳板机模式未注入 DB")
	}
	var rows []model.ACMEDeployTarget
	if err := d.db.Where("kind IN ? AND id <> ?", []string{acme.DeployKindSSH, acme.DeployKindFnOS}, id).Find(&rows).Error; err != nil {
		return "", false, fmt.Errorf("扫描跳板机引用失败：%w", err)
	}
	for _, r := range rows {
		cfg := TargetConfig{}
		if err := acme.JSONUnmarshal([]byte(acme.EmptyJSON(r.ConfigJSON)), &cfg); err != nil {
			continue
		}
		if cfg.BastionTargetID == id {
			return r.Name, true, nil
		}
	}
	return "", false, nil
}

// loadBastion 加载一台被引用作为跳板机的 SSH/fnOS 目标，校验类型/启用状态，
// 但不解析凭证（建连前再 resolveCredential）。
func (d *Driver) loadBastion(id int64) (*model.ACMESSHTarget, error) {
	if d.db == nil {
		return nil, errors.New("跳板机模式未注入 DB")
	}
	var row model.ACMEDeployTarget
	if err := d.db.First(&row, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("跳板机不存在：id=%d", id)
		}
		return nil, fmt.Errorf("加载跳板机失败：%w", err)
	}
	if row.Kind != acme.DeployKindSSH && row.Kind != acme.DeployKindFnOS {
		return nil, fmt.Errorf("跳板机必须是 SSH 或 fnOS 类型：id=%d, kind=%s", id, row.Kind)
	}
	if !bool(row.Enabled) {
		return nil, fmt.Errorf("跳板机已停用：%s", row.Name)
	}
	return targetFromDeployTarget(row)
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
		req.Logf("SSH 目标：%s", targetSummary(*target))
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

// targetFromDeployTarget 仅做 auth_json 解析，不解析凭证：
// UI 列表/详情走这里，凭证模式下保留 AuthSource/CredentialID，inline 字段为空。
// 真正建连前（Deploy/TestTarget）再调 resolveCredential 把凭证字段填进去。
func targetFromDeployTarget(target model.ACMEDeployTarget) (*model.ACMESSHTarget, error) {
	auth := TargetAuth{}
	if err := acme.JSONUnmarshal([]byte(acme.EmptyJSON(target.AuthJSON)), &auth); err != nil {
		return nil, fmt.Errorf("解析 SSH 认证配置失败：%w", err)
	}
	cfg := TargetConfig{}
	if err := acme.JSONUnmarshal([]byte(acme.EmptyJSON(target.ConfigJSON)), &cfg); err != nil {
		return nil, fmt.Errorf("解析 SSH 连接配置失败：%w", err)
	}
	host, port, err := splitEndpoint(target.Endpoint)
	if err != nil {
		return nil, err
	}
	out := &model.ACMESSHTarget{
		ID:              target.ID,
		Name:            target.Name,
		Host:            host,
		Port:            port,
		AuthSource:      auth.AuthSource,
		CredentialID:    auth.CredentialID,
		BastionTargetID: cfg.BastionTargetID,
		Username:        auth.Username,
		AuthType:        auth.AuthType,
		Password:        auth.Password,
		PrivateKey:      auth.PrivateKey,
		Passphrase:      auth.Passphrase,
		Enabled:         target.Enabled,
		CreatedAt:       target.CreatedAt,
		UpdatedAt:       target.UpdatedAt,
	}
	normalizeTarget(out)
	return out, nil
}

// resolveCredential 在凭证模式下把 ssh_credential 的认证信息覆盖到 target 上。
// inline 模式直接返回。
func (d *Driver) resolveCredential(t *model.ACMESSHTarget) error {
	return ResolveCredential(d.credentials, t)
}

// ResolveCredential 是 resolveCredential 的包外暴露形式，供其它 SSH 复用方
// （例如 fnos driver 把 SSH target 当跳板机）调用。inline 模式直接返回。
func ResolveCredential(credentials *acme.SSHCredentialStore, t *model.ACMESSHTarget) error {
	if t.AuthSource != AuthSourceCredential {
		return nil
	}
	if credentials == nil {
		return errors.New("凭证模式未注入 SSHCredentialStore")
	}
	if t.CredentialID <= 0 {
		return errors.New("凭证模式需要选择登录凭证")
	}
	cred, err := credentials.Get(t.CredentialID)
	if err != nil {
		return fmt.Errorf("加载 SSH 登录凭证失败：%w", err)
	}
	t.Username = strings.TrimSpace(cred.Username)
	t.AuthType = strings.ToLower(strings.TrimSpace(cred.AuthType))
	t.Password = strings.TrimSpace(cred.Password)
	t.PrivateKey = strings.TrimSpace(cred.PrivateKey)
	t.Passphrase = strings.TrimSpace(cred.Passphrase)
	if t.AuthType == "" {
		t.AuthType = "password"
	}
	return nil
}

// TargetFromDeployTarget 是 targetFromDeployTarget 的包外暴露形式，
// 供其它 driver（例如 fnos）把现成的 SSH target 解析成 ACMESSHTarget 视图。
func TargetFromDeployTarget(target model.ACMEDeployTarget) (*model.ACMESSHTarget, error) {
	return targetFromDeployTarget(target)
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

// connFor 把已解析的 SSH 目标（含凭证、跳板机）翻译成与 model 解耦的 sshx.Conn。
// 跳板机单跳，跳板机本身的 bastion 被忽略。
func (d *Driver) connFor(target *model.ACMESSHTarget) (*sshx.Conn, error) {
	if err := d.resolveCredential(target); err != nil {
		return nil, err
	}
	auth, err := sshx.AuthMethod(target.AuthType, target.Password, target.PrivateKey, target.Passphrase)
	if err != nil {
		return nil, err
	}
	conn := &sshx.Conn{Host: target.Host, Port: target.Port, User: target.Username, Auth: auth}
	if target.BastionTargetID <= 0 {
		return conn, nil
	}
	bastion, err := d.loadBastion(target.BastionTargetID)
	if err != nil {
		return nil, err
	}
	if err := d.resolveCredential(bastion); err != nil {
		return nil, err
	}
	bAuth, err := sshx.AuthMethod(bastion.AuthType, bastion.Password, bastion.PrivateKey, bastion.Passphrase)
	if err != nil {
		return nil, fmt.Errorf("跳板机认证准备失败：%w", err)
	}
	conn.Bastion = &sshx.Conn{Host: bastion.Host, Port: bastion.Port, User: bastion.Username, Auth: bAuth}
	return conn, nil
}

func (d *Driver) deployCert(logf func(string, ...any), target *model.ACMESSHTarget, cert model.ACMECert, opts DeployOptions) error {
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

func normalizeTarget(t *model.ACMESSHTarget) {
	t.Name = strings.TrimSpace(t.Name)
	t.Host = strings.TrimSpace(t.Host)
	t.AuthSource = strings.ToLower(strings.TrimSpace(t.AuthSource))
	t.Username = strings.TrimSpace(t.Username)
	t.AuthType = strings.ToLower(strings.TrimSpace(t.AuthType))
	t.Password = strings.TrimSpace(t.Password)
	t.PrivateKey = strings.TrimSpace(t.PrivateKey)
	t.Passphrase = strings.TrimSpace(t.Passphrase)
	if t.Port <= 0 {
		t.Port = 22
	}
	if t.AuthSource == "" {
		t.AuthSource = "inline"
	}
	if t.AuthSource != AuthSourceCredential && t.AuthType == "" {
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
	if t.Port <= 0 || t.Port > 65535 {
		return errors.New("SSH 端口无效")
	}
	// 凭证模式：用户名/密码/私钥都在 ssh_credential 上（且建凭证时已校验），
	// 这里只需确认引用了一个有效凭证 id。
	if t.AuthSource == AuthSourceCredential {
		if t.CredentialID <= 0 {
			return errors.New("凭证模式需要选择登录凭证")
		}
		return nil
	}
	if t.Username == "" {
		return errors.New("SSH 用户名不能为空")
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

