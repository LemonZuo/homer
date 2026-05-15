package acme

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/LemonZuo/homer/internal/model"
	"gorm.io/gorm"
)

// ErrAccountNotConfigured 表示域名引用的 ACME 账号不存在或未启用。
var ErrAccountNotConfigured = errors.New("ACME 账号未配置")

// AccountStore 管理 ACME CA 账号配置。
type AccountStore struct {
	db *gorm.DB
}

func NewAccountStore(db *gorm.DB) *AccountStore {
	return &AccountStore{db: db}
}

func (s *AccountStore) List() ([]model.ACMEAccount, error) {
	var rows []model.ACMEAccount
	if err := s.db.Order("id DESC").Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (s *AccountStore) Enabled() ([]model.ACMEAccount, error) {
	var rows []model.ACMEAccount
	if err := s.db.Where("enabled = ?", "1").Order("name").Find(&rows).Error; err != nil {
		return nil, err
	}
	sort.SliceStable(rows, func(i, j int) bool {
		return strings.ToLower(rows[i].Name) < strings.ToLower(rows[j].Name)
	})
	return rows, nil
}

func (s *AccountStore) Get(id int64) (*model.ACMEAccount, error) {
	if id <= 0 {
		return nil, fmt.Errorf("%w: id=%d", ErrAccountNotConfigured, id)
	}
	var row model.ACMEAccount
	if err := s.db.First(&row, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("%w: id=%d", ErrAccountNotConfigured, id)
		}
		return nil, err
	}
	if !bool(row.Enabled) {
		return nil, fmt.Errorf("%w: %s 已停用", ErrAccountNotConfigured, row.Name)
	}
	return &row, nil
}

func (s *AccountStore) Upsert(a *model.ACMEAccount) (*model.ACMEAccount, error) {
	if a == nil {
		return nil, errors.New("account 不能为空")
	}
	normalizeAccount(a)
	if err := validateAccount(*a); err != nil {
		return nil, err
	}
	if a.ID == 0 {
		if err := s.db.Create(a).Error; err != nil {
			return nil, err
		}
		return a, nil
	}
	var existing model.ACMEAccount
	if err := s.db.First(&existing, a.ID).Error; err != nil {
		return nil, err
	}
	existing.Name = a.Name
	existing.CA = a.CA
	existing.DirectoryURL = a.DirectoryURL
	existing.Email = a.Email
	existing.EABKID = a.EABKID
	existing.EABHMAC = a.EABHMAC
	existing.Enabled = a.Enabled
	if err := s.db.Save(&existing).Error; err != nil {
		return nil, err
	}
	return &existing, nil
}

func (s *AccountStore) Delete(id int64) error {
	if id <= 0 {
		return errors.New("id 无效")
	}
	var count int64
	if err := s.db.Model(&model.ACMEDomain{}).Where("account_id = ?", id).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return fmt.Errorf("仍有 %d 个域名使用该 ACME 账号，不能删除", count)
	}
	return s.db.Delete(&model.ACMEAccount{}, id).Error
}

func normalizeAccount(a *model.ACMEAccount) {
	a.Name = strings.TrimSpace(a.Name)
	a.CA = normalizeCA(a.CA)
	a.DirectoryURL = strings.TrimSpace(a.DirectoryURL)
	a.Email = strings.TrimSpace(a.Email)
	a.EABKID = strings.TrimSpace(a.EABKID)
	a.EABHMAC = strings.TrimSpace(a.EABHMAC)
	switch a.CA {
	case "letsencrypt":
		a.DirectoryURL = CADirLetsEncrypt
	case "zerossl":
		a.DirectoryURL = CADirZeroSSL
	case "":
		a.CA = "letsencrypt"
		a.DirectoryURL = CADirLetsEncrypt
	}
}

func validateAccount(a model.ACMEAccount) error {
	if a.Name == "" {
		return errors.New("账号名称不能为空")
	}
	if a.Email == "" {
		return errors.New("ACME 邮箱不能为空")
	}
	switch a.CA {
	case "letsencrypt", "zerossl":
	case "custom":
		if a.DirectoryURL == "" {
			return errors.New("自定义 ACME 账号需要 directory_url")
		}
	default:
		return fmt.Errorf("未知 CA：%s（支持 letsencrypt / zerossl / custom）", a.CA)
	}
	if (a.EABKID == "") != (a.EABHMAC == "") {
		return errors.New("EAB KID 与 EAB HMAC 需要同时填写")
	}
	if a.CA == "zerossl" && (a.EABKID == "" || a.EABHMAC == "") {
		return errors.New("ZeroSSL 需要填写 EAB KID 与 EAB HMAC")
	}
	return nil
}
