package acme

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/LemonZuo/homer/internal/logx"
	"github.com/LemonZuo/homer/internal/model"
	"gorm.io/gorm"
)

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
