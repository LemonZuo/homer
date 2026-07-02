package sshx

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"strings"
	"testing"

	"golang.org/x/crypto/ssh"
)

func TestShellQuote(t *testing.T) {
	cases := []struct{ in, want string }{
		{"plain", "'plain'"},
		{"", "''"},
		{"has space", "'has space'"},
		{"it's", `'it'"'"'s'`},
		{"$HOME`id`;rm", "'$HOME`id`;rm'"},
	}
	for _, c := range cases {
		if got := ShellQuote(c.in); got != c.want {
			t.Errorf("ShellQuote(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestConnAddr(t *testing.T) {
	c := &Conn{Host: "10.0.0.1", Port: 2222}
	if got := c.addr(); got != "10.0.0.1:2222" {
		t.Fatalf("addr = %q", got)
	}
	// IPv6 需要方括号包裹
	c6 := &Conn{Host: "::1", Port: 22}
	if got := c6.addr(); got != "[::1]:22" {
		t.Fatalf("ipv6 addr = %q", got)
	}
}

func TestAuthMethodPassword(t *testing.T) {
	// password 模式返回 password + keyboard-interactive 两种（兼容 ESXi）
	methods, err := AuthMethod("password", "pw", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(methods) != 2 {
		t.Fatalf("expected 2 methods, got %d", len(methods))
	}
}

func TestAuthMethodKey(t *testing.T) {
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	pemBlock, err := ssh.MarshalPrivateKey(priv, "")
	if err != nil {
		t.Fatal(err)
	}
	keyPEM := string(pem.EncodeToMemory(pemBlock))

	methods, err := AuthMethod("key", "", keyPEM, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(methods) != 1 {
		t.Fatalf("expected 1 method, got %d", len(methods))
	}
}

func TestAuthMethodKeyWithPassphrase(t *testing.T) {
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	pemBlock, err := ssh.MarshalPrivateKeyWithPassphrase(priv, "", []byte("secret"))
	if err != nil {
		t.Fatal(err)
	}
	keyPEM := string(pem.EncodeToMemory(pemBlock))

	if _, err := AuthMethod("key", "", keyPEM, "secret"); err != nil {
		t.Fatalf("correct passphrase: %v", err)
	}
	if _, err := AuthMethod("key", "", keyPEM, "wrong"); err == nil {
		t.Fatal("wrong passphrase must error")
	}
	// 加密私钥 + 空 passphrase → 走 ParsePrivateKey 分支 → 报错
	if _, err := AuthMethod("key", "", keyPEM, ""); err == nil {
		t.Fatal("encrypted key without passphrase must error")
	}
}

func TestAuthMethodErrors(t *testing.T) {
	if _, err := AuthMethod("key", "", "not a pem key", ""); err == nil || !strings.Contains(err.Error(), "私钥") {
		t.Fatalf("bad key error = %v", err)
	}
	if _, err := AuthMethod("agent", "", "", ""); err == nil || !strings.Contains(err.Error(), "未知") {
		t.Fatalf("unknown type error = %v", err)
	}
}
