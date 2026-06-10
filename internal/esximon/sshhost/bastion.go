package sshhost

import (
	"errors"
	"fmt"

	"github.com/LemonZuo/homer/internal/model"
	"gorm.io/gorm"
)

// FindUpstreamRef 反查是否有任何 esxi_host 把 id 当作 bastion 在用,返回第一个引用者名。
func FindUpstreamRef(db *gorm.DB, id int64) (string, bool, error) {
	if db == nil {
		return "", false, errors.New("跳板机模式未注入 DB")
	}
	var rows []model.EsxiHost
	if err := db.Where("id <> ?", id).Find(&rows).Error; err != nil {
		return "", false, fmt.Errorf("扫描跳板机引用失败:%w", err)
	}
	for _, r := range rows {
		cfg := TargetConfig{}
		if err := jsonUnmarshal(r.ConfigJSON, &cfg); err != nil {
			continue
		}
		if cfg.BastionHostID == id {
			return r.Name, true, nil
		}
	}
	return "", false, nil
}

// LoadBastion 加载一台被引用作为跳板机的 esxi_host,校验启用状态。
// 这里只解析连接配置,凭证留到 ConnFor 里再 ResolveCredential。
func LoadBastion(db *gorm.DB, id int64) (*Target, error) {
	if db == nil {
		return nil, errors.New("跳板机模式未注入 DB")
	}
	var row model.EsxiHost
	if err := db.First(&row, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("跳板机不存在:id=%d", id)
		}
		return nil, fmt.Errorf("加载跳板机失败:%w", err)
	}
	if !bool(row.Enabled) {
		return nil, fmt.Errorf("跳板机已停用:%s", row.Name)
	}
	return ParseTarget(row)
}

// ValidateBastion 用于 Upsert 校验:单跳约束 + 不能自指 + 不能被人引用后又指别人。
func ValidateBastion(db *gorm.DB, t Target) error {
	if t.BastionHostID <= 0 {
		return nil
	}
	if t.BastionHostID == t.ID {
		return errors.New("跳板机不能是自己")
	}
	if t.ID > 0 {
		name, ok, err := FindUpstreamRef(db, t.ID)
		if err != nil {
			return err
		}
		if ok {
			return fmt.Errorf("本机已被 %s 设为跳板机,不能再为自己设置跳板机", name)
		}
	}
	b, err := LoadBastion(db, t.BastionHostID)
	if err != nil {
		return err
	}
	if b.BastionHostID > 0 {
		return errors.New("所选跳板机已经设置了自己的跳板机,单跳模式不支持跳板机链")
	}
	return nil
}
