package model

import (
	"encoding/json"
	"testing"
)

func TestBoolFlagScan(t *testing.T) {
	tests := []struct {
		name    string
		in      any
		want    bool
		wantErr bool
	}{
		{"nil", nil, false, false},
		{"bool true", true, true, false},
		{"bool false", false, false, false},
		{"bytes 1", []byte("1"), true, false},
		{"bytes 0", []byte("0"), false, false},
		{"string 1", "1", true, false},
		{"string other", "x", false, false},
		{"int64 nonzero", int64(5), true, false},
		{"int64 zero", int64(0), false, false},
		{"unsupported", 3.14, false, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var b BoolFlag
			err := b.Scan(tc.in)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if bool(b) != tc.want {
				t.Fatalf("Scan(%v)=%v want %v", tc.in, bool(b), tc.want)
			}
		})
	}
}

func TestBoolFlagValue(t *testing.T) {
	v, _ := BoolFlag(true).Value()
	if v != "1" {
		t.Fatalf("true => %v want \"1\"", v)
	}
	v, _ = BoolFlag(false).Value()
	if v != "0" {
		t.Fatalf("false => %v want \"0\"", v)
	}
}

func TestBoolFlagJSONRoundTrip(t *testing.T) {
	type wrap struct {
		E BoolFlag `json:"e"`
	}
	b, _ := json.Marshal(wrap{E: true})
	if string(b) != `{"e":true}` {
		t.Fatalf("marshal = %s", b)
	}

	accepts := map[string]bool{
		`true`: true, `1`: true, `"1"`: true, `"true"`: true,
		`false`: false, `0`: false, `"0"`: false, `"false"`: false,
		`null`: false, `""`: false,
	}
	for raw, want := range accepts {
		var f BoolFlag
		if err := json.Unmarshal([]byte(raw), &f); err != nil {
			t.Fatalf("Unmarshal(%s) error: %v", raw, err)
		}
		if bool(f) != want {
			t.Fatalf("Unmarshal(%s)=%v want %v", raw, bool(f), want)
		}
	}

	var f BoolFlag
	if err := json.Unmarshal([]byte(`"maybe"`), &f); err == nil {
		t.Fatal("invalid json value should error")
	}
}
