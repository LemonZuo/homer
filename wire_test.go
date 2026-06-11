package main

import (
	"testing"
	"time"
)

func TestDailyMaintenanceWindowContains(t *testing.T) {
	w, err := parseDailyMaintenanceWindow("04:25-04:35")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	cases := []struct {
		name string
		at   time.Time
		want bool
	}{
		{name: "before", at: clockAt(4, 24), want: false},
		{name: "start", at: clockAt(4, 25), want: true},
		{name: "inside", at: clockAt(4, 34), want: true},
		{name: "end", at: clockAt(4, 35), want: false},
	}
	for _, tc := range cases {
		if got := w.Contains(tc.at); got != tc.want {
			t.Fatalf("%s: Contains(%s) = %v want %v", tc.name, tc.at.Format("15:04"), got, tc.want)
		}
	}
}

func TestDailyMaintenanceWindowContainsAcrossMidnight(t *testing.T) {
	w, err := parseDailyMaintenanceWindow("23:55-00:05")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	cases := []struct {
		at   time.Time
		want bool
	}{
		{at: clockAt(23, 54), want: false},
		{at: clockAt(23, 55), want: true},
		{at: clockAt(0, 4), want: true},
		{at: clockAt(0, 5), want: false},
	}
	for _, tc := range cases {
		if got := w.Contains(tc.at); got != tc.want {
			t.Fatalf("Contains(%s) = %v want %v", tc.at.Format("15:04"), got, tc.want)
		}
	}
}

func TestDailyMaintenanceWindowDisabled(t *testing.T) {
	for _, spec := range []string{"", "off", "none", "disabled"} {
		w, err := parseDailyMaintenanceWindow(spec)
		if err != nil {
			t.Fatalf("parse disabled %q: %v", spec, err)
		}
		if w.Contains(clockAt(4, 30)) {
			t.Fatalf("disabled window %q should not contain any time", spec)
		}
	}
}

func TestDailyMaintenanceWindowRejectsInvalidSpec(t *testing.T) {
	for _, spec := range []string{"04:25", "24:00-04:35", "04:35-04:35"} {
		if _, err := parseDailyMaintenanceWindow(spec); err == nil {
			t.Fatalf("parse %q should fail", spec)
		}
	}
}

func clockAt(hour, minute int) time.Time {
	return time.Date(2026, time.January, 2, hour, minute, 0, 0, time.Local)
}
