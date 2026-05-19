package sshlike

import (
	"strings"
	"testing"

	"github.com/LemonZuo/homer/internal/model"
)

func TestParseTarget(t *testing.T) {
	target, err := ParseTarget(model.ACMEDeployTarget{
		ID:       12,
		Name:     " web ",
		Endpoint: "example.com:2222",
		AuthJSON: `{
			"auth_source": "",
			"username": " root ",
			"auth_type": "",
			"password": " secret "
		}`,
		ConfigJSON: `{"bastion_target_id": 34}`,
		Enabled:    model.BoolFlag(true),
	}, Labels{Auth: "SSH", Config: "SSH", Host: "SSH"})
	if err != nil {
		t.Fatalf("ParseTarget returned error: %v", err)
	}
	if target.Name != "web" || target.Host != "example.com" || target.Port != 2222 {
		t.Fatalf("unexpected target identity: %#v", target)
	}
	if target.AuthSource != AuthSourceInline || target.AuthType != "password" || target.Username != "root" {
		t.Fatalf("unexpected auth normalization: %#v", target)
	}
	if target.BastionTargetID != 34 {
		t.Fatalf("bastion id = %d, want 34", target.BastionTargetID)
	}
}

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
