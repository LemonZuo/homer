package acmessh

import (
	"errors"
	"fmt"

	"github.com/LemonZuo/homer/internal/acme"
	"github.com/LemonZuo/homer/internal/model"
)

var ErrTargetNotConfigured = errors.New("SSH 部署目标未配置")

// TargetStore 是旧 HTTP/UI 形态的兼容层，实际读写 acme_deploy_target。
type TargetStore struct {
	targets *acme.DeployTargetStore
}

func NewTargetStore(targets *acme.DeployTargetStore) *TargetStore {
	return &TargetStore{targets: targets}
}

func (s *TargetStore) List() ([]model.ACMESSHTarget, error) {
	rows, err := s.targets.List(acme.DeployKindSSH)
	if err != nil {
		return nil, err
	}
	out := make([]model.ACMESSHTarget, 0, len(rows))
	for _, row := range rows {
		t, err := targetFromDeployTarget(row)
		if err != nil {
			continue
		}
		out = append(out, *t)
	}
	return out, nil
}

func (s *TargetStore) Get(id int64) (*model.ACMESSHTarget, error) {
	row, err := s.targets.Get(id)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrTargetNotConfigured, err)
	}
	if row.Kind != acme.DeployKindSSH {
		return nil, fmt.Errorf("%w: id=%d 类型不是 SSH", ErrTargetNotConfigured, id)
	}
	return targetFromDeployTarget(*row)
}

func (s *TargetStore) Upsert(t *model.ACMESSHTarget) (*model.ACMESSHTarget, error) {
	if t == nil {
		return nil, errors.New("target 不能为空")
	}
	row := deployTargetFromTarget(*t)
	saved, err := s.targets.Upsert(&row)
	if err != nil {
		return nil, err
	}
	return targetFromDeployTarget(*saved)
}

func (s *TargetStore) Delete(id int64) error {
	return s.targets.Delete(id)
}
