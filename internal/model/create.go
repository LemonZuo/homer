package model

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

// CreateKeepZero 新建一条记录，调用方显式给的零值（false / 0）按原样落库。
//
// GORM Create 遇到带 `default:` tag 的零值字段会改用默认值：DB 默认值字段（如 `default:'1'` 的
// BoolFlag）直接不进 INSERT，可解析的 tag 默认值（如 `default:5`）连结构体一起被替换。于是新建时
// 关掉「启用」、短信选「无(0)」、提前 0 天都会被推翻成默认值；Select("*") 也绕不开。
//
// 这里不去掉 tag（去掉会让 AutoMigrate 改列默认值），而是先记下这些零值，Create 后在同一事务里
// 按原值写回，并把结构体恢复成调用方给的值。只支持单条结构体指针。
func CreateKeepZero(db *gorm.DB, value any) error {
	stmt := &gorm.Statement{DB: db}
	if err := stmt.Parse(value); err != nil {
		return err
	}
	rv := reflect.Indirect(reflect.ValueOf(value))
	if rv.Kind() != reflect.Struct || !rv.CanAddr() {
		return fmt.Errorf("CreateKeepZero: 需要结构体指针，得到 %T", value)
	}
	ctx := db.Statement.Context
	if ctx == nil {
		ctx = context.Background()
	}
	zeros := zeroDefaultColumns(ctx, stmt.Schema, rv)
	if len(zeros) == 0 {
		return db.Create(value).Error
	}
	return db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(value).Error; err != nil {
			return err
		}
		for name, v := range zeros {
			if err := stmt.Schema.FieldsByDBName[name].Set(ctx, rv, v); err != nil {
				return err
			}
		}
		return tx.Model(value).UpdateColumns(zeros).Error
	})
}

// zeroDefaultColumns 找出「带非零默认值、但本次取零值」的列：这些列 Create 时会被默认值顶掉。
func zeroDefaultColumns(ctx context.Context, sch *schema.Schema, rv reflect.Value) map[string]any {
	out := map[string]any{}
	for _, f := range sch.Fields {
		if f.DBName == "" || f.PrimaryKey || !f.HasDefaultValue || f.AutoCreateTime > 0 || f.AutoUpdateTime > 0 ||
			isZeroDefault(f.DefaultValue) {
			continue
		}
		if v, isZero := f.ValueOf(ctx, rv); isZero {
			out[f.DBName] = v
		}
	}
	return out
}

// isZeroDefault 默认值本身就是零值（或未声明 / NULL）时，Create 用默认值和用零值结果一样，不必写回。
func isZeroDefault(def string) bool {
	switch strings.ToLower(strings.Trim(strings.TrimSpace(def), `'"`)) {
	case "", "0", "false", "null":
		return true
	}
	return false
}
