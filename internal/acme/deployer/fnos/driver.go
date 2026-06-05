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
//  4. systemctl restart 配置的服务（trim_nginx / webdav / smbftpd），未安装的服务跳过不阻断。
package acmefnos

import (
	"context"
	"crypto/ecdsa"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"strings"

	"github.com/LemonZuo/homer/internal/acme"
	"github.com/LemonZuo/homer/internal/acme/deployer/sshlike"
	"github.com/LemonZuo/homer/internal/acme/deployer/sshx"
	"github.com/LemonZuo/homer/internal/model"
	"golang.org/x/crypto/ssh"
	"gorm.io/gorm"
)

// fnOS 部署所需的路径/命令在飞牛 OS 上是固定的，没必要让用户填，全部内置。
const (
	fnosSSLsRoot = "/usr/trim/var/trim_connect/ssls"
	fnosDBName   = "trim_connect"
	fnosPsqlCmd  = "psql"
)

// fnosRestartServices 是部署证书后需要重启的服务列表。
// trim_nginx 一定有（面板入口）；webdav / smbftpd 在没启用对应功能的 fnOS 上 systemctl 会失败，
// 脚本里按 best-effort 处理，单个失败不阻断整个部署。
var fnosRestartServices = []string{"trim_nginx", "webdav", "smbftpd"}

// Driver 实现 acme.DeployDriver，把证书写入 fnOS 的时间戳目录并刷新 trim_connect。
// 连接复用 SSH 的认证体系：inline 凭证 / ssh_credential 凭证 / SSH 或 fnOS target 做跳板，
// 都按 sshlike 的 schema 解析与处理。
type Driver struct {
	credentials *acme.SSHCredentialStore
	db          *gorm.DB
}

// TargetAuth 是 acme_deploy_target.auth_json 在 fnOS 场景下的结构。与 SSH 同构：
//   - ""/"inline"：使用本结构的 Username + 密码/私钥
//   - "credential"：忽略 inline 字段，运行时按 CredentialID 加载 ssh_credential
type TargetAuth = sshlike.TargetAuth

// TargetConfig 是 acme_deploy_target.config_json 在 fnOS 场景下的结构。
// BastionTargetID 指向另一台 SSH/fnOS 类型的 ACMEDeployTarget，单跳。
type TargetConfig = sshlike.TargetConfig

// AuthSourceCredential 表示按凭证 id 解析认证信息（与 SSH driver 保持一致的常量字面值）。
const AuthSourceCredential = sshlike.AuthSourceCredential

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
	if err := sshlike.ValidateTarget(*t, "fnOS"); err != nil {
		return err
	}
	return sshlike.ValidateBastion(d.db, *t, "当前实例")
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
	target, args, err := d.prepareDeployArgs(req)
	if err != nil {
		return nil, err
	}

	conn, err := d.connFor(target)
	if err != nil {
		return nil, err
	}
	if req.Logf != nil {
		req.Logf("fnOS 目标：%s@%s:%d，域名：%s", target.Username, target.Host, target.Port, args.Domain)
	}
	client, cleanup, err := sshx.Dial(req.Logf, conn)
	if err != nil {
		return nil, err
	}
	defer cleanup()

	if err := uploadTempFiles(client, req.Cert); err != nil {
		return nil, err
	}
	if err := runDeployScript(client, req.Logf, args); err != nil {
		return nil, err
	}
	return &acme.DeployResult{}, nil
}

// prepareDeployArgs 解析 target/config，做证书完整性校验，组装脚本所需的所有参数。
func (d *Driver) prepareDeployArgs(req acme.DeployRequest) (*sshlike.Target, deployScriptVars, error) {
	var args deployScriptVars
	target, err := targetFromDeployTarget(req.Target)
	if err != nil {
		return nil, args, err
	}
	cfg, err := deployConfigFromGeneric(req.Config)
	if err != nil {
		return nil, args, err
	}
	domain := strings.TrimSpace(cfg.DomainOverride)
	if domain == "" {
		domain = strings.TrimSpace(req.Domain.MainDomain)
	}
	if domain == "" {
		return nil, args, errors.New("无法确定要更新的 fnOS 证书域名")
	}
	if strings.TrimSpace(req.Cert.FullchainPEM) == "" || strings.TrimSpace(req.Cert.KeyPEM) == "" {
		return nil, args, errors.New("当前证书内容不完整，无法部署到 fnOS")
	}
	if countCerts(req.Cert.FullchainPEM) < 2 {
		return nil, args, errors.New("fullchain 至少需要 2 段 BEGIN CERTIFICATE（cert + 中间证书）")
	}
	issuer, encryptType, err := parseCertMeta(req.Cert.CertPEM)
	if err != nil {
		return nil, args, err
	}
	args = deployScriptVars{
		Domain:      domain,
		TmpCert:     fmt.Sprintf("/tmp/homer-fnos-%d.crt", req.Cert.ID),
		TmpKey:      fmt.Sprintf("/tmp/homer-fnos-%d.key", req.Cert.ID),
		ValidFromMs: req.Cert.NotBefore.UnixMilli(),
		ValidToMs:   req.Cert.NotAfter.UnixMilli(),
		Issuer:      issuer,
		EncryptType: encryptType,
	}
	return target, args, nil
}

// uploadTempFiles 把 fullchain + key 写到远端 /tmp，供后续脚本读取。
func uploadTempFiles(client *ssh.Client, cert model.ACMECert) error {
	tmpCert := fmt.Sprintf("/tmp/homer-fnos-%d.crt", cert.ID)
	tmpKey := fmt.Sprintf("/tmp/homer-fnos-%d.key", cert.ID)
	if err := sshx.WriteFile(client, tmpCert, []byte(cert.FullchainPEM), "0644"); err != nil {
		return fmt.Errorf("写入临时证书失败：%w", err)
	}
	if err := sshx.WriteFile(client, tmpKey, []byte(cert.KeyPEM), "0600"); err != nil {
		return fmt.Errorf("写入临时私钥失败：%w", err)
	}
	return nil
}

// runDeployScript 在远端跑 bash 脚本，更新 trim_connect.cert + 重启服务。
func runDeployScript(client *ssh.Client, logf func(string, ...any), args deployScriptVars) error {
	script := buildDeployScript(args)
	if logf != nil {
		logf("执行 fnOS 部署脚本（更新 trim_connect.cert + 重启服务）")
	}
	out, err := sshx.Run(client, "bash -s", []byte(script))
	if logf != nil && strings.TrimSpace(out) != "" {
		logf("脚本输出：\n%s", strings.TrimSpace(out))
	}
	if err != nil {
		return fmt.Errorf("fnOS 部署脚本失败：%w", err)
	}
	return nil
}

// connFor 把已解析的 fnOS 目标（含凭证、跳板机）翻译成 sshx.Conn。
// 跳板机来自 SSH/fnOS target 表，单跳；跳板机自身的 bastion 不再展开。
func (d *Driver) connFor(t *sshlike.Target) (*sshx.Conn, error) {
	return sshlike.ConnFor(t, sshlike.ConnOptions{
		Credentials:        d.credentials,
		DB:                 d.db,
		RejectBastionChain: true,
	})
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
//   - psql 用 CTE 返回真实 UPDATE 命中行数，避免 RETURNING 的 command tag 干扰统计。
func buildDeployScript(v deployScriptVars) string {
	const tmpl = `#!/usr/bin/env bash
set -euo pipefail

DOMAIN=%s
SSLS_ROOT=%s
DBNAME=%s
PSQL=%s
RESTART_SVCS=%s
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
TS_DIR=""
for dir in "$DOMAIN_DIR"/*/; do
  [ -d "$dir" ] || continue
  name=${dir%%/}
  name=${name##*/}
  if [[ "$name" =~ ^[0-9]{10,}$ ]]; then
    if [ -z "$TS_DIR" ] || [ "$name" -gt "$TS_DIR" ]; then
      TS_DIR="$name"
    fi
  fi
done
if [ -z "$TS_DIR" ]; then
  echo "$DOMAIN_DIR 下没有 10 位以上数字时间戳子目录，无法覆盖" >&2
  exit 1
fi
TARGET_DIR="$DOMAIN_DIR/$TS_DIR"
echo "域名目录：$DOMAIN_DIR"
echo "时间戳目录：$TS_DIR"
echo "目标目录：$TARGET_DIR"

# key 用 0644:fnOS trim_nginx worker 不是 root,0600 会导致加载失败,SNI 落回 fallback 自签证书
install -m 0644 "$TMP_CERT" "$TARGET_DIR/$DOMAIN.crt"
install -m 0644 "$TMP_KEY"  "$TARGET_DIR/$DOMAIN.key"
echo "已覆盖 $DOMAIN.crt / $DOMAIN.key"

NOW_MS=$(($(date +%%s%%N)/1000000))
SQL="WITH u AS (UPDATE cert SET valid_from=$VALID_FROM, valid_to=$VALID_TO, last_renew_time=$NOW_MS, updated_time=$NOW_MS, issued_by='$ISSUED_BY', encrypt_type='$ENCRYPT_TYPE', status='suc' WHERE domain='$DOMAIN' AND source='upload' RETURNING 1) SELECT count(*) FROM u;"
HITS=$(sudo -u postgres "$PSQL" -d "$DBNAME" -v ON_ERROR_STOP=1 -tA -c "$SQL" | tr -d '[:space:]')
if [ "$HITS" != "1" ]; then
  echo "psql 更新行数不为 1（实际 $HITS），请确认 cert 表里存在 domain='$DOMAIN' AND source='upload' 的行" >&2
  exit 1
fi
echo "trim_connect.cert 已更新（1 行）"

for svc in $RESTART_SVCS; do
  echo "重启 $svc"
  if ! sudo systemctl restart "$svc"; then
    echo "重启 $svc 失败,继续(可能未启用该服务)" >&2
  fi
done
echo "fnOS 部署完成"
`
	return fmt.Sprintf(tmpl,
		sshx.ShellQuote(v.Domain),
		sshx.ShellQuote(fnosSSLsRoot),
		sshx.ShellQuote(fnosDBName),
		sshx.ShellQuote(fnosPsqlCmd),
		sshx.ShellQuote(strings.Join(fnosRestartServices, " ")),
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

func targetFromDeployTarget(target model.ACMEDeployTarget) (*sshlike.Target, error) {
	return sshlike.ParseTarget(target, sshlike.Labels{Auth: "fnOS", Config: "fnOS", Host: "fnOS"})
}

func deployConfigFromGeneric(cfg model.ACMEDeployConfig) (DeployConfig, error) {
	out := DeployConfig{}
	if err := acme.JSONUnmarshal([]byte(acme.EmptyJSON(cfg.ConfigJSON)), &out); err != nil {
		return out, fmt.Errorf("解析 fnOS 部署配置失败：%w", err)
	}
	return out, nil
}
