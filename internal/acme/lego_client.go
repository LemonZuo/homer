// Package acme 封装 lego，负责账号注册、DNS provider 注入、证书签发与续期。
package acme

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/LemonZuo/homer/internal/model"
	"github.com/go-acme/lego/v4/certcrypto"
	"github.com/go-acme/lego/v4/certificate"
	"github.com/go-acme/lego/v4/challenge"
	"github.com/go-acme/lego/v4/lego"
	legolog "github.com/go-acme/lego/v4/log"
	"github.com/go-acme/lego/v4/providers/dns"
	"github.com/go-acme/lego/v4/registration"
)

const (
	// CADirLetsEncrypt 生产环境 LE。
	CADirLetsEncrypt = "https://acme-v02.api.letsencrypt.org/directory"
	// CADirZeroSSL 生产环境 ZeroSSL。
	CADirZeroSSL = "https://acme.zerossl.com/v2/DV90"
)

// CAOptions 描述本次签发使用的 CA。
type CAOptions struct {
	ID           int64
	Name         string
	CA           string // "letsencrypt" | "zerossl" | "custom"
	DirectoryURL string
	Email        string
	EABKID       string // ZeroSSL 必填
	EABHMAC      string
}

func (o CAOptions) directoryURL() (string, error) {
	switch strings.ToLower(o.CA) {
	case "", "letsencrypt", "le":
		return CADirLetsEncrypt, nil
	case "zerossl":
		return CADirZeroSSL, nil
	case "custom":
		if strings.TrimSpace(o.DirectoryURL) == "" {
			return "", errors.New("自定义 ACME CA 需要配置 directory_url")
		}
		return strings.TrimSpace(o.DirectoryURL), nil
	}
	return "", fmt.Errorf("未知 CA：%s（仅支持 letsencrypt / zerossl / custom）", o.CA)
}

// providerMu 保护"设置 env → 构造 provider → 复位 env"这段临界区。
// lego 的 NewDNSChallengeProviderByName 内部读 env vars 后存到 config struct，
// 因此只需序列化构造阶段，签发阶段不受影响。
var providerMu sync.Mutex

// Client 持有 lego.Client，每次签发都从 Manager.newClient 新建（账号信息复用）。
type Client struct {
	lego *lego.Client
}

// Manager 维护 ACME 账号本地缓存；签发时按域名绑定的账号生成短期 Client。
type Manager struct {
	dataDir    string
	accountMu  sync.Mutex
	cachedUser map[int64]*acmeUser
}

func NewManager(dataDir string) *Manager {
	// lego 默认 logger 输出到 stderr；我们的 Service 会再包一层带 io.Writer 的 logger。
	legolog.Logger = nopLogger{}
	return &Manager{dataDir: dataDir, cachedUser: map[int64]*acmeUser{}}
}

// EnsureAccount 加载或注册 ACME 账号。账号文件存 <dataDir>/account/{ca}/...
// 用户私钥固定 EC P-256（lego 推荐）。
func (m *Manager) EnsureAccount(opts CAOptions) (*acmeUser, error) {
	m.accountMu.Lock()
	defer m.accountMu.Unlock()
	opts.normalize()
	if err := opts.validate(); err != nil {
		return nil, err
	}
	cacheKey := opts.cacheKey()
	if u := m.cachedUser[cacheKey]; u != nil {
		return u, nil
	}
	dir := filepath.Join(m.dataDir, "account", opts.dirName())
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	keyPath := filepath.Join(dir, "account.key")
	regPath := filepath.Join(dir, "account.json")

	key, err := loadOrCreateKey(keyPath)
	if err != nil {
		return nil, err
	}
	user := &acmeUser{email: opts.Email, key: key}
	if data, err := os.ReadFile(regPath); err == nil && len(data) > 0 {
		var reg registration.Resource
		if err := jsonUnmarshal(data, &reg); err == nil && reg.URI != "" {
			user.registration = &reg
			m.cachedUser[cacheKey] = user
			return user, nil
		}
	}

	dirURL, err := opts.directoryURL()
	if err != nil {
		return nil, err
	}
	cfg := lego.NewConfig(user)
	cfg.CADirURL = dirURL
	cfg.Certificate.KeyType = certcrypto.RSA2048
	client, err := lego.NewClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("lego.NewClient: %w", err)
	}

	var reg *registration.Resource
	if opts.EABKID != "" || opts.EABHMAC != "" {
		reg, err = client.Registration.RegisterWithExternalAccountBinding(registration.RegisterEABOptions{
			TermsOfServiceAgreed: true,
			Kid:                  opts.EABKID,
			HmacEncoded:          opts.EABHMAC,
		})
	} else {
		reg, err = client.Registration.Register(registration.RegisterOptions{TermsOfServiceAgreed: true})
	}
	if err != nil {
		return nil, fmt.Errorf("注册 ACME 账号失败：%w", err)
	}
	user.registration = reg
	if data, err := jsonMarshalIndent(reg); err == nil {
		_ = os.WriteFile(regPath, data, 0o600)
	}
	m.cachedUser[cacheKey] = user
	return user, nil
}

// newClient 构造一次性 lego.Client，并把日志 redirect 到提供的 writer。
func (m *Manager) newClient(opts CAOptions, logw io.Writer) (*Client, error) {
	opts.normalize()
	user, err := m.EnsureAccount(opts)
	if err != nil {
		return nil, err
	}
	dirURL, err := opts.directoryURL()
	if err != nil {
		return nil, err
	}
	cfg := lego.NewConfig(user)
	cfg.CADirURL = dirURL
	cfg.Certificate.KeyType = certcrypto.RSA2048
	c, err := lego.NewClient(cfg)
	if err != nil {
		return nil, err
	}
	if logw != nil {
		legolog.Logger = writerLogger{w: logw}
	}
	return &Client{lego: c}, nil
}

func optionsFromAccount(a model.ACMEAccount) CAOptions {
	return CAOptions{
		ID:           a.ID,
		Name:         a.Name,
		CA:           a.CA,
		DirectoryURL: a.DirectoryURL,
		Email:        a.Email,
		EABKID:       a.EABKID,
		EABHMAC:      a.EABHMAC,
	}
}

// Obtain 用指定 provider 跑 DNS-01 签发主域名 + SAN。
func (c *Client) Obtain(domains []string, provider string, store *CredentialStore) (*certificate.Resource, error) {
	prov, err := makeProvider(provider, store)
	if err != nil {
		return nil, err
	}
	if err := c.lego.Challenge.SetDNS01Provider(prov); err != nil {
		return nil, fmt.Errorf("SetDNS01Provider: %w", err)
	}
	req := certificate.ObtainRequest{Domains: domains, Bundle: true}
	return c.lego.Certificate.Obtain(req)
}

// Revoke 向当前 CA 吊销 PEM 编码证书。
func (c *Client) Revoke(cert []byte) error {
	return c.lego.Certificate.Revoke(cert)
}

// makeProvider 在加锁状态下临时 setenv 后构造 provider，然后还原。
func makeProvider(name string, store *CredentialStore) (challenge.Provider, error) {
	envs, err := store.Get(name)
	if err != nil {
		return nil, err
	}
	providerMu.Lock()
	defer providerMu.Unlock()

	// 备份并设置
	prev := map[string]struct {
		v  string
		ok bool
	}{}
	for k, v := range envs {
		old, ok := os.LookupEnv(k)
		prev[k] = struct {
			v  string
			ok bool
		}{old, ok}
		_ = os.Setenv(k, v)
	}
	defer func() {
		for k, p := range prev {
			if p.ok {
				_ = os.Setenv(k, p.v)
			} else {
				_ = os.Unsetenv(k)
			}
		}
	}()

	prov, err := dns.NewDNSChallengeProviderByName(name)
	if err != nil {
		return nil, fmt.Errorf("初始化 DNS provider %s 失败：%w", name, err)
	}
	return prov, nil
}

// ----- 账号实现 -----

type acmeUser struct {
	email        string
	key          crypto.PrivateKey
	registration *registration.Resource
}

func (u *acmeUser) GetEmail() string                        { return u.email }
func (u *acmeUser) GetRegistration() *registration.Resource { return u.registration }
func (u *acmeUser) GetPrivateKey() crypto.PrivateKey        { return u.key }

func loadOrCreateKey(path string) (crypto.PrivateKey, error) {
	if data, err := os.ReadFile(path); err == nil && len(data) > 0 {
		block, _ := pem.Decode(data)
		if block == nil {
			return nil, fmt.Errorf("解析账号私钥失败：%s", path)
		}
		k, err := x509.ParseECPrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("解析账号私钥 ECDSA 失败：%w", err)
		}
		return k, nil
	}
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}
	der, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		return nil, err
	}
	pemData := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: der})
	if err := os.WriteFile(path, pemData, 0o600); err != nil {
		return nil, err
	}
	return priv, nil
}

func normalizeCA(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" || s == "le" {
		return "letsencrypt"
	}
	return s
}

func (o *CAOptions) normalize() {
	o.Name = strings.TrimSpace(o.Name)
	o.CA = normalizeCA(o.CA)
	o.DirectoryURL = strings.TrimSpace(o.DirectoryURL)
	o.Email = strings.TrimSpace(o.Email)
	o.EABKID = strings.TrimSpace(o.EABKID)
	o.EABHMAC = strings.TrimSpace(o.EABHMAC)
	if o.CA == "letsencrypt" {
		o.DirectoryURL = CADirLetsEncrypt
	} else if o.CA == "zerossl" {
		o.DirectoryURL = CADirZeroSSL
	}
}

func (o CAOptions) validate() error {
	if strings.TrimSpace(o.Email) == "" {
		return errors.New("ACME 邮箱未配置")
	}
	if (o.EABKID == "") != (o.EABHMAC == "") {
		return errors.New("EAB KID 与 EAB HMAC 需要同时填写")
	}
	if strings.ToLower(o.CA) == "zerossl" && (o.EABKID == "" || o.EABHMAC == "") {
		return errors.New("ZeroSSL 需要配置 EAB KID 与 EAB HMAC")
	}
	if _, err := o.directoryURL(); err != nil {
		return err
	}
	return nil
}

func (o CAOptions) cacheKey() int64 {
	if o.ID > 0 {
		return o.ID
	}
	return -1
}

func (o CAOptions) dirName() string {
	if o.ID > 0 {
		name := strings.ToLower(o.Name)
		name = strings.Map(func(r rune) rune {
			if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-' || r == '_' {
				return r
			}
			return '-'
		}, name)
		name = strings.Trim(name, "-")
		if name == "" {
			name = normalizeCA(o.CA)
		}
		return fmt.Sprintf("%d-%s", o.ID, name)
	}
	return normalizeCA(o.CA)
}

// ----- logger 适配 -----

type writerLogger struct{ w io.Writer }

func (l writerLogger) Fatal(args ...any)                 { fmt.Fprintln(l.w, args...) }
func (l writerLogger) Fatalln(args ...any)               { fmt.Fprintln(l.w, args...) }
func (l writerLogger) Fatalf(format string, args ...any) { fmt.Fprintf(l.w, format+"\n", args...) }
func (l writerLogger) Print(args ...any)                 { fmt.Fprint(l.w, args...) }
func (l writerLogger) Println(args ...any)               { fmt.Fprintln(l.w, args...) }
func (l writerLogger) Printf(format string, args ...any) { fmt.Fprintf(l.w, format+"\n", args...) }

type nopLogger struct{}

func (nopLogger) Fatal(args ...any)                 {}
func (nopLogger) Fatalln(args ...any)               {}
func (nopLogger) Fatalf(format string, args ...any) {}
func (nopLogger) Print(args ...any)                 {}
func (nopLogger) Println(args ...any)               {}
func (nopLogger) Printf(format string, args ...any) {}
