package acme

import (
	"encoding/json"
	"strings"
)

// JSONMarshalIndent / JSONUnmarshal 是包外子包（ssh、safeline）与本包共用的 JSON 工具。
func JSONMarshalIndent(v any) ([]byte, error) {
	return json.MarshalIndent(v, "", "  ")
}

func JSONUnmarshal(data []byte, v any) error {
	return json.Unmarshal(data, v)
}

// MustJSON 把任意值序列化成 JSON 字符串，失败时返回 "{}"。供 driver/store 拼装 ConfigJSON 使用。
func MustJSON(v any) string {
	data, err := JSONMarshalIndent(v)
	if err != nil {
		return "{}"
	}
	return string(data)
}

// EmptyJSON 在 JSON 字符串为空白时回退成 "{}"，避免下游 json.Unmarshal 失败。
func EmptyJSON(s string) string {
	if strings.TrimSpace(s) == "" {
		return "{}"
	}
	return s
}
