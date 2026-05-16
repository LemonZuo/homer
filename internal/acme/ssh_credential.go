package acme

import (
	"errors"
	"fmt"
	"strings"

	"github.com/LemonZuo/homer/internal/model"
	"gorm.io/gorm"
)

// ErrSSHCredentialNotFound 凭证 id 不存在或被删除。
var ErrSSHCredentialNotFound = errors.New("SSH 登录凭证不存在")

// SSHCredentialStore SSH 登录凭证的 CRUD。
// 与 acme_deploy_target 解耦：driver 在解析 auth_json 时按需 Get。
type SSHCredentialStore struct {
	db *gorm.DB
}

func NewSSHCredentialStore(db *gorm.DB) *SSHCredentialStore {
	return &SSHCredentialStore{db: db}
}

func (s *SSHCredentialStore) List() ([]model.SSHCredential, error) {
	var rows []model.SSHCredential
	if err := s.db.Order("id DESC").Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (s *SSHCredentialStore) Get(id int64) (*model.SSHCredential, error) {
	if id <= 0 {
		return nil, ErrSSHCredentialNotFound
	}
	var row model.SSHCredential
	if err := s.db.First(&row, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("%w: id=%d", ErrSSHCredentialNotFound, id)
		}
		return nil, err
	}
	return &row, nil
}

// Upsert 创建或按 id 更新。id=0 走创建，否则走更新。
func (s *SSHCredentialStore) Upsert(c *model.SSHCredential) (*model.SSHCredential, error) {
	if c == nil {
		return nil, errors.New("credential 不能为空")
	}
	normalizeCredential(c)
	if err := validateCredential(*c); err != nil {
		return nil, err
	}
	if c.ID == 0 {
		if err := s.db.Create(c).Error; err != nil {
			return nil, err
		}
		return c, nil
	}
	var existing model.SSHCredential
	if err := s.db.First(&existing, c.ID).Error; err != nil {
		return nil, err
	}
	existing.Name = c.Name
	existing.Username = c.Username
	existing.AuthType = c.AuthType
	existing.Password = c.Password
	existing.PrivateKey = c.PrivateKey
	existing.Passphrase = c.Passphrase
	if err := s.db.Save(&existing).Error; err != nil {
		return nil, err
	}
	return &existing, nil
}

func (s *SSHCredentialStore) Delete(id int64) error {
	if id <= 0 {
		return errors.New("id 无效")
	}
	return s.db.Delete(&model.SSHCredential{}, id).Error
}

func normalizeCredential(c *model.SSHCredential) {
	c.Name = strings.TrimSpace(c.Name)
	c.Username = strings.TrimSpace(c.Username)
	c.AuthType = strings.ToLower(strings.TrimSpace(c.AuthType))
	c.Password = strings.TrimSpace(c.Password)
	c.PrivateKey = strings.TrimSpace(c.PrivateKey)
	c.Passphrase = strings.TrimSpace(c.Passphrase)
	if c.AuthType == "" {
		c.AuthType = "password"
	}
}

func validateCredential(c model.SSHCredential) error {
	if c.Name == "" {
		return errors.New("凭证名称不能为空")
	}
	if c.Username == "" {
		return errors.New("登录用户名不能为空")
	}
	switch c.AuthType {
	case "password":
		if c.Password == "" {
			return errors.New("密码模式需要填写登录密码")
		}
	case "key":
		if c.PrivateKey == "" {
			return errors.New("秘钥模式需要填写私钥")
		}
	default:
		return errors.New("认证方式仅支持 password / key")
	}
	return nil
}
