package upsmon

import (
	"fmt"
	"time"

	"github.com/LemonZuo/homer/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Store 包裹 ups_sample / ups_state 的读写。
type Store struct {
	db *gorm.DB
}

func NewStore(db *gorm.DB) *Store { return &Store{db: db} }

// SaveSamples 批量写入采样,空切片直接返回。
func (s *Store) SaveSamples(samples []model.UPSSample) error {
	if len(samples) == 0 {
		return nil
	}
	// 100 条一批,避免单 INSERT 过大
	return s.db.CreateInBatches(samples, 100).Error
}

// UpsertState 把当前一轮里每个 UPS 的最新状态 upsert 到 ups_state。
// 注意:不在这里触发告警 —— 告警由 notifier 在 service 里基于"前一轮 state vs 本轮 reading"判断。
func (s *Store) UpsertState(states []model.UPSState) error {
	if len(states) == 0 {
		return nil
	}
	return s.db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "host_kind"}, {Name: "host_id"}, {Name: "ups_name"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"host_name", "mfr", "model",
			"last_power_source", "last_battery_percent", "last_runtime_minutes",
			"last_input_voltage", "last_output_voltage", "last_load_percent", "last_real_power",
			"last_raw_status",
			"updated_at",
		}),
	}).Create(&states).Error
}

// MarkAlerted 写入 last_alert_at;告警发送后由 notifier 调用。
func (s *Store) MarkAlerted(hostKind string, hostID int64, upsName string, at time.Time) error {
	return s.db.Model(&model.UPSState{}).
		Where("host_kind = ? AND host_id = ? AND ups_name = ?", hostKind, hostID, upsName).
		Update("last_alert_at", at).Error
}

// LoadStates 读全部 state(行数 = 接 UPS 的台数 × 每台 UPS 数,极小)。
func (s *Store) LoadStates() ([]model.UPSState, error) {
	var rows []model.UPSState
	if err := s.db.Order("host_kind, host_id, ups_name").Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// LatestSamplesByHostUPS 取每台 (host, ups) 的最新一条 sample。
// 用 GROUP BY 子查询取 max(sampled_at),再 join 回 sample 表。
func (s *Store) LatestSamplesByHostUPS() ([]model.UPSSample, error) {
	var rows []model.UPSSample
	sub := s.db.Model(&model.UPSSample{}).
		Select("host_kind, host_id, ups_name, MAX(sampled_at) AS max_at").
		Group("host_kind, host_id, ups_name")
	err := s.db.Model(&model.UPSSample{}).
		Joins("JOIN (?) AS m ON m.host_kind = ups_sample.host_kind AND m.host_id = ups_sample.host_id AND m.ups_name = ups_sample.ups_name AND m.max_at = ups_sample.sampled_at", sub).
		Order("host_kind, host_id, ups_name").
		Find(&rows).Error
	return rows, err
}

// SeriesPoint 一个聚合桶,前端画曲线用。
type SeriesPoint struct {
	BucketStart    time.Time `json:"bucket_start"`
	BatteryPercent int       `json:"battery_percent"` // 桶内 min(让谷底可见)
	RuntimeMinutes int       `json:"runtime_minutes"` // 桶内 avg
	PowerSource    string    `json:"power_source"`    // 桶内最严重
}

// Series 按时间桶聚合某个 UPS 的采样序列。
// bucketSec ≥ 60,since 是包含下界。返回按 bucket_start 升序。
//
// 聚合规则:
//   - battery_percent: MIN()(谷底可见,与告警语义一致)
//   - runtime_minutes: AVG() 取整
//   - power_source: 桶内只要出现过 low_battery 就标 low_battery,否则 battery,否则 mains,否则 unknown
func (s *Store) Series(hostKind string, hostID int64, upsName string, since time.Time, bucketSec int) ([]SeriesPoint, error) {
	if bucketSec < 60 {
		bucketSec = 60
	}
	// 分桶用 UNIX_TIMESTAMP DIV bucketSec 取整,再 FROM_UNIXTIME 转回 DATETIME 字符串。
	// 为什么不直接 SELECT 那个 bucket 整数?— 因为 sampled_at 是 DATETIME(无时区),
	// 当 MySQL session.time_zone 与 Go Local 不一致(例如 MySQL=UTC、Go=CST)时,
	// 那个整数会带上 session.time_zone 的偏移误差。FROM_UNIXTIME 再用同一个 session
	// 时区转回字符串,误差被对称地消掉;driver loc=Local 把字符串解读为 Go Local 的
	// time.Time,得到的是钟面时间,跟 sampled_at 直接读出来语义一致。
	type row struct {
		BucketTime time.Time
		MinBattery int
		AvgRuntime float64
		HasLB      int
		HasOB      int
		HasOL      int
	}
	var rows []row
	bucketExpr := fmt.Sprintf("FROM_UNIXTIME((UNIX_TIMESTAMP(sampled_at) DIV %d) * %d)", bucketSec, bucketSec)
	err := s.db.Model(&model.UPSSample{}).
		Select(bucketExpr+" AS bucket_time, MIN(CASE WHEN battery_percent>=0 THEN battery_percent ELSE NULL END) AS min_battery, AVG(CASE WHEN runtime_minutes>=0 THEN runtime_minutes ELSE NULL END) AS avg_runtime, SUM(CASE WHEN power_source='low_battery' THEN 1 ELSE 0 END) AS has_lb, SUM(CASE WHEN power_source='battery' THEN 1 ELSE 0 END) AS has_ob, SUM(CASE WHEN power_source='mains' THEN 1 ELSE 0 END) AS has_ol").
		Where("host_kind = ? AND host_id = ? AND ups_name = ? AND sampled_at >= ?", hostKind, hostID, upsName, since).
		Group("bucket_time").
		Order("bucket_time ASC").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	points := make([]SeriesPoint, 0, len(rows))
	for _, r := range rows {
		ps := model.UPSPowerUnknown
		switch {
		case r.HasLB > 0:
			ps = model.UPSPowerLowBattery
		case r.HasOB > 0:
			ps = model.UPSPowerBattery
		case r.HasOL > 0:
			ps = model.UPSPowerMains
		}
		points = append(points, SeriesPoint{
			BucketStart:    r.BucketTime,
			BatteryPercent: r.MinBattery,
			RuntimeMinutes: int(r.AvgRuntime + 0.5),
			PowerSource:    ps,
		})
	}
	return points, nil
}

// PurgeOlderThan 清理 cutoff 之前的 sample,返回删除行数。
func (s *Store) PurgeOlderThan(cutoff time.Time) (int64, error) {
	res := s.db.Where("sampled_at < ?", cutoff).Delete(&model.UPSSample{})
	return res.RowsAffected, res.Error
}
