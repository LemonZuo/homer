package acme

import (
	"errors"
	"fmt"
	"strings"

	"github.com/LemonZuo/homer/internal/model"
	"gorm.io/gorm"
)

// ErrSSHTargetNotConfigured 表示 SSH 部署目标不存在或未启用。
var ErrSSHTargetNotConfigured = errors.New("SSH 部署目标未配置")

// SSHTargetStore 管理远程 SSH 部署目标。
type SSHTargetStore struct {
	db *gorm.DB
}

func NewSSHTargetStore(db *gorm.DB) *SSHTargetStore {
	return &SSHTargetStore{db: db}
}

func (s *SSHTargetStore) List() ([]model.ACMESSHTarget, error) {
	var rows []model.ACMESSHTarget
	if err := s.db.Order("id DESC").Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (s *SSHTargetStore) Get(id int64) (*model.ACMESSHTarget, error) {
	if id <= 0 {
		return nil, fmt.Errorf("%w: id=%d", ErrSSHTargetNotConfigured, id)
	}
	var row model.ACMESSHTarget
	if err := s.db.First(&row, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("%w: id=%d", ErrSSHTargetNotConfigured, id)
		}
		return nil, err
	}
	if !bool(row.Enabled) {
		return nil, fmt.Errorf("%w: %s 已停用", ErrSSHTargetNotConfigured, row.Name)
	}
	return &row, nil
}

func (s *SSHTargetStore) Upsert(t *model.ACMESSHTarget) (*model.ACMESSHTarget, error) {
	if t == nil {
		return nil, errors.New("target 不能为空")
	}
	normalizeSSHTarget(t)
	if err := validateSSHTarget(*t); err != nil {
		return nil, err
	}
	if t.ID == 0 {
		if err := s.db.Create(t).Error; err != nil {
			return nil, err
		}
		return t, nil
	}
	var existing model.ACMESSHTarget
	if err := s.db.First(&existing, t.ID).Error; err != nil {
		return nil, err
	}
	existing.Name = t.Name
	existing.Host = t.Host
	existing.Port = t.Port
	existing.Username = t.Username
	existing.AuthType = t.AuthType
	existing.Password = t.Password
	existing.PrivateKey = t.PrivateKey
	existing.Passphrase = t.Passphrase
	existing.Enabled = t.Enabled
	if err := s.db.Save(&existing).Error; err != nil {
		return nil, err
	}
	return &existing, nil
}

func (s *SSHTargetStore) Delete(id int64) error {
	if id <= 0 {
		return errors.New("id 无效")
	}
	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("target_id = ?", id).Delete(&model.ACMESSHDeployConfig{}).Error; err != nil {
			return err
		}
		return tx.Delete(&model.ACMESSHTarget{}, id).Error
	})
}

func normalizeSSHTarget(t *model.ACMESSHTarget) {
	t.Name = strings.TrimSpace(t.Name)
	t.Host = strings.TrimSpace(t.Host)
	t.Username = strings.TrimSpace(t.Username)
	t.AuthType = strings.ToLower(strings.TrimSpace(t.AuthType))
	t.Password = strings.TrimSpace(t.Password)
	t.PrivateKey = strings.TrimSpace(t.PrivateKey)
	t.Passphrase = strings.TrimSpace(t.Passphrase)
	if t.Port <= 0 {
		t.Port = 22
	}
	if t.AuthType == "" {
		t.AuthType = "password"
	}
}

func validateSSHTarget(t model.ACMESSHTarget) error {
	if t.Name == "" {
		return errors.New("目标名称不能为空")
	}
	if t.Host == "" {
		return errors.New("SSH 主机不能为空")
	}
	if t.Username == "" {
		return errors.New("SSH 用户名不能为空")
	}
	if t.Port <= 0 || t.Port > 65535 {
		return errors.New("SSH 端口无效")
	}
	switch t.AuthType {
	case "password":
		if t.Password == "" {
			return errors.New("密码认证需要填写密码")
		}
	case "key":
		if t.PrivateKey == "" {
			return errors.New("证书模式需要填写私钥")
		}
	default:
		return errors.New("认证方式仅支持 password / key")
	}
	return nil
}
