// Package acmefnos 是 ACME 证书部署到飞牛 OS（fnOS）的 driver 实现。
// 与 ssh / safeline / alicas 子包平行：core（internal/acme）只持有 DeployDriver
// 接口与通用 store，具体 driver 拆到各自子包。
//
// 部署机制（飞牛 OS 上 cert.source='upload' 的证书）：
//  1. SSH 连到 fnOS 主机；
//  2. 在 <ssls_root>/<domain>/ 下选最近的时间戳目录，覆盖其中的 <domain>.crt /
//     <domain>.key（fullchain ≥2 BEGIN CERTIFICATE 才认为是带 chain 的完整证书）；
//  3. psql 更新 trim_connect.cert 行（valid_from / valid_to / last_renew_time /
//     updated_time / issued_by / encrypt_type / status='suc'），按 domain + source='upload'
//     精确匹配，要求 RETURNING 命中且仅命中 1 行；
//  4. systemctl restart 配置的服务（默认 trim_nginx）。
package acmefnos

import (
	"context"
	"crypto/ecdsa"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/LemonZuo/homer/internal/acme"
	acmessh "github.com/LemonZuo/homer/internal/acme/deployer/ssh"
	"github.com/LemonZuo/homer/internal/acme/deployer/sshx"
	"github.com/LemonZuo/homer/internal/model"
	"gorm.io/gorm"
)

// fnOS 部署所需的路径/命令在飞牛 OS 上是固定的，没必要让用户填，全部内置。
const (
	fnosSSLsRoot       = "/usr/trim/var/trim_connect/ssls"
	fnosDBName         = "trim_connect"
	fnosPsqlCmd        = "psql"
	fnosRestartService = "trim_nginx"
)

// Driver 实现 acme.DeployDriver，把证书写入 fnOS 的时间戳目录并刷新 trim_connect。
// 连接复用 SSH 的认证体系：inline 凭证 / ssh_credential 凭证 / SSH target 做跳板，
// 都按 acmessh 的 schema 解析与处理。
type Driver struct {
	credentials *acme.SSHCredentialStore
	db          *gorm.DB
}

// TargetAuth 是 acme_deploy_target.auth_json 在 fnOS 场景下的结构。与 SSH 同构：
//   - ""/"inline"：使用本结构的 Username + 密码/私钥
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

// TargetConfig 是 acme_deploy_target.config_json 在 fnOS 场景下的结构。
// BastionTargetID 指向另一台 SSH 类型的 ACMEDeployTarget；fnOS 只能借用已有的 SSH 跳板。
type TargetConfig struct {
	BastionTargetID int64 `json:"bastion_target_id,omitempty"`
}

// AuthSourceCredential 表示按凭证 id 解析认证信息（与 acmessh 保持一致的常量字面值）。
const AuthSourceCredential = "credential"

// DeployConfig 是 acme_deploy_config.config_json 在 fnOS 场景下的结构。
type DeployConfig struct {
	DomainOverride string `json:"domain_override,omitempty"`
}

func NewDriver(credentials *acme.SSHCredentialStore, db *gorm.DB) *Driver {
	return &Driver{credentials: credentials, db: db}
}

func (d *Driver) Kind() string  { return acme.DeployKindFnOS }
func (d *Driver) Label() string { return "飞牛 OS" }

func (d *Driver) ValidateTarget(target model.ACMEDeployTarget) error {
	t, err := targetFromDeployTarget(target)
	if err != nil {
		return err
	}
	if err := validateTarget(*t); err != nil {
		return err
	}
	if t.BastionTargetID > 0 {
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

func (d *Driver) ValidateConfig(_ model.ACMEDeployTarget, _ model.ACMEDeployConfig) error {
	return nil
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
	// 顺便校验 psql 是否可执行 + ssls 根目录是否存在，提前发现配置错误
	out, err := sshx.Run(client, "command -v "+sshx.ShellQuote(fnosPsqlCmd)+" >/dev/null && test -d "+sshx.ShellQuote(fnosSSLsRoot), nil)
	if err != nil {
		return fmt.Errorf("环境检查失败：%w（输出：%s）", err, strings.TrimSpace(out))
	}
	return nil
}

func (d *Driver) Deploy(_ context.Context, req acme.DeployRequest) (*acme.DeployResult, error) {
	target, err := targetFromDeployTarget(req.Target)
	if err != nil {
		return nil, err
	}
	cfg, err := deployConfigFromGeneric(req.Config)
	if err != nil {
		return nil, err
	}
	domain := strings.TrimSpace(cfg.DomainOverride)
	if domain == "" {
		domain = strings.TrimSpace(req.Domain.MainDomain)
	}
	if domain == "" {
		return nil, errors.New("无法确定要更新的 fnOS 证书域名")
	}
	if strings.TrimSpace(req.Cert.FullchainPEM) == "" || strings.TrimSpace(req.Cert.KeyPEM) == "" {
		return nil, errors.New("当前证书内容不完整，无法部署到 fnOS")
	}
	if countCerts(req.Cert.FullchainPEM) < 2 {
		return nil, errors.New("fullchain 至少需要 2 段 BEGIN CERTIFICATE（cert + 中间证书）")
	}
	issuer, encryptType, err := parseCertMeta(req.Cert.CertPEM)
	if err != nil {
		return nil, err
	}
	validFromMs := req.Cert.NotBefore.UnixMilli()
	validToMs := req.Cert.NotAfter.UnixMilli()

	conn, err := d.connFor(target)
	if err != nil {
		return nil, err
	}
	if req.Logf != nil {
		req.Logf("fnOS 目标：%s@%s:%d，域名：%s", target.Username, target.Host, target.Port, domain)
	}
	client, cleanup, err := sshx.Dial(req.Logf, conn)
	if err != nil {
		return nil, err
	}
	defer cleanup()

	tmpCert := fmt.Sprintf("/tmp/homer-fnos-%d.crt", req.Cert.ID)
	tmpKey := fmt.Sprintf("/tmp/homer-fnos-%d.key", req.Cert.ID)
	if err := sshx.WriteFile(client, tmpCert, []byte(req.Cert.FullchainPEM), "0644"); err != nil {
		return nil, fmt.Errorf("写入临时证书失败：%w", err)
	}
	if err := sshx.WriteFile(client, tmpKey, []byte(req.Cert.KeyPEM), "0600"); err != nil {
		return nil, fmt.Errorf("写入临时私钥失败：%w", err)
	}

	script := buildDeployScript(deployScriptVars{
		Domain:      domain,
		TmpCert:     tmpCert,
		TmpKey:      tmpKey,
		ValidFromMs: validFromMs,
		ValidToMs:   validToMs,
		Issuer:      issuer,
		EncryptType: encryptType,
	})
	if req.Logf != nil {
		req.Logf("执行 fnOS 部署脚本（更新 trim_connect.cert + 重启服务）")
	}
	out, err := sshx.Run(client, "bash -s", []byte(script))
	if req.Logf != nil && strings.TrimSpace(out) != "" {
		req.Logf("脚本输出：\n%s", strings.TrimSpace(out))
	}
	if err != nil {
		return nil, fmt.Errorf("fnOS 部署脚本失败：%w", err)
	}
	return &acme.DeployResult{}, nil
}

// connFor 把已解析的 fnOS 目标（含凭证、跳板机）翻译成 sshx.Conn。
// 跳板机一律来自 SSH target 表，单跳；跳板机自身的 bastion 不再展开。
func (d *Driver) connFor(t *model.ACMEFnOSTarget) (*sshx.Conn, error) {
	if err := d.resolveCredential(t); err != nil {
		return nil, err
	}
	auth, err := sshx.AuthMethod(t.AuthType, t.Password, t.PrivateKey, t.Passphrase)
	if err != nil {
		return nil, err
	}
	conn := &sshx.Conn{Host: t.Host, Port: t.Port, User: t.Username, Auth: auth}
	if t.BastionTargetID <= 0 {
		return conn, nil
	}
	bastion, err := d.loadBastion(t.BastionTargetID)
	if err != nil {
		return nil, err
	}
	if err := acmessh.ResolveCredential(d.credentials, bastion); err != nil {
		return nil, err
	}
	bAuth, err := sshx.AuthMethod(bastion.AuthType, bastion.Password, bastion.PrivateKey, bastion.Passphrase)
	if err != nil {
		return nil, fmt.Errorf("跳板机认证准备失败：%w", err)
	}
	conn.Bastion = &sshx.Conn{Host: bastion.Host, Port: bastion.Port, User: bastion.Username, Auth: bAuth}
	return conn, nil
}

// resolveCredential 在凭证模式下从 ssh_credential 表加载认证信息并填回 target。
// 与 acmessh.ResolveCredential 共享同一份逻辑，避免行为漂移。
func (d *Driver) resolveCredential(t *model.ACMEFnOSTarget) error {
	if t.AuthSource != AuthSourceCredential {
		return nil
	}
	if d.credentials == nil {
		return errors.New("凭证模式未注入 SSHCredentialStore")
	}
	if t.CredentialID <= 0 {
		return errors.New("凭证模式需要选择登录凭证")
	}
	cred, err := d.credentials.Get(t.CredentialID)
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

// loadBastion 按 id 从 deploy_target 表里取一台 SSH 目标当跳板。
// 跳板必须是 SSH 类型、已启用，且自身不再叠 bastion（单跳）。
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
	if row.Kind != acme.DeployKindSSH {
		return nil, fmt.Errorf("跳板机必须是 SSH 类型：id=%d, kind=%s", id, row.Kind)
	}
	if !bool(row.Enabled) {
		return nil, fmt.Errorf("跳板机已停用：%s", row.Name)
	}
	b, err := acmessh.TargetFromDeployTarget(row)
	if err != nil {
		return nil, err
	}
	if b.BastionTargetID > 0 {
		return nil, errors.New("所选跳板机已经设置了自己的跳板机，单跳模式不支持跳板机链")
	}
	return b, nil
}

func countCerts(pem string) int {
	return strings.Count(pem, "BEGIN CERTIFICATE")
}

// parseCertMeta 从叶子证书 PEM 中解析签发者 CN 与公钥算法（RSA/ECC），
// 用于 fnOS UI 显示的 issued_by / encrypt_type 列。
func parseCertMeta(certPEM string) (issuer string, encryptType string, err error) {
	certPEM = strings.TrimSpace(certPEM)
	if certPEM == "" {
		return "", "", errors.New("证书 PEM 为空，无法解析签发者")
	}
	block, _ := pem.Decode([]byte(certPEM))
	if block == nil {
		return "", "", errors.New("证书 PEM 解析失败")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return "", "", fmt.Errorf("解析证书失败：%w", err)
	}
	issuer = strings.TrimSpace(cert.Issuer.CommonName)
	if issuer == "" && len(cert.Issuer.Organization) > 0 {
		issuer = cert.Issuer.Organization[0]
	}
	switch cert.PublicKey.(type) {
	case *rsa.PublicKey:
		encryptType = "RSA"
	case *ecdsa.PublicKey:
		encryptType = "ECC"
	default:
		encryptType = "RSA"
	}
	return issuer, encryptType, nil
}

type deployScriptVars struct {
	Domain      string
	TmpCert     string
	TmpKey      string
	ValidFromMs int64
	ValidToMs   int64
	Issuer      string
	EncryptType string
}

// buildDeployScript 生成在 fnOS 远端执行的 bash 脚本。
// 设计取舍：
//   - 全部通过位置参数传入，避免把 PEM 大文本塞进 here-doc；
//   - 使用 set -euo pipefail + trap 清理临时文件，避免半路失败留垃圾；
//   - psql 用 -tA + RETURNING domain 校验仅命中 1 行，多/少都视为失败。
func buildDeployScript(v deployScriptVars) string {
	const tmpl = `#!/usr/bin/env bash
set -euo pipefail

DOMAIN=%s
SSLS_ROOT=%s
DBNAME=%s
PSQL=%s
RESTART_SVC=%s
TMP_CERT=%s
TMP_KEY=%s
VALID_FROM=%d
VALID_TO=%d
ISSUED_BY=%s
ENCRYPT_TYPE=%s

cleanup() { rm -f "$TMP_CERT" "$TMP_KEY"; }
trap cleanup EXIT

DOMAIN_DIR="$SSLS_ROOT/$DOMAIN"
if [ ! -d "$DOMAIN_DIR" ]; then
  echo "fnOS 上找不到域名目录：$DOMAIN_DIR" >&2
  exit 1
fi
TS_DIR=$(ls -1 "$DOMAIN_DIR" | sort -r | head -n1)
if [ -z "$TS_DIR" ]; then
  echo "$DOMAIN_DIR 下没有时间戳子目录，无法覆盖" >&2
  exit 1
fi
TARGET_DIR="$DOMAIN_DIR/$TS_DIR"
echo "目标目录：$TARGET_DIR"

install -m 0644 "$TMP_CERT" "$TARGET_DIR/$DOMAIN.crt"
install -m 0600 "$TMP_KEY"  "$TARGET_DIR/$DOMAIN.key"
echo "已覆盖 $DOMAIN.crt / $DOMAIN.key"

NOW_MS=$(($(date +%%s%%N)/1000000))
SQL="UPDATE cert SET valid_from=$VALID_FROM, valid_to=$VALID_TO, last_renew_time=$NOW_MS, updated_time=$NOW_MS, issued_by='$ISSUED_BY', encrypt_type='$ENCRYPT_TYPE', status='suc' WHERE domain='$DOMAIN' AND source='upload' RETURNING domain;"
HITS=$(sudo -u postgres "$PSQL" -d "$DBNAME" -tA -c "$SQL" | wc -l)
if [ "$HITS" -ne 1 ]; then
  echo "psql 更新行数不为 1（实际 $HITS），请确认 cert 表里存在 domain='$DOMAIN' AND source='upload' 的行" >&2
  exit 1
fi
echo "trim_connect.cert 已更新（1 行）"

echo "重启 $RESTART_SVC"
sudo systemctl restart "$RESTART_SVC"
echo "fnOS 部署完成"
`
	return fmt.Sprintf(tmpl,
		sshx.ShellQuote(v.Domain),
		sshx.ShellQuote(fnosSSLsRoot),
		sshx.ShellQuote(fnosDBName),
		sshx.ShellQuote(fnosPsqlCmd),
		sshx.ShellQuote(fnosRestartService),
		sshx.ShellQuote(v.TmpCert),
		sshx.ShellQuote(v.TmpKey),
		v.ValidFromMs,
		v.ValidToMs,
		sshx.ShellQuote(sqlEscape(v.Issuer)),
		sshx.ShellQuote(sqlEscape(v.EncryptType)),
	)
}

// sqlEscape 处理 SQL 字面量里的单引号，配合外层 '$VAR' 拼接安全。
// issuer/encrypt_type 取自 x509 解析的字段，正常不会含特殊字符，加一层保险。
func sqlEscape(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}

func targetFromDeployTarget(target model.ACMEDeployTarget) (*model.ACMEFnOSTarget, error) {
	auth := TargetAuth{}
	if err := acme.JSONUnmarshal([]byte(acme.EmptyJSON(target.AuthJSON)), &auth); err != nil {
		return nil, fmt.Errorf("解析 fnOS 认证配置失败：%w", err)
	}
	cfg := TargetConfig{}
	if err := acme.JSONUnmarshal([]byte(acme.EmptyJSON(target.ConfigJSON)), &cfg); err != nil {
		return nil, fmt.Errorf("解析 fnOS 连接配置失败：%w", err)
	}
	host, port, err := splitEndpoint(target.Endpoint)
	if err != nil {
		return nil, err
	}
	out := &model.ACMEFnOSTarget{
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

func deployTargetFromTarget(t model.ACMEFnOSTarget) model.ACMEDeployTarget {
	normalizeTarget(&t)
	auth := TargetAuth{
		AuthSource:   t.AuthSource,
		CredentialID: t.CredentialID,
	}
	if t.AuthSource == AuthSourceCredential {
		// 凭证模式不持久化 inline 字段，避免脱节的脏数据
		auth.Username = ""
		auth.AuthType = ""
		auth.Password = ""
		auth.PrivateKey = ""
		auth.Passphrase = ""
	} else {
		auth.Username = t.Username
		auth.AuthType = t.AuthType
		auth.Password = t.Password
		auth.PrivateKey = t.PrivateKey
		auth.Passphrase = t.Passphrase
	}
	cfg := TargetConfig{BastionTargetID: t.BastionTargetID}
	return model.ACMEDeployTarget{
		ID:         t.ID,
		Name:       t.Name,
		Kind:       acme.DeployKindFnOS,
		Endpoint:   net.JoinHostPort(t.Host, strconv.Itoa(t.Port)),
		AuthJSON:   acme.MustJSON(auth),
		ConfigJSON: acme.MustJSON(cfg),
		Enabled:    t.Enabled,
		CreatedAt:  t.CreatedAt,
		UpdatedAt:  t.UpdatedAt,
	}
}

func deployConfigFromGeneric(cfg model.ACMEDeployConfig) (DeployConfig, error) {
	out := DeployConfig{}
	if err := acme.JSONUnmarshal([]byte(acme.EmptyJSON(cfg.ConfigJSON)), &out); err != nil {
		return out, fmt.Errorf("解析 fnOS 部署配置失败：%w", err)
	}
	return out, nil
}

func genericConfigFromDeployConfig(cfg model.ACMEFnOSDeployConfig) model.ACMEDeployConfig {
	normalizeDeployConfig(&cfg)
	return model.ACMEDeployConfig{
		ID:         cfg.ID,
		DomainID:   cfg.DomainID,
		TargetID:   cfg.TargetID,
		Kind:       acme.DeployKindFnOS,
		Name:       cfg.Name,
		ConfigJSON: acme.MustJSON(DeployConfig{DomainOverride: cfg.DomainOverride}),
		StateJSON:  "{}",
		AutoDeploy: cfg.AutoDeploy,
		Enabled:    cfg.Enabled,
		CreatedAt:  cfg.CreatedAt,
		UpdatedAt:  cfg.UpdatedAt,
	}
}

func deployConfigViewFromGeneric(cfg model.ACMEDeployConfig) model.ACMEFnOSDeployConfig {
	parsed, _ := deployConfigFromGeneric(cfg)
	return model.ACMEFnOSDeployConfig{
		ID:             cfg.ID,
		DomainID:       cfg.DomainID,
		TargetID:       cfg.TargetID,
		Name:           cfg.Name,
		DomainOverride: parsed.DomainOverride,
		AutoDeploy:     cfg.AutoDeploy,
		Enabled:        cfg.Enabled,
		CreatedAt:      cfg.CreatedAt,
		UpdatedAt:      cfg.UpdatedAt,
	}
}

func splitEndpoint(endpoint string) (string, int, error) {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		return "", 0, errors.New("fnOS 主机不能为空")
	}
	host, portText, err := net.SplitHostPort(endpoint)
	if err == nil {
		port, err := strconv.Atoi(portText)
		if err != nil {
			return "", 0, errors.New("fnOS 端口无效")
		}
		return host, port, nil
	}
	if strings.Count(endpoint, ":") == 1 {
		parts := strings.Split(endpoint, ":")
		port, err := strconv.Atoi(parts[1])
		if err != nil {
			return "", 0, errors.New("fnOS 端口无效")
		}
		return parts[0], port, nil
	}
	return endpoint, 22, nil
}

func normalizeTarget(t *model.ACMEFnOSTarget) {
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

func validateTarget(t model.ACMEFnOSTarget) error {
	if t.Name == "" {
		return errors.New("目标名称不能为空")
	}
	if t.Host == "" {
		return errors.New("fnOS 主机不能为空")
	}
	if t.Port <= 0 || t.Port > 65535 {
		return errors.New("fnOS 端口无效")
	}
	// 凭证模式：用户名/密码/私钥都在 ssh_credential 上，这里只确认引用了一个有效凭证 id。
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

func normalizeDeployConfig(c *model.ACMEFnOSDeployConfig) {
	c.Name = strings.TrimSpace(c.Name)
	c.DomainOverride = strings.ToLower(strings.TrimSpace(c.DomainOverride))
}
