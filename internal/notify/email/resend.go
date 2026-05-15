// Package email 通过 Resend HTTP API 发送邮件。
package email

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type ResendClient struct {
	apiKey string
	from   string
	http   *http.Client
}

func NewResend(apiKey, from string) *ResendClient {
	return &ResendClient{
		apiKey: apiKey,
		from:   from,
		http:   &http.Client{Timeout: 10 * time.Second},
	}
}

func (c *ResendClient) Enabled() bool {
	return c != nil && c.apiKey != "" && c.from != ""
}

// SendText 发送纯文本邮件。to 为单个收件人。
func (c *ResendClient) SendText(to, subject, text string) error {
	if !c.Enabled() {
		return fmt.Errorf("resend not configured")
	}
	body := map[string]any{
		"from":    c.from,
		"to":      []string{to},
		"subject": subject,
		"text":    text,
	}
	buf, _ := json.Marshal(body)
	req, err := http.NewRequest(http.MethodPost, "https://api.resend.com/emails", bytes.NewReader(buf))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("resend send: %d %s", resp.StatusCode, string(b))
	}
	return nil
}
