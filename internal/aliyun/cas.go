// Package aliyun 封装项目使用到的阿里云 OpenAPI 客户端。
package aliyun

import (
	cas20200407 "github.com/alibabacloud-go/cas-20200407/v4/client"
	openapi "github.com/alibabacloud-go/darabonba-openapi/v2/client"
	"github.com/alibabacloud-go/tea/tea"
)

// SDK 默认 Connect/ReadTimeout 为 0（不超时）：接口「连着不回」时调用方会一直挂着，
// 部署场景下还会长期占住 ACME 的签发锁。单位毫秒。
const (
	connectTimeoutMs = 10_000
	readTimeoutMs    = 60_000
)

func newConfig(accessKeyID, accessKeySecret, endpoint string) *openapi.Config {
	return &openapi.Config{
		AccessKeyId:     tea.String(accessKeyID),
		AccessKeySecret: tea.String(accessKeySecret),
		Endpoint:        tea.String(endpoint),
		ConnectTimeout:  tea.Int(connectTimeoutMs),
		ReadTimeout:     tea.Int(readTimeoutMs),
	}
}

// NewCASClient 创建 CAS 客户端；AK/SK 任一为空则返回 (nil, nil)，
// 由上层据此返回「未配置」而非报错。
func NewCASClient(accessKeyID, accessKeySecret string) (*cas20200407.Client, error) {
	if accessKeyID == "" || accessKeySecret == "" {
		return nil, nil
	}
	return cas20200407.NewClient(newConfig(accessKeyID, accessKeySecret, "cas.aliyuncs.com"))
}
