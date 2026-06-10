package sshlike

import (
	"errors"
	"fmt"

	"github.com/LemonZuo/homer/internal/sshx"
)

// ConnOptions 注入 sshlike 不该自己持有的依赖:
//   - Credentials: 凭证库(走凭证模式时用)
//   - LoadBastion: 加载跳板机目标的闭包(各模块查自己的 host 表)
//   - RejectBastionChain: 是否拒绝多跳。三家目前都只允许单跳,传 true 即可
type ConnOptions struct {
	Credentials        CredentialResolver
	LoadBastion        BastionLoader
	RejectBastionChain bool
}

// ConnFor 把 *Target 转成 sshx.Conn:解析凭证、构造 AuthMethod、按需加载跳板机。
// 注意 sshx.Conn 是更底层的 SSH 客户端封装,sshlike 只搭桥不复刻。
func ConnFor(t *Target, opts ConnOptions) (*sshx.Conn, error) {
	if err := ResolveCredential(opts.Credentials, t); err != nil {
		return nil, err
	}
	auth, err := sshx.AuthMethod(t.AuthType, t.Password, t.PrivateKey, t.Passphrase)
	if err != nil {
		return nil, err
	}
	conn := &sshx.Conn{Host: t.Host, Port: t.Port, User: t.Username, Auth: auth}
	if t.BastionID <= 0 {
		return conn, nil
	}
	if opts.LoadBastion == nil {
		return nil, errors.New("跳板机模式未注入 BastionLoader")
	}
	bastion, err := opts.LoadBastion(t.BastionID)
	if err != nil {
		return nil, err
	}
	if opts.RejectBastionChain && bastion.BastionID > 0 {
		return nil, errors.New("所选跳板机已经设置了自己的跳板机，单跳模式不支持跳板机链")
	}
	if err := ResolveCredential(opts.Credentials, bastion); err != nil {
		return nil, err
	}
	bAuth, err := sshx.AuthMethod(bastion.AuthType, bastion.Password, bastion.PrivateKey, bastion.Passphrase)
	if err != nil {
		return nil, fmt.Errorf("跳板机认证准备失败：%w", err)
	}
	conn.Bastion = &sshx.Conn{Host: bastion.Host, Port: bastion.Port, User: bastion.Username, Auth: bAuth}
	return conn, nil
}
