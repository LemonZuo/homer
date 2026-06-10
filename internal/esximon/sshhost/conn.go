package sshhost

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/LemonZuo/homer/internal/acme/deployer/sshx"
	"gorm.io/gorm"
)

// ConnOptions 注入跨表能力:凭证库 + DB(bastion 反查)。
type ConnOptions struct {
	Credentials CredentialResolver
	DB          *gorm.DB
}

// ConnFor 把 Target 转成 sshx.Conn,处理凭证解析与单跳 bastion。
func ConnFor(t *Target, opts ConnOptions) (*sshx.Conn, error) {
	if err := ResolveCredential(opts.Credentials, t); err != nil {
		return nil, err
	}
	auth, err := sshx.AuthMethod(t.AuthType, t.Password, t.PrivateKey, t.Passphrase)
	if err != nil {
		return nil, err
	}
	conn := &sshx.Conn{Host: t.Host, Port: t.Port, User: t.Username, Auth: auth}
	if t.BastionHostID <= 0 {
		return conn, nil
	}
	bastion, err := LoadBastion(opts.DB, t.BastionHostID)
	if err != nil {
		return nil, err
	}
	if bastion.BastionHostID > 0 {
		return nil, errors.New("所选跳板机已经设置了自己的跳板机,单跳模式不支持跳板机链")
	}
	if err := ResolveCredential(opts.Credentials, bastion); err != nil {
		return nil, err
	}
	bAuth, err := sshx.AuthMethod(bastion.AuthType, bastion.Password, bastion.PrivateKey, bastion.Passphrase)
	if err != nil {
		return nil, fmt.Errorf("跳板机认证准备失败:%w", err)
	}
	conn.Bastion = &sshx.Conn{Host: bastion.Host, Port: bastion.Port, User: bastion.Username, Auth: bAuth}
	return conn, nil
}

// jsonUnmarshal 给 bastion.go 用的 helper,容忍空串。
func jsonUnmarshal(s string, v any) error {
	if strings.TrimSpace(s) == "" {
		s = "{}"
	}
	return json.Unmarshal([]byte(s), v)
}
