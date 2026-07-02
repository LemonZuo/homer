package esximon

import "testing"

func TestParseKV(t *testing.T) {
	out := `
   Vendor Name: LENOVO
   Product Name: ThinkStation P340
   Serial Number: PC1XXXXX
   Empty Value:
   : no key
plain line without colon
`
	kv := parseKV(out)
	if kv["vendor_name"] != "LENOVO" || kv["product_name"] != "ThinkStation P340" {
		t.Fatalf("kv = %v", kv)
	}
	// 空值 / 空 key / 无冒号行都跳过
	if _, ok := kv["empty_value"]; ok {
		t.Fatal("empty value must be skipped")
	}
	if len(kv) != 3 {
		t.Fatalf("len = %d: %v", len(kv), kv)
	}
}

func TestParseUint64Auto(t *testing.T) {
	if n, err := parseUint64Auto("0x64"); err != nil || n != 100 {
		t.Fatalf("hex = %d, %v", n, err)
	}
	if n, err := parseUint64Auto("  100  "); err != nil || n != 100 {
		t.Fatalf("dec = %d, %v", n, err)
	}
	if _, err := parseUint64Auto("abc"); err == nil {
		t.Fatal("garbage must error")
	}
}

func TestParseFreqMHz(t *testing.T) {
	cases := []struct {
		in   string
		want int
	}{
		{"3504 MHz", 3504},
		{"3.5 GHz", 3500},
		{"3800000000 Hz", 3800},
		{"3800000000", 3800}, // 无单位大数视为 Hz(ESXi Core Speed 字段)
		{"3504", 3504},       // 无单位合理范围视为 MHz
		{"", 0},
		{"unknown", 0},
	}
	for _, c := range cases {
		if got := parseFreqMHz(c.in); got != c.want {
			t.Errorf("parseFreqMHz(%q) = %d, want %d", c.in, got, c.want)
		}
	}
}

func TestParseSizeKB(t *testing.T) {
	cases := []struct {
		in   string
		want int
	}{
		{"256 KB", 256},
		{"8 MB", 8192},
		{"1 GiB", 1048576},
		{"512", 512},
		{"", 0},
	}
	for _, c := range cases {
		if got := parseSizeKB(c.in); got != c.want {
			t.Errorf("parseSizeKB(%q) = %d, want %d", c.in, got, c.want)
		}
	}
}

func TestParseBytes(t *testing.T) {
	cases := []struct {
		in   string
		want int64
	}{
		{"137262243840 Bytes", 137262243840},
		{"131072 MB", 131072 << 20},
		{"16 GiB", 16 << 30},
		{"1024 KB", 1 << 20},
		{"", 0},
	}
	for _, c := range cases {
		if got := parseBytes(c.in); got != c.want {
			t.Errorf("parseBytes(%q) = %d, want %d", c.in, got, c.want)
		}
	}
}

func TestSplitCSV(t *testing.T) {
	got := splitCSV(" a, b ,, c ")
	if len(got) != 3 || got[0] != "a" || got[1] != "b" || got[2] != "c" {
		t.Fatalf("got %v", got)
	}
	if splitCSV("") != nil {
		t.Fatal("empty input must yield nil")
	}
}

func TestFirstQuoted(t *testing.T) {
	if got := firstQuoted(`Socket: "U3E1" extra "second"`); got != "U3E1" {
		t.Fatalf("got %q", got)
	}
	if got := firstQuoted("no quotes"); got != "" {
		t.Fatalf("got %q", got)
	}
}
