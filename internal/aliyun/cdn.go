package aliyun

import (
	cdn20180510 "github.com/alibabacloud-go/cdn-20180510/v5/client"
)

// NewCDNClient 创建 CDN 客户端；AK/SK 任一为空则返回 (nil, nil)，
// 由上层据此返回「未配置」而非报错。
func NewCDNClient(accessKeyID, accessKeySecret string) (*cdn20180510.Client, error) {
	if accessKeyID == "" || accessKeySecret == "" {
		return nil, nil
	}
	return cdn20180510.NewClient(newConfig(accessKeyID, accessKeySecret, "cdn.aliyuncs.com"))
}
