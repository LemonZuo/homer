package model

import (
	"context"
	"reflect"
	"sync"
	"testing"

	"gorm.io/gorm/schema"
)

func zeroCols(t *testing.T, v any) map[string]any {
	t.Helper()
	sch, err := schema.Parse(v, &sync.Map{}, schema.NamingStrategy{})
	if err != nil {
		t.Fatal(err)
	}
	return zeroDefaultColumns(context.Background(), sch, reflect.Indirect(reflect.ValueOf(v)))
}

func TestZeroDefaultColumns(t *testing.T) {
	got := zeroCols(t, &EventReminder{Title: "x"})
	if len(got) != 2 || got["lead_days"] != 0 || got["enabled"] != BoolFlag(false) {
		t.Fatalf("event zero cols = %#v", got)
	}

	if got := zeroCols(t, &EventReminder{LeadDays: 3, Enabled: true}); len(got) != 0 {
		t.Fatalf("non-zero values need no write-back, got %#v", got)
	}

	got = zeroCols(t, &SmsForwarder{TimeoutSeconds: 30, Enabled: true})
	if len(got) != 1 || got["auth_mode"] != 0 {
		t.Fatalf("sms auth_mode=0 must be kept, got %#v", got)
	}

	// 默认值本身是零值（default:'0'）的列不写回；主键 / 自动时间列不处理。
	got = zeroCols(t, &ACMEDeployConfig{Enabled: true})
	if _, ok := got["auto_deploy"]; ok {
		t.Fatalf("zero-default column must be skipped, got %#v", got)
	}
	for _, col := range []string{"id", "created_at", "updated_at"} {
		if _, ok := got[col]; ok {
			t.Fatalf("%s must be skipped, got %#v", col, got)
		}
	}
}

func TestIsZeroDefault(t *testing.T) {
	for def, want := range map[string]bool{"": true, "'0'": true, "0": true, "false": true, "NULL": true, "'1'": false, "5": false, "30": false} {
		if got := isZeroDefault(def); got != want {
			t.Errorf("isZeroDefault(%q) = %v, want %v", def, got, want)
		}
	}
}
