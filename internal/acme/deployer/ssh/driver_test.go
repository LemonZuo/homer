package acmessh

import (
	"strings"
	"testing"
)

func TestRenderTemplate(t *testing.T) {
	if got := renderTemplate("/etc/nginx/{domain}/cert.pem", "a.example.com"); got != "/etc/nginx/a.example.com/cert.pem" {
		t.Fatalf("got %q", got)
	}
	if got := renderTemplate("no placeholder", "x"); got != "no placeholder" {
		t.Fatalf("got %q", got)
	}
	// 多处占位符全部替换
	if got := renderTemplate("{domain}/{domain}.key", "d"); got != "d/d.key" {
		t.Fatalf("got %q", got)
	}
}

func TestDeployOptionsNormalize(t *testing.T) {
	o := DeployOptions{
		CertPath:      "  /a/cert.pem  ",
		KeyPath:       "\t/a/key.pem",
		DeployCommand: " systemctl reload nginx \n",
	}
	o.normalize()
	if o.CertPath != "/a/cert.pem" || o.KeyPath != "/a/key.pem" || o.DeployCommand != "systemctl reload nginx" {
		t.Fatalf("normalized = %+v", o)
	}
}

func TestDeployOptionsValidate(t *testing.T) {
	// key 必填
	o := DeployOptions{CertPath: "/a/cert.pem"}
	if err := o.validate(); err == nil || !strings.Contains(err.Error(), "key.pem") {
		t.Fatalf("missing key err = %v", err)
	}
	// cert 与 fullchain 至少一个
	o = DeployOptions{KeyPath: "/a/key.pem"}
	if err := o.validate(); err == nil || !strings.Contains(err.Error(), "至少") {
		t.Fatalf("missing cert err = %v", err)
	}
	// key + cert 合法
	o = DeployOptions{KeyPath: "/a/key.pem", CertPath: "/a/cert.pem"}
	if err := o.validate(); err != nil {
		t.Fatal(err)
	}
	// key + fullchain 合法
	o = DeployOptions{KeyPath: "/a/key.pem", FullchainPath: "/a/fullchain.pem"}
	if err := o.validate(); err != nil {
		t.Fatal(err)
	}
}
