package upsmon

import (
	"errors"
	"fmt"
	"strings"

	"github.com/LemonZuo/homer/internal/model"
	"github.com/LemonZuo/homer/internal/sshlike"
	"gorm.io/gorm"
)

// ErrCredentialNotFound 凭证 id 不存在。
var ErrCredentialNotFound = errors.New("UPS SSH 登录凭证不存在")

// CredentialStore ups_ssh_credential 表的 CRUD。
// 实现 sshlike.CredentialResolver(Resolve),供 sampler 在凭证模式下解析使用。
type CredentialStore struct {
	db    *gorm.DB
	hosts *HostStore
}

func NewCredentialStore(db *gorm.DB, hosts *HostStore) *CredentialStore {
	return &CredentialStore{db: db, hosts: hosts}
}

// List 返回所有凭证;RefCount 为引用此凭证的 ups_host 数。
func (s *CredentialStore) List() ([]model.UPSSSHCredential, error) {
	var rows []model.UPSSSHCredential
	if err := s.db.Order("id ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	counts, err := s.hosts.CredentialUsage()
	if err != nil {
		return nil, err
	}
	for i := range rows {
		rows[i].RefCount = counts[rows[i].ID]
	}
	return rows, nil
}

// Get 按 id 取一条。
func (s *CredentialStore) Get(id int64) (*model.UPSSSHCredential, error) {
	if id <= 0 {
		return nil, ErrCredentialNotFound
	}
	var row model.UPSSSHCredential
	if err := s.db.First(&row, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("%w: id=%d", ErrCredentialNotFound, id)
		}
		return nil, err
	}
	return &row, nil
}

// Resolve 实现 sshlike.CredentialResolver,把 GORM 行扁平化成 sshlike.Credential。
func (s *CredentialStore) Resolve(id int64) (sshlike.Credential, error) {
	row, err := s.Get(id)
	if err != nil {
		return sshlike.Credential{}, err
	}
	return sshlike.Credential{
		Username:   row.Username,
		AuthType:   row.AuthType,
		Password:   row.Password,
		PrivateKey: row.PrivateKey,
		Passphrase: row.Passphrase,
	}, nil
}

// Upsert id=0 创建,否则更新。
func (s *CredentialStore) Upsert(c *model.UPSSSHCredential) (*model.UPSSSHCredential, error) {
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
	var existing model.UPSSSHCredential
	if err := s.db.First(&existing, c.ID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("%w: id=%d", ErrCredentialNotFound, c.ID)
		}
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

// Delete 删除前校验是否被任何 ups_host 引用。
func (s *CredentialStore) Delete(id int64) error {
	if id <= 0 {
		return errors.New("id 无效")
	}
	used, err := s.hosts.HostsByCredential(id)
	if err != nil {
		return err
	}
	if len(used) > 0 {
		return errors.New("该凭证仍被机器使用,无法删除")
	}
	res := s.db.Delete(&model.UPSSSHCredential{}, id)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return fmt.Errorf("%w: id=%d", ErrCredentialNotFound, id)
	}
	return nil
}

func normalizeCredential(c *model.UPSSSHCredential) {
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

func validateCredential(c model.UPSSSHCredential) error {
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
			return errors.New("证书模式需要填写私钥")
		}
	default:
		return errors.New("认证方式仅支持 password / key")
	}
	return nil
}
