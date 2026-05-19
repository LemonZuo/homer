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
