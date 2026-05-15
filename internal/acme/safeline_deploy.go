package acme

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/LemonZuo/homer/internal/model"
)

type SafelineDeployDriver struct{}

type SafelineTargetAuth struct {
	APIToken string `json:"api_token"`
}

type SafelineTargetConfig struct {
	SkipTLSVerify bool `json:"skip_tls_verify"`
}

type SafelineDeployOptions struct {
	CertType int `json:"cert_type"`
}

type SafelineDeployState struct {
	CertID  int64   `json:"cert_id"`
	CertIDs []int64 `json:"cert_ids,omitempty"`
}

func NewSafelineDeployDriver() *SafelineDeployDriver { return &SafelineDeployDriver{} }

func (d *SafelineDeployDriver) Kind() string  { return DeployKindSafeline }
func (d *SafelineDeployDriver) Label() string { return "雷池 WAF" }

func (d *SafelineDeployDriver) ValidateTarget(target model.ACMEDeployTarget) error {
	t, err := safelineTargetFromDeployTarget(target)
	if err != nil {
		return err
	}
	return validateSafelineTarget(*t)
}

func (d *SafelineDeployDriver) ValidateConfig(_ model.ACMEDeployTarget, cfg model.ACMEDeployConfig) error {
	opts, _, err := safelineDeployPartsFromGenericConfig(cfg)
	if err != nil {
		return err
	}
	if opts.CertType <= 0 {
		return errors.New("雷池证书类型无效")
	}
	return nil
}

func (d *SafelineDeployDriver) TestTarget(_ context.Context, target model.ACMEDeployTarget) error {
	t, err := safelineTargetFromDeployTarget(target)
	if err != nil {
		return err
	}
	_, err = newSafelineClient(*t).ListCerts()
	return err
}

func (d *SafelineDeployDriver) Deploy(_ context.Context, req DeployRequest) (*DeployResult, error) {
	target, err := safelineTargetFromDeployTarget(req.Target)
	if err != nil {
		return nil, err
	}
	opts, state, err := safelineDeployPartsFromGenericConfig(req.Config)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(req.Cert.FullchainPEM) == "" || strings.TrimSpace(req.Cert.KeyPEM) == "" {
		return nil, errors.New("当前证书内容不完整，无法上传到雷池")
	}
	domains := buildDomains(req.Domain)
	if len(domains) == 0 {
		return nil, errors.New("没有可用于匹配雷池证书的域名")
	}
	client := newSafelineClient(*target)
	if req.Logf != nil {
		req.Logf("雷池地址：%s", target.BaseURL)
		req.Logf("匹配域名：%s", strings.Join(domains, ", "))
	}

	certs, err := client.ListCerts()
	if err != nil {
		return nil, fmt.Errorf("获取雷池证书列表失败：%w", err)
	}
	if req.Logf != nil {
		req.Logf("获取雷池证书列表成功：total=%d", certs.Total)
	}
	matches := matchSafelineCerts(certs.Nodes, domains)
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
	if len(updatedIDs) == 0 {
		id, err := client.CreateCert(opts.CertType, req.Cert.FullchainPEM, req.Cert.KeyPEM)
		if err != nil {
			return nil, fmt.Errorf("新增雷池证书失败：%w", err)
		}
		if id <= 0 {
			return nil, errors.New("雷池未返回有效 cert_id")
		}
		updatedIDs = append(updatedIDs, id)
		if req.Logf != nil {
			req.Logf("没有匹配到雷池已有证书，已新增证书：cert_id=%d", id)
		}
	}
	state.CertID = updatedIDs[0]
	state.CertIDs = updatedIDs
	return &DeployResult{StateJSON: mustJSON(state)}, nil
}

func matchSafelineCerts(items []safelineCertItem, domains []string) []safelineCertItem {
	wanted := make(map[string]struct{}, len(domains))
	for _, domain := range domains {
		domain = strings.ToLower(strings.TrimSpace(domain))
		if domain != "" {
			wanted[domain] = struct{}{}
		}
	}
	out := make([]safelineCertItem, 0)
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
			if wildcard := safelineWildcardDomain(domain); wildcard != "" {
				if _, ok := wanted[wildcard]; ok {
					out = append(out, item)
					break
				}
			}
		}
	}
	return out
}

func safelineWildcardDomain(domain string) string {
	i := strings.Index(domain, ".")
	if i < 0 || i == len(domain)-1 {
		return ""
	}
	return "*" + domain[i:]
}

func safelineTargetFromDeployTarget(target model.ACMEDeployTarget) (*model.ACMESafelineTarget, error) {
	auth := SafelineTargetAuth{}
	cfg := SafelineTargetConfig{}
	if err := jsonUnmarshal([]byte(emptyJSON(target.AuthJSON)), &auth); err != nil {
		return nil, fmt.Errorf("解析雷池认证配置失败：%w", err)
	}
	if err := jsonUnmarshal([]byte(emptyJSON(target.ConfigJSON)), &cfg); err != nil {
		return nil, fmt.Errorf("解析雷池目标配置失败：%w", err)
	}
	out := &model.ACMESafelineTarget{
		ID:            target.ID,
		Name:          target.Name,
		BaseURL:       target.Endpoint,
		APIToken:      auth.APIToken,
		SkipTLSVerify: model.BoolFlag(cfg.SkipTLSVerify),
		Enabled:       target.Enabled,
		CreatedAt:     target.CreatedAt,
		UpdatedAt:     target.UpdatedAt,
	}
	normalizeSafelineTarget(out)
	return out, nil
}

func deployTargetFromSafelineTarget(t model.ACMESafelineTarget) model.ACMEDeployTarget {
	normalizeSafelineTarget(&t)
	return model.ACMEDeployTarget{
		ID:       t.ID,
		Name:     t.Name,
		Kind:     DeployKindSafeline,
		Endpoint: t.BaseURL,
		AuthJSON: mustJSON(SafelineTargetAuth{
			APIToken: t.APIToken,
		}),
		ConfigJSON: mustJSON(SafelineTargetConfig{
			SkipTLSVerify: bool(t.SkipTLSVerify),
		}),
		Enabled:   t.Enabled,
		CreatedAt: t.CreatedAt,
		UpdatedAt: t.UpdatedAt,
	}
}

func safelineDeployPartsFromGenericConfig(cfg model.ACMEDeployConfig) (SafelineDeployOptions, SafelineDeployState, error) {
	opts := SafelineDeployOptions{CertType: 2}
	state := SafelineDeployState{}
	if err := jsonUnmarshal([]byte(emptyJSON(cfg.ConfigJSON)), &opts); err != nil {
		return opts, state, fmt.Errorf("解析雷池部署配置失败：%w", err)
	}
	if opts.CertType <= 0 {
		opts.CertType = 2
	}
	if err := jsonUnmarshal([]byte(emptyJSON(cfg.StateJSON)), &state); err != nil {
		return opts, state, fmt.Errorf("解析雷池部署状态失败：%w", err)
	}
	return opts, state, nil
}

func genericConfigFromSafelineDeployConfig(cfg model.ACMESafelineDeployConfig) model.ACMEDeployConfig {
	normalizeSafelineDeployConfig(&cfg)
	state := SafelineDeployState{}
	if cfg.CertID > 0 {
		state.CertID = cfg.CertID
		state.CertIDs = []int64{cfg.CertID}
	}
	return model.ACMEDeployConfig{
		ID:       cfg.ID,
		DomainID: cfg.DomainID,
		TargetID: cfg.TargetID,
		Kind:     DeployKindSafeline,
		Name:     cfg.Name,
		ConfigJSON: mustJSON(SafelineDeployOptions{
			CertType: cfg.CertType,
		}),
		StateJSON:  mustJSON(state),
		AutoDeploy: cfg.AutoDeploy,
		Enabled:    cfg.Enabled,
		CreatedAt:  cfg.CreatedAt,
		UpdatedAt:  cfg.UpdatedAt,
	}
}

func safelineDeployConfigFromGenericConfig(cfg model.ACMEDeployConfig) model.ACMESafelineDeployConfig {
	opts, state, _ := safelineDeployPartsFromGenericConfig(cfg)
	return model.ACMESafelineDeployConfig{
		ID:         cfg.ID,
		DomainID:   cfg.DomainID,
		TargetID:   cfg.TargetID,
		Name:       cfg.Name,
		CertID:     state.CertID,
		CertType:   opts.CertType,
		AutoDeploy: cfg.AutoDeploy,
		Enabled:    cfg.Enabled,
		CreatedAt:  cfg.CreatedAt,
		UpdatedAt:  cfg.UpdatedAt,
	}
}

func normalizeSafelineTarget(t *model.ACMESafelineTarget) {
	t.Name = strings.TrimSpace(t.Name)
	t.BaseURL = strings.TrimRight(strings.TrimSpace(t.BaseURL), "/")
	t.APIToken = strings.TrimSpace(t.APIToken)
}

func validateSafelineTarget(t model.ACMESafelineTarget) error {
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

func normalizeSafelineDeployConfig(c *model.ACMESafelineDeployConfig) {
	c.Name = strings.TrimSpace(c.Name)
	if c.CertType <= 0 {
		c.CertType = 2
	}
}

func safelineDeployConfigLabel(c model.ACMESafelineDeployConfig) string {
	if strings.TrimSpace(c.Name) != "" {
		return c.Name
	}
	return fmt.Sprintf("#%d", c.ID)
}

var (
	ErrSafelineTargetNotConfigured       = errors.New("雷池实例未配置")
	ErrSafelineDeployConfigNotConfigured = errors.New("雷池部署配置未配置")
)

// SafelineTargetStore 是旧 HTTP/UI 形态的兼容层，实际读写 acme_deploy_target。
type SafelineTargetStore struct {
	targets *DeployTargetStore
}

func NewSafelineTargetStore(targets *DeployTargetStore) *SafelineTargetStore {
	return &SafelineTargetStore{targets: targets}
}

func (s *SafelineTargetStore) List() ([]model.ACMESafelineTarget, error) {
	rows, err := s.targets.List(DeployKindSafeline)
	if err != nil {
		return nil, err
	}
	out := make([]model.ACMESafelineTarget, 0, len(rows))
	for _, row := range rows {
		t, err := safelineTargetFromDeployTarget(row)
		if err != nil {
			continue
		}
		out = append(out, *t)
	}
	return out, nil
}

func (s *SafelineTargetStore) Get(id int64) (*model.ACMESafelineTarget, error) {
	row, err := s.targets.Get(id)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrSafelineTargetNotConfigured, err)
	}
	if row.Kind != DeployKindSafeline {
		return nil, fmt.Errorf("%w: id=%d 类型不是雷池", ErrSafelineTargetNotConfigured, id)
	}
	return safelineTargetFromDeployTarget(*row)
}

func (s *SafelineTargetStore) Upsert(t *model.ACMESafelineTarget) (*model.ACMESafelineTarget, error) {
	if t == nil {
		return nil, errors.New("雷池实例不能为空")
	}
	row := deployTargetFromSafelineTarget(*t)
	saved, err := s.targets.Upsert(&row)
	if err != nil {
		return nil, err
	}
	return safelineTargetFromDeployTarget(*saved)
}

func (s *SafelineTargetStore) Delete(id int64) error {
	return s.targets.Delete(id)
}

func (s *SafelineTargetStore) Test(id int64) error {
	return s.targets.Test(context.Background(), id)
}

// SafelineDeployConfigStore 是旧 HTTP/UI 形态的兼容层，实际读写 acme_deploy_config。
type SafelineDeployConfigStore struct {
	configs *DeployConfigStore
}

func NewSafelineDeployConfigStore(configs *DeployConfigStore) *SafelineDeployConfigStore {
	return &SafelineDeployConfigStore{configs: configs}
}

func (s *SafelineDeployConfigStore) ListByDomain(domainID int64) ([]model.ACMESafelineDeployConfig, error) {
	rows, err := s.configs.ListByDomain(domainID, DeployKindSafeline)
	if err != nil {
		return nil, err
	}
	out := make([]model.ACMESafelineDeployConfig, 0, len(rows))
	for _, row := range rows {
		out = append(out, safelineDeployConfigFromGenericConfig(row))
	}
	return out, nil
}

func (s *SafelineDeployConfigStore) ListAutoByDomain(domainID int64) ([]model.ACMESafelineDeployConfig, error) {
	rows, err := s.configs.ListAutoByDomain(domainID, DeployKindSafeline)
	if err != nil {
		return nil, err
	}
	out := make([]model.ACMESafelineDeployConfig, 0, len(rows))
	for _, row := range rows {
		out = append(out, safelineDeployConfigFromGenericConfig(row))
	}
	return out, nil
}

func (s *SafelineDeployConfigStore) Get(id int64) (*model.ACMESafelineDeployConfig, error) {
	row, err := s.configs.Get(id)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrSafelineDeployConfigNotConfigured, err)
	}
	if row.Kind != DeployKindSafeline {
		return nil, fmt.Errorf("%w: id=%d 类型不是雷池", ErrSafelineDeployConfigNotConfigured, id)
	}
	cfg := safelineDeployConfigFromGenericConfig(*row)
	return &cfg, nil
}

func (s *SafelineDeployConfigStore) Upsert(domainID int64, c *model.ACMESafelineDeployConfig) (*model.ACMESafelineDeployConfig, error) {
	if c == nil {
		return nil, errors.New("雷池部署配置不能为空")
	}
	row := genericConfigFromSafelineDeployConfig(*c)
	saved, err := s.configs.Upsert(domainID, &row)
	if err != nil {
		return nil, err
	}
	cfg := safelineDeployConfigFromGenericConfig(*saved)
	return &cfg, nil
}

func (s *SafelineDeployConfigStore) Delete(id int64) error {
	return s.configs.Delete(id)
}
