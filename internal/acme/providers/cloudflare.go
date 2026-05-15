package acmeproviders

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

func validateCloudflare(ctx context.Context, envs map[string]string) error {
	token := strings.TrimSpace(envs["CLOUDFLARE_DNS_API_TOKEN"])
	if token != "" {
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet,
			"https://api.cloudflare.com/client/v4/user/tokens/verify", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		return cloudflareCall(req)
	}
	email := strings.TrimSpace(envs["CLOUDFLARE_EMAIL"])
	key := strings.TrimSpace(envs["CLOUDFLARE_API_KEY"])
	if email == "" || key == "" {
		return errors.New("cloudflare: 需提供 CLOUDFLARE_DNS_API_TOKEN，或 CLOUDFLARE_EMAIL + CLOUDFLARE_API_KEY")
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet,
		"https://api.cloudflare.com/client/v4/user", nil)
	req.Header.Set("X-Auth-Email", email)
	req.Header.Set("X-Auth-Key", key)
	return cloudflareCall(req)
}

func cloudflareCall(req *http.Request) error {
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("cloudflare: 请求失败：%w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<10))
	var r struct {
		Success bool `json:"success"`
		Errors  []struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"errors"`
	}
	_ = json.Unmarshal(body, &r)
	if !r.Success {
		if len(r.Errors) > 0 {
			return fmt.Errorf("cloudflare: %d %s", r.Errors[0].Code, r.Errors[0].Message)
		}
		return fmt.Errorf("cloudflare: HTTP %d %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return nil
}
