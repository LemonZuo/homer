// Package sms 对接「短信转发器」(SmsForwarder Android) HTTP 服务。
// 支持服务端「客户端安全措施」全部四种模式：无 / 签名 / RSA / SM4。
// 加解密细节与 SmsForwarder Android 源码（AppMessageConverter / RSACrypt /
// SM4Crypt / 各 client Fragment）严格对齐，便于直接对接。
package sms

import (
	"bytes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/LemonZuo/homer/internal/model"
	"github.com/tjfoc/gmsm/sm4"
)

// SM4Crypt.kt 里写死的 CBC IV。
var sm4IV = []byte{3, 5, 6, 9, 6, 9, 5, 9, 3, 5, 6, 9, 6, 9, 5, 9}

type Client struct {
	serverURL string
	mode      int
	signKey   string
	rsaPub    *rsa.PublicKey
	sm4Key    []byte
	http      *http.Client
}

// New 按转发器配置构造客户端；密钥不合法（如 RSA 公钥无法解析）时返回 error。
func New(serverURL string, mode int, signKey, rsaPub, sm4Key string, timeoutSeconds int) (*Client, error) {
	if timeoutSeconds <= 0 {
		timeoutSeconds = 30
	}
	c := &Client{
		serverURL: strings.TrimRight(serverURL, "/"),
		mode:      mode,
		signKey:   signKey,
		http:      &http.Client{Timeout: time.Duration(timeoutSeconds) * time.Second},
	}
	switch mode {
	case model.SmsAuthRSA:
		rp, err := parseRSAPublicKey(rsaPub)
		if err != nil {
			return nil, err
		}
		c.rsaPub = rp
	case model.SmsAuthSM4:
		key, err := hex.DecodeString(strings.TrimSpace(sm4Key))
		if err != nil {
			return nil, fmt.Errorf("SM4 密钥需为 hex 字符串: %w", err)
		}
		if len(key) != 16 {
			return nil, fmt.Errorf("SM4 密钥需为 16 字节（32 位 hex），当前 %d 字节", len(key))
		}
		c.sm4Key = key
	}
	return c, nil
}

var b64Cleanup = regexp.MustCompile(`\s`)

// parseRSAPublicKey 兼容 SmsForwarder App 导出的裸 Base64(SPKI DER)，也兼容
// 用户从别处拷来的 PEM（含头尾/换行）或 PKCS#1 公钥。
func parseRSAPublicKey(s string) (*rsa.PublicKey, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, fmt.Errorf("RSA 公钥为空")
	}

	var der []byte
	if block, _ := pem.Decode([]byte(s)); block != nil {
		der = block.Bytes
	} else {
		raw := b64Cleanup.ReplaceAllString(s, "")
		d, err := base64.StdEncoding.DecodeString(raw)
		if err != nil {
			return nil, fmt.Errorf("RSA 公钥 Base64 解码失败（应为 App 生成的公钥串）: %w", err)
		}
		der = d
	}

	if pub, err := x509.ParsePKIXPublicKey(der); err == nil {
		rp, ok := pub.(*rsa.PublicKey)
		if !ok {
			return nil, fmt.Errorf("提供的不是 RSA 公钥")
		}
		return rp, nil
	}
	if rp, err := x509.ParsePKCS1PublicKey(der); err == nil {
		return rp, nil
	}
	return nil, fmt.Errorf("RSA 公钥解析失败（需 X.509/SPKI 或 PKCS#1 的 DER/Base64/PEM）")
}

func (c *Client) Enabled() bool {
	if c == nil || c.serverURL == "" {
		return false
	}
	switch c.mode {
	case model.SmsAuthSign:
		return c.signKey != ""
	case model.SmsAuthRSA:
		return c.rsaPub != nil
	case model.SmsAuthSM4:
		return len(c.sm4Key) == 16
	default:
		return true
	}
}

func (c *Client) sign(timestamp int64) string {
	stringToSign := strconv.FormatInt(timestamp, 10) + "\n" + c.signKey
	mac := hmac.New(sha256.New, []byte(c.signKey))
	mac.Write([]byte(stringToSign))
	return url.QueryEscape(base64.StdEncoding.EncodeToString(mac.Sum(nil)))
}

// Post 把 data 包成 {data, timestamp, sign}，按当前模式加密后提交，返回解密后的
// 明文响应（始终为服务端原始 JSON 字节），调用方据此透传给前端。
func (c *Client) Post(path string, data any) ([]byte, error) {
	if !c.Enabled() {
		return nil, fmt.Errorf("sms forwarder 未配置或密钥不合法")
	}
	if data == nil {
		data = map[string]any{}
	}
	ts := time.Now().UnixMilli()
	envelope := map[string]any{"data": data, "timestamp": ts, "sign": ""}
	if c.mode == model.SmsAuthSign {
		envelope["sign"] = c.sign(ts)
	}
	plain, _ := json.Marshal(envelope)

	body, contentType, err := c.encodeRequest(plain)
	if err != nil {
		return nil, err
	}

	resp, err := c.http.Post(c.serverURL+path, contentType, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	// SmsForwarder 在 mode 2/3 下连错误响应体也会加密，故无论状态码都先尝试解密。
	out, derr := c.decodeResponse(raw)
	if derr != nil {
		// 解密失败时回传原始体，便于前端看到服务端可能的明文错误
		if resp.StatusCode >= 300 {
			return raw, fmt.Errorf("sms forwarder %s: HTTP %d", path, resp.StatusCode)
		}
		return raw, derr
	}
	if resp.StatusCode >= 300 {
		return out, fmt.Errorf("sms forwarder %s: HTTP %d", path, resp.StatusCode)
	}
	return out, nil
}

func (c *Client) encodeRequest(plain []byte) (body []byte, contentType string, err error) {
	switch c.mode {
	case model.SmsAuthRSA:
		// 与 RSACrypt.encryptByPublicKey 对齐：先 Base64(json)，再分段公钥加密，整体 Base64
		b64 := []byte(base64.StdEncoding.EncodeToString(plain))
		enc, err := c.rsaEncryptByPublicKey(b64)
		if err != nil {
			return nil, "", err
		}
		return []byte(base64.StdEncoding.EncodeToString(enc)), "text/plain; charset=utf-8", nil
	case model.SmsAuthSM4:
		ct, err := sm4CBCEncrypt(c.sm4Key, plain)
		if err != nil {
			return nil, "", err
		}
		return []byte(hex.EncodeToString(ct)), "text/plain; charset=utf-8", nil
	default:
		return plain, "application/json; charset=utf-8", nil
	}
}

func (c *Client) decodeResponse(raw []byte) ([]byte, error) {
	switch c.mode {
	case model.SmsAuthRSA:
		// 服务端：encryptByPrivateKey(Base64(respJson)) → 整体 Base64 字符串
		cipherBytes, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(raw)))
		if err != nil {
			return nil, fmt.Errorf("RSA 响应 Base64 解码失败: %w", err)
		}
		b64, err := c.rsaDecryptByPublicKey(cipherBytes)
		if err != nil {
			return nil, err
		}
		jsonBytes, err := base64.StdEncoding.DecodeString(string(b64))
		if err != nil {
			return nil, fmt.Errorf("RSA 响应内层 Base64 解码失败: %w", err)
		}
		return jsonBytes, nil
	case model.SmsAuthSM4:
		ct, err := hex.DecodeString(strings.ToLower(strings.TrimSpace(string(raw))))
		if err != nil {
			return nil, fmt.Errorf("SM4 响应 hex 解码失败: %w", err)
		}
		return sm4CBCDecrypt(c.sm4Key, ct)
	default:
		return raw, nil
	}
}

// RSACrypt 的 ENCRYPT_MAX_SIZE：每段明文最大字节数（硬编码常量）。
const rsaEncryptChunk = 245

// rsaEncryptByPublicKey 对齐 RSACrypt.encryptByPublicKey。
// 实测该 App 的 Cipher.getInstance("RSA") 解析为 RSA/ECB/NoPadding（裸 RSA，
// 左侧补零），故这里用教科书式 m^e mod n，按 245 字节分段，密文块直接拼接。
func (c *Client) rsaEncryptByPublicKey(data []byte) ([]byte, error) {
	k := c.rsaPub.Size()
	e := big.NewInt(int64(c.rsaPub.E))
	n := c.rsaPub.N
	var out bytes.Buffer
	for off := 0; off < len(data); off += rsaEncryptChunk {
		end := off + rsaEncryptChunk
		if end > len(data) {
			end = len(data)
		}
		m := new(big.Int).SetBytes(data[off:end])
		if m.Cmp(n) >= 0 {
			return nil, fmt.Errorf("RSA 明文分段超出模数范围")
		}
		out.Write(new(big.Int).Exp(m, e, n).FillBytes(make([]byte, k)))
	}
	return out.Bytes(), nil
}

// rsaDecryptByPublicKey 对齐 RSACrypt.decryptByPublicKey（撤销私钥 NoPadding 加密）：
// 逐块 c^e mod n 得到左补零的明文块，去掉前导 0x00 后拼接。
// 服务端载荷是 Base64(json) 的 ASCII，绝不含 0x00，故去前导零安全。
func (c *Client) rsaDecryptByPublicKey(data []byte) ([]byte, error) {
	k := c.rsaPub.Size()
	if len(data) == 0 || len(data)%k != 0 {
		return nil, fmt.Errorf("RSA 密文长度 %d 非块大小 %d 的整数倍", len(data), k)
	}
	e := big.NewInt(int64(c.rsaPub.E))
	n := c.rsaPub.N
	var out bytes.Buffer
	for off := 0; off < len(data); off += k {
		m := new(big.Int).Exp(new(big.Int).SetBytes(data[off:off+k]), e, n)
		blk := m.FillBytes(make([]byte, k))
		i := 0
		for i < len(blk) && blk[i] == 0x00 {
			i++
		}
		out.Write(blk[i:])
	}
	return out.Bytes(), nil
}

func sm4CBCEncrypt(key, plain []byte) ([]byte, error) {
	block, err := sm4.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("SM4 初始化失败: %w", err)
	}
	bs := block.BlockSize()
	pad := bs - len(plain)%bs
	padded := append(plain, bytes.Repeat([]byte{byte(pad)}, pad)...)
	out := make([]byte, len(padded))
	cipher.NewCBCEncrypter(block, sm4IV).CryptBlocks(out, padded)
	return out, nil
}

func sm4CBCDecrypt(key, ct []byte) ([]byte, error) {
	block, err := sm4.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("SM4 初始化失败: %w", err)
	}
	bs := block.BlockSize()
	if len(ct) == 0 || len(ct)%bs != 0 {
		return nil, fmt.Errorf("SM4 密文长度 %d 非法", len(ct))
	}
	out := make([]byte, len(ct))
	cipher.NewCBCDecrypter(block, sm4IV).CryptBlocks(out, ct)
	pad := int(out[len(out)-1])
	if pad <= 0 || pad > bs || pad > len(out) {
		return nil, fmt.Errorf("SM4 PKCS7 填充非法")
	}
	return out[:len(out)-pad], nil
}
