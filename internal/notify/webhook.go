package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Webhook 极简出站通道：POST {"title","text"} JSON 到指定 URL。
// url 为空即 Enabled()=false，默认不参与任何下发。
func Webhook(url string) Notifier {
	return &webhookNotifier{url: url, http: &http.Client{Timeout: 10 * time.Second}}
}

type webhookNotifier struct {
	url  string
	http *http.Client
}

func (w *webhookNotifier) Name() string  { return "webhook" }
func (w *webhookNotifier) Enabled() bool { return w != nil && w.url != "" }

func (w *webhookNotifier) Send(ctx context.Context, m Message) error {
	if !w.Enabled() {
		return fmt.Errorf("webhook not configured")
	}
	buf, _ := json.Marshal(map[string]string{"title": m.Title, "text": m.Text})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, w.url, bytes.NewReader(buf))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := w.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("webhook send: %d", resp.StatusCode)
	}
	return nil
}
