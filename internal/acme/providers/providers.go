// Package acmeproviders 按 provider 拆分 DNS 凭证的深度校验实现。
// cloudflare / dnspod / alidns / tencentcloud / huaweicloud 各自一个文件，
// 都是「同类型不同实现」，统一通过 Validate 注册表派发。
package acmeproviders

import (
	"context"
	"errors"
	"strings"
	"time"
)

// ErrNoValidator 表示该 provider 没有注册深度校验函数。调用方可选择放行或拒绝。
var ErrNoValidator = errors.New("该 provider 暂未实现深度校验，已按 JSON 形式保存")

// Validator 用提供的环境变量调一次轻量 API，验证凭证可用。
type Validator func(ctx context.Context, envs map[string]string) error

var validators = map[string]Validator{
	"cloudflare":   validateCloudflare,
	"dnspod":       validateDNSPod,
	"alidns":       validateAlidns,
	"tencentcloud": validateTencentcloud,
	"huaweicloud":  validateHuaweicloud,
}

// Validate 调用 provider 对应的深度校验函数；未注册时返回 ErrNoValidator。
func Validate(provider string, envs map[string]string) error {
	v, ok := validators[provider]
	if !ok {
		return ErrNoValidator
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	return v(ctx, envs)
}

// trimSDKErr 砍掉 SDK 错误里冗长的 RequestId / 调用栈，留主要信息给用户看。
func trimSDKErr(err error) string {
	msg := err.Error()
	if i := strings.Index(msg, "\n"); i > 0 {
		msg = msg[:i]
	}
	if len(msg) > 240 {
		msg = msg[:240] + "..."
	}
	return msg
}
