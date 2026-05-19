package sms

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"testing"

	"github.com/LemonZuo/homer/internal/model"
)

func TestSM4CBCRoundTrip(t *testing.T) {
	key := bytes.Repeat([]byte{0xAB}, 16)
	cases := [][]byte{
		[]byte(""),
		[]byte("a"),
		bytes.Repeat([]byte("x"), 15),
		bytes.Repeat([]byte("y"), 16), // 整块,需补一整块 PKCS7
		[]byte(`{"data":{"k":"v"},"timestamp":1700000000000,"sign":""}`),
	}
	for _, plain := range cases {
		ct, err := sm4CBCEncrypt(key, plain)
		if err != nil {
			t.Fatalf("encrypt(%q): %v", plain, err)
		}
		if len(ct)%16 != 0 || len(ct) == 0 {
			t.Fatalf("ciphertext len %d not positive multiple of block", len(ct))
		}
		got, err := sm4CBCDecrypt(key, ct)
		if err != nil {
			t.Fatalf("decrypt(%q): %v", plain, err)
		}
		if !bytes.Equal(got, plain) {
			t.Fatalf("roundtrip mismatch: got %q want %q", got, plain)
		}
	}
}

func TestSM4DecryptRejectsBadInput(t *testing.T) {
	key := bytes.Repeat([]byte{0x01}, 16)
	if _, err := sm4CBCDecrypt(key, nil); err == nil {
		t.Fatal("empty ciphertext should error")
	}
	if _, err := sm4CBCDecrypt(key, []byte("123")); err == nil {
		t.Fatal("non-block-multiple ciphertext should error")
	}
	// 合法长度但填充字节非法(末字节为 0)。
	bad := make([]byte, 16)
	if _, err := sm4CBCDecrypt(key, bad); err == nil {
		t.Fatal("invalid PKCS7 padding should error")
	}
}

func TestSignIsDeterministicAndEscaped(t *testing.T) {
	c := &Client{signKey: "secret"}
	a := c.sign(1700000000000)
	b := c.sign(1700000000000)
	if a != b {
		t.Fatalf("sign not deterministic: %q vs %q", a, b)
	}
	if c.sign(1700000000001) == a {
		t.Fatal("different timestamp should yield different sign")
	}
	// QueryEscape 后不应残留 + / =(base64 原文里常见)。
	if bytes.ContainsAny([]byte(a), "+/=") {
		t.Fatalf("sign not url-escaped: %q", a)
	}
}

func TestParseRSAPublicKeyFormats(t *testing.T) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("gen key: %v", err)
	}
	spki, _ := x509.MarshalPKIXPublicKey(&priv.PublicKey)
	pkcs1 := x509.MarshalPKCS1PublicKey(&priv.PublicKey)

	t.Run("bare base64 SPKI", func(t *testing.T) {
		if _, err := parseRSAPublicKey(base64.StdEncoding.EncodeToString(spki)); err != nil {
			t.Fatalf("SPKI base64: %v", err)
		}
	})
	t.Run("PEM with newlines", func(t *testing.T) {
		p := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: spki})
		if _, err := parseRSAPublicKey(string(p)); err != nil {
			t.Fatalf("PEM: %v", err)
		}
	})
	t.Run("PKCS1 base64", func(t *testing.T) {
		if _, err := parseRSAPublicKey(base64.StdEncoding.EncodeToString(pkcs1)); err != nil {
			t.Fatalf("PKCS1 base64: %v", err)
		}
	})
	t.Run("base64 with embedded whitespace", func(t *testing.T) {
		s := base64.StdEncoding.EncodeToString(spki)
		if _, err := parseRSAPublicKey(s[:20] + "\n  " + s[20:]); err != nil {
			t.Fatalf("whitespace-tolerant base64: %v", err)
		}
	})
	t.Run("empty", func(t *testing.T) {
		if _, err := parseRSAPublicKey("   "); err == nil {
			t.Fatal("empty key should error")
		}
	})
	t.Run("garbage base64", func(t *testing.T) {
		if _, err := parseRSAPublicKey("!!!not base64!!!"); err == nil {
			t.Fatal("garbage should error")
		}
	})
	t.Run("valid base64 but not a key", func(t *testing.T) {
		if _, err := parseRSAPublicKey(base64.StdEncoding.EncodeToString([]byte("hello"))); err == nil {
			t.Fatal("non-key DER should error")
		}
	})
}

func TestClientEnabledByMode(t *testing.T) {
	var nilClient *Client
	if nilClient.Enabled() {
		t.Fatal("nil client must not be enabled")
	}
	if (&Client{}).Enabled() {
		t.Fatal("empty serverURL must not be enabled")
	}
	tests := []struct {
		name string
		c    *Client
		want bool
	}{
		{"none always on", &Client{serverURL: "x", mode: model.SmsAuthNone}, true},
		{"sign needs key", &Client{serverURL: "x", mode: model.SmsAuthSign}, false},
		{"sign with key", &Client{serverURL: "x", mode: model.SmsAuthSign, signKey: "k"}, true},
		{"rsa needs pub", &Client{serverURL: "x", mode: model.SmsAuthRSA}, false},
		{"rsa with pub", &Client{serverURL: "x", mode: model.SmsAuthRSA, rsaPub: &rsa.PublicKey{}}, true},
		{"sm4 needs 16B", &Client{serverURL: "x", mode: model.SmsAuthSM4, sm4Key: []byte("short")}, false},
		{"sm4 with 16B", &Client{serverURL: "x", mode: model.SmsAuthSM4, sm4Key: bytes.Repeat([]byte{0}, 16)}, true},
	}
	for _, tc := range tests {
		if got := tc.c.Enabled(); got != tc.want {
			t.Errorf("%s: Enabled()=%v want %v", tc.name, got, tc.want)
		}
	}
}

func TestNewRejectsBadSM4Key(t *testing.T) {
	if _, err := New("http://x", model.SmsAuthSM4, "", "", "zz", 5); err == nil {
		t.Fatal("non-hex SM4 key should error")
	}
	if _, err := New("http://x", model.SmsAuthSM4, "", "", hex.EncodeToString([]byte("only8byte")), 5); err == nil {
		t.Fatal("wrong-length SM4 key should error")
	}
	if _, err := New("http://x", model.SmsAuthSM4, "", "", hex.EncodeToString(bytes.Repeat([]byte{1}, 16)), 5); err != nil {
		t.Fatalf("valid 16-byte SM4 key: %v", err)
	}
}
