package upsmon

import (
	"strings"
	"testing"

	"github.com/LemonZuo/homer/internal/model"
	"github.com/LemonZuo/homer/internal/sshlike"
)

func TestParseUPSTargetInlinePassword(t *testing.T) {
	h := model.UPSHost{
		ID:       7,
		Name:     "  fnOS  ",
		Endpoint: "192.168.31.2:2222",
		AuthJSON: `{"username":"upsmon","auth_type":"password","password":"secret"}`,
		Enabled:  true,
	}
	got, err := ParseUPSTarget(h)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != 7 || got.Name != "fnOS" {
		t.Fatalf("identity = %+v", got)
	}
	if got.Host != "192.168.31.2" || got.Port != 2222 {
		t.Fatalf("endpoint = %s:%d", got.Host, got.Port)
	}
	if got.AuthSource != sshlike.AuthSourceInline || got.AuthType != "password" || got.Password != "secret" {
		t.Fatalf("auth = %+v", got)
	}
	if !got.Enabled {
		t.Fatal("enabled must carry over")
	}
}

func TestParseUPSTargetDefaultsAndCredentialSource(t *testing.T) {
	// 无端口 → 默认 22;credential 引用 + bastion
	h := model.UPSHost{
		ID:         1,
		Name:       "nas",
		Endpoint:   "192.168.31.3",
		AuthJSON:   `{"auth_source":"credential","credential_id":5}`,
		ConfigJSON: `{"bastion_id":9}`,
		Enabled:    true,
	}
	got, err := ParseUPSTarget(h)
	if err != nil {
		t.Fatal(err)
	}
	if got.Port != 22 {
		t.Fatalf("default port = %d", got.Port)
	}
	if got.AuthSource != sshlike.AuthSourceCredential || got.CredentialID != 5 {
		t.Fatalf("credential source = %+v", got)
	}
	if got.BastionID != 9 {
		t.Fatalf("bastion = %d", got.BastionID)
	}
}

func TestParseUPSTargetEmptyJSONTolerated(t *testing.T) {
	// auth_json/config_json 空串按 {} 处理,不报错
	h := model.UPSHost{ID: 1, Name: "n", Endpoint: "10.0.0.1"}
	got, err := ParseUPSTarget(h)
	if err != nil {
		t.Fatal(err)
	}
	// Normalize 补默认:inline + password
	if got.AuthSource != sshlike.AuthSourceInline || got.AuthType != "password" {
		t.Fatalf("defaults = %+v", got)
	}
}

func TestParseUPSTargetBadJSON(t *testing.T) {
	h := model.UPSHost{ID: 1, Name: "n", Endpoint: "10.0.0.1", AuthJSON: "{not json"}
	if _, err := ParseUPSTarget(h); err == nil || !strings.Contains(err.Error(), "认证配置") {
		t.Fatalf("expected auth json error, got %v", err)
	}
	h = model.UPSHost{ID: 1, Name: "n", Endpoint: "10.0.0.1", ConfigJSON: "{not json"}
	if _, err := ParseUPSTarget(h); err == nil || !strings.Contains(err.Error(), "连接配置") {
		t.Fatalf("expected config json error, got %v", err)
	}
}

func TestParseUPSTargetBadEndpoint(t *testing.T) {
	h := model.UPSHost{ID: 1, Name: "n", Endpoint: "  "}
	if _, err := ParseUPSTarget(h); err == nil {
		t.Fatal("empty endpoint must error")
	}
	h = model.UPSHost{ID: 1, Name: "n", Endpoint: "10.0.0.1:notaport"}
	if _, err := ParseUPSTarget(h); err == nil {
		t.Fatal("invalid port must error")
	}
}
