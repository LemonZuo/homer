package notify

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/LemonZuo/homer/internal/model"
	"gorm.io/gorm"
)

// Store 通道与模块映射的 CRUD，供 handler 使用。
type Store struct{ db *gorm.DB }

func NewStore(db *gorm.DB) *Store { return &Store{db: db} }

func validChannelType(t string) bool {
	for _, ct := range ChannelTypes {
		if ct.Type == t {
			return true
		}
	}
	return false
}

// ListChannels 返回所有通道；RefCount 为引用该通道的模块绑定数。
func (s *Store) ListChannels() ([]model.NotifyChannel, error) {
	var rows []model.NotifyChannel
	if err := s.db.Order("id ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	var counts []struct {
		ChannelID int64
		N         int64
	}
	if err := s.db.Model(&model.NotifyBinding{}).
		Select("channel_id, COUNT(*) AS n").Group("channel_id").Scan(&counts).Error; err != nil {
		return nil, err
	}
	byID := make(map[int64]int64, len(counts))
	for _, c := range counts {
		byID[c.ChannelID] = c.N
	}
	for i := range rows {
		rows[i].RefCount = byID[rows[i].ID]
	}
	return rows, nil
}

func (s *Store) UpsertChannel(c *model.NotifyChannel) (*model.NotifyChannel, error) {
	c.Name = strings.TrimSpace(c.Name)
	c.Type = strings.ToLower(strings.TrimSpace(c.Type))
	if c.Name == "" {
		return nil, errors.New("通道名称不能为空")
	}
	if !validChannelType(c.Type) {
		return nil, fmt.Errorf("未知通道类型：%s", c.Type)
	}
	if strings.TrimSpace(c.ConfigJSON) == "" {
		c.ConfigJSON = "{}"
	}
	tmp := map[string]string{}
	if err := json.Unmarshal([]byte(c.ConfigJSON), &tmp); err != nil {
		return nil, fmt.Errorf("config_json 必须是 {\"KEY\":\"VALUE\"} 形式：%w", err)
	}
	if c.ID == 0 {
		if err := s.db.Create(c).Error; err != nil {
			return nil, err
		}
		return c, nil
	}
	var existing model.NotifyChannel
	if err := s.db.First(&existing, c.ID).Error; err != nil {
		return nil, err
	}
	existing.Name = c.Name
	existing.Type = c.Type
	existing.ConfigJSON = c.ConfigJSON
	existing.Enabled = c.Enabled
	if err := s.db.Save(&existing).Error; err != nil {
		return nil, err
	}
	return &existing, nil
}

func (s *Store) DeleteChannel(id int64) error {
	if id <= 0 {
		return errors.New("id 无效")
	}
	var cnt int64
	if err := s.db.Model(&model.NotifyBinding{}).Where("channel_id = ?", id).Count(&cnt).Error; err != nil {
		return err
	}
	if cnt > 0 {
		return fmt.Errorf("仍有 %d 个模块绑定该通道，请先解绑", cnt)
	}
	return s.db.Delete(&model.NotifyChannel{}, id).Error
}

// Bindings 返回 module -> []channelID。
func (s *Store) Bindings() (map[string][]int64, error) {
	var rows []model.NotifyBinding
	if err := s.db.Find(&rows).Error; err != nil {
		return nil, err
	}
	out := map[string][]int64{}
	for _, r := range rows {
		out[r.Module] = append(out[r.Module], r.ChannelID)
	}
	return out, nil
}

// SetBinding 整体覆盖某模块的通道绑定。
func (s *Store) SetBinding(module string, channelIDs []int64) error {
	module = strings.TrimSpace(module)
	if module == "" {
		return errors.New("module 不能为空")
	}
	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("module = ?", module).Delete(&model.NotifyBinding{}).Error; err != nil {
			return err
		}
		for _, cid := range channelIDs {
			if cid <= 0 {
				continue
			}
			if err := tx.Create(&model.NotifyBinding{Module: module, ChannelID: cid}).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// Test 用指定通道发一条测试消息。
func (s *Store) Test(id int64) error {
	var ch model.NotifyChannel
	if err := s.db.First(&ch, id).Error; err != nil {
		return err
	}
	nf := buildChannel(ch)
	if nf == nil || !nf.Enabled() {
		return errors.New("通道配置不完整或已停用")
	}
	return nf.Send(context.Background(), Message{Title: "homer 测试通知", Text: "这是一条来自 homer 的测试消息"})
}
