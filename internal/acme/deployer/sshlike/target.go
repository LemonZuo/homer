// Package sshlike 处理 ACME 部署目标里的 SSH-like 连接语义：
// auth_json/config_json 解析、凭证模式、单跳跳板机校验与 sshx.Conn 构造。
package sshlike

import (
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/LemonZuo/homer/internal/acme"
	"github.com/LemonZuo/homer/internal/model"
)

const (
	AuthSourceInline     = "inline"
	AuthSourceCredential = "credential"
)

type TargetAuth struct {
	AuthSource   string `json:"auth_source,omitempty"`
	CredentialID int64  `json:"credential_id,omitempty"`
	Username     string `json:"username,omitempty"`
	AuthType     string `json:"auth_type,omitempty"`
	Password     string `json:"password,omitempty"`
	PrivateKey   string `json:"private_key,omitempty"`
	Passphrase   string `json:"passphrase,omitempty"`
}

type TargetConfig struct {
	BastionTargetID int64 `json:"bastion_target_id,omitempty"`
}

type Target struct {
	ID              int64
	Name            string
	Host            string
	Port            int
	AuthSource      string
	CredentialID    int64
	BastionTargetID int64
	Username        string
	AuthType        string
	Password        string
	PrivateKey      string
	Passphrase      string
	Enabled         model.BoolFlag
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type Labels struct {
	Auth   string
	Config string
	Host   string
}

func (l Labels) withDefaults() Labels {
	if strings.TrimSpace(l.Auth) == "" {
		l.Auth = "SSH"
	}
	if strings.TrimSpace(l.Config) == "" {
		l.Config = l.Auth
	}
	if strings.TrimSpace(l.Host) == "" {
		l.Host = l.Auth
	}
	return l
}

func ParseTarget(target model.ACMEDeployTarget, labels Labels) (*Target, error) {
	labels = labels.withDefaults()
	auth := TargetAuth{}
	if err := acme.JSONUnmarshal([]byte(acme.EmptyJSON(target.AuthJSON)), &auth); err != nil {
		return nil, fmt.Errorf("解析 %s 认证配置失败：%w", labels.Auth, err)
	}
	cfg := TargetConfig{}
	if err := acme.JSONUnmarshal([]byte(acme.EmptyJSON(target.ConfigJSON)), &cfg); err != nil {
		return nil, fmt.Errorf("解析 %s 连接配置失败：%w", labels.Config, err)
	}
	host, port, err := SplitEndpoint(target.Endpoint, labels.Host)
	if err != nil {
		return nil, err
	}
	out := &Target{
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
	Normalize(out)
	return out, nil
}

func SplitEndpoint(endpoint string, hostLabel string) (string, int, error) {
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

func Summary(t Target) string {
	return fmt.Sprintf("%s@%s:%d", t.Username, t.Host, t.Port)
}
