package upsmon

// UPS 历史采样清理。

import (
	"time"

	"github.com/LemonZuo/homer/internal/logx"
)

// RunCleanup 清理过期 sample,返回删除行数(用于日志)。
func (s *Service) RunCleanup() error {
	cutoff := time.Now().Add(-s.retention)
	n, err := s.store.PurgeOlderThan(cutoff)
	if err != nil {
		return err
	}
	if n > 0 {
		logx.Info("ups sample purged", "cutoff", cutoff.Format(time.RFC3339), "rows", n)
	}
	return nil
}
