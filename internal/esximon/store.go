package esximon

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/LemonZuo/homer/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Store 包裹 esxi_sample / esxi_state 的读写。
type Store struct {
	db *gorm.DB
}

func NewStore(db *gorm.DB) *Store { return &Store{db: db} }

// SaveSamples 批量写入采样,空切片直接返回。
func (s *Store) SaveSamples(samples []model.EsxiSample) error {
	if len(samples) == 0 {
		return nil
	}
	return s.db.CreateInBatches(samples, 100).Error
}

// UpsertState 把当前一轮里每台机器的最新快照 upsert 到 esxi_state。
// 不在这里触发告警 —— 告警由 service 状态机基于"前一轮 state vs 本轮 reading"判断。
func (s *Store) UpsertState(states []model.EsxiState) error {
	if len(states) == 0 {
		return nil
	}
	return s.db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "host_kind"}, {Name: "host_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"host_name", "reachable", "last_error",
			"platform_json", "cpu_static_json", "memory_json", "runtime_json",
			"cpu_temp_json", "mce_json", "disk_json", "usb_json", "vm_json",
			"boot_json", "nic_json",
			"sampled_at", "updated_at",
		}),
	}).Create(&states).Error
}

// MarkAlerted 写入 last_alert_at;告警发送后由 service 调用。
func (s *Store) MarkAlerted(hostKind string, hostID int64, at time.Time) error {
	return s.db.Model(&model.EsxiState{}).
		Where("host_kind = ? AND host_id = ?", hostKind, hostID).
		Update("last_alert_at", at).Error
}

// LoadStates 读全部 state(每台 host 一行,极小)。
func (s *Store) LoadStates() ([]model.EsxiState, error) {
	var rows []model.EsxiState
	if err := s.db.Order("host_kind, host_id").Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// SeriesPoint 一个聚合桶,前端画曲线用。
// 缺数据的指标用 -1 表示(与 sample 表里的哨兵约定一致)。
//
// CPUCores / Disks 是当桶内"代表样本"(MAX(id) 那行,即桶内最新)的明细数组,
// 让前端能按核 / 按盘单独画线;旧行(改造前的样本)这两列为 NULL,
// 桶代表为空即数组为 nil,前端跳过该桶不画线。
type SeriesPoint struct {
	BucketStart         time.Time       `json:"bucket_start"`
	CPUMaxC             int             `json:"cpu_max_c"`             // °C,桶内 max(峰值更有意义)
	CPUAvgC             int             `json:"cpu_avg_c"`             // °C,桶内 avg
	DiskMaxC            int             `json:"disk_max_c"`            // °C,桶内 max
	MCECorrectedTotal   int64           `json:"mce_corrected_total"`   // 桶内 max(累计单调)
	MCEUncorrectedTotal int64           `json:"mce_uncorrected_total"` // 桶内 max
	VMPoweredOn         int             `json:"vm_powered_on"`         // 桶内 max
	CPUCores            []CoreTempPoint `json:"cpu_cores,omitempty"`   // 每核温度明细
	Disks               []DiskTempPoint `json:"disks,omitempty"`       // 每盘温度明细
}

// Series 按时间桶聚合某台 ESXi 的采样序列。
// bucketSec ≥ 60,since 是包含下界。返回按 bucket_start 升序。
//
// 聚合规则:温度类用 MAX(看高点),avg 用 AVG;MCE 累计 / VM 数用 MAX(单调递增,
// 但跨重启会重置,桶内取 max 即可)。
func (s *Store) Series(hostKind string, hostID int64, since time.Time, bucketSec int) ([]SeriesPoint, error) {
	if bucketSec < 60 {
		bucketSec = 60
	}
	type row struct {
		BucketTime   time.Time
		MaxCPUMax    *int
		AvgCPUAvg    *float64
		MaxDiskMax   *int
		MaxMCECorr   *int64
		MaxMCEUncorr *int64
		MaxVMOn      *int
	}
	var rows []row
	bucketExpr := fmt.Sprintf("FROM_UNIXTIME((UNIX_TIMESTAMP(sampled_at) DIV %d) * %d)", bucketSec, bucketSec)
	err := s.db.Model(&model.EsxiSample{}).
		Select(bucketExpr+" AS bucket_time,"+
			" MAX(CASE WHEN cpu_max_c>=0 THEN cpu_max_c ELSE NULL END) AS max_cpu_max,"+
			" AVG(CASE WHEN cpu_avg_c>=0 THEN cpu_avg_c ELSE NULL END) AS avg_cpu_avg,"+
			" MAX(CASE WHEN disk_max_c>=0 THEN disk_max_c ELSE NULL END) AS max_disk_max,"+
			" MAX(mce_corrected_total) AS max_mce_corr,"+
			" MAX(mce_uncorrected_total) AS max_mce_uncorr,"+
			" MAX(CASE WHEN vm_powered_on>=0 THEN vm_powered_on ELSE NULL END) AS max_vm_on").
		Where("host_kind = ? AND host_id = ? AND sampled_at >= ?", hostKind, hostID, since).
		Group("bucket_time").
		Order("bucket_time ASC").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	points := make([]SeriesPoint, 0, len(rows))
	for _, r := range rows {
		p := SeriesPoint{BucketStart: r.BucketTime, CPUMaxC: -1, CPUAvgC: -1, DiskMaxC: -1, VMPoweredOn: -1}
		if r.MaxCPUMax != nil {
			p.CPUMaxC = *r.MaxCPUMax
		}
		if r.AvgCPUAvg != nil {
			p.CPUAvgC = int(*r.AvgCPUAvg)
		}
		if r.MaxDiskMax != nil {
			p.DiskMaxC = *r.MaxDiskMax
		}
		if r.MaxMCECorr != nil {
			p.MCECorrectedTotal = *r.MaxMCECorr
		}
		if r.MaxMCEUncorr != nil {
			p.MCEUncorrectedTotal = *r.MaxMCEUncorr
		}
		if r.MaxVMOn != nil {
			p.VMPoweredOn = *r.MaxVMOn
		}
		points = append(points, p)
	}

	// 二次 query:取每桶 MAX(id) 那行的明细 JSON 作为"代表样本",
	// 让前端能按核 / 按盘单独画线。bucketSec 是程序内固定整数,无注入风险。
	detailSQL := fmt.Sprintf(`
SELECT FROM_UNIXTIME((UNIX_TIMESTAMP(s.sampled_at) DIV %d) * %d) AS bucket_time,
       s.cpu_temp_json  AS cpu_temp_json,
       s.disk_temp_json AS disk_temp_json
FROM esxi_sample s
INNER JOIN (
  SELECT MAX(id) AS max_id
  FROM esxi_sample
  WHERE host_kind = ? AND host_id = ? AND sampled_at >= ?
  GROUP BY FROM_UNIXTIME((UNIX_TIMESTAMP(sampled_at) DIV %d) * %d)
) latest ON s.id = latest.max_id
ORDER BY bucket_time ASC
`, bucketSec, bucketSec, bucketSec, bucketSec)
	type detailRow struct {
		BucketTime   time.Time
		CPUTempJSON  string
		DiskTempJSON string
	}
	var details []detailRow
	if err := s.db.Raw(detailSQL, hostKind, hostID, since).Scan(&details).Error; err != nil {
		return nil, err
	}
	detailByBucket := map[int64]detailRow{}
	for _, d := range details {
		detailByBucket[d.BucketTime.Unix()] = d
	}
	for i := range points {
		d, ok := detailByBucket[points[i].BucketStart.Unix()]
		if !ok {
			continue
		}
		if d.CPUTempJSON != "" {
			var cores []CoreTempPoint
			if json.Unmarshal([]byte(d.CPUTempJSON), &cores) == nil {
				points[i].CPUCores = cores
			}
		}
		if d.DiskTempJSON != "" {
			var disks []DiskTempPoint
			if json.Unmarshal([]byte(d.DiskTempJSON), &disks) == nil {
				points[i].Disks = disks
			}
		}
	}
	return points, nil
}

// PurgeOlderThan 清理 cutoff 之前的 sample,返回删除行数。
func (s *Store) PurgeOlderThan(cutoff time.Time) (int64, error) {
	res := s.db.Where("sampled_at < ?", cutoff).Delete(&model.EsxiSample{})
	return res.RowsAffected, res.Error
}
