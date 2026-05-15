package acmeproviders

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	hwauthbasic "github.com/huaweicloud/huaweicloud-sdk-go-v3/core/auth/basic"
	hwconfig "github.com/huaweicloud/huaweicloud-sdk-go-v3/core/config"
	hwdns "github.com/huaweicloud/huaweicloud-sdk-go-v3/services/dns/v2"
	hwmodel "github.com/huaweicloud/huaweicloud-sdk-go-v3/services/dns/v2/model"
	hwregion "github.com/huaweicloud/huaweicloud-sdk-go-v3/services/dns/v2/region"
)

func validateHuaweicloud(ctx context.Context, envs map[string]string) error {
	ak := strings.TrimSpace(envs["HUAWEICLOUD_ACCESS_KEY_ID"])
	sk := strings.TrimSpace(envs["HUAWEICLOUD_SECRET_ACCESS_KEY"])
	region := strings.TrimSpace(envs["HUAWEICLOUD_REGION"])
	if ak == "" || sk == "" || region == "" {
		return errors.New("huaweicloud: ACCESS_KEY_ID / SECRET_ACCESS_KEY / REGION 缺失")
	}
	auth, err := hwauthbasic.NewCredentialsBuilder().
		WithAk(ak).WithSk(sk).SafeBuild()
	if err != nil {
		return fmt.Errorf("huaweicloud: 凭证构造失败：%w", err)
	}
	r, err := hwregion.SafeValueOf(region)
	if err != nil {
		return fmt.Errorf("huaweicloud: region 无效：%w", err)
	}
	client, err := hwdns.DnsClientBuilder().
		WithHttpConfig(hwconfig.DefaultHttpConfig().WithTimeout(10 * time.Second)).
		WithRegion(r).
		WithCredential(auth).
		SafeBuild()
	if err != nil {
		return fmt.Errorf("huaweicloud: 客户端构造失败：%w", err)
	}
	dnsCli := hwdns.NewDnsClient(client)
	one := int32(1)
	if _, err := dnsCli.ListPublicZones(&hwmodel.ListPublicZonesRequest{Limit: &one}); err != nil {
		_ = ctx // SDK 不接受 context；以 HTTP timeout 兜底
		return fmt.Errorf("huaweicloud: %s", trimSDKErr(err))
	}
	return nil
}
