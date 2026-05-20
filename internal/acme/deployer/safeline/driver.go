// Package acmesafeline 是 ACME 证书部署到雷池 WAF 的 driver 实现。
// 与 ssh 子包平行：core（internal/acme）只持有 DeployDriver 接口与通用 store。
package acmesafeline

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/LemonZuo/homer/internal/acme"
	"github.com/LemonZuo/homer/internal/model"
)

// Driver 实现 acme.DeployDriver，把证书上传/更新到雷池 WAF。
type Driver struct{}

// TargetAuth 是 acme_deploy_target.auth_json 在雷池场景下的结构。
type TargetAuth struct {
	APIToken string `json:"api_token"`
}

// TargetConfig 是 acme_deploy_target.config_json 在雷池场景下的结构。
type TargetConfig struct {
	SkipTLSVerify bool `json:"skip_tls_verify"`
}

// Target 是雷池 driver 从通用 ACMEDeployTarget 解析出的目标视图。
type Target struct {
	ID            int64
	Name          string
	BaseURL       string
	APIToken      string
	SkipTLSVerify bool
	Enabled       bool
}

// DeployOptions 是 acme_deploy_config.config_json 在雷池场景下的结构。
type DeployOptions struct {
	CertType int `json:"cert_type"`
}

// DeployState 是 acme_deploy_config.state_json，记录上次部署命中的雷池 cert_id。
type DeployState struct {
	CertID  int64   `json:"cert_id"`
	CertIDs []int64 `json:"cert_ids,omitempty"`
}

func NewDriver() *Driver { return &Driver{} }

func (d *Driver) Kind() string  { return acme.DeployKindSafeline }
func (d *Driver) Label() string { return "雷池 WAF" }

func (d *Driver) ValidateTarget(target model.ACMEDeployTarget) error {
	t, err := targetFromDeployTarget(target)
	if err != nil {
		return err
	}
	return validateTarget(*t)
}

func (d *Driver) ValidateConfig(_ model.ACMEDeployTarget, cfg model.ACMEDeployConfig) error {
	opts, _, err := deployPartsFromGenericConfig(cfg)
	if err != nil {
		return err
	}
	if opts.CertType <= 0 {
		return errors.New("雷池证书类型无效")
	}
	return nil
}

func (d *Driver) TestTarget(_ context.Context, target model.ACMEDeployTarget) error {
	t, err := targetFromDeployTarget(target)
	if err != nil {
		return err
	}
	_, err = newClient(*t).ListCerts()
	return err
}

func (d *Driver) Deploy(_ context.Context, req acme.DeployRequest) (*acme.DeployResult, error) {
	target, err := targetFromDeployTarget(req.Target)
	if err != nil {
		return nil, err
	}
	opts, state, err := deployPartsFromGenericConfig(req.Config)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(req.Cert.FullchainPEM) == "" || strings.TrimSpace(req.Cert.KeyPEM) == "" {
		return nil, errors.New("当前证书内容不完整，无法上传到雷池")
	}
	domains := acme.BuildDomains(req.Domain)
	if len(domains) == 0 {
		return nil, errors.New("没有可用于匹配雷池证书的域名")
	}
	client := newClient(*target)
	if req.Logf != nil {
		req.Logf("雷池地址：%s", target.BaseURL)
	}

	var updatedIDs []int64
	if state.CertID > 0 {
		updatedIDs, err = deployByCertID(client, opts, state.CertID, req)
	} else {
		updatedIDs, err = deployByDomains(client, opts, domains, req)
	}
	if err != nil {
		return nil, err
	}
	state.CertID = updatedIDs[0]
	state.CertIDs = updatedIDs
	return &acme.DeployResult{StateJSON: acme.MustJSON(state)}, nil
}

// deployByCertID 走"已知 cert_id"路径：直接更新指定证书，不做域名反查。
func deployByCertID(client *client, opts DeployOptions, certID int64, req acme.DeployRequest) ([]int64, error) {
	if req.Logf != nil {
		req.Logf("已指定雷池证书 cert_id=%d，直接更新", certID)
	}
	id, err := client.UpsertCert(certID, opts.CertType, req.Cert.FullchainPEM, req.Cert.KeyPEM)
	if err != nil {
		return nil, fmt.Errorf("更新雷池证书 cert_id=%d 失败：%w", certID, err)
	}
	if id <= 0 {
		id = certID
	}
	if req.Logf != nil {
		req.Logf("雷池证书更新成功：cert_id=%d", id)
	}
	return []int64{id}, nil
}

// deployByDomains 按域名反查雷池现有证书：命中则逐个更新，未命中则新增。
func deployByDomains(client *client, opts DeployOptions, domains []string, req acme.DeployRequest) ([]int64, error) {
	if req.Logf != nil {
		req.Logf("匹配域名：%s", strings.Join(domains, ", "))
	}
	certs, err := client.ListCerts()
	if err != nil {
		return nil, fmt.Errorf("获取雷池证书列表失败：%w", err)
	}
	if req.Logf != nil {
		req.Logf("获取雷池证书列表成功：total=%d", certs.Total)
	}
	matches := matchCerts(certs.Nodes, domains)
	updatedIDs := make([]int64, 0, len(matches))
	for _, item := range matches {
		id, err := client.UpsertCert(item.ID, opts.CertType, req.Cert.FullchainPEM, req.Cert.KeyPEM)
		if err != nil {
			return nil, fmt.Errorf("更新雷池证书 cert_id=%d 失败：%w", item.ID, err)
		}
		if id <= 0 {
			id = item.ID
		}
		updatedIDs = append(updatedIDs, id)
		if req.Logf != nil {
			req.Logf("雷池证书更新成功：cert_id=%d，domains=%s", id, strings.Join(item.Domains, ", "))
		}
	}
	if len(updatedIDs) > 0 {
		return updatedIDs, nil
	}
	id, err := client.UpsertCert(0, opts.CertType, req.Cert.FullchainPEM, req.Cert.KeyPEM)
	if err != nil {
		return nil, fmt.Errorf("新增雷池证书失败：%w", err)
	}
	if id <= 0 {
		return nil, errors.New("雷池未返回有效 cert_id")
	}
	if req.Logf != nil {
		req.Logf("没有匹配到雷池已有证书，已新增证书：cert_id=%d", id)
	}
	return []int64{id}, nil
}

func matchCerts(items []certItem, domains []string) []certItem {
	wanted := make(map[string]struct{}, len(domains))
	for _, domain := range domains {
		domain = strings.ToLower(strings.TrimSpace(domain))
		if domain != "" {
			wanted[domain] = struct{}{}
		}
	}
	out := make([]certItem, 0)
	for _, item := range items {
		if len(item.Domains) == 0 {
			continue
		}
		for _, domain := range item.Domains {
			domain = strings.ToLower(strings.TrimSpace(domain))
			if domain == "" {
				continue
			}
			if _, ok := wanted[domain]; ok {
				out = append(out, item)
				break
			}
			if wildcard := wildcardDomain(domain); wildcard != "" {
				if _, ok := wanted[wildcard]; ok {
					out = append(out, item)
					break
				}
			}
		}
	}
	return out
}

func wildcardDomain(domain string) string {
	i := strings.Index(domain, ".")
	if i < 0 || i == len(domain)-1 {
		return ""
	}
	return "*" + domain[i:]
}

func targetFromDeployTarget(target model.ACMEDeployTarget) (*Target, error) {
	auth := TargetAuth{}
	cfg := TargetConfig{}
	if err := acme.JSONUnmarshal([]byte(acme.EmptyJSON(target.AuthJSON)), &auth); err != nil {
		return nil, fmt.Errorf("解析雷池认证配置失败：%w", err)
	}
	if err := acme.JSONUnmarshal([]byte(acme.EmptyJSON(target.ConfigJSON)), &cfg); err != nil {
		return nil, fmt.Errorf("解析雷池目标配置失败：%w", err)
	}
	out := &Target{
		ID:            target.ID,
		Name:          target.Name,
		BaseURL:       target.Endpoint,
		APIToken:      auth.APIToken,
		SkipTLSVerify: cfg.SkipTLSVerify,
		Enabled:       bool(target.Enabled),
	}
	normalizeTarget(out)
	return out, nil
}

func deployPartsFromGenericConfig(cfg model.ACMEDeployConfig) (DeployOptions, DeployState, error) {
	opts := DeployOptions{CertType: 2}
	state := DeployState{}
	if err := acme.JSONUnmarshal([]byte(acme.EmptyJSON(cfg.ConfigJSON)), &opts); err != nil {
		return opts, state, fmt.Errorf("解析雷池部署配置失败：%w", err)
	}
	if opts.CertType <= 0 {
		opts.CertType = 2
	}
	if err := acme.JSONUnmarshal([]byte(acme.EmptyJSON(cfg.StateJSON)), &state); err != nil {
		return opts, state, fmt.Errorf("解析雷池部署状态失败：%w", err)
	}
	return opts, state, nil
}

func normalizeTarget(t *Target) {
	t.Name = strings.TrimSpace(t.Name)
	t.BaseURL = strings.TrimRight(strings.TrimSpace(t.BaseURL), "/")
	t.APIToken = strings.TrimSpace(t.APIToken)
}

func validateTarget(t Target) error {
	if t.Name == "" {
		return errors.New("雷池实例名称不能为空")
	}
	if t.BaseURL == "" {
		return errors.New("雷池地址不能为空")
	}
	if !strings.HasPrefix(t.BaseURL, "http://") && !strings.HasPrefix(t.BaseURL, "https://") {
		return errors.New("雷池地址需要以 http:// 或 https:// 开头")
	}
	if t.APIToken == "" {
		return errors.New("雷池 API Token 不能为空")
	}
	return nil
}
