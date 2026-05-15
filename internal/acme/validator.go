package acme

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strings"
	"time"

	openapi "github.com/alibabacloud-go/darabonba-openapi/v2/client"
	"github.com/alibabacloud-go/tea/dara"
	alidnssdk "github.com/go-acme/alidns-20150109/v4/client"
	dnspodsdk "github.com/go-acme/tencentclouddnspod/v20210323"
	hwauthbasic "github.com/huaweicloud/huaweicloud-sdk-go-v3/core/auth/basic"
	hwconfig "github.com/huaweicloud/huaweicloud-sdk-go-v3/core/config"
	hwdns "github.com/huaweicloud/huaweicloud-sdk-go-v3/services/dns/v2"
	hwmodel "github.com/huaweicloud/huaweicloud-sdk-go-v3/services/dns/v2/model"
	hwregion "github.com/huaweicloud/huaweicloud-sdk-go-v3/services/dns/v2/region"
	tcommon "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common"
	tprofile "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/profile"
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

// ---- Cloudflare ----

func validateCloudflare(ctx context.Context, envs map[string]string) error {
	token := strings.TrimSpace(envs["CLOUDFLARE_DNS_API_TOKEN"])
	if token != "" {
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet,
			"https://api.cloudflare.com/client/v4/user/tokens/verify", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		return cloudflareCall(req)
	}
	email := strings.TrimSpace(envs["CLOUDFLARE_EMAIL"])
	key := strings.TrimSpace(envs["CLOUDFLARE_API_KEY"])
	if email == "" || key == "" {
		return errors.New("cloudflare: 需提供 CLOUDFLARE_DNS_API_TOKEN，或 CLOUDFLARE_EMAIL + CLOUDFLARE_API_KEY")
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet,
		"https://api.cloudflare.com/client/v4/user", nil)
	req.Header.Set("X-Auth-Email", email)
	req.Header.Set("X-Auth-Key", key)
	return cloudflareCall(req)
}

func cloudflareCall(req *http.Request) error {
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("cloudflare: 请求失败：%w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<10))
	var r struct {
		Success bool `json:"success"`
		Errors  []struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"errors"`
	}
	_ = json.Unmarshal(body, &r)
	if !r.Success {
		if len(r.Errors) > 0 {
			return fmt.Errorf("cloudflare: %d %s", r.Errors[0].Code, r.Errors[0].Message)
		}
		return fmt.Errorf("cloudflare: HTTP %d %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return nil
}

// ---- DNSPod 旧版（基于 login_token） ----

func validateDNSPod(ctx context.Context, envs map[string]string) error {
	token := strings.TrimSpace(envs["DNSPOD_API_KEY"])
	if token == "" {
		return errors.New("dnspod: DNSPOD_API_KEY 缺失")
	}
	form := url.Values{"login_token": {token}, "format": {"json"}}
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://dnsapi.cn/Info.Version", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", "homer-acme-validator/1.0 (lemon@local)")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("dnspod: 请求失败：%w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<10))
	var r struct {
		Status struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"status"`
	}
	if err := json.Unmarshal(body, &r); err != nil {
		return fmt.Errorf("dnspod: 响应解析失败：HTTP %d %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	if r.Status.Code != "1" {
		return fmt.Errorf("dnspod: %s %s", r.Status.Code, r.Status.Message)
	}
	return nil
}

// ---- 阿里云 DNS（alidns） ----

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

// ---- 腾讯云 DNS（dnspod 新版） ----

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

// ---- 华为云 DNS ----

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
