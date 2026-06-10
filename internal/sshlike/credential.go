package sshlike

import (
	"errors"
	"fmt"
	"strings"
)

// Credential 是凭证库返回给 sshlike 的扁平形态。
// 各模块的凭证库实现 CredentialResolver,把自家 GORM 模型(SSHCredential /
// UPSSSHCredential / EsxiSSHCredential)拷成这个。
type Credential struct {
	Username   string
	AuthType   string
	Password   string
	PrivateKey string
	Passphrase string
}

// CredentialResolver 是凭证解析的抽象。各模块的 SSHCredentialStore 实现 Resolve。
type CredentialResolver interface {
	Resolve(id int64) (Credential, error)
}

// ResolveCredential 在 Target 走凭证模式时从 resolver 加载并回填字段。
// 非凭证模式直接返回 nil(inline 已经在 Target 里)。
func ResolveCredential(resolver CredentialResolver, t *Target) error {
	if t.AuthSource != AuthSourceCredential {
		return nil
	}
	if resolver == nil {
		return errors.New("凭证模式未注入 SSH 凭证库")
	}
	if t.CredentialID <= 0 {
		return errors.New("凭证模式需要选择登录凭证")
	}
	cred, err := resolver.Resolve(t.CredentialID)
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
