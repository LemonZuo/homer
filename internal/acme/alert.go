package acme

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/LemonZuo/homer/internal/logx"
	"github.com/LemonZuo/homer/internal/model"
	"github.com/LemonZuo/homer/internal/notify"
	"gorm.io/gorm"
)

// alertSendTimeout 单条失败告警的发送时限（含通道内部重试）。
const alertSendTimeout = time.Minute

// SetNotifier 注入终态失败告警通道（hub.For(notify.ModuleACME)）；nil 时不告警。
func (s *Service) SetNotifier(n notify.Notifier) { s.notifier = n }

// alertFailure 异步推送一条失败告警。调用点都在持有 issueMu 的任务 goroutine 里，
// 通道超时不能拖住签发/部署队列；告警本身失败只记日志（任务失败已落库，任务页可查）。
// 续期 cron 每天都会再跑，凭证类问题会每天再告警一次，不需要单独重试。
func (s *Service) alertFailure(title, body string) {
	n := s.notifier
	if n == nil {
		return
	}
	go func() {
		if !n.Enabled() {
			logx.Warn("acme alert skipped: no channel", "title", title)
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), alertSendTimeout)
		defer cancel()
		if err := n.Send(ctx, notify.Message{Title: title, Text: body}); err != nil {
			logx.Error("acme alert send failed", "title", title, "err", err)
		}
	}()
}

// alertIssueFailed 签发 / 续期失败告警。续期失败附上现有证书的到期时间，便于判断紧急程度。
func (s *Service) alertIssueFailed(taskID int64, kind string, d model.ACMEDomain, err error) {
	title, kindLabel := "证书签发失败", "签发"
	if kind == "renew" {
		title, kindLabel = "证书续期失败", "续期"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "域名：%s\n任务：#%d（%s）", d.MainDomain, taskID, kindLabel)
	if line := s.certExpiryLine(d.ID); line != "" {
		b.WriteString("\n")
		b.WriteString(line)
	}
	fmt.Fprintf(&b, "\n错误：%s", truncate(err.Error(), 500))
	s.alertFailure(title, b.String())
}

// alertDeployFailed 部署终态失败（重试已用尽 / 不参与重试 / 重试准备失败）告警。
func (s *Service) alertDeployFailed(taskID int64, domain, target, config, attemptInfo string, err error) {
	var b strings.Builder
	fmt.Fprintf(&b, "域名：%s", domain)
	if target != "" {
		fmt.Fprintf(&b, "\n目标：%s", target)
	}
	if config != "" {
		fmt.Fprintf(&b, "\n配置：%s", config)
	}
	fmt.Fprintf(&b, "\n任务：#%d", taskID)
	if attemptInfo != "" {
		fmt.Fprintf(&b, "（%s）", attemptInfo)
	}
	fmt.Fprintf(&b, "\n错误：%s", truncate(err.Error(), 500))
	s.alertFailure("证书部署失败", b.String())
}

// alertAutoDeploySkipped 签发成功但有自动部署配置没能提交（目标被删 / 类型不匹配 / 建任务失败）。
// 模块停用导致的未提交属于用户主动操作，不走这里。
func (s *Service) alertAutoDeploySkipped(d model.ACMEDomain, reasons []string) {
	if len(reasons) == 0 {
		return
	}
	var b strings.Builder
	fmt.Fprintf(&b, "域名：%s\n新证书已签发，以下自动部署未提交：", d.MainDomain)
	for _, r := range reasons {
		b.WriteString("\n- ")
		b.WriteString(truncate(r, 200))
	}
	s.alertFailure("证书自动部署未提交", b.String())
}

// certExpiryLine 返回「现有证书到期：… （剩 N 天）」；无证书或查询失败返回空串。
func (s *Service) certExpiryLine(domainID int64) string {
	if s.db == nil || domainID <= 0 {
		return ""
	}
	var cert model.ACMECert
	err := s.db.Select("not_after").Where("domain_id = ?", domainID).First(&cert).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "现有证书：无"
	}
	if err != nil || cert.NotAfter.IsZero() {
		return ""
	}
	days := int(time.Until(cert.NotAfter).Hours() / 24)
	if days < 0 {
		return fmt.Sprintf("现有证书：已于 %s 过期", cert.NotAfter.Format("2006-01-02"))
	}
	return fmt.Sprintf("现有证书到期：%s（剩 %d 天）", cert.NotAfter.Format("2006-01-02"), days)
}
