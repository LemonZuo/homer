// Package cdn 加速域名管理业务逻辑：复刻老 Java CdnServiceImpl。
// 仅做阿里云只读视图 + 证书部署（off→on workaround）。
package cdn

import (
	"errors"
	"fmt"
	"strings"

	sdk "github.com/alibabacloud-go/cdn-20180510/v5/client"
	"github.com/alibabacloud-go/tea/tea"

	"github.com/LemonZuo/homer/internal/aliyun"
)

// ErrNotConfigured 表示未配置阿里云 CDN AK/SK。
var ErrNotConfigured = errors.New("阿里云 CDN 未配置")

// Service 持有 AK/SK，按需创建 SDK 客户端（与老实现一致：每次调用新建 client）。
type Service struct {
	accessKeyID     string
	accessKeySecret string
}

func NewService(accessKeyID, accessKeySecret string) *Service {
	return &Service{accessKeyID: accessKeyID, accessKeySecret: accessKeySecret}
}

func (s *Service) Configured() bool {
	return s.accessKeyID != "" && s.accessKeySecret != ""
}

func (s *Service) client() (*sdk.Client, error) {
	c, err := aliyun.NewCDNClient(s.accessKeyID, s.accessKeySecret)
	if err != nil {
		return nil, err
	}
	if c == nil {
		return nil, ErrNotConfigured
	}
	return c, nil
}

// DomainView 前端只读表格用：对齐老 Vue 页面列。
type DomainView struct {
	DomainName    string `json:"domainName"`
	Cname         string `json:"cname"`
	DomainStatus  string `json:"domainStatus"`
	SslProtocol   string `json:"sslProtocol"`
	CertName      string `json:"certName"`
	GmtCreated    string `json:"gmtCreated"`
	SourceType    string `json:"sourceType"`
	SourceContent string `json:"sourceContent"`
	SourcePort    int32  `json:"sourcePort"`
}

// ListDomains 获取加速域名列表，并合并 https 证书信息补 certName。
func (s *Service) ListDomains() ([]DomainView, error) {
	c, err := s.client()
	if err != nil {
		return nil, err
	}
	domainsResp, err := c.DescribeUserDomains(&sdk.DescribeUserDomainsRequest{})
	if err != nil {
		return nil, err
	}
	certName := map[string]string{}
	if httpsResp, err := c.DescribeCdnHttpsDomainList(&sdk.DescribeCdnHttpsDomainListRequest{}); err == nil &&
		httpsResp.Body != nil && httpsResp.Body.CertInfos != nil {
		for _, ci := range httpsResp.Body.CertInfos.CertInfo {
			certName[tea.StringValue(ci.DomainName)] = tea.StringValue(ci.CertName)
		}
	}

	out := []DomainView{}
	if domainsResp.Body == nil || domainsResp.Body.Domains == nil {
		return out, nil
	}
	for _, d := range domainsResp.Body.Domains.PageData {
		dn := tea.StringValue(d.DomainName)
		v := DomainView{
			DomainName:   dn,
			Cname:        tea.StringValue(d.Cname),
			DomainStatus: tea.StringValue(d.DomainStatus),
			SslProtocol:  tea.StringValue(d.SslProtocol),
			CertName:     certName[dn],
			GmtCreated:   tea.StringValue(d.GmtCreated),
		}
		if d.Sources != nil && len(d.Sources.Source) > 0 {
			src := d.Sources.Source[0]
			v.SourceType = tea.StringValue(src.Type)
			v.SourceContent = tea.StringValue(src.Content)
			v.SourcePort = tea.Int32Value(src.Port)
		}
		out = append(out, v)
	}
	return out, nil
}

// certExists 证书是否存在于 CDN 证书中心（部署前校验）。
func (s *Service) certExists(c *sdk.Client, certName string) (bool, error) {
	resp, err := c.DescribeCdnCertificateDetail(&sdk.DescribeCdnCertificateDetailRequest{
		CertName: tea.String(certName),
	})
	if err != nil {
		return false, err
	}
	return resp.Body != nil && resp.Body.CertId != nil && tea.Int64Value(resp.Body.CertId) != 0, nil
}

// setDomainCertificate 批量设置加速域名证书（domains 逗号分隔）。
func (s *Service) setDomainCertificate(c *sdk.Client, domains, sslProtocol, certName string) error {
	req := &sdk.BatchSetCdnDomainServerCertificateRequest{
		DomainName:  tea.String(domains),
		SSLProtocol: tea.String(sslProtocol),
	}
	if certName != "" {
		req.CertName = tea.String(certName)
	}
	_, err := c.BatchSetCdnDomainServerCertificate(req)
	if err != nil {
		return fmt.Errorf("设置加速域名证书失败 domains=%s sslProtocol=%s certName=%s: %w",
			domains, sslProtocol, certName, err)
	}
	return nil
}

// DeployCertificate 把证书部署到所有开启 HTTPS 但证书不匹配的加速域名。
// 复刻老逻辑：先 off 再 on（否则 DescribeUserDomains 返回数据与实际不一致）。
// 返回 message 为人可读结果，供阶段 3 CAS 复用。
func (s *Service) DeployCertificate(certName string) (string, error) {
	certName = strings.TrimSpace(certName)
	if certName == "" {
		return "", errors.New("certName：证书名称不能为空")
	}
	c, err := s.client()
	if err != nil {
		return "", err
	}
	exists, err := s.certExists(c, certName)
	if err != nil {
		return "", err
	}
	if !exists {
		return fmt.Sprintf("【%s】证书不存在于证书中心", certName), nil
	}

	domainsResp, err := c.DescribeUserDomains(&sdk.DescribeUserDomainsRequest{})
	if err != nil {
		return "", err
	}
	sslOn := map[string]bool{}
	if domainsResp.Body != nil && domainsResp.Body.Domains != nil {
		for _, d := range domainsResp.Body.Domains.PageData {
			if tea.StringValue(d.SslProtocol) == "on" {
				sslOn[tea.StringValue(d.DomainName)] = true
			}
		}
	}
	if len(sslOn) == 0 {
		return "开启 sslProtocol 的加速域名为空，无需更新部署证书", nil
	}

	httpsResp, err := c.DescribeCdnHttpsDomainList(&sdk.DescribeCdnHttpsDomainListRequest{})
	if err != nil {
		return "", err
	}
	if httpsResp.Body == nil || httpsResp.Body.CertInfos == nil || len(httpsResp.Body.CertInfos.CertInfo) == 0 {
		return "获取到 cdnHttpsDomainList 数据为空，无需更新部署证书", nil
	}

	var targets []string
	for _, ci := range httpsResp.Body.CertInfos.CertInfo {
		dn := tea.StringValue(ci.DomainName)
		if sslOn[dn] && tea.StringValue(ci.CertName) != certName {
			targets = append(targets, dn)
		}
	}
	if len(targets) == 0 {
		return fmt.Sprintf("所有开启 sslProtocol 的加速域名都已使用证书名：【%s】", certName), nil
	}

	domains := strings.Join(targets, ",")
	if err := s.setDomainCertificate(c, domains, "off", certName); err != nil {
		return "", err
	}
	if err := s.setDomainCertificate(c, domains, "on", certName); err != nil {
		return "", err
	}
	return fmt.Sprintf("部署证书【%s】到 CDN 域名成功：%s", certName, domains), nil
}
