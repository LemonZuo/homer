package acme

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/LemonZuo/homer/internal/model"
	"gorm.io/gorm"
)

const (
	DeployKindSSH      = "ssh"
	DeployKindSafeline = "safeline"
)

var (
	ErrDeployTargetNotConfigured = errors.New("部署目标未配置")
	ErrDeployConfigNotConfigured = errors.New("部署配置未配置")
)

type DeployDriver interface {
	Kind() string
	Label() string
	ValidateTarget(target model.ACMEDeployTarget) error
	ValidateConfig(target model.ACMEDeployTarget, cfg model.ACMEDeployConfig) error
	TestTarget(ctx context.Context, target model.ACMEDeployTarget) error
	Deploy(ctx context.Context, req DeployRequest) (*DeployResult, error)
}

type DeployRequest struct {
	Domain model.ACMEDomain
	Cert   model.ACMECert
	Target model.ACMEDeployTarget
	Config model.ACMEDeployConfig
	Logf   func(format string, args ...any)
}

type DeployResult struct {
	StateJSON string
}

type DeployRegistry struct {
	drivers map[string]DeployDriver
}

func NewDeployRegistry(drivers ...DeployDriver) *DeployRegistry {
	r := &DeployRegistry{drivers: map[string]DeployDriver{}}
	for _, d := range drivers {
		if d == nil {
			continue
		}
		r.drivers[d.Kind()] = d
	}
	return r
}

func (r *DeployRegistry) Get(kind string) (DeployDriver, error) {
	if r == nil {
		return nil, errors.New("部署 driver registry 未初始化")
	}
	kind = strings.ToLower(strings.TrimSpace(kind))
	d, ok := r.drivers[kind]
	if !ok {
		return nil, fmt.Errorf("不支持的部署类型：%s", kind)
	}
	return d, nil
}

type DeployTargetStore struct {
	db       *gorm.DB
	registry *DeployRegistry
}

func NewDeployTargetStore(db *gorm.DB, registry *DeployRegistry) *DeployTargetStore {
	return &DeployTargetStore{db: db, registry: registry}
}

func (s *DeployTargetStore) List(kind string) ([]model.ACMEDeployTarget, error) {
	var rows []model.ACMEDeployTarget
	q := s.db.Order("id DESC")
	if strings.TrimSpace(kind) != "" {
		q = q.Where("kind = ?", strings.ToLower(strings.TrimSpace(kind)))
	}
	if err := q.Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (s *DeployTargetStore) Get(id int64) (*model.ACMEDeployTarget, error) {
	if id <= 0 {
		return nil, fmt.Errorf("%w: id=%d", ErrDeployTargetNotConfigured, id)
	}
	var row model.ACMEDeployTarget
	if err := s.db.First(&row, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("%w: id=%d", ErrDeployTargetNotConfigured, id)
		}
		return nil, err
	}
	if !bool(row.Enabled) {
		return nil, fmt.Errorf("%w: %s 已停用", ErrDeployTargetNotConfigured, row.Name)
	}
	return &row, nil
}

func (s *DeployTargetStore) Upsert(t *model.ACMEDeployTarget) (*model.ACMEDeployTarget, error) {
	if t == nil {
		return nil, errors.New("部署目标不能为空")
	}
	normalizeDeployTarget(t)
	driver, err := s.registry.Get(t.Kind)
	if err != nil {
		return nil, err
	}
	if err := driver.ValidateTarget(*t); err != nil {
		return nil, err
	}
	if t.ID == 0 {
		if err := s.db.Create(t).Error; err != nil {
			return nil, err
		}
		return t, nil
	}
	var existing model.ACMEDeployTarget
	if err := s.db.First(&existing, t.ID).Error; err != nil {
		return nil, err
	}
	existing.Name = t.Name
	existing.Kind = t.Kind
	existing.Endpoint = t.Endpoint
	existing.AuthJSON = t.AuthJSON
	existing.ConfigJSON = t.ConfigJSON
	existing.Enabled = t.Enabled
	if err := s.db.Save(&existing).Error; err != nil {
		return nil, err
	}
	return &existing, nil
}

func (s *DeployTargetStore) Delete(id int64) error {
	if id <= 0 {
		return errors.New("id 无效")
	}
	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("target_id = ?", id).Delete(&model.ACMEDeployConfig{}).Error; err != nil {
			return err
		}
		return tx.Delete(&model.ACMEDeployTarget{}, id).Error
	})
}

func (s *DeployTargetStore) Test(ctx context.Context, id int64) error {
	target, err := s.Get(id)
	if err != nil {
		return err
	}
	driver, err := s.registry.Get(target.Kind)
	if err != nil {
		return err
	}
	return driver.TestTarget(ctx, *target)
}

func normalizeDeployTarget(t *model.ACMEDeployTarget) {
	t.Name = strings.TrimSpace(t.Name)
	t.Kind = strings.ToLower(strings.TrimSpace(t.Kind))
	t.Endpoint = strings.TrimRight(strings.TrimSpace(t.Endpoint), "/")
	t.AuthJSON = strings.TrimSpace(t.AuthJSON)
	t.ConfigJSON = strings.TrimSpace(t.ConfigJSON)
	if t.AuthJSON == "" {
		t.AuthJSON = "{}"
	}
	if t.ConfigJSON == "" {
		t.ConfigJSON = "{}"
	}
}

type DeployConfigStore struct {
	db       *gorm.DB
	targets  *DeployTargetStore
	registry *DeployRegistry
}

func NewDeployConfigStore(db *gorm.DB, targets *DeployTargetStore, registry *DeployRegistry) *DeployConfigStore {
	return &DeployConfigStore{db: db, targets: targets, registry: registry}
}

func (s *DeployConfigStore) ListByDomain(domainID int64, kind string) ([]model.ACMEDeployConfig, error) {
	var rows []model.ACMEDeployConfig
	q := s.db.Where("domain_id = ?", domainID).Order("id DESC")
	if strings.TrimSpace(kind) != "" {
		q = q.Where("kind = ?", strings.ToLower(strings.TrimSpace(kind)))
	}
	if err := q.Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (s *DeployConfigStore) ListAutoByDomain(domainID int64, kind string) ([]model.ACMEDeployConfig, error) {
	var rows []model.ACMEDeployConfig
	q := s.db.Where("domain_id = ? AND enabled = ? AND auto_deploy = ?", domainID, "1", "1").
		Order("id ASC")
	if strings.TrimSpace(kind) != "" {
		q = q.Where("kind = ?", strings.ToLower(strings.TrimSpace(kind)))
	}
	if err := q.Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (s *DeployConfigStore) Get(id int64) (*model.ACMEDeployConfig, error) {
	if id <= 0 {
		return nil, fmt.Errorf("%w: id=%d", ErrDeployConfigNotConfigured, id)
	}
	var row model.ACMEDeployConfig
	if err := s.db.First(&row, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("%w: id=%d", ErrDeployConfigNotConfigured, id)
		}
		return nil, err
	}
	if !bool(row.Enabled) {
		return nil, fmt.Errorf("%w: %s 已停用", ErrDeployConfigNotConfigured, deployConfigName(row))
	}
	return &row, nil
}

func (s *DeployConfigStore) Upsert(domainID int64, c *model.ACMEDeployConfig) (*model.ACMEDeployConfig, error) {
	if c == nil {
		return nil, errors.New("部署配置不能为空")
	}
	if domainID <= 0 {
		return nil, errors.New("domain_id 无效")
	}
	normalizeDeployConfig(c)
	c.DomainID = domainID
	if err := s.validate(*c); err != nil {
		return nil, err
	}
	if c.ID == 0 {
		if err := s.db.Create(c).Error; err != nil {
			return nil, err
		}
		return c, nil
	}
	var existing model.ACMEDeployConfig
	if err := s.db.First(&existing, c.ID).Error; err != nil {
		return nil, err
	}
	if existing.DomainID != domainID {
		return nil, errors.New("部署配置不属于当前域名")
	}
	existing.TargetID = c.TargetID
	existing.Kind = c.Kind
	existing.Name = c.Name
	existing.ConfigJSON = c.ConfigJSON
	existing.StateJSON = c.StateJSON
	existing.AutoDeploy = c.AutoDeploy
	existing.Enabled = c.Enabled
	if err := s.db.Save(&existing).Error; err != nil {
		return nil, err
	}
	return &existing, nil
}

func (s *DeployConfigStore) Delete(id int64) error {
	if id <= 0 {
		return errors.New("id 无效")
	}
	return s.db.Delete(&model.ACMEDeployConfig{}, id).Error
}

func (s *DeployConfigStore) SaveState(id int64, stateJSON string) error {
	if id <= 0 {
		return nil
	}
	stateJSON = strings.TrimSpace(stateJSON)
	if stateJSON == "" {
		stateJSON = "{}"
	}
	return s.db.Model(&model.ACMEDeployConfig{}).Where("id = ?", id).
		Update("state_json", stateJSON).Error
}

func (s *DeployConfigStore) validate(c model.ACMEDeployConfig) error {
	var domain model.ACMEDomain
	if err := s.db.First(&domain, c.DomainID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("域名配置不存在")
		}
		return err
	}
	target, err := s.targets.Get(c.TargetID)
	if err != nil {
		return err
	}
	if target.Kind != c.Kind {
		return fmt.Errorf("部署目标类型 %s 与配置类型 %s 不一致", target.Kind, c.Kind)
	}
	driver, err := s.registry.Get(c.Kind)
	if err != nil {
		return err
	}
	return driver.ValidateConfig(*target, c)
}

func normalizeDeployConfig(c *model.ACMEDeployConfig) {
	c.Kind = strings.ToLower(strings.TrimSpace(c.Kind))
	c.Name = strings.TrimSpace(c.Name)
	c.ConfigJSON = strings.TrimSpace(c.ConfigJSON)
	c.StateJSON = strings.TrimSpace(c.StateJSON)
	if c.ConfigJSON == "" {
		c.ConfigJSON = "{}"
	}
	if c.StateJSON == "" {
		c.StateJSON = "{}"
	}
}

func deployConfigName(c model.ACMEDeployConfig) string {
	if strings.TrimSpace(c.Name) != "" {
		return c.Name
	}
	return fmt.Sprintf("#%d", c.ID)
}

func mustJSON(v any) string {
	data, err := jsonMarshalIndent(v)
	if err != nil {
		return "{}"
	}
	return string(data)
}
