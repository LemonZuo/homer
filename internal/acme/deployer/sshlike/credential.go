package sshlike

import (
	"errors"
	"fmt"
	"strings"

	"github.com/LemonZuo/homer/internal/acme"
)

func ResolveCredential(credentials *acme.SSHCredentialStore, t *Target) error {
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
