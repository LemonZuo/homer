package model

import (
	"os"
	"strings"
	"testing"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// 使用专用临时数据库验证 GORM default tag 行为；默认不连接任何数据库。
func createTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("HOMER_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("HOMER_TEST_MYSQL_DSN 未配置，跳过独立 MySQL 集成测试")
	}
	if !strings.Contains(dsn, "/homer_modules_test?") {
		t.Fatal("只允许专用 homer_modules_test 数据库")
	}
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := db.AutoMigrate(&EventReminder{}, &SmsForwarder{}); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestCreateKeepZeroMySQL(t *testing.T) {
	db := createTestDB(t)

	// 先确认问题本身：普通 Create 会把用户给的 0 / false 换成默认值。
	plain := EventReminder{Title: "plain", EventDate: "2026-11-07"}
	if err := db.Create(&plain).Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Delete(&EventReminder{}, plain.ID) })
	var got EventReminder
	if err := db.First(&got, plain.ID).Error; err != nil {
		t.Fatal(err)
	}
	if got.LeadDays != 5 || !bool(got.Enabled) {
		t.Fatalf("expected GORM default override (lead=5 enabled=true), got lead=%d enabled=%v", got.LeadDays, got.Enabled)
	}

	ev := EventReminder{Title: "zero", EventDate: "2026-11-07"}
	if err := CreateKeepZero(db, &ev); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Delete(&EventReminder{}, ev.ID) })
	if ev.ID == 0 || ev.LeadDays != 0 || bool(ev.Enabled) {
		t.Fatalf("struct after create: id=%d lead=%d enabled=%v", ev.ID, ev.LeadDays, ev.Enabled)
	}
	got = EventReminder{}
	if err := db.First(&got, ev.ID).Error; err != nil {
		t.Fatal(err)
	}
	if got.LeadDays != 0 || bool(got.Enabled) || got.CreatedAt.IsZero() {
		t.Fatalf("db row: lead=%d enabled=%v created=%v", got.LeadDays, got.Enabled, got.CreatedAt)
	}

	sms := SmsForwarder{Name: "fw", AuthMode: 0, TimeoutSeconds: 15, Enabled: true}
	if err := CreateKeepZero(db, &sms); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Delete(&SmsForwarder{}, sms.ID) })
	var gotSMS SmsForwarder
	if err := db.First(&gotSMS, sms.ID).Error; err != nil {
		t.Fatal(err)
	}
	if gotSMS.AuthMode != 0 || gotSMS.TimeoutSeconds != 15 || !bool(gotSMS.Enabled) {
		t.Fatalf("sms row: auth=%d timeout=%d enabled=%v", gotSMS.AuthMode, gotSMS.TimeoutSeconds, gotSMS.Enabled)
	}
}
