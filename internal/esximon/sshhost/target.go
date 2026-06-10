// Package sshhost 是 ESXi 模块自带的 SSH 连接语义。
// 与 internal/upsmon/sshhost 同形但完全独立(操作 esxi_host / esxi_ssh_credential 表),
// 这样 ESXi 可以单独配凭证、单独走跳板机,不与 UPS / ACME 互相牵连。
package sshhost

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/LemonZuo/homer/internal/model"
)

const (
	AuthSourceInline     = "inline"
	AuthSourceCredential = "credential"
)

// TargetAuth esxi_host.auth_json 的反序列化形态。
type TargetAuth struct {
	AuthSource   string `json:"auth_source,omitempty"`
	CredentialID int64  `json:"credential_id,omitempty"`
	Username     string `json:"username,omitempty"`
	AuthType     string `json:"auth_type,omitempty"`
	Password     string `json:"password,omitempty"`
	PrivateKey   string `json:"private_key,omitempty"`
	Passphrase   string `json:"passphrase,omitempty"`
}

// TargetConfig esxi_host.config_json 的反序列化形态。
type TargetConfig struct {
	BastionHostID int64 `json:"bastion_host_id,omitempty"`
}

// Target 解析后扁平化的目标。
type Target struct {
	ID            int64
	Name          string
	Host          string
	Port          int
	AuthSource    string
	CredentialID  int64
	BastionHostID int64
	Username      string
	AuthType      string
	Password      string
	PrivateKey    string
	Passphrase    string
	Enabled       model.BoolFlag
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

func emptyJSON(s string) string {
	if strings.TrimSpace(s) == "" {
		return "{}"
	}
	return s
}

// ParseTarget 把一行 esxi_host 解析成 Target。
func ParseTarget(h model.EsxiHost) (*Target, error) {
	auth := TargetAuth{}
	if err := json.Unmarshal([]byte(emptyJSON(h.AuthJSON)), &auth); err != nil {
		return nil, fmt.Errorf("解析 ESXi 主机认证配置失败:%w", err)
	}
	cfg := TargetConfig{}
	if err := json.Unmarshal([]byte(emptyJSON(h.ConfigJSON)), &cfg); err != nil {
		return nil, fmt.Errorf("解析 ESXi 主机连接配置失败:%w", err)
	}
	host, port, err := SplitEndpoint(h.Endpoint)
	if err != nil {
		return nil, err
	}
	out := &Target{
		ID:            h.ID,
		Name:          h.Name,
		Host:          host,
		Port:          port,
		AuthSource:    auth.AuthSource,
		CredentialID:  auth.CredentialID,
		BastionHostID: cfg.BastionHostID,
		Username:      auth.Username,
		AuthType:      auth.AuthType,
		Password:      auth.Password,
		PrivateKey:    auth.PrivateKey,
		Passphrase:    auth.Passphrase,
		Enabled:       h.Enabled,
		CreatedAt:     h.CreatedAt,
		UpdatedAt:     h.UpdatedAt,
	}
	Normalize(out)
	return out, nil
}

// SplitEndpoint 拆 host:port,缺省 22。
func SplitEndpoint(endpoint string) (string, int, error) {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		return "", 0, errors.New("ESXi 主机不能为空")
	}
	host, portText, err := net.SplitHostPort(endpoint)
	if err == nil {
		port, err := strconv.Atoi(portText)
		if err != nil {
			return "", 0, errors.New("ESXi 端口无效")
		}
		return host, port, nil
	}
	if strings.Count(endpoint, ":") == 1 {
		parts := strings.Split(endpoint, ":")
		port, err := strconv.Atoi(parts[1])
		if err != nil {
			return "", 0, errors.New("ESXi 端口无效")
		}
		return parts[0], port, nil
	}
	return endpoint, 22, nil
}

// Normalize 给 Target 字段去空白、补默认。
func Normalize(t *Target) {
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
		t.AuthSource = AuthSourceInline
	}
	if t.AuthSource != AuthSourceCredential && t.AuthType == "" {
		t.AuthType = "password"
	}
}

// ValidateTarget 在 Upsert 时做表单校验。
func ValidateTarget(t Target) error {
	if t.Name == "" {
		return errors.New("机器名称不能为空")
	}
	if t.Host == "" {
		return errors.New("ESXi 主机不能为空")
	}
	if t.Port <= 0 || t.Port > 65535 {
		return errors.New("ESXi 端口无效")
	}
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

// MarshalAuthJSON 把 Target 的认证字段序列化回 auth_json 形态。
func MarshalAuthJSON(t Target) (string, error) {
	auth := TargetAuth{AuthSource: t.AuthSource}
	if t.AuthSource == AuthSourceCredential {
		auth.CredentialID = t.CredentialID
	} else {
		auth.Username = t.Username
		auth.AuthType = t.AuthType
		auth.Password = t.Password
		auth.PrivateKey = t.PrivateKey
		auth.Passphrase = t.Passphrase
	}
	buf, err := json.Marshal(auth)
	if err != nil {
		return "", err
	}
	return string(buf), nil
}

// MarshalConfigJSON 把 Target 的连接字段序列化回 config_json 形态。
func MarshalConfigJSON(t Target) (string, error) {
	cfg := TargetConfig{BastionHostID: t.BastionHostID}
	buf, err := json.Marshal(cfg)
	if err != nil {
		return "", err
	}
	return string(buf), nil
}

// Summary 调试 / 错误信息里用的简短描述。
func Summary(t Target) string {
	return fmt.Sprintf("%s@%s:%d", t.Username, t.Host, t.Port)
}
