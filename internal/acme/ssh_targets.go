package acme

import (
	"errors"
	"fmt"
	"strings"

	"github.com/LemonZuo/homer/internal/model"
)

var ErrSSHTargetNotConfigured = errors.New("SSH 部署目标未配置")

// SSHTargetStore 是旧 HTTP/UI 形态的兼容层，实际读写 acme_deploy_target。
type SSHTargetStore struct {
	targets *DeployTargetStore
}

func NewSSHTargetStore(targets *DeployTargetStore) *SSHTargetStore {
	return &SSHTargetStore{targets: targets}
}

func (s *SSHTargetStore) List() ([]model.ACMESSHTarget, error) {
	rows, err := s.targets.List(DeployKindSSH)
	if err != nil {
		return nil, err
	}
	out := make([]model.ACMESSHTarget, 0, len(rows))
	for _, row := range rows {
		t, err := sshTargetFromDeployTarget(row)
		if err != nil {
			continue
		}
		out = append(out, *t)
	}
	return out, nil
}

func (s *SSHTargetStore) Get(id int64) (*model.ACMESSHTarget, error) {
	row, err := s.targets.Get(id)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrSSHTargetNotConfigured, err)
	}
	if row.Kind != DeployKindSSH {
		return nil, fmt.Errorf("%w: id=%d 类型不是 SSH", ErrSSHTargetNotConfigured, id)
	}
	return sshTargetFromDeployTarget(*row)
}

func (s *SSHTargetStore) Upsert(t *model.ACMESSHTarget) (*model.ACMESSHTarget, error) {
	if t == nil {
		return nil, errors.New("target 不能为空")
	}
	row := deployTargetFromSSHTarget(*t)
	saved, err := s.targets.Upsert(&row)
	if err != nil {
		return nil, err
	}
	return sshTargetFromDeployTarget(*saved)
}

func (s *SSHTargetStore) Delete(id int64) error {
	return s.targets.Delete(id)
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
