// Package cas 数字证书管理（CAS）业务逻辑：复刻老 Java CasServiceImpl。
// 仅做证书列表 / 删除；「部署到 CDN」复用 cdn.Service。
package cas

import (
	"errors"

	sdk "github.com/alibabacloud-go/cas-20200407/v4/client"
	"github.com/alibabacloud-go/tea/tea"

	aliyuncas "github.com/LemonZuo/homer/internal/aliyun/cas"
)

// ErrNotConfigured 表示未配置阿里云 CAS AK/SK。
var ErrNotConfigured = errors.New("阿里云 CAS 未配置")

// listShowSize 单次拉取条数；前端无分页，取较大值覆盖个人证书量。
const listShowSize = 50

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
	c, err := aliyuncas.NewClient(s.accessKeyID, s.accessKeySecret)
	if err != nil {
		return nil, err
	}
	if c == nil {
		return nil, ErrNotConfigured
	}
	return c, nil
}

// CertView 证书列表项：对齐老 Java CertificateVO。
type CertView struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	OrgName     string `json:"orgName"`
	StartDate   string `json:"startDate"`
	EndDate     string `json:"endDate"`
	Expired     bool   `json:"expired"`
	Sans        string `json:"sans"`
	Common      string `json:"common"`
	Issuer      string `json:"issuer"`
	Country     string `json:"country"`
	Fingerprint string `json:"fingerprint"`
	Province    string `json:"province"`
	City        string `json:"city"`
}

// ListCertificates 查询证书列表（orderType=CERT，含已签发与上传的证书）。
func (s *Service) ListCertificates() ([]CertView, error) {
	c, err := s.client()
	if err != nil {
		return nil, err
	}
	resp, err := c.ListUserCertificateOrder(&sdk.ListUserCertificateOrderRequest{
		OrderType:   tea.String("CERT"),
		CurrentPage: tea.Int64(1),
		ShowSize:    tea.Int64(listShowSize),
	})
	if err != nil {
		return nil, err
	}
	out := []CertView{}
	if resp == nil || resp.Body == nil {
		return out, nil
	}
	for _, it := range resp.Body.CertificateOrderList {
		if it == nil {
			continue
		}
		out = append(out, CertView{
			ID:          tea.Int64Value(it.CertificateId),
			Name:        tea.StringValue(it.Name),
			OrgName:     tea.StringValue(it.OrgName),
			StartDate:   tea.StringValue(it.StartDate),
			EndDate:     tea.StringValue(it.EndDate),
			Expired:     tea.BoolValue(it.Expired),
			Sans:        tea.StringValue(it.Sans),
			Common:      tea.StringValue(it.CommonName),
			Issuer:      tea.StringValue(it.Issuer),
			Country:     tea.StringValue(it.Country),
			Fingerprint: tea.StringValue(it.Fingerprint),
			Province:    tea.StringValue(it.Province),
			City:        tea.StringValue(it.City),
		})
	}
	return out, nil
}

// DeleteCertificate 按证书 ID 删除用户证书。
func (s *Service) DeleteCertificate(id int64) error {
	if id <= 0 {
		return errors.New("证书 ID 不能为空")
	}
	c, err := s.client()
	if err != nil {
		return err
	}
	_, err = c.DeleteUserCertificate(&sdk.DeleteUserCertificateRequest{
		CertId: tea.Int64(id),
	})
	return err
}
