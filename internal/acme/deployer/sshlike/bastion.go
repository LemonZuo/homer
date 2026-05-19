package sshlike

import (
	"errors"
	"fmt"

	"github.com/LemonZuo/homer/internal/acme"
	"github.com/LemonZuo/homer/internal/model"
	"gorm.io/gorm"
)

// FindUpstreamRef 反查是否有任何 SSH/fnOS target 把 id 当作 bastion 在用。
// 返回第一个引用者的 name，用于错误提示。
func FindUpstreamRef(db *gorm.DB, id int64) (string, bool, error) {
	if db == nil {
		return "", false, errors.New("跳板机模式未注入 DB")
	}
	var rows []model.ACMEDeployTarget
	if err := db.Where("kind IN ? AND id <> ?", []string{acme.DeployKindSSH, acme.DeployKindFnOS}, id).Find(&rows).Error; err != nil {
		return "", false, fmt.Errorf("扫描跳板机引用失败：%w", err)
	}
	for _, r := range rows {
		cfg := TargetConfig{}
		if err := acme.JSONUnmarshal([]byte(acme.EmptyJSON(r.ConfigJSON)), &cfg); err != nil {
			continue
		}
		if cfg.BastionTargetID == id {
			return r.Name, true, nil
		}
	}
	return "", false, nil
}

// LoadBastion 加载一台被引用作为跳板机的 SSH/fnOS 目标，校验类型与启用状态。
// 这里只解析连接配置，不解析凭证；建连前再 ResolveCredential。
func LoadBastion(db *gorm.DB, id int64) (*Target, error) {
	if db == nil {
		return nil, errors.New("跳板机模式未注入 DB")
	}
	var row model.ACMEDeployTarget
	if err := db.First(&row, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("跳板机不存在：id=%d", id)
		}
		return nil, fmt.Errorf("加载跳板机失败：%w", err)
	}
	if row.Kind != acme.DeployKindSSH && row.Kind != acme.DeployKindFnOS {
		return nil, fmt.Errorf("跳板机必须是 SSH 或 fnOS 类型：id=%d, kind=%s", id, row.Kind)
	}
	if !bool(row.Enabled) {
		return nil, fmt.Errorf("跳板机已停用：%s", row.Name)
	}
	return ParseTarget(row, Labels{Auth: "SSH", Config: "SSH", Host: "SSH"})
}

func ValidateBastion(db *gorm.DB, t Target, currentLabel string) error {
	if t.BastionTargetID <= 0 {
		return nil
	}
	if t.BastionTargetID == t.ID {
		return errors.New("跳板机不能是自己")
	}
	if t.ID > 0 {
		name, ok, err := FindUpstreamRef(db, t.ID)
		if err != nil {
			return err
		}
		if ok {
			return fmt.Errorf("%s已被 %s 设为跳板机，不能再为自己设置跳板机", currentLabel, name)
		}
	}
	b, err := LoadBastion(db, t.BastionTargetID)
	if err != nil {
		return err
	}
	if b.BastionTargetID > 0 {
		return errors.New("所选跳板机已经设置了自己的跳板机，单跳模式不支持跳板机链")
	}
	return nil
}
