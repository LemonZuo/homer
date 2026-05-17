package model

import (
	"database/sql/driver"
	"fmt"
	"strings"
	"time"

	"github.com/LemonZuo/homer/internal/chinesedate"
	"gorm.io/gorm"
)

// BoolFlag 桥接老库的 varchar("0"/"1") 与前端 / Go 侧的 bool。
//   - DB 读写：通过 Scan / Value，物理列保持 varchar
//   - JSON：序列化为 true / false，便于前端 Switch 控件直接使用
type BoolFlag bool

func (b *BoolFlag) Scan(v any) error {
	switch s := v.(type) {
	case nil:
		*b = false
	case bool:
		*b = BoolFlag(s)
	case []byte:
		*b = BoolFlag(string(s) == "1")
	case string:
		*b = BoolFlag(s == "1")
	case int64:
		*b = BoolFlag(s != 0)
	default:
		return fmt.Errorf("BoolFlag: unsupported scan type %T", v)
	}
	return nil
}

func (b BoolFlag) Value() (driver.Value, error) {
	if b {
		return "1", nil
	}
	return "0", nil
}

func (b BoolFlag) MarshalJSON() ([]byte, error) {
	if b {
		return []byte("true"), nil
	}
	return []byte("false"), nil
}

func (b *BoolFlag) UnmarshalJSON(data []byte) error {
	s := strings.TrimSpace(string(data))
	switch s {
	case "true", "1", `"1"`, `"true"`:
		*b = true
	case "false", "0", `"0"`, `"false"`, "null", `""`:
		*b = false
	default:
		return fmt.Errorf("BoolFlag: invalid json value %s", s)
	}
	return nil
}

// BirthdayRemind 生日提醒记录。
// 公历日期由用户输入，chinese_birthday / zodiac 在 BeforeSave 钩子自动计算。
type BirthdayRemind struct {
	ID              int      `gorm:"primaryKey;column:id" json:"id"`
	Name            string   `gorm:"column:name;size:30" json:"name"`
	Birthday        string   `gorm:"column:birthday;size:10" json:"birthday"`
	ChineseBirthday string   `gorm:"column:chinese_birthday;size:30" json:"chinese_birthday"`
	Zodiac          string   `gorm:"column:zodiac;size:30" json:"zodiac"`
	IsRemind        BoolFlag `gorm:"column:enabled;type:varchar(1);default:'1'" json:"enabled"`
}

func (BirthdayRemind) TableName() string { return "birthday_reminder" }

// BeforeSave 在 Create / Update 时自动根据公历生日回填 chinese_birthday 与 zodiac。
func (b *BirthdayRemind) BeforeSave(_ *gorm.DB) error {
	if b.Birthday == "" {
		return nil
	}
	t, err := time.ParseInLocation("2006-01-02", b.Birthday, time.Local)
	if err != nil {
		return err
	}
	b.ChineseBirthday = chinesedate.LunarString(t)
	b.Zodiac = chinesedate.Zodiac(t)
	return nil
}
