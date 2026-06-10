package upsmon

import (
	"errors"
	"fmt"

	"github.com/LemonZuo/homer/internal/model"
	"github.com/LemonZuo/homer/internal/sshlike"
	"gorm.io/gorm"
)

// ParseUPSTarget 把一行 ups_host 翻成 sshlike.Target。
// sshlike 包不依赖 internal/model,翻译留在 upsmon 模块自己做。
func ParseUPSTarget(h model.UPSHost) (*sshlike.Target, error) {
	auth, err := sshlike.UnmarshalAuthJSON(h.AuthJSON)
	if err != nil {
		return nil, fmt.Errorf("解析 UPS 主机认证配置失败:%w", err)
	}
	cfg, err := sshlike.UnmarshalConfigJSON(h.ConfigJSON)
	if err != nil {
		return nil, fmt.Errorf("解析 UPS 主机连接配置失败:%w", err)
	}
	host, port, err := sshlike.SplitEndpoint(h.Endpoint, "UPS")
	if err != nil {
		return nil, err
	}
	out := &sshlike.Target{
		ID:           h.ID,
		Name:         h.Name,
		Host:         host,
		Port:         port,
		AuthSource:   auth.AuthSource,
		CredentialID: auth.CredentialID,
		BastionID:    cfg.BastionID,
		Username:     auth.Username,
		AuthType:     auth.AuthType,
		Password:     auth.Password,
		PrivateKey:   auth.PrivateKey,
		Passphrase:   auth.Passphrase,
		Enabled:      bool(h.Enabled),
		CreatedAt:    h.CreatedAt,
		UpdatedAt:    h.UpdatedAt,
	}
	sshlike.Normalize(out)
	return out, nil
}

// LoadUPSBastion 加载一台被引用作为跳板机的 ups_host,校验启用状态。
// 只解析连接,凭证留到 ConnFor 里再 Resolve。
func LoadUPSBastion(db *gorm.DB, id int64) (*sshlike.Target, error) {
	if db == nil {
		return nil, errors.New("跳板机模式未注入 DB")
	}
	var row model.UPSHost
	if err := db.First(&row, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("跳板机不存在:id=%d", id)
		}
		return nil, fmt.Errorf("加载跳板机失败:%w", err)
	}
	if !bool(row.Enabled) {
		return nil, fmt.Errorf("跳板机已停用:%s", row.Name)
	}
	return ParseUPSTarget(row)
}

// FindUPSUpstream 反查是否有别的 ups_host 把 id 当作 bastion 在用。
func FindUPSUpstream(db *gorm.DB, id int64) (string, bool, error) {
	if db == nil {
		return "", false, errors.New("跳板机模式未注入 DB")
	}
	var rows []model.UPSHost
	if err := db.Where("id <> ?", id).Find(&rows).Error; err != nil {
		return "", false, fmt.Errorf("扫描跳板机引用失败:%w", err)
	}
	for _, r := range rows {
		cfg, err := sshlike.UnmarshalConfigJSON(r.ConfigJSON)
		if err != nil {
			continue
		}
		if cfg.BastionID == id {
			return r.Name, true, nil
		}
	}
	return "", false, nil
}
