// Package sms 对接「短信转发器」(SmsForwarder Android) HTTP 服务。
// 仅实现签名鉴权（HMAC-SHA256 + Base64 + URLEncode）模式；
// RSA 模式在老 Java 里未跑通（用公钥做"解密"），SM4 模式从未实现，这里都不带过来。
package sms

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type Client struct {
	serverURL string
	secret    string
	http      *http.Client
}

func New(serverURL, secret string, timeoutSeconds int) *Client {
	if timeoutSeconds <= 0 {
		timeoutSeconds = 30
	}
	return &Client{
		serverURL: strings.TrimRight(serverURL, "/"),
		secret:    secret,
		http:      &http.Client{Timeout: time.Duration(timeoutSeconds) * time.Second},
	}
}

func (c *Client) Enabled() bool {
	return c != nil && c.serverURL != "" && c.secret != ""
}

func (c *Client) sign(timestamp int64) string {
	stringToSign := strconv.FormatInt(timestamp, 10) + "\n" + c.secret
	mac := hmac.New(sha256.New, []byte(c.secret))
	mac.Write([]byte(stringToSign))
	return url.QueryEscape(base64.StdEncoding.EncodeToString(mac.Sum(nil)))
}

// Post 把 data 包成 {data, timestamp, sign} 提交到 c.serverURL+path，返回原始响应字节。
// 调用方负责把 body 当成 JSON 透传给前端。
func (c *Client) Post(path string, data any) ([]byte, error) {
	if !c.Enabled() {
		return nil, fmt.Errorf("sms forwarder 未配置")
	}
	if data == nil {
		data = map[string]any{}
	}
	ts := time.Now().UnixMilli()
	body, _ := json.Marshal(map[string]any{
		"data":      data,
		"timestamp": ts,
		"sign":      c.sign(ts),
	})

	u := c.serverURL + path
	resp, err := c.http.Post(u, "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	out, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 300 {
		return out, fmt.Errorf("sms forwarder %s: HTTP %d", path, resp.StatusCode)
	}
	return out, nil
}
