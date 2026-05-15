package acmeproviders

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	dnspodsdk "github.com/go-acme/tencentclouddnspod/v20210323"
	tcommon "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common"
	tprofile "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/profile"
)

// validateTencentcloud 走腾讯云 DNSPod 新版 SDK。
func validateTencentcloud(ctx context.Context, envs map[string]string) error {
	id := strings.TrimSpace(envs["TENCENTCLOUD_SECRET_ID"])
	sk := strings.TrimSpace(envs["TENCENTCLOUD_SECRET_KEY"])
	if id == "" || sk == "" {
		return errors.New("tencentcloud: TENCENTCLOUD_SECRET_ID / TENCENTCLOUD_SECRET_KEY 缺失")
	}
	region := strings.TrimSpace(envs["TENCENTCLOUD_REGION"])
	cred := tcommon.NewCredential(id, sk)
	if tok := strings.TrimSpace(envs["TENCENTCLOUD_SESSION_TOKEN"]); tok != "" {
		cred = tcommon.NewTokenCredential(id, sk, tok)
	}
	cpf := tprofile.NewClientProfile()
	cpf.HttpProfile.Endpoint = "dnspod.tencentcloudapi.com"
	cpf.HttpProfile.ReqTimeout = int(math.Round((10 * time.Second).Seconds()))
	client, err := dnspodsdk.NewClient(cred, region, cpf)
	if err != nil {
		return fmt.Errorf("tencentcloud: 客户端构造失败：%w", err)
	}
	req := dnspodsdk.NewDescribeDomainListRequest()
	one := int64(1)
	req.Limit = &one
	if _, err := dnspodsdk.DescribeDomainListWithContext(ctx, client, req); err != nil {
		return fmt.Errorf("tencentcloud: %s", trimSDKErr(err))
	}
	return nil
}
