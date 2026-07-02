package acmesafeline

import (
	"encoding/json"
	"testing"
)

func TestNormalizeCertType(t *testing.T) {
	cases := []struct{ in, want int }{
		{0, 2},
		{-1, 2},
		{1, 1},
		{2, 2},
	}
	for _, c := range cases {
		if got := normalizeCertType(c.in); got != c.want {
			t.Errorf("normalizeCertType(%d) = %d, want %d", c.in, got, c.want)
		}
	}
}

func TestDecodeID(t *testing.T) {
	// 雷池不同版本返回形态不一:整数 / 浮点 / 字符串 / 对象包 id / cert_id
	cases := []struct {
		raw  string
		want int64
	}{
		{`7`, 7},
		{`7.0`, 7},
		{`"42"`, 42},
		{`{"id": 9}`, 9},
		{`{"cert_id": "11"}`, 11},
	}
	for _, c := range cases {
		got, err := decodeID(json.RawMessage(c.raw))
		if err != nil {
			t.Errorf("decodeID(%s) err: %v", c.raw, err)
			continue
		}
		if got != c.want {
			t.Errorf("decodeID(%s) = %d, want %d", c.raw, got, c.want)
		}
	}
	if _, err := decodeID(json.RawMessage(`{"other": 1}`)); err == nil {
		t.Fatal("no id field must error")
	}
	if _, err := decodeID(json.RawMessage(`"not-a-number"`)); err == nil {
		t.Fatal("non-numeric string must error")
	}
}

func TestDecodeOut(t *testing.T) {
	// out=nil → 丢弃
	if err := decodeOut(json.RawMessage(`{"x":1}`), nil); err != nil {
		t.Fatal(err)
	}
	// *int64 → decodeID 兜底
	var id int64
	if err := decodeOut(json.RawMessage(`"33"`), &id); err != nil || id != 33 {
		t.Fatalf("id = %d, err = %v", id, err)
	}
	// 普通结构体
	var v struct {
		Name string `json:"name"`
	}
	if err := decodeOut(json.RawMessage(`{"name":"waf"}`), &v); err != nil || v.Name != "waf" {
		t.Fatalf("v = %+v, err = %v", v, err)
	}
	// data 为空 / null → 不动 out 也不报错
	v.Name = "keep"
	if err := decodeOut(nil, &v); err != nil || v.Name != "keep" {
		t.Fatalf("empty data: v = %+v, err = %v", v, err)
	}
	if err := decodeOut(json.RawMessage(`null`), &v); err != nil || v.Name != "keep" {
		t.Fatalf("null data: v = %+v, err = %v", v, err)
	}
}
