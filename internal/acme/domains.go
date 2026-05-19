package acme

import (
	"errors"
	"strings"
	"time"

	"github.com/LemonZuo/homer/internal/model"
	"gorm.io/gorm"
)

// DomainView 联查域名 + 最近一次证书（NotAfter 用于前端显示剩余天数）。
type DomainView struct {
	model.ACMEDomain
	NotAfter   *time.Time `json:"not_after,omitempty"`
	NotBefore  *time.Time `json:"not_before,omitempty"`
	CertStatus string     `json:"cert_status,omitempty"`
	RevokedAt  *time.Time `json:"revoked_at,omitempty"`
	IssuedAt   *time.Time `json:"issued_at,omitempty"`
}

// ListDomains 列出所有域名（按 id 升序），附带最近一次证书摘要。
func (s *Service) ListDomains() ([]DomainView, error) {
	var items []model.ACMEDomain
	if err := s.db.Order("id ASC").Find(&items).Error; err != nil {
		return nil, err
	}
	out := make([]DomainView, 0, len(items))
	for _, d := range items {
		v := DomainView{ACMEDomain: d}
		var c model.ACMECert
		if err := s.db.Where("domain_id = ?", d.ID).First(&c).Error; err == nil {
			na := c.NotAfter
			nb := c.NotBefore
			ia := c.IssuedAt
			v.NotAfter = &na
			v.NotBefore = &nb
			v.IssuedAt = &ia
			v.CertStatus = c.Status
			v.RevokedAt = c.RevokedAt
		}
		out = append(out, v)
	}
	return out, nil
}

// normalizeSanProviders 校验并归一化 san_providers JSON；空串保持空串。
func normalizeSanProviders(s string) (string, error) {
	if strings.TrimSpace(s) == "" {
		return "", nil
	}
	m := map[string]string{}
	if err := JSONUnmarshal([]byte(s), &m); err != nil {
		return "", errors.New("san_providers 不是合法的 JSON 对象")
	}
	out := map[string]string{}
	for k, v := range m {
		k = strings.TrimSpace(k)
		v = strings.TrimSpace(v)
		if k != "" && v != "" {
			out[k] = v
		}
	}
	if len(out) == 0 {
		return "", nil
	}
	b, err := JSONMarshalIndent(out)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// CreateDomain 新增域名。
func (s *Service) CreateDomain(d *model.ACMEDomain) error {
	d.MainDomain = strings.TrimSpace(d.MainDomain)
	d.Provider = strings.TrimSpace(d.Provider)
	if d.MainDomain == "" || d.Provider == "" {
		return errors.New("main_domain 与 provider 必填")
	}
	sp, err := normalizeSanProviders(d.SanProviders)
	if err != nil {
		return err
	}
	d.SanProviders = sp
	if _, err := s.accountStore.Get(d.AccountID); err != nil {
		return err
	}
	return s.db.Create(d).Error
}

// UpdateDomain 更新域名（按 id）。
func (s *Service) UpdateDomain(d *model.ACMEDomain) error {
	if d.ID == 0 {
		return errors.New("id 必填")
	}
	d.MainDomain = strings.TrimSpace(d.MainDomain)
	d.Provider = strings.TrimSpace(d.Provider)
	if d.MainDomain == "" || d.Provider == "" {
		return errors.New("main_domain 与 provider 必填")
	}
	sp, err := normalizeSanProviders(d.SanProviders)
	if err != nil {
		return err
	}
	d.SanProviders = sp
	if _, err := s.accountStore.Get(d.AccountID); err != nil {
		return err
	}
	return s.db.Save(d).Error
}

// DeleteDomain 删除域名及其证书/任务流水。
func (s *Service) DeleteDomain(id int64) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("domain_id = ?", id).Delete(&model.ACMECert{}).Error; err != nil {
			return err
		}
		if err := tx.Where("domain_id = ?", id).Delete(&model.ACMEDeployConfig{}).Error; err != nil {
			return err
		}
		if err := tx.Where("domain_id = ?", id).Delete(&model.ACMEIssueTask{}).Error; err != nil {
			return err
		}
		return tx.Delete(&model.ACMEDomain{}, id).Error
	})
}

// DomainByID 按主键取一条域名记录。
func (s *Service) DomainByID(id int64) (*model.ACMEDomain, error) {
	var d model.ACMEDomain
	if err := s.db.First(&d, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &d, nil
}

// GetCertByDomain 返回最近一次签发的证书（空时 nil, nil）。
func (s *Service) GetCertByDomain(domainID int64) (*model.ACMECert, error) {
	var c model.ACMECert
	if err := s.db.Where("domain_id = ?", domainID).First(&c).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &c, nil
}
