package acme

import (
	"errors"
	"fmt"
	"strings"

	"github.com/LemonZuo/homer/internal/model"
)

var ErrSSHDeployConfigNotConfigured = errors.New("SSH 部署配置未配置")

// SSHDeployConfigStore 是旧 HTTP/UI 形态的兼容层，实际读写 acme_deploy_config。
type SSHDeployConfigStore struct {
	configs *DeployConfigStore
}

func NewSSHDeployConfigStore(configs *DeployConfigStore) *SSHDeployConfigStore {
	return &SSHDeployConfigStore{configs: configs}
}

func (s *SSHDeployConfigStore) ListByDomain(domainID int64) ([]model.ACMESSHDeployConfig, error) {
	rows, err := s.configs.ListByDomain(domainID, DeployKindSSH)
	if err != nil {
		return nil, err
	}
	out := make([]model.ACMESSHDeployConfig, 0, len(rows))
	for _, row := range rows {
		out = append(out, sshDeployConfigFromGenericConfig(row))
	}
	return out, nil
}

func (s *SSHDeployConfigStore) ListAutoByDomain(domainID int64) ([]model.ACMESSHDeployConfig, error) {
	rows, err := s.configs.ListAutoByDomain(domainID, DeployKindSSH)
	if err != nil {
		return nil, err
	}
	out := make([]model.ACMESSHDeployConfig, 0, len(rows))
	for _, row := range rows {
		out = append(out, sshDeployConfigFromGenericConfig(row))
	}
	return out, nil
}

func (s *SSHDeployConfigStore) Get(id int64) (*model.ACMESSHDeployConfig, error) {
	row, err := s.configs.Get(id)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrSSHDeployConfigNotConfigured, err)
	}
	if row.Kind != DeployKindSSH {
		return nil, fmt.Errorf("%w: id=%d 类型不是 SSH", ErrSSHDeployConfigNotConfigured, id)
	}
	cfg := sshDeployConfigFromGenericConfig(*row)
	return &cfg, nil
}

func (s *SSHDeployConfigStore) Upsert(domainID int64, c *model.ACMESSHDeployConfig) (*model.ACMESSHDeployConfig, error) {
	if c == nil {
		return nil, errors.New("部署配置不能为空")
	}
	row := genericConfigFromSSHDeployConfig(*c)
	saved, err := s.configs.Upsert(domainID, &row)
	if err != nil {
		return nil, err
	}
	cfg := sshDeployConfigFromGenericConfig(*saved)
	return &cfg, nil
}

func (s *SSHDeployConfigStore) Delete(id int64) error {
	return s.configs.Delete(id)
}

func normalizeSSHDeployConfig(c *model.ACMESSHDeployConfig) {
	c.Name = strings.TrimSpace(c.Name)
	c.CertPath = strings.TrimSpace(c.CertPath)
	c.KeyPath = strings.TrimSpace(c.KeyPath)
	c.ChainPath = strings.TrimSpace(c.ChainPath)
	c.FullchainPath = strings.TrimSpace(c.FullchainPath)
	c.DeployCommand = strings.TrimSpace(c.DeployCommand)
}

func deployConfigLabel(c model.ACMESSHDeployConfig) string {
	if strings.TrimSpace(c.Name) != "" {
		return c.Name
	}
	return fmt.Sprintf("#%d", c.ID)
}
