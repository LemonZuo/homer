package acme

import (
	"bytes"
	"crypto/x509"
	"encoding/pem"
	"strings"
	"time"

	"github.com/LemonZuo/homer/internal/model"
)

// ParseSanProviders 解析 ACMEDomain.SanProviders（按域名覆盖 DNS provider）。
// 空键/空值、与默认 provider 相同的项直接丢弃，返回有效覆盖映射。
func ParseSanProviders(d model.ACMEDomain) map[string]string {
	raw := map[string]string{}
	_ = JSONUnmarshal([]byte(EmptyJSON(d.SanProviders)), &raw)
	out := map[string]string{}
	for k, v := range raw {
		k = strings.TrimSpace(k)
		v = strings.TrimSpace(v)
		if k != "" && v != "" && v != d.Provider {
			out[k] = v
		}
	}
	return out
}

// BuildDomains 主域名 + SAN 拆成 lego.Obtain / 雷池匹配等所需的字符串切片。
func BuildDomains(d model.ACMEDomain) []string {
	out := []string{d.MainDomain}
	for _, s := range strings.Split(d.SanDomains, ",") {
		s = strings.TrimSpace(s)
		if s != "" && s != d.MainDomain {
			out = append(out, s)
		}
	}
	return out
}

func parseCertMeta(certPEM []byte) (time.Time, time.Time, string) {
	block, _ := pem.Decode(certPEM)
	if block == nil {
		return time.Time{}, time.Time{}, ""
	}
	c, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return time.Time{}, time.Time{}, ""
	}
	return c.NotBefore, c.NotAfter, c.SerialNumber.Text(16)
}

func assembleFullchain(cert, chain []byte) []byte {
	if len(chain) == 0 {
		return cert
	}
	if bytes.Contains(cert, chain) {
		return cert
	}
	buf := bytes.Buffer{}
	buf.Write(cert)
	if !bytes.HasSuffix(cert, []byte("\n")) {
		buf.WriteByte('\n')
	}
	buf.Write(chain)
	return buf.Bytes()
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
