package sshlike

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestSplitEndpoint(t *testing.T) {
	tests := []struct {
		name     string
		endpoint string
		wantHost string
		wantPort int
		wantErr  string
	}{
		{name: "host only", endpoint: "example.com", wantHost: "example.com", wantPort: 22},
		{name: "host colon port", endpoint: "example.com:2200", wantHost: "example.com", wantPort: 2200},
		{name: "bracket ipv6", endpoint: "[2001:db8::1]:2222", wantHost: "2001:db8::1", wantPort: 2222},
		{name: "blank", endpoint: " ", wantErr: "fnOS 主机不能为空"},
		{name: "bad port", endpoint: "example.com:ssh", wantErr: "fnOS 端口无效"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			host, port, err := SplitEndpoint(tc.endpoint, "fnOS")
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("err = %v, want contains %q", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("SplitEndpoint returned error: %v", err)
			}
			if host != tc.wantHost || port != tc.wantPort {
				t.Fatalf("got %s:%d, want %s:%d", host, port, tc.wantHost, tc.wantPort)
			}
		})
	}
}

func TestValidateTarget(t *testing.T) {
	t.Run("credential mode only requires credential id", func(t *testing.T) {
		target := Target{
			Name:         "web",
			Host:         "example.com",
			Port:         22,
			AuthSource:   AuthSourceCredential,
			CredentialID: 1,
		}
		if err := ValidateTarget(target, "SSH"); err != nil {
			t.Fatalf("ValidateTarget returned error: %v", err)
		}
	})

	t.Run("inline password requires username and password", func(t *testing.T) {
		target := Target{
			Name:       "web",
			Host:       "example.com",
			Port:       22,
			AuthSource: AuthSourceInline,
			AuthType:   "password",
			Username:   "root",
		}
		err := ValidateTarget(target, "SSH")
		if err == nil || !strings.Contains(err.Error(), "密码认证需要填写密码") {
			t.Fatalf("err = %v, want password error", err)
		}
	})
}

func TestNormalize(t *testing.T) {
	target := Target{
		Name:       " web ",
		Host:       " example.com ",
		AuthSource: " Credential ",
		Username:   " root ",
		AuthType:   "",
	}
	Normalize(&target)
	if target.Name != "web" || target.Host != "example.com" || target.Username != "root" {
		t.Fatalf("trim failed: %#v", target)
	}
	if target.AuthSource != AuthSourceCredential {
		t.Fatalf("auth source should lowercase: %s", target.AuthSource)
	}
	if target.Port != 22 {
		t.Fatalf("port should default to 22, got %d", target.Port)
	}
	// 凭证模式下 AuthType 留空(凭证库会填),非凭证模式下补 password。
	if target.AuthType != "" {
		t.Fatalf("credential mode auth_type should stay empty, got %q", target.AuthType)
	}
}

func TestMarshalAuthJSON_CredentialMode(t *testing.T) {
	target := Target{
		AuthSource:   AuthSourceCredential,
		CredentialID: 42,
		Username:     "ignored",
		Password:     "ignored",
	}
	got, err := MarshalAuthJSON(target)
	if err != nil {
		t.Fatalf("MarshalAuthJSON: %v", err)
	}
	var parsed TargetAuth
	if err := json.Unmarshal([]byte(got), &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if parsed.AuthSource != AuthSourceCredential || parsed.CredentialID != 42 {
		t.Fatalf("unexpected: %#v", parsed)
	}
	if parsed.Username != "" || parsed.Password != "" {
		t.Fatalf("credential mode should not serialize inline fields, got %#v", parsed)
	}
}

func TestMarshalConfigJSON(t *testing.T) {
	got, err := MarshalConfigJSON(Target{BastionID: 7})
	if err != nil {
		t.Fatalf("MarshalConfigJSON: %v", err)
	}
	if got != `{"bastion_id":7}` {
		t.Fatalf("got %s, want bastion_id=7", got)
	}

	got, err = MarshalConfigJSON(Target{})
	if err != nil {
		t.Fatalf("MarshalConfigJSON empty: %v", err)
	}
	if got != `{}` {
		t.Fatalf("empty target should serialize to {}, got %s", got)
	}
}

func TestUnmarshalConfigJSON(t *testing.T) {
	tests := []struct {
		name      string
		in        string
		wantBaste int64
	}{
		{name: "empty", in: "", wantBaste: 0},
		{name: "blank object", in: "{}", wantBaste: 0},
		{name: "with bastion", in: `{"bastion_id":99}`, wantBaste: 99},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := UnmarshalConfigJSON(tc.in)
			if err != nil {
				t.Fatalf("UnmarshalConfigJSON: %v", err)
			}
			if cfg.BastionID != tc.wantBaste {
				t.Fatalf("bastion = %d, want %d", cfg.BastionID, tc.wantBaste)
			}
		})
	}
}

type stubResolver struct {
	cred Credential
	err  error
}

func (s stubResolver) Resolve(_ int64) (Credential, error) { return s.cred, s.err }

func TestResolveCredential_InlineModeNoOp(t *testing.T) {
	target := Target{AuthSource: AuthSourceInline, Username: "root", Password: "p"}
	if err := ResolveCredential(stubResolver{}, &target); err != nil {
		t.Fatalf("inline mode should be no-op, got err: %v", err)
	}
	if target.Username != "root" || target.Password != "p" {
		t.Fatalf("inline fields touched: %#v", target)
	}
}

func TestResolveCredential_CredentialMode(t *testing.T) {
	target := Target{
		AuthSource:   AuthSourceCredential,
		CredentialID: 7,
	}
	resolver := stubResolver{cred: Credential{
		Username: " admin ", AuthType: " KEY ", PrivateKey: "PEM",
	}}
	if err := ResolveCredential(resolver, &target); err != nil {
		t.Fatalf("ResolveCredential: %v", err)
	}
	if target.Username != "admin" || target.AuthType != "key" || target.PrivateKey != "PEM" {
		t.Fatalf("credential not applied: %#v", target)
	}
}

func TestValidateBastion_SelfRef(t *testing.T) {
	err := ValidateBastion(nil, nil, Target{ID: 5, BastionID: 5}, "本机")
	if err == nil || !strings.Contains(err.Error(), "跳板机不能是自己") {
		t.Fatalf("err = %v, want self-ref", err)
	}
}

func TestValidateBastion_UpstreamRef(t *testing.T) {
	finder := func(id int64) (string, bool, error) { return "downstream", true, nil }
	loader := func(id int64) (*Target, error) { return &Target{ID: 7}, nil }
	err := ValidateBastion(loader, finder, Target{ID: 3, BastionID: 7}, "本机")
	if err == nil || !strings.Contains(err.Error(), "downstream") {
		t.Fatalf("err = %v, want upstream-ref", err)
	}
}

func TestValidateBastion_Chain(t *testing.T) {
	finder := func(id int64) (string, bool, error) { return "", false, nil }
	loader := func(id int64) (*Target, error) { return &Target{ID: 7, BastionID: 9}, nil }
	err := ValidateBastion(loader, finder, Target{ID: 3, BastionID: 7}, "本机")
	if err == nil || !strings.Contains(err.Error(), "单跳") {
		t.Fatalf("err = %v, want chain rejection", err)
	}
}
