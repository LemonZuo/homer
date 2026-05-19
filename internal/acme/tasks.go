package acme

import (
	"errors"

	"github.com/LemonZuo/homer/internal/model"
	"gorm.io/gorm"
)

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
