package acme

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/LemonZuo/homer/internal/model"
	"gorm.io/gorm"
)

type DeployTargetStore struct {
	db       *gorm.DB
	registry *DeployRegistry
}

func NewDeployTargetStore(db *gorm.DB, registry *DeployRegistry) *DeployTargetStore {
	return &DeployTargetStore{db: db, registry: registry}
}

func (s *DeployTargetStore) List(kind string) ([]model.ACMEDeployTarget, error) {
	var rows []model.ACMEDeployTarget
	q := s.db.Order("id ASC")
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
	if name, ok, err := s.findBastionRef(id); err != nil {
		return err
	} else if ok {
		return fmt.Errorf("该机器被 %s 设为跳板机，请先去 %s 取消引用", name, name)
	}
	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("target_id = ?", id).Delete(&model.ACMEDeployConfig{}).Error; err != nil {
			return err
		}
		return tx.Delete(&model.ACMEDeployTarget{}, id).Error
	})
}

// findBastionRef 查找是否有别的 SSH/fnOS 目标把 id 当作跳板机引用，避免删除后另一台连不通。
// 直接扫一遍可作为跳板的目标 config_json，量小（个位数到几十）够用。
func (s *DeployTargetStore) findBastionRef(id int64) (string, bool, error) {
	var rows []model.ACMEDeployTarget
	if err := s.db.Where("kind IN ? AND id <> ?", []string{DeployKindSSH, DeployKindFnOS}, id).Find(&rows).Error; err != nil {
		return "", false, err
	}
	for _, r := range rows {
		cfg := map[string]any{}
		if err := JSONUnmarshal([]byte(EmptyJSON(r.ConfigJSON)), &cfg); err != nil {
			continue
		}
		v, _ := cfg["bastion_id"].(float64)
		if int64(v) == id {
			return r.Name, true, nil
		}
	}
	return "", false, nil
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
