package acme

import (
	"errors"
	"fmt"
	"strings"

	"github.com/LemonZuo/homer/internal/model"
	"github.com/LemonZuo/homer/internal/sshlike"
	"gorm.io/gorm"
)

// Labels 用于在错误消息里区分 SSH / fnOS：sshlike 自己不知道是 SSH 还是 fnOS,
// 各驱动调用 ParseSSHTarget 时把自己的标签塞进来。
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

// ParseSSHTarget 把 ACMEDeployTarget GORM 行翻译成 sshlike.Target。
// sshlike 包不依赖 internal/model,所以这一步留在 acme 模块自己做。
func ParseSSHTarget(target model.ACMEDeployTarget, labels Labels) (*sshlike.Target, error) {
	labels = labels.withDefaults()
	auth, err := sshlike.UnmarshalAuthJSON(target.AuthJSON)
	if err != nil {
		return nil, fmt.Errorf("解析 %s 认证配置失败：%w", labels.Auth, err)
	}
	cfg, err := sshlike.UnmarshalConfigJSON(target.ConfigJSON)
	if err != nil {
		return nil, fmt.Errorf("解析 %s 连接配置失败：%w", labels.Config, err)
	}
	host, port, err := sshlike.SplitEndpoint(target.Endpoint, labels.Host)
	if err != nil {
		return nil, err
	}
	out := &sshlike.Target{
		ID:           target.ID,
		Name:         target.Name,
		Host:         host,
		Port:         port,
		AuthSource:   auth.AuthSource,
		CredentialID: auth.CredentialID,
		BastionID:    cfg.BastionID,
		Username:     auth.Username,
		AuthType:     auth.AuthType,
		Password:     auth.Password,
		PrivateKey:   auth.PrivateKey,
		Passphrase:   auth.Passphrase,
		Enabled:      bool(target.Enabled),
		CreatedAt:    target.CreatedAt,
		UpdatedAt:    target.UpdatedAt,
	}
	sshlike.Normalize(out)
	return out, nil
}

// LoadSSHBastion 加载一台被引用作为跳板机的 SSH/fnOS 目标,校验类型与启用状态。
// 这里只解析连接配置,不解析凭证;建连前再 ResolveCredential。
func LoadSSHBastion(db *gorm.DB, id int64) (*sshlike.Target, error) {
	if db == nil {
		return nil, errors.New("跳板机模式未注入 DB")
	}
	var row model.ACMEDeployTarget
	if err := db.First(&row, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("跳板机不存在：id=%d", id)
		}
		return nil, fmt.Errorf("加载跳板机失败：%w", err)
	}
	if row.Kind != DeployKindSSH && row.Kind != DeployKindFnOS {
		return nil, fmt.Errorf("跳板机必须是 SSH 或 fnOS 类型：id=%d, kind=%s", id, row.Kind)
	}
	if !bool(row.Enabled) {
		return nil, fmt.Errorf("跳板机已停用：%s", row.Name)
	}
	return ParseSSHTarget(row, Labels{Auth: "SSH"})
}

// FindSSHUpstream 反查是否有别的 SSH/fnOS target 把 id 当作 bastion 在用。
// 返回第一个引用者的 name,用于错误提示。
func FindSSHUpstream(db *gorm.DB, id int64) (string, bool, error) {
	if db == nil {
		return "", false, errors.New("跳板机模式未注入 DB")
	}
	var rows []model.ACMEDeployTarget
	if err := db.Where("kind IN ? AND id <> ?", []string{DeployKindSSH, DeployKindFnOS}, id).Find(&rows).Error; err != nil {
		return "", false, fmt.Errorf("扫描跳板机引用失败：%w", err)
	}
	for _, r := range rows {
		cfg, err := sshlike.UnmarshalConfigJSON(r.ConfigJSON)
		if err != nil {
			continue
		}
		if cfg.BastionID == id {
			return r.Name, true, nil
		}
	}
	return "", false, nil
}
