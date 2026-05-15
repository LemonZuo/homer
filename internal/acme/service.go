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

	"github.com/LemonZuo/homer/internal/cas"
	"github.com/LemonZuo/homer/internal/model"
	"gorm.io/gorm"
)

// Service ACME 业务编排：域名 CRUD、签发、续期、落盘、上传 CAS、SSE 日志。
type Service struct {
	db             *gorm.DB
	manager        *Manager
	credstore      *CredentialStore
	accountStore   *AccountStore
	deployTargets  *DeployTargetStore
	deployConfigs  *DeployConfigStore
	deployRegistry *DeployRegistry
	cas            *cas.Service
	hub            *SSEHub
	dataDir        string
	renewDays      int

	issueMu sync.Mutex // 串行化签发（lego logger / env 是全局状态）
}

func NewService(db *gorm.DB, mgr *Manager, store *CredentialStore, accounts *AccountStore, deployTargets *DeployTargetStore, deployConfigs *DeployConfigStore, deployRegistry *DeployRegistry, casSvc *cas.Service, hub *SSEHub, dataDir string, renewDays int) *Service {
	return &Service{db: db, manager: mgr, credstore: store, accountStore: accounts, deployTargets: deployTargets, deployConfigs: deployConfigs, deployRegistry: deployRegistry, cas: casSvc, hub: hub, dataDir: dataDir, renewDays: renewDays}
}

func (s *Service) Hub() *SSEHub                      { return s.hub }
func (s *Service) Credentials() *CredentialStore     { return s.credstore }
func (s *Service) Accounts() *AccountStore           { return s.accountStore }
func (s *Service) DeployTargets() *DeployTargetStore { return s.deployTargets }
func (s *Service) DeployConfigs() *DeployConfigStore { return s.deployConfigs }
func (s *Service) SSHTargets() *SSHTargetStore       { return NewSSHTargetStore(s.deployTargets) }
func (s *Service) SSHDeploys() *SSHDeployConfigStore {
	return NewSSHDeployConfigStore(s.deployConfigs)
}
func (s *Service) SafelineTargets() *SafelineTargetStore {
	return NewSafelineTargetStore(s.deployTargets)
}
func (s *Service) SafelineDeploys() *SafelineDeployConfigStore {
	return NewSafelineDeployConfigStore(s.deployConfigs)
}

// DomainView 联查域名 + 最近一次证书（NotAfter 用于前端显示剩余天数）。
type DomainView struct {
	model.ACMEDomain
	NotAfter   *time.Time `json:"not_after,omitempty"`
	NotBefore  *time.Time `json:"not_before,omitempty"`
	CASCertID  int64      `json:"cas_cert_id,omitempty"`
	CertStatus string     `json:"cert_status,omitempty"`
	RevokedAt  *time.Time `json:"revoked_at,omitempty"`
	IssuedAt   *time.Time `json:"issued_at,omitempty"`
}

// ListDomains 列出所有域名（按 id 倒序），附带最近一次证书摘要。
func (s *Service) ListDomains() ([]DomainView, error) {
	var items []model.ACMEDomain
	if err := s.db.Order("id DESC").Find(&items).Error; err != nil {
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
			v.CASCertID = c.CASCertID
			v.CertStatus = c.Status
			v.RevokedAt = c.RevokedAt
		}
		out = append(out, v)
	}
	return out, nil
}

// CreateDomain 新增域名。
func (s *Service) CreateDomain(d *model.ACMEDomain) error {
	d.MainDomain = strings.TrimSpace(d.MainDomain)
	d.Provider = strings.TrimSpace(d.Provider)
	if d.MainDomain == "" || d.Provider == "" {
		return errors.New("main_domain 与 provider 必填")
	}
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

// ListTasks 任务流水（最近 N 条）。
func (s *Service) ListTasks(limit int) ([]model.ACMEIssueTask, error) {
	if limit <= 0 {
		limit = 50
	}
	var items []model.ACMEIssueTask
	if err := s.db.Order("id DESC").Limit(limit).Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
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

	logf(logw, "开始签发：%s（provider=%s）", d.MainDomain, d.Provider)
	domains := buildDomains(d)
	logf(logw, "目标域名：%s", strings.Join(domains, ", "))

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
		res, err := client.Obtain(domains, d.Provider, s.credstore)
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
		upd["status"] = "failed"
		upd["error_msg"] = truncate(err.Error(), 1000)
		// 重新写一次 log_text 以包含错误
		upd["log_text"] = logBuf.String()
	} else {
		logf(logw, "签发完成")
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
		if cert.CASCertID > 0 {
			logf(logw, "注意：已上传的 CAS 证书 cert_id=%d 不会自动删除，CDN 也不会自动切换", cert.CASCertID)
		}
		return nil
	}()

	finish := time.Now()
	upd := map[string]any{
		"finished_at": &finish,
		"log_text":    logBuf.String(),
	}
	if err != nil {
		logf(logw, "吊销失败：%v", err)
		upd["status"] = "failed"
		upd["error_msg"] = truncate(err.Error(), 1000)
		upd["log_text"] = logBuf.String()
	} else {
		logf(logw, "吊销完成")
		upd["status"] = "success"
		upd["log_text"] = logBuf.String()
	}
	_ = s.db.Model(&model.ACMEIssueTask{}).Where("id = ?", taskID).Updates(upd).Error
	s.hub.Close(taskID)
}

// persistCert 落盘到 ./data/acme/certs/<domain>/，写入 acme_cert 表，并上传 CAS。
func (s *Service) persistCert(logw *teeWriter, d model.ACMEDomain, cert, key, chain []byte) (*model.ACMECert, error) {
	notBefore, notAfter, serial := parseCertMeta(cert)
	dir := filepath.Join(s.dataDir, "certs", d.MainDomain)
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

	// 自动上传 CAS（失败不回滚签发，仅记日志）。前端仍保留手动上传按钮，方便补传或重传。
	if s.cas != nil && s.cas.Configured() {
		name := buildCASName(time.Now())
		id, err := s.cas.UploadCertificate(name, string(full), string(key))
		if err != nil {
			logf(logw, "上传 CAS 失败（不影响本地证书）：%v", err)
		} else {
			logf(logw, "已上传 CAS：cert_id=%d, name=%s", id, name)
			_ = s.db.Model(&model.ACMECert{}).Where("id = ?", rec.ID).
				Update("cas_cert_id", id).Error
			rec.CASCertID = id
		}
	}

	return rec, nil
}

// UploadCASTaskAsync 异步把当前域名最近一次证书上传到 CAS。
func (s *Service) UploadCASTaskAsync(domainID int64) (int64, error) {
	var d model.ACMEDomain
	if err := s.db.First(&d, domainID).Error; err != nil {
		return 0, err
	}
	cert, err := s.GetCertByDomain(domainID)
	if err != nil {
		return 0, err
	}
	if cert == nil {
		return 0, errors.New("当前域名还没有可上传的证书")
	}
	if cert.Status == "revoked" {
		return 0, errors.New("当前证书已吊销，不能上传 CAS")
	}
	if strings.TrimSpace(cert.FullchainPEM) == "" || strings.TrimSpace(cert.KeyPEM) == "" {
		return 0, errors.New("当前证书内容不完整，无法上传 CAS")
	}
	if s.cas == nil || !s.cas.Configured() {
		return 0, errors.New("阿里云 CAS 未配置")
	}
	task := &model.ACMEIssueTask{
		DomainID:   d.ID,
		MainDomain: d.MainDomain,
		Kind:       "upload_cas",
		Status:     "pending",
	}
	if err := s.db.Create(task).Error; err != nil {
		return 0, err
	}
	go s.runUploadCAS(task.ID, d, *cert)
	return task.ID, nil
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

// DeploySSHTaskAsync 保留直接部署接口，内部构造成临时通用 SSH 配置。
func (s *Service) DeploySSHTaskAsync(domainID, targetID int64, opts SSHDeployOptions) (int64, error) {
	var d model.ACMEDomain
	if err := s.db.First(&d, domainID).Error; err != nil {
		return 0, err
	}
	cfg := model.ACMEDeployConfig{
		DomainID:   d.ID,
		TargetID:   targetID,
		Kind:       DeployKindSSH,
		ConfigJSON: mustJSON(opts),
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
	if target.Kind != DeployKindSSH {
		return 0, errors.New("部署目标不是 SSH 机器")
	}
	driver, err := s.deployRegistry.Get(DeployKindSSH)
	if err != nil {
		return 0, err
	}
	if err := driver.ValidateConfig(*target, cfg); err != nil {
		return 0, err
	}
	task := &model.ACMEIssueTask{
		DomainID:   d.ID,
		MainDomain: d.MainDomain,
		Kind:       "deploy_ssh",
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
	default:
		return "deploy"
	}
}

func (s *Service) runUploadCAS(taskID int64, d model.ACMEDomain, cert model.ACMECert) {
	s.issueMu.Lock()
	defer s.issueMu.Unlock()

	_ = s.db.Model(&model.ACMEIssueTask{}).Where("id = ?", taskID).
		Updates(map[string]any{"status": "running"}).Error

	logBuf := &bytes.Buffer{}
	logw := &teeWriter{buf: logBuf, hub: s.hub, taskID: taskID}

	logf(logw, "开始上传 CAS：%s", d.MainDomain)

	err := func() error {
		if s.cas == nil || !s.cas.Configured() {
			return errors.New("阿里云 CAS 未配置")
		}
		name := buildCASName(time.Now())
		id, err := s.cas.UploadCertificate(name, cert.FullchainPEM, cert.KeyPEM)
		if err != nil {
			return fmt.Errorf("上传 CAS 失败：%w", err)
		}
		logf(logw, "已上传 CAS：cert_id=%d, name=%s", id, name)
		if err := s.db.Model(&model.ACMECert{}).Where("id = ?", cert.ID).
			Update("cas_cert_id", id).Error; err != nil {
			return fmt.Errorf("更新 CAS cert_id 失败：%w", err)
		}
		return nil
	}()

	finish := time.Now()
	upd := map[string]any{
		"finished_at": &finish,
		"log_text":    logBuf.String(),
	}
	if err != nil {
		logf(logw, "上传 CAS 失败：%v", err)
		upd["status"] = "failed"
		upd["error_msg"] = truncate(err.Error(), 1000)
		upd["log_text"] = logBuf.String()
	} else {
		logf(logw, "上传 CAS 完成")
		upd["status"] = "success"
		upd["log_text"] = logBuf.String()
	}
	_ = s.db.Model(&model.ACMEIssueTask{}).Where("id = ?", taskID).Updates(upd).Error
	s.hub.Close(taskID)
}

func (s *Service) runDeploy(taskID int64, d model.ACMEDomain, cert model.ACMECert, target model.ACMEDeployTarget, cfg model.ACMEDeployConfig) {
	s.issueMu.Lock()
	defer s.issueMu.Unlock()

	_ = s.db.Model(&model.ACMEIssueTask{}).Where("id = ?", taskID).
		Updates(map[string]any{"status": "running"}).Error

	logBuf := &bytes.Buffer{}
	logw := &teeWriter{buf: logBuf, hub: s.hub, taskID: taskID}

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

	finish := time.Now()
	upd := map[string]any{
		"finished_at": &finish,
		"log_text":    logBuf.String(),
	}
	if err != nil {
		logf(logw, "部署失败：%v", err)
		upd["status"] = "failed"
		upd["error_msg"] = truncate(err.Error(), 1000)
		upd["log_text"] = logBuf.String()
	} else {
		logf(logw, "部署完成")
		upd["status"] = "success"
		upd["log_text"] = logBuf.String()
	}
	_ = s.db.Model(&model.ACMEIssueTask{}).Where("id = ?", taskID).Updates(upd).Error
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

// ----- helpers -----

func buildDomains(d model.ACMEDomain) []string {
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

// buildCASName CAS 内证书命名：<timestamp>。
func buildCASName(ts time.Time) string {
	return ts.Format("20060102150405")
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
