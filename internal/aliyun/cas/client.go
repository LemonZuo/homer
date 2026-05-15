// Package cas 封装阿里云数字证书管理服务（CAS）OpenAPI 客户端。
// 与老 Java CasClientUtil 等价：固定 endpoint cas.aliyuncs.com，AK/SK 来自配置。
package cas

import (
	cas20200407 "github.com/alibabacloud-go/cas-20200407/v4/client"
	openapi "github.com/alibabacloud-go/darabonba-openapi/v2/client"
	"github.com/alibabacloud-go/tea/tea"
)

// NewClient 创建 CAS 客户端；AK/SK 任一为空则返回 (nil, nil)，
// 由上层据此返回「未配置」而非报错。
func NewClient(accessKeyID, accessKeySecret string) (*cas20200407.Client, error) {
	if accessKeyID == "" || accessKeySecret == "" {
		return nil, nil
	}
	cfg := &openapi.Config{
		AccessKeyId:     tea.String(accessKeyID),
		AccessKeySecret: tea.String(accessKeySecret),
		Endpoint:        tea.String("cas.aliyuncs.com"),
	}
	return cas20200407.NewClient(cfg)
}
