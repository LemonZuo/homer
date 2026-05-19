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
	"github.com/LemonZuo/homer/internal/aliyun"
	"github.com/LemonZuo/homer/internal/model"
)

// Driver 实现 acme.DeployDriver，把证书上传到阿里云 CAS。
type Driver struct{}

// TargetAuth 是 acme_deploy_target.auth_json 在阿里云 CAS 场景下的结构。
type TargetAuth struct {
	AccessKeyID     string `json:"access_key_id"`
	AccessKeySecret string `json:"access_key_secret"`
}

// Target 是阿里云 CAS driver 从通用 ACMEDeployTarget 解析出的目标视图。
type Target struct {
	ID              int64
	Name            string
	AccessKeyID     string
	AccessKeySecret string
	Enabled         bool
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
	client, err := aliyun.NewCASClient(t.AccessKeyID, t.AccessKeySecret)
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
	client, err := aliyun.NewCASClient(target.AccessKeyID, target.AccessKeySecret)
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

func targetFromDeployTarget(target model.ACMEDeployTarget) (*Target, error) {
	auth := TargetAuth{}
	if err := acme.JSONUnmarshal([]byte(acme.EmptyJSON(target.AuthJSON)), &auth); err != nil {
		return nil, fmt.Errorf("解析阿里云 CAS 认证配置失败：%w", err)
	}
	out := &Target{
		ID:              target.ID,
		Name:            target.Name,
		AccessKeyID:     auth.AccessKeyID,
		AccessKeySecret: auth.AccessKeySecret,
		Enabled:         bool(target.Enabled),
	}
	normalizeTarget(out)
	return out, nil
}

func stateFromGenericConfig(cfg model.ACMEDeployConfig) (DeployState, error) {
	state := DeployState{}
	if err := acme.JSONUnmarshal([]byte(acme.EmptyJSON(cfg.StateJSON)), &state); err != nil {
		return state, fmt.Errorf("解析阿里云 CAS 部署状态失败：%w", err)
	}
	return state, nil
}

func normalizeTarget(t *Target) {
	t.Name = strings.TrimSpace(t.Name)
	t.AccessKeyID = strings.TrimSpace(t.AccessKeyID)
	t.AccessKeySecret = strings.TrimSpace(t.AccessKeySecret)
}

func validateTarget(t Target) error {
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
