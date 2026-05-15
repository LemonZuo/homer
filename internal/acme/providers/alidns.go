package acmeproviders

import (
	"context"
	"errors"
	"fmt"
	"strings"

	openapi "github.com/alibabacloud-go/darabonba-openapi/v2/client"
	"github.com/alibabacloud-go/tea/dara"
	alidnssdk "github.com/go-acme/alidns-20150109/v4/client"
)

func validateAlidns(ctx context.Context, envs map[string]string) error {
	ak := strings.TrimSpace(envs["ALICLOUD_ACCESS_KEY"])
	sk := strings.TrimSpace(envs["ALICLOUD_SECRET_KEY"])
	if ak == "" || sk == "" {
		return errors.New("alidns: ALICLOUD_ACCESS_KEY / ALICLOUD_SECRET_KEY 缺失")
	}
	regionID := strings.TrimSpace(envs["ALICLOUD_REGION_ID"])
	if regionID == "" {
		regionID = "cn-hangzhou"
	}
	cfg := new(openapi.Config).
		SetAccessKeyId(ak).
		SetAccessKeySecret(sk).
		SetRegionId(regionID).
		SetReadTimeout(10000)
	if tok := strings.TrimSpace(envs["ALICLOUD_SECURITY_TOKEN"]); tok != "" {
		cfg.SetSecurityToken(tok)
	}
	client, err := alidnssdk.NewClient(cfg)
	if err != nil {
		return fmt.Errorf("alidns: 客户端构造失败：%w", err)
	}
	req := new(alidnssdk.DescribeDomainsRequest).SetPageNumber(1).SetPageSize(1)
	_, err = alidnssdk.DescribeDomainsWithContext(ctx, client, req, &dara.RuntimeOptions{})
	if err != nil {
		return fmt.Errorf("alidns: %s", trimSDKErr(err))
	}
	return nil
}
