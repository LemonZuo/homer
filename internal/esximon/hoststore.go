package esximon

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/LemonZuo/homer/internal/model"
	"github.com/LemonZuo/homer/internal/sshlike"
	"gorm.io/gorm"
)

// ErrHostNotFound 主机 id 不存在。
var ErrHostNotFound = errors.New("ESXi 主机不存在")

// HostStore esxi_host 表的 CRUD。校验委托给 sshlike.ValidateTarget / ValidateBastion。
type HostStore struct {
	db *gorm.DB
}

func NewHostStore(db *gorm.DB) *HostStore { return &HostStore{db: db} }

// List 按 id 升序返回所有 ESXi 主机。
func (s *HostStore) List() ([]model.EsxiHost, error) {
	var rows []model.EsxiHost
	if err := s.db.Order("id ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// ListEnabled 仅返回 enabled='1' 的主机,sampler 用。
func (s *HostStore) ListEnabled() ([]model.EsxiHost, error) {
	var rows []model.EsxiHost
	if err := s.db.Where("enabled = ?", "1").Order("id ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// Get 按 id 取一条。
func (s *HostStore) Get(id int64) (*model.EsxiHost, error) {
	if id <= 0 {
		return nil, ErrHostNotFound
	}
	var row model.EsxiHost
	if err := s.db.First(&row, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("%w: id=%d", ErrHostNotFound, id)
		}
		return nil, err
	}
	return &row, nil
}

// HostInput 是前端 Upsert 请求的扁平形态,内部转回 auth_json/config_json。
type HostInput struct {
	ID           int64  `json:"id"`
	Name         string `json:"name"`
	Endpoint     string `json:"endpoint"`
	AuthSource   string `json:"auth_source"`
	CredentialID int64  `json:"credential_id"`
	Username     string `json:"username"`
	AuthType     string `json:"auth_type"`
	Password     string `json:"password"`
	PrivateKey   string `json:"private_key"`
	Passphrase   string `json:"passphrase"`
	BastionID    int64  `json:"bastion_id"`
	Enabled      *bool  `json:"enabled,omitempty"`
}

// Upsert 创建或按 id 更新。
func (s *HostStore) Upsert(in HostInput) (*model.EsxiHost, error) {
	in.Name = strings.TrimSpace(in.Name)
	in.Endpoint = strings.TrimSpace(in.Endpoint)
	host, port, err := sshlike.SplitEndpoint(in.Endpoint, "ESXi")
	if err != nil {
		return nil, err
	}
	t := sshlike.Target{
		ID:           in.ID,
		Name:         in.Name,
		Host:         host,
		Port:         port,
		AuthSource:   in.AuthSource,
		CredentialID: in.CredentialID,
		BastionID:    in.BastionID,
		Username:     in.Username,
		AuthType:     in.AuthType,
		Password:     in.Password,
		PrivateKey:   in.PrivateKey,
		Passphrase:   in.Passphrase,
	}
	sshlike.Normalize(&t)
	if err := sshlike.ValidateTarget(t, "ESXi"); err != nil {
		return nil, err
	}
	loader := func(id int64) (*sshlike.Target, error) { return LoadEsxiBastion(s.db, id) }
	finder := func(id int64) (string, bool, error) { return FindEsxiUpstream(s.db, id) }
	if err := sshlike.ValidateBastion(loader, finder, t, "本机"); err != nil {
		return nil, err
	}
	authJSON, err := sshlike.MarshalAuthJSON(t)
	if err != nil {
		return nil, fmt.Errorf("序列化认证配置失败:%w", err)
	}
	cfgJSON, err := sshlike.MarshalConfigJSON(t)
	if err != nil {
		return nil, fmt.Errorf("序列化连接配置失败:%w", err)
	}

	if in.ID == 0 {
		row := model.EsxiHost{
			Name:       t.Name,
			Endpoint:   in.Endpoint,
			AuthJSON:   authJSON,
			ConfigJSON: cfgJSON,
			Enabled:    boolFlagFromPtr(in.Enabled, true),
		}
		if err := s.db.Create(&row).Error; err != nil {
			return nil, err
		}
		return &row, nil
	}

	var existing model.EsxiHost
	if err := s.db.First(&existing, in.ID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("%w: id=%d", ErrHostNotFound, in.ID)
		}
		return nil, err
	}
	existing.Name = t.Name
	existing.Endpoint = in.Endpoint
	existing.AuthJSON = authJSON
	existing.ConfigJSON = cfgJSON
	if in.Enabled != nil {
		existing.Enabled = model.BoolFlag(*in.Enabled)
	}
	if err := s.db.Save(&existing).Error; err != nil {
		return nil, err
	}
	return &existing, nil
}

// SetEnabled 单独切换启用状态(toggle 用)。
func (s *HostStore) SetEnabled(id int64, enabled bool) error {
	if id <= 0 {
		return errors.New("id 无效")
	}
	res := s.db.Model(&model.EsxiHost{}).Where("id = ?", id).Update("enabled", model.BoolFlag(enabled))
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return fmt.Errorf("%w: id=%d", ErrHostNotFound, id)
	}
	return nil
}

// Delete 删除一条,先检查是否被人引用作 bastion。
func (s *HostStore) Delete(id int64) error {
	if id <= 0 {
		return errors.New("id 无效")
	}
	name, ok, err := FindEsxiUpstream(s.db, id)
	if err != nil {
		return err
	}
	if ok {
		return fmt.Errorf("该主机被 %s 用作跳板机,无法删除", name)
	}
	res := s.db.Delete(&model.EsxiHost{}, id)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return fmt.Errorf("%w: id=%d", ErrHostNotFound, id)
	}
	return nil
}

// CredentialUsage 统计每条 esxi_ssh_credential 被多少台 esxi_host 引用。
func (s *HostStore) CredentialUsage() (map[int64]int64, error) {
	var rows []model.EsxiHost
	if err := s.db.Find(&rows).Error; err != nil {
		return nil, err
	}
	counts := make(map[int64]int64)
	for _, row := range rows {
		var auth struct {
			AuthSource   string `json:"auth_source"`
			CredentialID int64  `json:"credential_id"`
		}
		if json.Unmarshal([]byte(row.AuthJSON), &auth) != nil {
			continue
		}
		if auth.AuthSource == sshlike.AuthSourceCredential && auth.CredentialID > 0 {
			counts[auth.CredentialID]++
		}
	}
	return counts, nil
}

// HostsByCredential 列出引用了指定凭证的所有主机名。
func (s *HostStore) HostsByCredential(credID int64) ([]string, error) {
	var rows []model.EsxiHost
	if err := s.db.Find(&rows).Error; err != nil {
		return nil, err
	}
	var names []string
	for _, row := range rows {
		var auth struct {
			AuthSource   string `json:"auth_source"`
			CredentialID int64  `json:"credential_id"`
		}
		if json.Unmarshal([]byte(row.AuthJSON), &auth) != nil {
			continue
		}
		if auth.AuthSource == sshlike.AuthSourceCredential && auth.CredentialID == credID {
			names = append(names, row.Name)
		}
	}
	return names, nil
}

func boolFlagFromPtr(p *bool, def bool) model.BoolFlag {
	if p == nil {
		return model.BoolFlag(def)
	}
	return model.BoolFlag(*p)
}
