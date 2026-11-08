package acme

import (
	"bytes"
	"crypto/x509"
	"encoding/pem"
	"strings"
	"time"
	"unicode/utf8"

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

// truncate 按字符（rune）截断并替换非法 UTF-8：error_msg 是 varchar(1024) 按字符计，
// 按字节切可能切出半个汉字，MySQL 严格模式下整条 UPDATE 会被拒。
func truncate(s string, n int) string {
	s = strings.ToValidUTF8(s, "\uFFFD")
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	i := 0
	for pos := range s {
		if i == n {
			return s[:pos]
		}
		i++
	}
	return s
}
