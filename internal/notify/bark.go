package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// Bark 推送到 bark-server (Finb/bark-server) 或官方 api.day.app。
// 通过 POST {server}/{device_key} 下发 JSON {title, body}。
func Bark(server, deviceKey string) Notifier {
	server = strings.TrimRight(strings.TrimSpace(server), "/")
	deviceKey = strings.TrimSpace(deviceKey)
	return &barkNotifier{server: server, deviceKey: deviceKey, http: &http.Client{Timeout: 10 * time.Second}}
}

type barkNotifier struct {
	server    string
	deviceKey string
	http      *http.Client
}

func (b *barkNotifier) Name() string  { return "bark" }
func (b *barkNotifier) Enabled() bool { return b != nil && b.server != "" && b.deviceKey != "" }

func (b *barkNotifier) Send(ctx context.Context, m Message) error {
	if !b.Enabled() {
		return fmt.Errorf("bark not configured")
	}
	buf, _ := json.Marshal(map[string]string{
		"title": m.Title,
		"body":  m.Text,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, b.server+"/"+b.deviceKey, bytes.NewReader(buf))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := b.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("bark send: %d", resp.StatusCode)
	}
	return nil
}
