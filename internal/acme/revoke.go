package acme

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/LemonZuo/homer/internal/logx"
	"github.com/LemonZuo/homer/internal/model"
)

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
