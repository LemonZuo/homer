package acme

import (
	"context"
	"errors"

	"github.com/LemonZuo/homer/internal/model"
)

const (
	DeployKindSSH       = "ssh"
	DeployKindSafeline  = "safeline"
	DeployKindUploadCAS = "upload_cas"
	DeployKindFnOS      = "fnos"
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
