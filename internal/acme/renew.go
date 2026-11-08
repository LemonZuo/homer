package acme

import (
	"errors"
	"time"

	"github.com/LemonZuo/homer/internal/logx"
	"github.com/LemonZuo/homer/internal/model"
	"gorm.io/gorm"
)

// pausedRenewGrace 最近一次签发为 paused 时阻止自动续期的时长。小于 24h：
// 续期 cron 每天跑一次，凌晨续期被重启打断后，次日那一轮必须能照常执行。
const pausedRenewGrace = 12 * time.Hour

// RenewExpiring 由 cron 调用：扫所有 enabled 域名，剩余 ≤ renewDays 触发续期。
// 返回触发的 taskID 列表（方便日志）。
func (s *Service) RenewExpiring() ([]int64, error) {
	ticket, err := s.modules.Ticket("acme")
	if err != nil {
		return nil, err
	}
	var domains []model.ACMEDomain
	if err := s.db.Where("enabled = ?", "1").Find(&domains).Error; err != nil {
		return nil, err
	}
	threshold := time.Now().Add(time.Duration(s.renewDays) * 24 * time.Hour)
	var taskIDs []int64
	for _, d := range domains {
		if !ticket.Valid() {
			break
		}
		// 刚暂停的最近一次签发先留给用户处理（宽限期见 pausedRenewGrace）；后续主动签发后，不让旧暂停记录阻止续期。
		outstanding, err := s.hasOutstandingIssue(d.ID)
		if err != nil {
			return taskIDs, err
		}
		if outstanding {
			continue
		}
		var cert model.ACMECert
		err = s.db.Where("domain_id = ?", d.ID).First(&cert).Error
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
		id, err := s.issueAsyncWithTicket(d.ID, "renew", ticket)
		if err == nil {
			taskIDs = append(taskIDs, id)
		}
	}
	return taskIDs, nil
}

func (s *Service) hasOutstandingIssue(domainID int64) (bool, error) {
	query := func() *gorm.DB {
		return s.db.Model(&model.ACMEIssueTask{}).Where("domain_id = ? AND kind IN ?", domainID, []string{"issue", "renew"})
	}
	var active int64
	if err := query().Where("status IN ?", []string{"pending", "running"}).Count(&active).Error; err != nil {
		return false, err
	}
	if active > 0 {
		return true, nil
	}
	var latest model.ACMEIssueTask
	err := query().Select("id", "status", "started_at", "finished_at").Order("id DESC").Take(&latest).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	}
	if err != nil || latest.Status != "paused" {
		return false, err
	}
	// paused 多半是进程重启 / 模块停用打断的，并非用户的决定：只在宽限期内留给用户手动重试，
	// 过期后照常续期，否则一次重启就让该域名永久跳过自动续期、直到证书过期。
	pausedAt := latest.StartedAt
	if latest.FinishedAt != nil && !latest.FinishedAt.IsZero() {
		pausedAt = *latest.FinishedAt
	}
	if time.Since(pausedAt) < pausedRenewGrace {
		return true, nil
	}
	logx.Info("acme renew: stale paused task no longer blocks renewal", "domain_id", domainID,
		"task", latest.ID, "paused_at", pausedAt.Format(time.DateTime))
	return false, nil
}
