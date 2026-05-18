package acmefnos

import (
	"errors"
	"fmt"

	"github.com/LemonZuo/homer/internal/acme"
	"github.com/LemonZuo/homer/internal/model"
)

var ErrDeployConfigNotConfigured = errors.New("fnOS 部署配置未配置")

// DeployConfigStore 是旧 HTTP/UI 形态的兼容层，实际读写 acme_deploy_config。
type DeployConfigStore struct {
	configs *acme.DeployConfigStore
}

func NewDeployConfigStore(configs *acme.DeployConfigStore) *DeployConfigStore {
	return &DeployConfigStore{configs: configs}
}

func (s *DeployConfigStore) ListByDomain(domainID int64) ([]model.ACMEFnOSDeployConfig, error) {
	rows, err := s.configs.ListByDomain(domainID, acme.DeployKindFnOS)
	if err != nil {
		return nil, err
	}
	out := make([]model.ACMEFnOSDeployConfig, 0, len(rows))
	for _, row := range rows {
		out = append(out, deployConfigViewFromGeneric(row))
	}
	return out, nil
}

func (s *DeployConfigStore) ListAutoByDomain(domainID int64) ([]model.ACMEFnOSDeployConfig, error) {
	rows, err := s.configs.ListAutoByDomain(domainID, acme.DeployKindFnOS)
	if err != nil {
		return nil, err
	}
	out := make([]model.ACMEFnOSDeployConfig, 0, len(rows))
	for _, row := range rows {
		out = append(out, deployConfigViewFromGeneric(row))
	}
	return out, nil
}

func (s *DeployConfigStore) Get(id int64) (*model.ACMEFnOSDeployConfig, error) {
	row, err := s.configs.Get(id)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrDeployConfigNotConfigured, err)
	}
	if row.Kind != acme.DeployKindFnOS {
		return nil, fmt.Errorf("%w: id=%d 类型不是 fnOS", ErrDeployConfigNotConfigured, id)
	}
	cfg := deployConfigViewFromGeneric(*row)
	return &cfg, nil
}

func (s *DeployConfigStore) Upsert(domainID int64, c *model.ACMEFnOSDeployConfig) (*model.ACMEFnOSDeployConfig, error) {
	if c == nil {
		return nil, errors.New("fnOS 部署配置不能为空")
	}
	row := genericConfigFromDeployConfig(*c)
	saved, err := s.configs.Upsert(domainID, &row)
	if err != nil {
		return nil, err
	}
	cfg := deployConfigViewFromGeneric(*saved)
	return &cfg, nil
}

func (s *DeployConfigStore) Delete(id int64) error {
	return s.configs.Delete(id)
}
