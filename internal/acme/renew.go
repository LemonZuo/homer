package acme

import (
	"errors"
	"time"

	"github.com/LemonZuo/homer/internal/model"
	"gorm.io/gorm"
)

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
		// 暂停的最近一次签发留给用户处理；后续主动签发后，不让旧暂停记录永久阻止续期。
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
	err := query().Select("status").Order("id DESC").Take(&latest).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	}
	return latest.Status == "paused", err
}
