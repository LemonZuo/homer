package sshhost

import (
	"errors"
	"fmt"
	"strings"

	"github.com/LemonZuo/homer/internal/model"
)

// CredentialResolver 把 credential id 解成可登录的身份。
// Store 在 internal/esximon 包里实现,这里只依赖 interface,避免双向 import。
type CredentialResolver interface {
	Get(id int64) (*model.EsxiSSHCredential, error)
}

// ResolveCredential 若 Target 走凭证模式则从 resolver 加载并写回字段。
func ResolveCredential(resolver CredentialResolver, t *Target) error {
	if t.AuthSource != AuthSourceCredential {
		return nil
	}
	if resolver == nil {
		return errors.New("凭证模式未注入 ESXi SSH 凭证库")
	}
	if t.CredentialID <= 0 {
		return errors.New("凭证模式需要选择登录凭证")
	}
	cred, err := resolver.Get(t.CredentialID)
	if err != nil {
		return fmt.Errorf("加载 ESXi SSH 登录凭证失败:%w", err)
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
