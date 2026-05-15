package acmeproviders

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// validateDNSPod 走 DNSPod 旧版 login_token 接口。
func validateDNSPod(ctx context.Context, envs map[string]string) error {
	token := strings.TrimSpace(envs["DNSPOD_API_KEY"])
	if token == "" {
		return errors.New("dnspod: DNSPOD_API_KEY 缺失")
	}
	form := url.Values{"login_token": {token}, "format": {"json"}}
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://dnsapi.cn/Info.Version", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", "homer-acme-validator/1.0 (lemon@local)")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("dnspod: 请求失败：%w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<10))
	var r struct {
		Status struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"status"`
	}
	if err := json.Unmarshal(body, &r); err != nil {
		return fmt.Errorf("dnspod: 响应解析失败：HTTP %d %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	if r.Status.Code != "1" {
		return fmt.Errorf("dnspod: %s %s", r.Status.Code, r.Status.Message)
	}
	return nil
}
