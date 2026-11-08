package acme

import (
	"errors"
	"strings"
	"time"

	"github.com/LemonZuo/homer/internal/logx"
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

func (s *Service) pauseTask(id int64, reason string) {
	res := s.db.Model(&model.ACMEIssueTask{}).Where("id = ? AND status IN ?", id, []string{"pending", "retrying"}).
		Updates(map[string]any{"status": "paused", "error_msg": reason, "log_text": gorm.Expr("CONCAT(COALESCE(log_text, ''), ?)", "\n"+reason+"\n"), "finished_at": time.Now(), "next_retry_at": nil})
	if res.Error != nil {
		logx.Error("pause acme task failed", "task", id, "err", res.Error)
	}
	s.hub.Close(id)
}

// writeTaskFinal 写任务终态。log_text 可能夹带远端命令输出的非法 UTF-8，先统一清洗；
// 整条更新仍失败时退化为只写状态字段，避免任务永远停在 running（签发任务卡 running
// 会让该域名的自动续期一直被跳过）。
func (s *Service) writeTaskFinal(id int64, fields map[string]any) {
	for k, v := range fields {
		if str, ok := v.(string); ok {
			fields[k] = strings.ToValidUTF8(str, "\uFFFD")
		}
	}
	err := s.db.Model(&model.ACMEIssueTask{}).Where("id = ?", id).Updates(fields).Error
	if err == nil {
		return
	}
	logx.Error("write acme task result failed, falling back to status only", "task", id, "status", fields["status"], "err", err)
	fallback := map[string]any{"error_msg": "任务结果写库失败，详见服务端日志"}
	for _, k := range []string{"status", "finished_at", "next_retry_at"} {
		if v, ok := fields[k]; ok {
			fallback[k] = v
		}
	}
	if err := s.db.Model(&model.ACMEIssueTask{}).Where("id = ?", id).Updates(fallback).Error; err != nil {
		logx.Error("write acme task status failed", "task", id, "err", err)
	}
}

// 进程重启后原 goroutine 不存在，保留任务供手动重试，不把中断任务显示成永久运行中。
func (s *Service) PauseInterruptedTasks() error {
	reason := "进程已重启，任务已暂停；请核对实际结果后重新发起操作"
	return s.db.Model(&model.ACMEIssueTask{}).Where("status IN ?", []string{"pending", "running"}).
		Updates(map[string]any{"status": "paused", "error_msg": reason, "log_text": gorm.Expr("CONCAT(COALESCE(log_text, ''), ?)", "\n"+reason+"\n"), "finished_at": time.Now(), "next_retry_at": nil}).Error
}

func (s *Service) RetryTaskNow(id int64) error {
	ticket, err := s.modules.Ticket("acme")
	if err != nil {
		return err
	}
	t, err := s.GetTask(id)
	if err != nil {
		return err
	}
	if t == nil {
		return errors.New("任务不存在")
	}
	if t.ConfigID > 0 {
		return s.RetryDeployTaskNow(id)
	}
	if t.Status != "paused" || (t.Kind != "issue" && t.Kind != "renew") {
		return errors.New("该任务不支持重试")
	}
	var domain model.ACMEDomain
	if err := s.db.First(&domain, t.DomainID).Error; err != nil {
		return err
	}
	if !bool(domain.Enabled) {
		return errors.New("域名已停用，请先启用域名")
	}
	res := s.db.Model(&model.ACMEIssueTask{}).Where("id = ? AND status = ?", id, "paused").
		Updates(map[string]any{"status": "pending", "error_msg": "", "finished_at": nil})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected != 1 {
		return errors.New("任务状态已变化，请刷新")
	}
	go s.runIssue(id, domain, ticket)
	return nil
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
