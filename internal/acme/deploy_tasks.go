package acme

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/LemonZuo/homer/internal/logx"
	"github.com/LemonZuo/homer/internal/model"
	"github.com/LemonZuo/homer/internal/modulecontrol"
)

// deployTimeout 是单次部署（含 SSH 拨号、上传、远端部署命令）的总时限。
const deployTimeout = 10 * time.Minute

// DeployConfigTaskAsync 异步按保存的部署配置发布当前域名最近一次证书。
func (s *Service) DeployConfigTaskAsync(configID int64) (int64, error) {
	ticket, err := s.modules.Ticket("acme")
	if err != nil {
		return 0, err
	}
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
	go s.runDeploy(task.ID, d, *cert, *target, *cfg, ticket)
	return task.ID, nil
}

// DeployConfigsByDomainAsync 异步按当前域名所有启用的部署配置发布最近一次证书。
func (s *Service) DeployConfigsByDomainAsync(domainID int64, kind string) ([]int64, error) {
	ticket, err := s.modules.Ticket("acme")
	if err != nil {
		return nil, err
	}
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
		if !ticket.Valid() {
			break
		}
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
		go s.runDeploy(task.ID, d, *cert, *target, cfg, ticket)
	}
	if len(taskIDs) == 0 {
		return nil, errors.New("没有可部署的启用配置")
	}
	return taskIDs, nil
}

func (s *Service) submitAutoDeploy(logw *teeWriter, d model.ACMEDomain, cert model.ACMECert, ticket modulecontrol.Ticket) {
	if !ticket.Valid() {
		logf(logw, "证书已保存；模块已停用，未启动自动部署，请在启用后手动部署")
		return
	}
	cfgs, err := s.deployConfigs.ListAutoByDomain(d.ID, "")
	if err != nil {
		logf(logw, "查询自动部署配置失败：%v", err)
		s.alertAutoDeploySkipped(d, []string{fmt.Sprintf("查询自动部署配置失败：%v", err)})
		return
	}
	if len(cfgs) == 0 {
		return
	}
	logf(logw, "发现 %d 个自动部署配置", len(cfgs))
	// 证书已更新但没部署出去，服务器上的旧证书照样会过期：未提交的配置要告警。
	var skipped []string
	defer func() { s.alertAutoDeploySkipped(d, skipped) }()
	for _, cfg := range cfgs {
		if !ticket.Valid() {
			logf(logw, "模块已停用，剩余自动部署未提交")
			break
		}
		target, err := s.deployTargets.Get(cfg.TargetID)
		if err != nil {
			logf(logw, "跳过自动部署配置 %s：%v", deployConfigName(cfg), err)
			skipped = append(skipped, fmt.Sprintf("%s：%v", deployConfigName(cfg), err))
			continue
		}
		if target.Kind != cfg.Kind {
			logf(logw, "跳过自动部署配置 %s：目标类型不匹配", deployConfigName(cfg))
			skipped = append(skipped, deployConfigName(cfg)+"：目标类型不匹配")
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
			skipped = append(skipped, fmt.Sprintf("%s：创建任务失败：%v", deployConfigName(cfg), err))
			continue
		}
		logf(logw, "已提交自动部署任务：%s -> %s，任务 #%d", deployConfigName(cfg), target.Name, task.ID)
		go s.runDeploy(task.ID, d, cert, *target, cfg, ticket)
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

func (s *Service) runDeploy(taskID int64, d model.ACMEDomain, cert model.ACMECert, target model.ACMEDeployTarget, cfg model.ACMEDeployConfig, ticket modulecontrol.Ticket) {
	s.issueMu.Lock()
	defer s.issueMu.Unlock()
	finishOperation, err := ticket.Begin()
	if err != nil {
		s.pauseTask(taskID, "模块已停用，重新启用后可手动重试")
		return
	}
	defer finishOperation()

	var task model.ACMEIssueTask
	if err := s.db.First(&task, taskID).Error; err != nil {
		return
	}
	if task.Status != "pending" && task.Status != "retrying" {
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

	err = s.executeDeploy(logw, d, cert, target, cfg)

	if err == nil {
		logf(logw, "部署完成")
		logx.Info("acme deploy done", "task", taskID, "domain", d.MainDomain, "target", target.Name, "attempt", attempt)
		finish := time.Now()
		s.finalizeDeployTask(taskID, logBuf, map[string]any{
			"status":        "success",
			"finished_at":   &finish,
			"next_retry_at": nil,
		})
		return
	}

	if !ticket.Valid() {
		logf(logw, "部署失败：%v；模块已停用，暂停后续重试", err)
		s.finalizeDeployTask(taskID, logBuf, map[string]any{"status": "paused", "error_msg": truncate(err.Error(), 1000), "finished_at": time.Now(), "next_retry_at": nil})
		return
	}
	// 仅持久化配置触发的任务（config_id>0）且仍有剩余次数才安排重试。
	if task.ConfigID > 0 && attempt < maxAttempt {
		next := time.Now().Add(s.deployRetryBackoff * time.Duration(attempt))
		logf(logw, "部署失败：%v", err)
		logf(logw, "已安排第 %d/%d 次重试，约 %s 后", attempt+1, maxAttempt, next.Format("15:04:05"))
		logx.Warn("acme deploy failed, retry scheduled", "task", taskID, "domain", d.MainDomain,
			"target", target.Name, "attempt", attempt, "max_attempt", maxAttempt, "next_retry", next.Format("15:04:05"), "err", err)
		s.finalizeDeployTask(taskID, logBuf, map[string]any{
			"status":        "retrying",
			"error_msg":     truncate(err.Error(), 1000),
			"next_retry_at": &next,
		})
		return
	}

	logf(logw, "部署失败：%v", err)
	logx.Error("acme deploy failed", "task", taskID, "domain", d.MainDomain,
		"target", target.Name, "attempt", attempt, "max_attempt", maxAttempt, "err", err)
	finish := time.Now()
	s.finalizeDeployTask(taskID, logBuf, map[string]any{
		"status":        "failed",
		"error_msg":     truncate(err.Error(), 1000),
		"finished_at":   &finish,
		"next_retry_at": nil,
	})
	attemptInfo := ""
	if task.ConfigID > 0 && maxAttempt > 1 {
		attemptInfo = fmt.Sprintf("已尝试 %d/%d 次，重试用尽", attempt, maxAttempt)
	}
	s.alertDeployFailed(taskID, d.MainDomain, target.Name, deployConfigName(cfg), attemptInfo, err)
}

// executeDeploy 查 driver 并执行 Deploy，成功后落盘 state_json，返回单一 err。
func (s *Service) executeDeploy(logw *teeWriter, d model.ACMEDomain, cert model.ACMECert, target model.ACMEDeployTarget, cfg model.ACMEDeployConfig) error {
	driver, err := s.deployRegistry.Get(cfg.Kind)
	if err != nil {
		return err
	}
	logf(logw, "开始部署证书到%s：%s -> %s / %s", driver.Label(), d.MainDomain, target.Name, deployConfigName(cfg))
	// runDeploy 持有 issueMu：没有总时限的话，一次卡住的远端命令会让签发/续期/吊销全部排队挂死。
	ctx, cancel := context.WithTimeout(context.Background(), deployTimeout)
	defer cancel()
	result, err := driver.Deploy(ctx, DeployRequest{
		Domain: d,
		Cert:   cert,
		Target: target,
		Config: cfg,
		Logf: func(format string, args ...any) {
			logf(logw, format, args...)
		},
	})
	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return fmt.Errorf("部署超时（超过 %s，已中断连接）：%w", deployTimeout, err)
		}
		return err
	}
	if result == nil || strings.TrimSpace(result.StateJSON) == "" {
		return nil
	}
	if err := s.deployConfigs.SaveState(cfg.ID, result.StateJSON); err != nil {
		return fmt.Errorf("保存部署状态失败：%w", err)
	}
	return nil
}

// finalizeDeployTask 统一把终态字段写回 task + 关闭 hub 订阅。log_text 由 logBuf 当前快照决定。
func (s *Service) finalizeDeployTask(taskID int64, logBuf *bytes.Buffer, fields map[string]any) {
	fields["log_text"] = logBuf.String()
	s.writeTaskFinal(taskID, fields)
	s.hub.Close(taskID)
}

// RetryDeployTaskNow 手动立即重试一条失败/待重试的部署任务（单次）。
// 仅持久化配置触发的任务可重试；耗尽次数的 failed 任务抬高一次上限，
// 失败后回到 failed，不会无限重试。立即异步执行，不等 cron。
func (s *Service) RetryDeployTaskNow(taskID int64) error {
	ticket, err := s.modules.Ticket("acme")
	if err != nil {
		return err
	}
	var t model.ACMEIssueTask
	if err := s.db.First(&t, taskID).Error; err != nil {
		return err
	}
	if t.ConfigID <= 0 {
		return errors.New("该任务无持久化部署配置，无法重试")
	}
	if t.Status != "failed" && t.Status != "retrying" && t.Status != "paused" {
		return errors.New("仅失败、待重试或暂停的任务可手动重试")
	}
	cfg, err := s.deployConfigs.Get(t.ConfigID)
	if err != nil {
		return fmt.Errorf("部署配置不可用：%w", err)
	}
	d, cert, target, err := s.prepareDeploy(*cfg)
	if err != nil {
		return err
	}
	if !bool(d.Enabled) || !bool(cfg.Enabled) || !bool(target.Enabled) {
		return errors.New("域名、部署配置或目标已停用")
	}
	maxAttempt := t.MaxAttempt
	if t.Attempt >= maxAttempt {
		maxAttempt = t.Attempt + 1
	}
	res := s.db.Model(&model.ACMEIssueTask{}).Where("id = ? AND status = ?", taskID, t.Status).Updates(map[string]any{
		"status":        "pending",
		"max_attempt":   maxAttempt,
		"next_retry_at": nil,
		"finished_at":   nil,
	})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected != 1 {
		return errors.New("任务状态已变化，请刷新")
	}
	logx.Info("acme deploy manual retry", "task", taskID, "config_id", t.ConfigID, "domain", d.MainDomain)
	go s.runDeploy(taskID, d, *cert, *target, *cfg, ticket)
	return nil
}

// RetryDeployTasks 由 cron 调用：拉起到期的待重试部署任务。
// 只扫 status='retrying' 且 next_retry_at<=now 的行（历史数据无此状态，零影响），
// 双保险再校验 attempt<max_attempt 且 config_id>0。返回本轮拉起的任务数。
func (s *Service) RetryDeployTasks() (int, error) {
	ticket, err := s.modules.Ticket("acme")
	if err != nil {
		return 0, err
	}
	now := time.Now()
	var tasks []model.ACMEIssueTask
	if err := s.db.
		Where("status = ? AND next_retry_at IS NOT NULL AND next_retry_at <= ? AND attempt < max_attempt AND config_id > 0", "retrying", now).
		Order("id ASC").Find(&tasks).Error; err != nil {
		return 0, err
	}
	n := 0
	for _, t := range tasks {
		if !ticket.Valid() {
			break
		}
		cfg, err := s.deployConfigs.Get(t.ConfigID)
		if err != nil {
			s.failDeployRetry(t, fmt.Sprintf("#%d", t.ConfigID), fmt.Errorf("重试时部署配置已不可用：%w", err))
			continue
		}
		d, cert, target, err := s.prepareDeploy(*cfg)
		if err != nil {
			s.failDeployRetry(t, deployConfigName(*cfg), fmt.Errorf("重试时准备部署失败：%w", err))
			continue
		}
		s.runDeploy(t.ID, d, *cert, *target, *cfg, ticket)
		n++
	}
	return n, nil
}

// failDeployRetry 重试拉起前准备失败：任务直接终态失败并告警（不会再有下一次重试）。
func (s *Service) failDeployRetry(t model.ACMEIssueTask, configName string, err error) {
	logx.Error("acme deploy retry prepare failed", "task", t.ID, "domain", t.MainDomain, "config_id", t.ConfigID, "err", err)
	finish := time.Now()
	s.writeTaskFinal(t.ID, map[string]any{
		"status":        "failed",
		"error_msg":     truncate(err.Error(), 1000),
		"finished_at":   &finish,
		"next_retry_at": nil,
	})
	s.alertDeployFailed(t.ID, t.MainDomain, "", configName, fmt.Sprintf("第 %d 次重试未能开始", t.Attempt+1), err)
}
