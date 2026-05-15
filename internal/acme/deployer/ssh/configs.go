package acmessh

import (
	"errors"
	"fmt"

	"github.com/LemonZuo/homer/internal/acme"
	"github.com/LemonZuo/homer/internal/model"
)

var ErrDeployConfigNotConfigured = errors.New("SSH 部署配置未配置")

// DeployConfigStore 是旧 HTTP/UI 形态的兼容层，实际读写 acme_deploy_config。
type DeployConfigStore struct {
	configs *acme.DeployConfigStore
}

func NewDeployConfigStore(configs *acme.DeployConfigStore) *DeployConfigStore {
	return &DeployConfigStore{configs: configs}
}

func (s *DeployConfigStore) ListByDomain(domainID int64) ([]model.ACMESSHDeployConfig, error) {
	rows, err := s.configs.ListByDomain(domainID, acme.DeployKindSSH)
	if err != nil {
		return nil, err
	}
	out := make([]model.ACMESSHDeployConfig, 0, len(rows))
	for _, row := range rows {
		out = append(out, deployConfigFromGenericConfig(row))
	}
	return out, nil
}

func (s *DeployConfigStore) ListAutoByDomain(domainID int64) ([]model.ACMESSHDeployConfig, error) {
	rows, err := s.configs.ListAutoByDomain(domainID, acme.DeployKindSSH)
	if err != nil {
		return nil, err
	}
	out := make([]model.ACMESSHDeployConfig, 0, len(rows))
	for _, row := range rows {
		out = append(out, deployConfigFromGenericConfig(row))
	}
	return out, nil
}

func (s *DeployConfigStore) Get(id int64) (*model.ACMESSHDeployConfig, error) {
	row, err := s.configs.Get(id)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrDeployConfigNotConfigured, err)
	}
	if row.Kind != acme.DeployKindSSH {
		return nil, fmt.Errorf("%w: id=%d 类型不是 SSH", ErrDeployConfigNotConfigured, id)
	}
	cfg := deployConfigFromGenericConfig(*row)
	return &cfg, nil
}

func (s *DeployConfigStore) Upsert(domainID int64, c *model.ACMESSHDeployConfig) (*model.ACMESSHDeployConfig, error) {
	if c == nil {
		return nil, errors.New("部署配置不能为空")
	}
	row := genericConfigFromDeployConfig(*c)
	saved, err := s.configs.Upsert(domainID, &row)
	if err != nil {
		return nil, err
	}
	cfg := deployConfigFromGenericConfig(*saved)
	return &cfg, nil
}

func (s *DeployConfigStore) Delete(id int64) error {
	return s.configs.Delete(id)
}
