package acme

import (
	"errors"
	"fmt"
	"strings"

	"github.com/LemonZuo/homer/internal/model"
	"gorm.io/gorm"
)

// ErrSSHDeployConfigNotConfigured 表示 SSH 部署配置不存在或未启用。
var ErrSSHDeployConfigNotConfigured = errors.New("SSH 部署配置未配置")

// SSHDeployConfigStore 管理域名到 SSH 机器的证书部署配置。
type SSHDeployConfigStore struct {
	db *gorm.DB
}

func NewSSHDeployConfigStore(db *gorm.DB) *SSHDeployConfigStore {
	return &SSHDeployConfigStore{db: db}
}

func (s *SSHDeployConfigStore) ListByDomain(domainID int64) ([]model.ACMESSHDeployConfig, error) {
	var rows []model.ACMESSHDeployConfig
	if err := s.db.Where("domain_id = ?", domainID).Order("id DESC").Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (s *SSHDeployConfigStore) ListAutoByDomain(domainID int64) ([]model.ACMESSHDeployConfig, error) {
	var rows []model.ACMESSHDeployConfig
	if err := s.db.Where("domain_id = ? AND enabled = ? AND auto_deploy = ?", domainID, "1", "1").
		Order("id ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (s *SSHDeployConfigStore) Get(id int64) (*model.ACMESSHDeployConfig, error) {
	if id <= 0 {
		return nil, fmt.Errorf("%w: id=%d", ErrSSHDeployConfigNotConfigured, id)
	}
	var row model.ACMESSHDeployConfig
	if err := s.db.First(&row, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("%w: id=%d", ErrSSHDeployConfigNotConfigured, id)
		}
		return nil, err
	}
	if !bool(row.Enabled) {
		return nil, fmt.Errorf("%w: %s 已停用", ErrSSHDeployConfigNotConfigured, deployConfigLabel(row))
	}
	return &row, nil
}

func (s *SSHDeployConfigStore) Upsert(domainID int64, c *model.ACMESSHDeployConfig) (*model.ACMESSHDeployConfig, error) {
	if c == nil {
		return nil, errors.New("部署配置不能为空")
	}
	if domainID <= 0 {
		return nil, errors.New("domain_id 无效")
	}
	normalizeSSHDeployConfig(c)
	c.DomainID = domainID
	if err := validateSSHDeployConfig(s.db, *c); err != nil {
		return nil, err
	}
	if c.ID == 0 {
		if err := s.db.Create(c).Error; err != nil {
			return nil, err
		}
		return c, nil
	}
	var existing model.ACMESSHDeployConfig
	if err := s.db.First(&existing, c.ID).Error; err != nil {
		return nil, err
	}
	if existing.DomainID != domainID {
		return nil, errors.New("部署配置不属于当前域名")
	}
	existing.TargetID = c.TargetID
	existing.Name = c.Name
	existing.CertPath = c.CertPath
	existing.KeyPath = c.KeyPath
	existing.ChainPath = c.ChainPath
	existing.FullchainPath = c.FullchainPath
	existing.DeployCommand = c.DeployCommand
	existing.AutoDeploy = c.AutoDeploy
	existing.Enabled = c.Enabled
	if err := s.db.Save(&existing).Error; err != nil {
		return nil, err
	}
	return &existing, nil
}

func (s *SSHDeployConfigStore) Delete(id int64) error {
	if id <= 0 {
		return errors.New("id 无效")
	}
	return s.db.Delete(&model.ACMESSHDeployConfig{}, id).Error
}

func normalizeSSHDeployConfig(c *model.ACMESSHDeployConfig) {
	c.Name = strings.TrimSpace(c.Name)
	c.CertPath = strings.TrimSpace(c.CertPath)
	c.KeyPath = strings.TrimSpace(c.KeyPath)
	c.ChainPath = strings.TrimSpace(c.ChainPath)
	c.FullchainPath = strings.TrimSpace(c.FullchainPath)
	c.DeployCommand = strings.TrimSpace(c.DeployCommand)
}

func validateSSHDeployConfig(db *gorm.DB, c model.ACMESSHDeployConfig) error {
	if c.TargetID <= 0 {
		return errors.New("请选择 SSH 机器")
	}
	if strings.TrimSpace(c.KeyPath) == "" {
		return errors.New("远端 key.pem 路径不能为空")
	}
	if strings.TrimSpace(c.CertPath) == "" && strings.TrimSpace(c.FullchainPath) == "" {
		return errors.New("cert.pem 路径和 fullchain.pem 路径至少填写一个")
	}
	var domain model.ACMEDomain
	if err := db.First(&domain, c.DomainID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("域名配置不存在")
		}
		return err
	}
	var target model.ACMESSHTarget
	if err := db.First(&target, c.TargetID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("SSH 机器不存在")
		}
		return err
	}
	return nil
}

func deployConfigLabel(c model.ACMESSHDeployConfig) string {
	if strings.TrimSpace(c.Name) != "" {
		return c.Name
	}
	return fmt.Sprintf("#%d", c.ID)
}
