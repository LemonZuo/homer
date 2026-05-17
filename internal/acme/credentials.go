package acme

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/LemonZuo/homer/internal/model"
	"gorm.io/gorm"
)

// ErrProviderNotConfigured 表示 DB 里没有该 provider 的凭证。
var ErrProviderNotConfigured = errors.New("DNS provider 未配置凭证")

// CredentialStore 把 acme_credential 表当作 lego env 变量的 key-value store。
// envs_json 列存 JSON：{"<LEGO_ENV_KEY>": "<value>"}。
type CredentialStore struct {
	db *gorm.DB
}

func NewCredentialStore(db *gorm.DB) *CredentialStore {
	return &CredentialStore{db: db}
}

// Get 返回指定 provider 的环境变量集合。
func (s *CredentialStore) Get(provider string) (map[string]string, error) {
	var row model.ACMECredential
	if err := s.db.Where("provider = ?", provider).First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("%w: %s", ErrProviderNotConfigured, provider)
		}
		return nil, err
	}
	out := map[string]string{}
	if strings.TrimSpace(row.EnvsJSON) == "" {
		return out, nil
	}
	if err := json.Unmarshal([]byte(row.EnvsJSON), &out); err != nil {
		return nil, fmt.Errorf("解析 provider %s 凭证 JSON 失败：%w", provider, err)
	}
	return out, nil
}

// Providers 返回已配置凭证的 provider key 列表（升序）。
func (s *CredentialStore) Providers() []string {
	var rows []model.ACMECredential
	if err := s.db.Order("provider").Find(&rows).Error; err != nil {
		return []string{}
	}
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.Provider)
	}
	sort.Strings(out)
	return out
}

// List 返回所有凭证记录（envs_json 原样回前端，方便编辑）。
// RefCount 为引用该 provider 的域名数。
func (s *CredentialStore) List() ([]model.ACMECredential, error) {
	var rows []model.ACMECredential
	if err := s.db.Order("id ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	var counts []struct {
		Provider string
		N        int64
	}
	if err := s.db.Model(&model.ACMEDomain{}).
		Select("provider, COUNT(*) AS n").Group("provider").Scan(&counts).Error; err != nil {
		return nil, err
	}
	byProvider := make(map[string]int64, len(counts))
	for _, c := range counts {
		byProvider[c.Provider] = c.N
	}
	for i := range rows {
		rows[i].RefCount = byProvider[rows[i].Provider]
	}
	return rows, nil
}

// Upsert 创建或更新一条凭证。envsJSON 必须是合法 JSON object。
func (s *CredentialStore) Upsert(provider, envsJSON string) (*model.ACMECredential, error) {
	provider = strings.TrimSpace(provider)
	if provider == "" {
		return nil, errors.New("provider 不能为空")
	}
	// 校验 JSON
	tmp := map[string]string{}
	if strings.TrimSpace(envsJSON) == "" {
		envsJSON = "{}"
	}
	if err := json.Unmarshal([]byte(envsJSON), &tmp); err != nil {
		return nil, fmt.Errorf("envs_json 必须是 {\"KEY\":\"VALUE\"} 形式：%w", err)
	}
	var row model.ACMECredential
	err := s.db.Where("provider = ?", provider).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		row = model.ACMECredential{Provider: provider, EnvsJSON: envsJSON}
		if err := s.db.Create(&row).Error; err != nil {
			return nil, err
		}
		return &row, nil
	} else if err != nil {
		return nil, err
	}
	row.EnvsJSON = envsJSON
	if err := s.db.Save(&row).Error; err != nil {
		return nil, err
	}
	return &row, nil
}

// Delete 按 id 删除；仍被 acme_domain 引用的 provider 拒绝删除。
func (s *CredentialStore) Delete(id int64) error {
	if id <= 0 {
		return errors.New("id 无效")
	}
	var row model.ACMECredential
	if err := s.db.First(&row, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return err
	}
	var count int64
	if err := s.db.Model(&model.ACMEDomain{}).Where("provider = ?", row.Provider).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return fmt.Errorf("仍有 %d 个域名使用该 DNS provider 凭证，不能删除", count)
	}
	return s.db.Delete(&model.ACMECredential{}, id).Error
}
