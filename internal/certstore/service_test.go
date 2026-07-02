package certstore

import (
	"errors"
	"testing"
)

func TestConfigured(t *testing.T) {
	if NewService("", "").Configured() {
		t.Fatal("empty AK/SK must be unconfigured")
	}
	if NewService("", "sk").Configured() {
		t.Fatal("missing AK must be unconfigured")
	}
	if !NewService("ak", "sk").Configured() {
		t.Fatal("both set must be configured")
	}
}

func TestUnconfiguredReturnsErrNotConfigured(t *testing.T) {
	s := NewService("", "")
	if _, err := s.ListCertificates(); !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("ListCertificates err = %v", err)
	}
	if err := s.DeleteCertificate(1); !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("DeleteCertificate err = %v", err)
	}
}

func TestUploadCertificateValidation(t *testing.T) {
	s := NewService("ak", "sk")
	// 入参校验先于 client 创建,空参不发外部请求
	for _, c := range []struct{ name, cert, key string }{
		{"", "CERT", "KEY"},
		{"n", "", "KEY"},
		{"n", "CERT", ""},
	} {
		if _, err := s.UploadCertificate(c.name, c.cert, c.key); err == nil {
			t.Fatalf("empty field must error: %+v", c)
		}
	}
}
