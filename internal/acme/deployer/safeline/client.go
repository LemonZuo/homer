package acmesafeline

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type client struct {
	baseURL string
	token   string
	http    *http.Client
}

type response struct {
	Data json.RawMessage `json:"data"`
	Err  any             `json:"err"`
	Msg  string          `json:"msg"`
}

type certList struct {
	Nodes []certItem `json:"nodes"`
	Total int        `json:"total"`
}

type certItem struct {
	ID          int64    `json:"id"`
	Domains     []string `json:"domains"`
	Issuer      string   `json:"issuer"`
	ValidBefore string   `json:"valid_before"`
}

func newClient(target Target) *client {
	transport := &http.Transport{}
	if target.SkipTLSVerify {
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} //nolint:gosec // 雷池常见自签证书部署场景。
	}
	return &client{
		baseURL: strings.TrimRight(target.BaseURL, "/"),
		token:   target.APIToken,
		http: &http.Client{
			Timeout:   20 * time.Second,
			Transport: transport,
		},
	}
}

func (c *client) ListCerts() (*certList, error) {
	var out certList
	if err := c.doJSON(http.MethodGet, "/api/open/cert", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *client) UpsertCert(certID int64, certType int, crt, key string) (int64, error) {
	body := map[string]any{
		"type": normalizeCertType(certType),
		"manual": map[string]string{
			"crt": crt,
			"key": key,
		},
	}
	if certID > 0 {
		body["id"] = certID
	}
	var id int64
	if err := c.doJSON(http.MethodPost, "/api/open/cert", body, &id); err != nil {
		return 0, err
	}
	return id, nil
}

func normalizeCertType(certType int) int {
	if certType <= 0 {
		return 2
	}
	return certType
}

func (c *client) doJSON(method, path string, body any, out any) error {
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(data)
	}
	req, err := http.NewRequest(method, c.baseURL+path, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-SLCE-API-TOKEN", c.token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("雷池 API HTTP %d：%s", resp.StatusCode, strings.TrimSpace(string(data)))
	}
	var wrap response
	if err := json.Unmarshal(data, &wrap); err != nil {
		return fmt.Errorf("解析雷池响应失败：%w", err)
	}
	if wrap.Err != nil && fmt.Sprint(wrap.Err) != "<nil>" && fmt.Sprint(wrap.Err) != "" {
		msg := strings.TrimSpace(wrap.Msg)
		if msg == "" {
			msg = fmt.Sprint(wrap.Err)
		}
		return errors.New(msg)
	}
	if out == nil {
		return nil
	}
	if p, ok := out.(*int64); ok {
		id, err := decodeID(wrap.Data)
		if err != nil {
			return err
		}
		*p = id
		return nil
	}
	if len(wrap.Data) == 0 || string(wrap.Data) == "null" {
		return nil
	}
	return json.Unmarshal(wrap.Data, out)
}

func decodeID(data json.RawMessage) (int64, error) {
	var n int64
	if err := json.Unmarshal(data, &n); err == nil {
		return n, nil
	}
	var f float64
	if err := json.Unmarshal(data, &f); err == nil {
		return int64(f), nil
	}
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		return strconv.ParseInt(s, 10, 64)
	}
	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err == nil {
		for _, key := range []string{"id", "cert_id"} {
			if v, ok := obj[key]; ok {
				switch x := v.(type) {
				case float64:
					return int64(x), nil
				case string:
					return strconv.ParseInt(x, 10, 64)
				}
			}
		}
	}
	return 0, fmt.Errorf("雷池响应未返回证书 ID：%s", strings.TrimSpace(string(data)))
}
