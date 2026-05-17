// Package acmealicas 是 ACME 证书部署到阿里云 CAS（数字证书管理）的 driver 实现。
// 与 ssh / safeline 子包平行：core（internal/acme）只持有 DeployDriver 接口与通用 store。
// 设计取舍：CAS UploadUserCertificate 不支持原地更新，每次都是新增；state 仅记录最近一次
// 上传的 cert_id 供前端/审计可见，旧证书清理由用户在 CAS 控制台自行处理。
package acmealicas

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	sdk "github.com/alibabacloud-go/cas-20200407/v4/client"
	"github.com/alibabacloud-go/tea/tea"

	"github.com/LemonZuo/homer/internal/acme"
	aliyuncas "github.com/LemonZuo/homer/internal/aliyun/cas"
	"github.com/LemonZuo/homer/internal/model"
)

// Driver 实现 acme.DeployDriver，把证书上传到阿里云 CAS。
type Driver struct{}

// TargetAuth 是 acme_deploy_target.auth_json 在阿里云 CAS 场景下的结构。
type TargetAuth struct {
	AccessKeyID     string `json:"access_key_id"`
	AccessKeySecret string `json:"access_key_secret"`
}

// DeployState 记录最近一次上传的 CAS cert_id（仅用于前端展示）。
type DeployState struct {
	CertID int64 `json:"cert_id"`
}

func NewDriver() *Driver { return &Driver{} }

func (d *Driver) Kind() string  { return acme.DeployKindUploadCAS }
func (d *Driver) Label() string { return "阿里云 CAS" }

func (d *Driver) ValidateTarget(target model.ACMEDeployTarget) error {
	t, err := targetFromDeployTarget(target)
	if err != nil {
		return err
	}
	return validateTarget(*t)
}

func (d *Driver) ValidateConfig(_ model.ACMEDeployTarget, cfg model.ACMEDeployConfig) error {
	_, err := stateFromGenericConfig(cfg)
	return err
}

func (d *Driver) TestTarget(_ context.Context, target model.ACMEDeployTarget) error {
	t, err := targetFromDeployTarget(target)
	if err != nil {
		return err
	}
	client, err := aliyuncas.NewClient(t.AccessKeyID, t.AccessKeySecret)
	if err != nil {
		return fmt.Errorf("初始化 CAS 客户端失败：%w", err)
	}
	if client == nil {
		return errors.New("AK/SK 不完整")
	}
	_, err = client.ListUserCertificateOrder(&sdk.ListUserCertificateOrderRequest{
		OrderType:   tea.String("CERT"),
		CurrentPage: tea.Int64(1),
		ShowSize:    tea.Int64(1),
	})
	return err
}

func (d *Driver) Deploy(_ context.Context, req acme.DeployRequest) (*acme.DeployResult, error) {
	target, err := targetFromDeployTarget(req.Target)
	if err != nil {
		return nil, err
	}
	state, err := stateFromGenericConfig(req.Config)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(req.Cert.FullchainPEM) == "" || strings.TrimSpace(req.Cert.KeyPEM) == "" {
		return nil, errors.New("当前证书内容不完整，无法上传到阿里云 CAS")
	}
	client, err := aliyuncas.NewClient(target.AccessKeyID, target.AccessKeySecret)
	if err != nil {
		return nil, fmt.Errorf("初始化 CAS 客户端失败：%w", err)
	}
	if client == nil {
		return nil, errors.New("阿里云 CAS AK/SK 不完整")
	}
	name := time.Now().Format("20060102150405")
	if req.Logf != nil {
		req.Logf("准备上传 CAS：name=%s", name)
	}
	resp, err := client.UploadUserCertificate(&sdk.UploadUserCertificateRequest{
		Name: tea.String(name),
		Cert: tea.String(req.Cert.FullchainPEM),
		Key:  tea.String(req.Cert.KeyPEM),
	})
	if err != nil {
		return nil, fmt.Errorf("上传 CAS 失败：%w", err)
	}
	if resp == nil || resp.Body == nil {
		return nil, errors.New("CAS 返回空 body")
	}
	id := tea.Int64Value(resp.Body.CertId)
	if id <= 0 {
		return nil, errors.New("CAS 未返回有效 cert_id")
	}
	if req.Logf != nil {
		req.Logf("已上传 CAS：cert_id=%d, name=%s", id, name)
	}
	state.CertID = id
	return &acme.DeployResult{StateJSON: acme.MustJSON(state)}, nil
}

func targetFromDeployTarget(target model.ACMEDeployTarget) (*model.ACMEUploadCASTarget, error) {
	auth := TargetAuth{}
	if err := acme.JSONUnmarshal([]byte(acme.EmptyJSON(target.AuthJSON)), &auth); err != nil {
		return nil, fmt.Errorf("解析阿里云 CAS 认证配置失败：%w", err)
	}
	out := &model.ACMEUploadCASTarget{
		ID:              target.ID,
		Name:            target.Name,
		AccessKeyID:     auth.AccessKeyID,
		AccessKeySecret: auth.AccessKeySecret,
		Enabled:         target.Enabled,
		CreatedAt:       target.CreatedAt,
		UpdatedAt:       target.UpdatedAt,
	}
	normalizeTarget(out)
	return out, nil
}

func deployTargetFromTarget(t model.ACMEUploadCASTarget) model.ACMEDeployTarget {
	normalizeTarget(&t)
	return model.ACMEDeployTarget{
		ID:   t.ID,
		Name: t.Name,
		Kind: acme.DeployKindUploadCAS,
		AuthJSON: acme.MustJSON(TargetAuth{
			AccessKeyID:     t.AccessKeyID,
			AccessKeySecret: t.AccessKeySecret,
		}),
		ConfigJSON: "{}",
		Enabled:    t.Enabled,
		CreatedAt:  t.CreatedAt,
		UpdatedAt:  t.UpdatedAt,
	}
}

func stateFromGenericConfig(cfg model.ACMEDeployConfig) (DeployState, error) {
	state := DeployState{}
	if err := acme.JSONUnmarshal([]byte(acme.EmptyJSON(cfg.StateJSON)), &state); err != nil {
		return state, fmt.Errorf("解析阿里云 CAS 部署状态失败：%w", err)
	}
	return state, nil
}

func genericConfigFromDeployConfig(cfg model.ACMEUploadCASDeployConfig) model.ACMEDeployConfig {
	normalizeDeployConfig(&cfg)
	state := DeployState{}
	if cfg.CertID > 0 {
		state.CertID = cfg.CertID
	}
	return model.ACMEDeployConfig{
		ID:         cfg.ID,
		DomainID:   cfg.DomainID,
		TargetID:   cfg.TargetID,
		Kind:       acme.DeployKindUploadCAS,
		Name:       cfg.Name,
		ConfigJSON: "{}",
		StateJSON:  acme.MustJSON(state),
		AutoDeploy: cfg.AutoDeploy,
		Enabled:    cfg.Enabled,
		CreatedAt:  cfg.CreatedAt,
		UpdatedAt:  cfg.UpdatedAt,
	}
}

func deployConfigFromGenericConfig(cfg model.ACMEDeployConfig) model.ACMEUploadCASDeployConfig {
	state, _ := stateFromGenericConfig(cfg)
	return model.ACMEUploadCASDeployConfig{
		ID:         cfg.ID,
		DomainID:   cfg.DomainID,
		TargetID:   cfg.TargetID,
		Name:       cfg.Name,
		CertID:     state.CertID,
		AutoDeploy: cfg.AutoDeploy,
		Enabled:    cfg.Enabled,
		CreatedAt:  cfg.CreatedAt,
		UpdatedAt:  cfg.UpdatedAt,
	}
}

func normalizeTarget(t *model.ACMEUploadCASTarget) {
	t.Name = strings.TrimSpace(t.Name)
	t.AccessKeyID = strings.TrimSpace(t.AccessKeyID)
	t.AccessKeySecret = strings.TrimSpace(t.AccessKeySecret)
}

func validateTarget(t model.ACMEUploadCASTarget) error {
	if t.Name == "" {
		return errors.New("阿里云 CAS 实例名称不能为空")
	}
	if t.AccessKeyID == "" {
		return errors.New("AccessKeyId 不能为空")
	}
	if t.AccessKeySecret == "" {
		return errors.New("AccessKeySecret 不能为空")
	}
	return nil
}

func normalizeDeployConfig(c *model.ACMEUploadCASDeployConfig) {
	c.Name = strings.TrimSpace(c.Name)
}
