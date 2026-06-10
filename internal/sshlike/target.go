package sshlike

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"
)

const (
	AuthSourceInline     = "inline"
	AuthSourceCredential = "credential"
)

// TargetAuth 是三家 (acme/ups/esxi) auth_json 完全共用的 JSON 形态。
type TargetAuth struct {
	AuthSource   string `json:"auth_source,omitempty"`
	CredentialID int64  `json:"credential_id,omitempty"`
	Username     string `json:"username,omitempty"`
	AuthType     string `json:"auth_type,omitempty"`
	Password     string `json:"password,omitempty"`
	PrivateKey   string `json:"private_key,omitempty"`
	Passphrase   string `json:"passphrase,omitempty"`
}

// TargetConfig 是三家 config_json 收口后的 JSON 形态(只含 bastion_id 一个字段)。
// 历史命名 bastion_target_id / bastion_host_id 已通过 sql/14_bastion_key_rename.sql 迁移。
type TargetConfig struct {
	BastionID int64 `json:"bastion_id,omitempty"`
}

// Target 是模块无关的扁平 SSH 目标。各模块的 ParseXxxTarget 适配器把自己的
// GORM row(ACMEDeployTarget / UPSHost / EsxiHost)转成这个结构。
type Target struct {
	ID           int64
	Name         string
	Host         string
	Port         int
	AuthSource   string
	CredentialID int64
	BastionID    int64
	Username     string
	AuthType     string
	Password     string
	PrivateKey   string
	Passphrase   string
	Enabled      bool
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// SplitEndpoint 拆 "host:port" 字符串,缺端口回退 22。
// hostLabel 用于错误消息("SSH 主机" / "UPS 主机" / "ESXi 主机")。
func SplitEndpoint(endpoint, hostLabel string) (string, int, error) {
	hostLabel = strings.TrimSpace(hostLabel)
	if hostLabel == "" {
		hostLabel = "SSH"
	}
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		return "", 0, fmt.Errorf("%s 主机不能为空", hostLabel)
	}
	host, portText, err := net.SplitHostPort(endpoint)
	if err == nil {
		port, err := strconv.Atoi(portText)
		if err != nil {
			return "", 0, fmt.Errorf("%s 端口无效", hostLabel)
		}
		return host, port, nil
	}
	if strings.Count(endpoint, ":") == 1 {
		parts := strings.Split(endpoint, ":")
		port, err := strconv.Atoi(parts[1])
		if err != nil {
			return "", 0, fmt.Errorf("%s 端口无效", hostLabel)
		}
		return parts[0], port, nil
	}
	return endpoint, 22, nil
}

// Normalize 给 Target 字段去空白、补默认值。
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

// ValidateTarget 用于 Upsert 前的表单校验。hostLabel 进错误消息。
func ValidateTarget(t Target, hostLabel string) error {
	hostLabel = strings.TrimSpace(hostLabel)
	if hostLabel == "" {
		hostLabel = "SSH"
	}
	if t.Name == "" {
		return errors.New("目标名称不能为空")
	}
	if t.Host == "" {
		return fmt.Errorf("%s 主机不能为空", hostLabel)
	}
	if t.Port <= 0 || t.Port > 65535 {
		return fmt.Errorf("%s 端口无效", hostLabel)
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

// Summary 调试 / 错误信息里的简短描述。
func Summary(t Target) string {
	return fmt.Sprintf("%s@%s:%d", t.Username, t.Host, t.Port)
}

// MarshalAuthJSON 把 Target 的认证字段序列化回 auth_json 形态。
// 凭证模式只序列化 credential_id,inline 模式序列化用户名/密码/密钥。
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
	cfg := TargetConfig{BastionID: t.BastionID}
	buf, err := json.Marshal(cfg)
	if err != nil {
		return "", err
	}
	return string(buf), nil
}

// UnmarshalAuthJSON 把 auth_json 字符串反序列化成 TargetAuth(空串视为 "{}").
// 各模块适配器调用这个,而不是自己写一遍 json.Unmarshal。
func UnmarshalAuthJSON(s string) (TargetAuth, error) {
	var a TargetAuth
	if err := json.Unmarshal([]byte(emptyJSON(s)), &a); err != nil {
		return TargetAuth{}, err
	}
	return a, nil
}

// UnmarshalConfigJSON 同上。
func UnmarshalConfigJSON(s string) (TargetConfig, error) {
	var c TargetConfig
	if err := json.Unmarshal([]byte(emptyJSON(s)), &c); err != nil {
		return TargetConfig{}, err
	}
	return c, nil
}

func emptyJSON(s string) string {
	if strings.TrimSpace(s) == "" {
		return "{}"
	}
	return s
}
