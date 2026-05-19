package acme

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"testing"
	"time"

	"github.com/LemonZuo/homer/internal/model"
)

func TestNormalizeSanProviders(t *testing.T) {
	tests := []struct {
		name     string
		in       string
		wantErr  bool
		emptyOut bool
	}{
		{name: "blank stays blank", in: "  ", emptyOut: true},
		{name: "invalid json errors", in: "{not json", wantErr: true},
		{name: "all empty values collapse to blank", in: `{"a":"","  ":"x"}`, emptyOut: true},
		{name: "valid kept", in: `{"a.com":"alidns"}`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			out, err := normalizeSanProviders(tc.in)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tc.emptyOut && out != "" {
				t.Fatalf("expected empty, got %q", out)
			}
			if !tc.emptyOut && out == "" {
				t.Fatal("expected non-empty normalized json")
			}
		})
	}
}

func TestParseSanProviders(t *testing.T) {
	d := model.ACMEDomain{
		Provider:     "cloudflare",
		SanProviders: `{" a.com ":" alidns ","b.com":"cloudflare","c.com":"","":"x"}`,
	}
	got := ParseSanProviders(d)
	if len(got) != 1 {
		t.Fatalf("expected only a.com kept, got %v", got)
	}
	if got["a.com"] != "alidns" {
		t.Fatalf("trim failed: %v", got)
	}
	if _, ok := got["b.com"]; ok {
		t.Fatal("entry equal to default provider must be dropped")
	}
}

func TestBuildDomains(t *testing.T) {
	d := model.ACMEDomain{
		MainDomain: "example.com",
		SanDomains: " www.example.com , example.com ,, api.example.com ",
	}
	got := BuildDomains(d)
	want := []string{"example.com", "www.example.com", "api.example.com"}
	if len(got) != len(want) {
		t.Fatalf("got %v want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v want %v", got, want)
		}
	}
}

func TestAssembleFullchain(t *testing.T) {
	t.Run("empty chain returns cert", func(t *testing.T) {
		cert := []byte("CERT")
		if got := assembleFullchain(cert, nil); !bytes.Equal(got, cert) {
			t.Fatalf("got %q", got)
		}
	})
	t.Run("chain already contained", func(t *testing.T) {
		cert := []byte("CERT\nCHAIN\n")
		if got := assembleFullchain(cert, []byte("CHAIN")); !bytes.Equal(got, cert) {
			t.Fatalf("got %q", got)
		}
	})
	t.Run("appends with missing trailing newline", func(t *testing.T) {
		got := assembleFullchain([]byte("CERT"), []byte("CHAIN"))
		if !bytes.Equal(got, []byte("CERT\nCHAIN")) {
			t.Fatalf("got %q", got)
		}
	})
	t.Run("appends preserving existing newline", func(t *testing.T) {
		got := assembleFullchain([]byte("CERT\n"), []byte("CHAIN"))
		if !bytes.Equal(got, []byte("CERT\nCHAIN")) {
			t.Fatalf("got %q", got)
		}
	})
}

func TestParseCertMeta(t *testing.T) {
	t.Run("garbage returns zero", func(t *testing.T) {
		nb, na, serial := parseCertMeta([]byte("not a pem"))
		if !nb.IsZero() || !na.IsZero() || serial != "" {
			t.Fatalf("expected zero values, got %v %v %q", nb, na, serial)
		}
	})
	t.Run("valid cert parsed", func(t *testing.T) {
		key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		notBefore := time.Now().Add(-time.Hour).Truncate(time.Second)
		notAfter := notBefore.Add(90 * 24 * time.Hour)
		tmpl := &x509.Certificate{
			SerialNumber: big.NewInt(0x1234abcd),
			Subject:      pkix.Name{CommonName: "test"},
			NotBefore:    notBefore,
			NotAfter:     notAfter,
		}
		der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
		if err != nil {
			t.Fatalf("create cert: %v", err)
		}
		p := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
		nb, na, serial := parseCertMeta(p)
		if !nb.Equal(notBefore.UTC()) || !na.Equal(notAfter.UTC()) {
			t.Fatalf("time mismatch: nb=%v na=%v", nb, na)
		}
		if serial != "1234abcd" {
			t.Fatalf("serial = %q want 1234abcd", serial)
		}
	})
}

func TestTruncate(t *testing.T) {
	if truncate("abc", 5) != "abc" {
		t.Fatal("shorter string unchanged")
	}
	if truncate("abcdef", 3) != "abc" {
		t.Fatal("longer string cut")
	}
}
