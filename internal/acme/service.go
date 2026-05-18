package acme

import (
	"bytes"
	"context"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/LemonZuo/homer/internal/logx"
	"github.com/LemonZuo/homer/internal/model"
	"gorm.io/gorm"
)

// Service ACME 业务编排：域名 CRUD、签发、续期、落盘、部署、SSE 日志。
type Service struct {
	db             *gorm.DB
	manager        *Manager
	credstore      *CredentialStore
	sshCredstore   *SSHCredentialStore
	accountStore   *AccountStore
	deployTargets  *DeployTargetStore
	deployConfigs  *DeployConfigStore
	deployRegistry *DeployRegistry
	hub            *SSEHub
	dataDir        string
	renewDays      int

	deployRetry        int           // 部署任务允许总执行次数（含首次），1=不重试
	deployRetryBackoff time.Duration // 退避基数，实际间隔 = backoff * 已执行次数

	issueMu sync.Mutex // 串行化签发（lego logger / env 是全局状态）
}

func NewService(db *gorm.DB, mgr *Manager, store *CredentialStore, sshCreds *SSHCredentialStore, accounts *AccountStore, deployTargets *DeployTargetStore, deployConfigs *DeployConfigStore, deployRegistry *DeployRegistry, hub *SSEHub, dataDir string, renewDays int, deployRetry int, deployRetryBackoff time.Duration) *Service {
	if deployRetry < 1 {
		deployRetry = 1
	}
	return &Service{db: db, manager: mgr, credstore: store, sshCredstore: sshCreds, accountStore: accounts, deployTargets: deployTargets, deployConfigs: deployConfigs, deployRegistry: deployRegistry, hub: hub, dataDir: dataDir, renewDays: renewDays, deployRetry: deployRetry, deployRetryBackoff: deployRetryBackoff}
}

func (s *Service) Hub() *SSEHub                        { return s.hub }
func (s *Service) Credentials() *CredentialStore       { return s.credstore }
func (s *Service) SSHCredentials() *SSHCredentialStore { return s.sshCredstore }
func (s *Service) Accounts() *AccountStore             { return s.accountStore }
func (s *Service) DeployTargets() *DeployTargetStore   { return s.deployTargets }
func (s *Service) DeployConfigs() *DeployConfigStore   { return s.deployConfigs }

// DomainView 联查域名 + 最近一次证书（NotAfter 用于前端显示剩余天数）。
type DomainView struct {
	model.ACMEDomain
	NotAfter   *time.Time `json:"not_after,omitempty"`
	NotBefore  *time.Time `json:"not_before,omitempty"`
	CertStatus string     `json:"cert_status,omitempty"`
	RevokedAt  *time.Time `json:"revoked_at,omitempty"`
	IssuedAt   *time.Time `json:"issued_at,omitempty"`
}

// ListDomains 列出所有域名（按 id 升序），附带最近一次证书摘要。
func (s *Service) ListDomains() ([]DomainView, error) {
	var items []model.ACMEDomain
	if err := s.db.Order("id ASC").Find(&items).Error; err != nil {
		return nil, err
	}
	out := make([]DomainView, 0, len(items))
	for _, d := range items {
		v := DomainView{ACMEDomain: d}
		var c model.ACMECert
		if err := s.db.Where("domain_id = ?", d.ID).First(&c).Error; err == nil {
			na := c.NotAfter
			nb := c.NotBefore
			ia := c.IssuedAt
			v.NotAfter = &na
			v.NotBefore = &nb
			v.IssuedAt = &ia
			v.CertStatus = c.Status
			v.RevokedAt = c.RevokedAt
		}
		out = append(out, v)
	}
	return out, nil
}

// normalizeSanProviders 校验并归一化 san_providers JSON；空串保持空串。
func normalizeSanProviders(s string) (string, error) {
	if strings.TrimSpace(s) == "" {
		return "", nil
	}
	m := map[string]string{}
	if err := JSONUnmarshal([]byte(s), &m); err != nil {
		return "", errors.New("san_providers 不是合法的 JSON 对象")
	}
	out := map[string]string{}
	for k, v := range m {
		k = strings.TrimSpace(k)
		v = strings.TrimSpace(v)
		if k != "" && v != "" {
			out[k] = v
		}
	}
	if len(out) == 0 {
		return "", nil
	}
	b, err := JSONMarshalIndent(out)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// CreateDomain 新增域名。
func (s *Service) CreateDomain(d *model.ACMEDomain) error {
	d.MainDomain = strings.TrimSpace(d.MainDomain)
	d.Provider = strings.TrimSpace(d.Provider)
	if d.MainDomain == "" || d.Provider == "" {
		return errors.New("main_domain 与 provider 必填")
	}
	sp, err := normalizeSanProviders(d.SanProviders)
	if err != nil {
		return err
	}
	d.SanProviders = sp
	if _, err := s.accountStore.Get(d.AccountID); err != nil {
		return err
	}
	return s.db.Create(d).Error
}

// UpdateDomain 更新域名（按 id）。
func (s *Service) UpdateDomain(d *model.ACMEDomain) error {
	if d.ID == 0 {
		return errors.New("id 必填")
	}
	d.MainDomain = strings.TrimSpace(d.MainDomain)
	d.Provider = strings.TrimSpace(d.Provider)
	if d.MainDomain == "" || d.Provider == "" {
		return errors.New("main_domain 与 provider 必填")
	}
	sp, err := normalizeSanProviders(d.SanProviders)
	if err != nil {
		return err
	}
	d.SanProviders = sp
	if _, err := s.accountStore.Get(d.AccountID); err != nil {
		return err
	}
	return s.db.Save(d).Error
}

// DeleteDomain 删除域名及其证书/任务流水。
func (s *Service) DeleteDomain(id int64) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("domain_id = ?", id).Delete(&model.ACMECert{}).Error; err != nil {
			return err
		}
		if err := tx.Where("domain_id = ?", id).Delete(&model.ACMEDeployConfig{}).Error; err != nil {
			return err
		}
		if err := tx.Where("domain_id = ?", id).Delete(&model.ACMEIssueTask{}).Error; err != nil {
			return err
		}
		return tx.Delete(&model.ACMEDomain{}, id).Error
	})
}

// GetCertByDomain 返回最近一次签发的证书（空时 nil, nil）。
func (s *Service) GetCertByDomain(domainID int64) (*model.ACMECert, error) {
	var c model.ACMECert
	if err := s.db.Where("domain_id = ?", domainID).First(&c).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &c, nil
}

// ListTasks 任务流水分页（按 id 倒序），返回当前页数据与总条数。
// status 非空时按状态过滤（pending|running|success|failed|retrying）。
func (s *Service) ListTasks(page, pageSize int, status string) ([]model.ACMEIssueTask, int64, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 10
	}
	q := s.db.Model(&model.ACMEIssueTask{})
	if status != "" {
		q = q.Where("status = ?", status)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []model.ACMEIssueTask
	if err := q.Order("id DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// GetTask 单条任务详情（用于前端在 SSE 关闭后拉全量日志）。
func (s *Service) GetTask(id int64) (*model.ACMEIssueTask, error) {
	var t model.ACMEIssueTask
	if err := s.db.First(&t, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &t, nil
}

// IssueAsync 异步触发签发/续期；立即返回 taskID。
// 调用方（HTTP handler）应订阅 hub.Subscribe(taskID) 拿实时日志，
// 任务结束时 hub 关闭对应 channel，前端再拉 GET /tasks/:id 取全文。
func (s *Service) IssueAsync(domainID int64, kind string) (int64, error) {
	var d model.ACMEDomain
	if err := s.db.First(&d, domainID).Error; err != nil {
		return 0, err
	}
	if kind != "issue" && kind != "renew" {
		kind = "issue"
	}
	task := &model.ACMEIssueTask{
		DomainID:   d.ID,
		MainDomain: d.MainDomain,
		Kind:       kind,
		Status:     "pending",
	}
	if err := s.db.Create(task).Error; err != nil {
		return 0, err
	}
	go s.runIssue(task.ID, d)
	return task.ID, nil
}

// RevokeAsync 异步吊销当前域名最近一次证书。
func (s *Service) RevokeAsync(domainID int64) (int64, error) {
	var d model.ACMEDomain
	if err := s.db.First(&d, domainID).Error; err != nil {
		return 0, err
	}
	cert, err := s.GetCertByDomain(domainID)
	if err != nil {
		return 0, err
	}
	if cert == nil {
		return 0, errors.New("当前域名还没有可吊销的证书")
	}
	if cert.Status == "revoked" {
		return 0, errors.New("当前证书已吊销")
	}
	if strings.TrimSpace(cert.CertPEM) == "" {
		return 0, errors.New("当前证书内容为空，无法吊销")
	}
	task := &model.ACMEIssueTask{
		DomainID:   d.ID,
		MainDomain: d.MainDomain,
		Kind:       "revoke",
		Status:     "pending",
	}
	if err := s.db.Create(task).Error; err != nil {
		return 0, err
	}
	go s.runRevoke(task.ID, d, *cert)
	return task.ID, nil
}

// runIssue 在 goroutine 里跑实际签发流程。
func (s *Service) runIssue(taskID int64, d model.ACMEDomain) {
	s.issueMu.Lock()
	defer s.issueMu.Unlock()

	// 标记 running
	_ = s.db.Model(&model.ACMEIssueTask{}).Where("id = ?", taskID).
		Updates(map[string]any{"status": "running"}).Error

	logBuf := &bytes.Buffer{}
	logw := &teeWriter{buf: logBuf, hub: s.hub, taskID: taskID}

	logx.Info("acme issue start", "task", taskID, "domain", d.MainDomain, "provider", d.Provider)
	logf(logw, "开始签发：%s（provider=%s）", d.MainDomain, d.Provider)
	domains := BuildDomains(d)
	logf(logw, "目标域名：%s", strings.Join(domains, ", "))
	sanProviders := ParseSanProviders(d)
	if len(sanProviders) > 0 {
		parts := make([]string, 0, len(sanProviders))
		for dom, pv := range sanProviders {
			parts = append(parts, fmt.Sprintf("%s→%s", dom, pv))
		}
		logf(logw, "按域名指定 provider：%s", strings.Join(parts, ", "))
	}

	err := func() error {
		account, err := s.accountStore.Get(d.AccountID)
		if err != nil {
			return err
		}
		logf(logw, "ACME 账号：%s（%s）", account.Name, account.CA)
		client, err := s.manager.newClient(optionsFromAccount(*account), logw)
		if err != nil {
			return fmt.Errorf("初始化 lego 失败：%w", err)
		}
		res, err := client.Obtain(domains, d.Provider, sanProviders, s.credstore)
		if err != nil {
			return err
		}
		rec, err := s.persistCert(logw, d, res.Certificate, res.PrivateKey, res.IssuerCertificate)
		if err != nil {
			return err
		}
		s.submitAutoDeploy(logw, d, *rec)
		return nil
	}()

	finish := time.Now()
	upd := map[string]any{
		"finished_at": &finish,
		"log_text":    logBuf.String(),
	}
	if err != nil {
		logf(logw, "签发失败：%v", err)
		logx.Error("acme issue failed", "task", taskID, "domain", d.MainDomain, "err", err)
		upd["status"] = "failed"
		upd["error_msg"] = truncate(err.Error(), 1000)
		// 重新写一次 log_text 以包含错误
		upd["log_text"] = logBuf.String()
	} else {
		logf(logw, "签发完成")
		logx.Info("acme issue done", "task", taskID, "domain", d.MainDomain)
		upd["status"] = "success"
		upd["log_text"] = logBuf.String()
	}
	_ = s.db.Model(&model.ACMEIssueTask{}).Where("id = ?", taskID).Updates(upd).Error
	s.hub.Close(taskID)
}

// runRevoke 在 goroutine 里向 CA 吊销当前证书。
func (s *Service) runRevoke(taskID int64, d model.ACMEDomain, cert model.ACMECert) {
	s.issueMu.Lock()
	defer s.issueMu.Unlock()

	_ = s.db.Model(&model.ACMEIssueTask{}).Where("id = ?", taskID).
		Updates(map[string]any{"status": "running"}).Error

	logBuf := &bytes.Buffer{}
	logw := &teeWriter{buf: logBuf, hub: s.hub, taskID: taskID}

	logx.Info("acme revoke start", "task", taskID, "domain", d.MainDomain, "serial", cert.Serial)
	logf(logw, "开始吊销证书：%s", d.MainDomain)
	if cert.Serial != "" {
		logf(logw, "证书序列号：%s", cert.Serial)
	}

	err := func() error {
		account, err := s.accountStore.Get(d.AccountID)
		if err != nil {
			return err
		}
		logf(logw, "ACME 账号：%s（%s）", account.Name, account.CA)
		client, err := s.manager.newClient(optionsFromAccount(*account), logw)
		if err != nil {
			return fmt.Errorf("初始化 lego 失败：%w", err)
		}
		if err := client.Revoke([]byte(cert.CertPEM)); err != nil {
			return fmt.Errorf("吊销证书失败：%w", err)
		}
		now := time.Now()
		if err := s.db.Model(&model.ACMECert{}).Where("id = ?", cert.ID).Updates(map[string]any{
			"status":     "revoked",
			"revoked_at": &now,
		}).Error; err != nil {
			return fmt.Errorf("更新证书吊销状态失败：%w", err)
		}
		logf(logw, "证书已被 CA 接受吊销")
		return nil
	}()

	finish := time.Now()
	upd := map[string]any{
		"finished_at": &finish,
		"log_text":    logBuf.String(),
	}
	if err != nil {
		logf(logw, "吊销失败：%v", err)
		logx.Error("acme revoke failed", "task", taskID, "domain", d.MainDomain, "err", err)
		upd["status"] = "failed"
		upd["error_msg"] = truncate(err.Error(), 1000)
		upd["log_text"] = logBuf.String()
	} else {
		logf(logw, "吊销完成")
		logx.Info("acme revoke done", "task", taskID, "domain", d.MainDomain)
		upd["status"] = "success"
		upd["log_text"] = logBuf.String()
	}
	_ = s.db.Model(&model.ACMEIssueTask{}).Where("id = ?", taskID).Updates(upd).Error
	s.hub.Close(taskID)
}

// persistCert 落盘到 ./data/acme/certs/<domain>/，写入 acme_cert 表。
func (s *Service) persistCert(logw *teeWriter, d model.ACMEDomain, cert, key, chain []byte) (*model.ACMECert, error) {
	notBefore, notAfter, serial := parseCertMeta(cert)
	// 目录加 ID 后缀，避免「同一主域名多张证书」时互相覆盖
	dir := filepath.Join(s.dataDir, "certs", fmt.Sprintf("%s-%d", d.MainDomain, d.ID))
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("创建证书目录失败：%w", err)
	}
	full := assembleFullchain(cert, chain)
	files := map[string][]byte{
		"cert.pem":      cert,
		"chain.pem":     chain,
		"fullchain.pem": full,
		"key.pem":       key,
	}
	for name, data := range files {
		mode := os.FileMode(0o644)
		if name == "key.pem" {
			mode = 0o600
		}
		if err := os.WriteFile(filepath.Join(dir, name), data, mode); err != nil {
			return nil, fmt.Errorf("写入 %s 失败：%w", name, err)
		}
	}
	logf(logw, "证书已落盘：%s", dir)

	rec := &model.ACMECert{
		DomainID:     d.ID,
		CertPEM:      string(cert),
		KeyPEM:       string(key),
		ChainPEM:     string(chain),
		FullchainPEM: string(full),
		Serial:       serial,
		NotBefore:    notBefore,
		NotAfter:     notAfter,
		Status:       "active",
		RevokedAt:    nil,
	}

	// upsert：同一 domain 只保留最近一条
	var existing model.ACMECert
	if err := s.db.Where("domain_id = ?", d.ID).First(&existing).Error; err == nil {
		rec.ID = existing.ID
		rec.IssuedAt = time.Now()
		if err := s.db.Save(rec).Error; err != nil {
			return nil, fmt.Errorf("保存证书记录失败：%w", err)
		}
	} else if errors.Is(err, gorm.ErrRecordNotFound) {
		if err := s.db.Create(rec).Error; err != nil {
			return nil, fmt.Errorf("保存证书记录失败：%w", err)
		}
	} else {
		return nil, fmt.Errorf("查询证书记录失败：%w", err)
	}

	return rec, nil
}

// DeployConfigTaskAsync 异步按保存的部署配置发布当前域名最近一次证书。
func (s *Service) DeployConfigTaskAsync(configID int64) (int64, error) {
	cfg, err := s.deployConfigs.Get(configID)
	if err != nil {
		return 0, err
	}
	d, cert, target, err := s.prepareDeploy(*cfg)
	if err != nil {
		return 0, err
	}
	task := &model.ACMEIssueTask{
		DomainID:   d.ID,
		MainDomain: d.MainDomain,
		Kind:       deployTaskKind(cfg.Kind),
		Status:     "pending",
		MaxAttempt: s.deployRetry,
		ConfigID:   cfg.ID,
	}
	if err := s.db.Create(task).Error; err != nil {
		return 0, err
	}
	go s.runDeploy(task.ID, d, *cert, *target, *cfg)
	return task.ID, nil
}

// DeployConfigsByDomainAsync 异步按当前域名所有启用的部署配置发布最近一次证书。
func (s *Service) DeployConfigsByDomainAsync(domainID int64, kind string) ([]int64, error) {
	var d model.ACMEDomain
	if err := s.db.First(&d, domainID).Error; err != nil {
		return nil, err
	}
	cert, err := s.readyCert(d.ID)
	if err != nil {
		return nil, err
	}
	cfgs, err := s.deployConfigs.ListByDomain(d.ID, kind)
	if err != nil {
		return nil, err
	}
	taskIDs := make([]int64, 0, len(cfgs))
	for _, cfg := range cfgs {
		if !bool(cfg.Enabled) {
			continue
		}
		target, err := s.deployTargets.Get(cfg.TargetID)
		if err != nil || target.Kind != cfg.Kind {
			continue
		}
		task := &model.ACMEIssueTask{
			DomainID:   d.ID,
			MainDomain: d.MainDomain,
			Kind:       deployTaskKind(cfg.Kind),
			Status:     "pending",
			MaxAttempt: s.deployRetry,
			ConfigID:   cfg.ID,
		}
		if err := s.db.Create(task).Error; err != nil {
			return nil, err
		}
		taskIDs = append(taskIDs, task.ID)
		go s.runDeploy(task.ID, d, *cert, *target, cfg)
	}
	if len(taskIDs) == 0 {
		return nil, errors.New("没有可部署的启用配置")
	}
	return taskIDs, nil
}

// DeployAdHocTaskAsync 用一份临时（未持久化）部署配置直接发布当前域名最近一次证书。
// kind 决定 driver；configJSON 是该 driver 的配置（由调用方按 driver 约定拼装）。
func (s *Service) DeployAdHocTaskAsync(domainID, targetID int64, kind, configJSON string) (int64, error) {
	kind = strings.ToLower(strings.TrimSpace(kind))
	if kind == "" {
		return 0, errors.New("部署类型不能为空")
	}
	var d model.ACMEDomain
	if err := s.db.First(&d, domainID).Error; err != nil {
		return 0, err
	}
	cfg := model.ACMEDeployConfig{
		DomainID:   d.ID,
		TargetID:   targetID,
		Kind:       kind,
		ConfigJSON: EmptyJSON(configJSON),
		StateJSON:  "{}",
		Enabled:    true,
	}
	cert, err := s.readyCert(d.ID)
	if err != nil {
		return 0, err
	}
	target, err := s.deployTargets.Get(targetID)
	if err != nil {
		return 0, err
	}
	if target.Kind != kind {
		return 0, fmt.Errorf("部署目标类型 %s 与请求类型 %s 不一致", target.Kind, kind)
	}
	driver, err := s.deployRegistry.Get(kind)
	if err != nil {
		return 0, err
	}
	if err := driver.ValidateConfig(*target, cfg); err != nil {
		return 0, err
	}
	task := &model.ACMEIssueTask{
		DomainID:   d.ID,
		MainDomain: d.MainDomain,
		Kind:       deployTaskKind(kind),
		Status:     "pending",
	}
	if err := s.db.Create(task).Error; err != nil {
		return 0, err
	}
	go s.runDeploy(task.ID, d, *cert, *target, cfg)
	return task.ID, nil
}

func (s *Service) DeploySSHConfigTaskAsync(configID int64) (int64, error) {
	return s.DeployConfigTaskAsync(configID)
}

func (s *Service) DeploySSHConfigsByDomainAsync(domainID int64) ([]int64, error) {
	return s.DeployConfigsByDomainAsync(domainID, DeployKindSSH)
}

func (s *Service) DeploySafelineConfigTaskAsync(configID int64) (int64, error) {
	return s.DeployConfigTaskAsync(configID)
}

func (s *Service) DeploySafelineConfigsByDomainAsync(domainID int64) ([]int64, error) {
	return s.DeployConfigsByDomainAsync(domainID, DeployKindSafeline)
}

func (s *Service) submitAutoDeploy(logw *teeWriter, d model.ACMEDomain, cert model.ACMECert) {
	cfgs, err := s.deployConfigs.ListAutoByDomain(d.ID, "")
	if err != nil {
		logf(logw, "查询自动部署配置失败：%v", err)
		return
	}
	if len(cfgs) == 0 {
		return
	}
	logf(logw, "发现 %d 个自动部署配置", len(cfgs))
	for _, cfg := range cfgs {
		target, err := s.deployTargets.Get(cfg.TargetID)
		if err != nil {
			logf(logw, "跳过自动部署配置 %s：%v", deployConfigName(cfg), err)
			continue
		}
		if target.Kind != cfg.Kind {
			logf(logw, "跳过自动部署配置 %s：目标类型不匹配", deployConfigName(cfg))
			continue
		}
		task := &model.ACMEIssueTask{
			DomainID:   d.ID,
			MainDomain: d.MainDomain,
			Kind:       deployTaskKind(cfg.Kind),
			Status:     "pending",
			MaxAttempt: s.deployRetry,
			ConfigID:   cfg.ID,
		}
		if err := s.db.Create(task).Error; err != nil {
			logf(logw, "创建自动部署任务失败（%s）：%v", deployConfigName(cfg), err)
			continue
		}
		logf(logw, "已提交自动部署任务：%s -> %s，任务 #%d", deployConfigName(cfg), target.Name, task.ID)
		go s.runDeploy(task.ID, d, cert, *target, cfg)
	}
}

func (s *Service) prepareDeploy(cfg model.ACMEDeployConfig) (model.ACMEDomain, *model.ACMECert, *model.ACMEDeployTarget, error) {
	var d model.ACMEDomain
	if err := s.db.First(&d, cfg.DomainID).Error; err != nil {
		return d, nil, nil, err
	}
	cert, err := s.readyCert(d.ID)
	if err != nil {
		return d, nil, nil, err
	}
	target, err := s.deployTargets.Get(cfg.TargetID)
	if err != nil {
		return d, nil, nil, err
	}
	if target.Kind != cfg.Kind {
		return d, nil, nil, errors.New("部署目标类型与配置类型不一致")
	}
	return d, cert, target, nil
}

func (s *Service) readyCert(domainID int64) (*model.ACMECert, error) {
	cert, err := s.GetCertByDomain(domainID)
	if err != nil {
		return nil, err
	}
	if cert == nil {
		return nil, errors.New("当前域名还没有可部署的证书")
	}
	if cert.Status == "revoked" {
		return nil, errors.New("当前证书已吊销，不能部署")
	}
	return cert, nil
}

func deployTaskKind(kind string) string {
	switch kind {
	case DeployKindSSH:
		return "deploy_ssh"
	case DeployKindSafeline:
		return "deploy_safeline"
	case DeployKindUploadCAS:
		return "deploy_upload_cas"
	case DeployKindFnOS:
		return "deploy_fnos"
	default:
		return "deploy"
	}
}

func (s *Service) runDeploy(taskID int64, d model.ACMEDomain, cert model.ACMECert, target model.ACMEDeployTarget, cfg model.ACMEDeployConfig) {
	s.issueMu.Lock()
	defer s.issueMu.Unlock()

	var task model.ACMEIssueTask
	if err := s.db.First(&task, taskID).Error; err != nil {
		return
	}
	attempt := task.Attempt + 1
	maxAttempt := task.MaxAttempt
	if maxAttempt < 1 {
		maxAttempt = 1
	}

	_ = s.db.Model(&model.ACMEIssueTask{}).Where("id = ?", taskID).
		Updates(map[string]any{"status": "running", "attempt": attempt, "next_retry_at": nil}).Error

	logBuf := &bytes.Buffer{}
	if strings.TrimSpace(task.LogText) != "" {
		logBuf.WriteString(task.LogText)
	}
	logw := &teeWriter{buf: logBuf, hub: s.hub, taskID: taskID}

	if attempt > 1 {
		logf(logw, "—— 第 %d/%d 次尝试 ——", attempt, maxAttempt)
	}

	logx.Info("acme deploy start", "task", taskID, "domain", d.MainDomain,
		"kind", cfg.Kind, "target", target.Name, "attempt", attempt, "max_attempt", maxAttempt)

	driver, err := s.deployRegistry.Get(cfg.Kind)
	if err == nil {
		logf(logw, "开始部署证书到%s：%s -> %s / %s", driver.Label(), d.MainDomain, target.Name, deployConfigName(cfg))
	}
	if err == nil {
		var result *DeployResult
		result, err = driver.Deploy(context.Background(), DeployRequest{
			Domain: d,
			Cert:   cert,
			Target: target,
			Config: cfg,
			Logf: func(format string, args ...any) {
				logf(logw, format, args...)
			},
		})
		if err == nil && result != nil && strings.TrimSpace(result.StateJSON) != "" {
			if saveErr := s.deployConfigs.SaveState(cfg.ID, result.StateJSON); saveErr != nil {
				err = fmt.Errorf("保存部署状态失败：%w", saveErr)
			}
		}
	}

	if err == nil {
		logf(logw, "部署完成")
		logx.Info("acme deploy done", "task", taskID, "domain", d.MainDomain, "target", target.Name, "attempt", attempt)
		finish := time.Now()
		_ = s.db.Model(&model.ACMEIssueTask{}).Where("id = ?", taskID).Updates(map[string]any{
			"status":        "success",
			"finished_at":   &finish,
			"log_text":      logBuf.String(),
			"next_retry_at": nil,
		}).Error
		s.hub.Close(taskID)
		return
	}

	// 仅持久化配置触发的任务（config_id>0）且仍有剩余次数才安排重试。
	canRetry := task.ConfigID > 0 && attempt < maxAttempt
	if canRetry {
		next := time.Now().Add(s.deployRetryBackoff * time.Duration(attempt))
		logf(logw, "部署失败：%v", err)
		logf(logw, "已安排第 %d/%d 次重试，约 %s 后", attempt+1, maxAttempt, next.Format("15:04:05"))
		logx.Warn("acme deploy failed, retry scheduled", "task", taskID, "domain", d.MainDomain,
			"target", target.Name, "attempt", attempt, "max_attempt", maxAttempt, "next_retry", next.Format("15:04:05"), "err", err)
		_ = s.db.Model(&model.ACMEIssueTask{}).Where("id = ?", taskID).Updates(map[string]any{
			"status":        "retrying",
			"error_msg":     truncate(err.Error(), 1000),
			"log_text":      logBuf.String(),
			"next_retry_at": &next,
		}).Error
		s.hub.Close(taskID)
		return
	}

	logf(logw, "部署失败：%v", err)
	logx.Error("acme deploy failed", "task", taskID, "domain", d.MainDomain,
		"target", target.Name, "attempt", attempt, "max_attempt", maxAttempt, "err", err)
	finish := time.Now()
	_ = s.db.Model(&model.ACMEIssueTask{}).Where("id = ?", taskID).Updates(map[string]any{
		"status":        "failed",
		"error_msg":     truncate(err.Error(), 1000),
		"finished_at":   &finish,
		"log_text":      logBuf.String(),
		"next_retry_at": nil,
	}).Error
	s.hub.Close(taskID)
}

// RenewExpiring 由 cron 调用：扫所有 enabled 域名，剩余 ≤ renewDays 触发续期。
// 返回触发的 taskID 列表（方便日志）。
func (s *Service) RenewExpiring() ([]int64, error) {
	var domains []model.ACMEDomain
	if err := s.db.Where("enabled = ?", "1").Find(&domains).Error; err != nil {
		return nil, err
	}
	threshold := time.Now().Add(time.Duration(s.renewDays) * 24 * time.Hour)
	var taskIDs []int64
	for _, d := range domains {
		var cert model.ACMECert
		err := s.db.Where("domain_id = ?", d.ID).First(&cert).Error
		needRenew := false
		if errors.Is(err, gorm.ErrRecordNotFound) {
			needRenew = true // 从未签发
		} else if err == nil && (cert.NotAfter.IsZero() || cert.NotAfter.Before(threshold)) {
			needRenew = true
		} else if err != nil {
			continue
		}
		if !needRenew {
			continue
		}
		id, err := s.IssueAsync(d.ID, "renew")
		if err == nil {
			taskIDs = append(taskIDs, id)
		}
	}
	return taskIDs, nil
}

// RetryDeployTaskNow 手动立即重试一条失败/待重试的部署任务（单次）。
// 仅持久化配置触发的任务可重试；耗尽次数的 failed 任务抬高一次上限，
// 失败后回到 failed，不会无限重试。立即异步执行，不等 cron。
func (s *Service) RetryDeployTaskNow(taskID int64) error {
	var t model.ACMEIssueTask
	if err := s.db.First(&t, taskID).Error; err != nil {
		return err
	}
	if t.ConfigID <= 0 {
		return errors.New("该任务无持久化部署配置，无法重试")
	}
	if t.Status != "failed" && t.Status != "retrying" {
		return errors.New("仅失败或待重试的任务可手动重试")
	}
	cfg, err := s.deployConfigs.Get(t.ConfigID)
	if err != nil {
		return fmt.Errorf("部署配置不可用：%w", err)
	}
	d, cert, target, err := s.prepareDeploy(*cfg)
	if err != nil {
		return err
	}
	maxAttempt := t.MaxAttempt
	if t.Attempt >= maxAttempt {
		maxAttempt = t.Attempt + 1
	}
	if err := s.db.Model(&model.ACMEIssueTask{}).Where("id = ?", taskID).Updates(map[string]any{
		"status":        "retrying",
		"max_attempt":   maxAttempt,
		"next_retry_at": nil,
		"finished_at":   nil,
	}).Error; err != nil {
		return err
	}
	logx.Info("acme deploy manual retry", "task", taskID, "config_id", t.ConfigID, "domain", d.MainDomain)
	go s.runDeploy(taskID, d, *cert, *target, *cfg)
	return nil
}

// RetryDeployTasks 由 cron 调用：拉起到期的待重试部署任务。
// 只扫 status='retrying' 且 next_retry_at<=now 的行（历史数据无此状态，零影响），
// 双保险再校验 attempt<max_attempt 且 config_id>0。返回本轮拉起的任务数。
func (s *Service) RetryDeployTasks() (int, error) {
	now := time.Now()
	var tasks []model.ACMEIssueTask
	if err := s.db.
		Where("status = ? AND next_retry_at IS NOT NULL AND next_retry_at <= ? AND attempt < max_attempt AND config_id > 0", "retrying", now).
		Order("id ASC").Find(&tasks).Error; err != nil {
		return 0, err
	}
	n := 0
	for _, t := range tasks {
		cfg, err := s.deployConfigs.Get(t.ConfigID)
		if err != nil {
			finish := time.Now()
			_ = s.db.Model(&model.ACMEIssueTask{}).Where("id = ?", t.ID).Updates(map[string]any{
				"status":        "failed",
				"error_msg":     truncate(fmt.Sprintf("重试时部署配置已不可用：%v", err), 1000),
				"finished_at":   &finish,
				"next_retry_at": nil,
			}).Error
			continue
		}
		d, cert, target, err := s.prepareDeploy(*cfg)
		if err != nil {
			finish := time.Now()
			_ = s.db.Model(&model.ACMEIssueTask{}).Where("id = ?", t.ID).Updates(map[string]any{
				"status":        "failed",
				"error_msg":     truncate(fmt.Sprintf("重试时准备部署失败：%v", err), 1000),
				"finished_at":   &finish,
				"next_retry_at": nil,
			}).Error
			continue
		}
		s.runDeploy(t.ID, d, *cert, *target, *cfg)
		n++
	}
	return n, nil
}

// ----- helpers -----

// ParseSanProviders 解析 ACMEDomain.SanProviders（按域名覆盖 DNS provider）。
// 空键/空值、与默认 provider 相同的项直接丢弃，返回有效覆盖映射。
func ParseSanProviders(d model.ACMEDomain) map[string]string {
	raw := map[string]string{}
	_ = JSONUnmarshal([]byte(EmptyJSON(d.SanProviders)), &raw)
	out := map[string]string{}
	for k, v := range raw {
		k = strings.TrimSpace(k)
		v = strings.TrimSpace(v)
		if k != "" && v != "" && v != d.Provider {
			out[k] = v
		}
	}
	return out
}

// BuildDomains 主域名 + SAN 拆成 lego.Obtain / 雷池匹配等所需的字符串切片。
func BuildDomains(d model.ACMEDomain) []string {
	out := []string{d.MainDomain}
	for _, s := range strings.Split(d.SanDomains, ",") {
		s = strings.TrimSpace(s)
		if s != "" && s != d.MainDomain {
			out = append(out, s)
		}
	}
	return out
}

func parseCertMeta(certPEM []byte) (time.Time, time.Time, string) {
	block, _ := pem.Decode(certPEM)
	if block == nil {
		return time.Time{}, time.Time{}, ""
	}
	c, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return time.Time{}, time.Time{}, ""
	}
	return c.NotBefore, c.NotAfter, c.SerialNumber.Text(16)
}

func assembleFullchain(cert, chain []byte) []byte {
	if len(chain) == 0 {
		return cert
	}
	if bytes.Contains(cert, chain) {
		return cert
	}
	buf := bytes.Buffer{}
	buf.Write(cert)
	if !bytes.HasSuffix(cert, []byte("\n")) {
		buf.WriteByte('\n')
	}
	buf.Write(chain)
	return buf.Bytes()
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

// teeWriter 把 lego 日志同时写入 buffer（最终落库）和 SSE hub（实时推送）。
type teeWriter struct {
	buf    *bytes.Buffer
	hub    *SSEHub
	taskID int64
}

func (w *teeWriter) Write(p []byte) (int, error) {
	w.buf.Write(p)
	// 按行拆分推送
	for _, line := range strings.Split(strings.TrimRight(string(p), "\n"), "\n") {
		if line == "" {
			continue
		}
		w.hub.Publish(w.taskID, line)
	}
	return len(p), nil
}

func logf(w *teeWriter, format string, args ...any) {
	line := fmt.Sprintf("["+time.Now().Format("15:04:05")+"] "+format, args...)
	w.buf.WriteString(line)
	w.buf.WriteByte('\n')
	w.hub.Publish(w.taskID, line)
}
