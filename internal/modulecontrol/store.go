package modulecontrol

import (
	"context"
	"time"

	"github.com/LemonZuo/homer/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Store interface {
	Load(context.Context) ([]model.ModuleSetting, error)
	Save(context.Context, model.ModuleSetting, uint64) error
}

type DBStore struct{ db *gorm.DB }

func NewDBStore(db *gorm.DB) *DBStore { return &DBStore{db: db} }

func (s *DBStore) Load(ctx context.Context) ([]model.ModuleSetting, error) {
	var rows []model.ModuleSetting
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, d := range definitions {
			row := model.ModuleSetting{Key: d.Key, Enabled: true, Version: 1, UpdatedAt: time.Now()}
			if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&row).Error; err != nil {
				return err
			}
		}
		return tx.Find(&rows).Error
	})
	return rows, err
}

func (s *DBStore) Save(ctx context.Context, row model.ModuleSetting, expected uint64) error {
	res := s.db.WithContext(ctx).Model(&model.ModuleSetting{}).
		Where("module_key = ? AND version = ?", row.Key, expected).
		Updates(map[string]any{"enabled": row.Enabled, "version": row.Version, "updated_at": row.UpdatedAt})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected != 1 {
		return ErrConflict
	}
	return nil
}
