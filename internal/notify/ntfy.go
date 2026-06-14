package notify

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// Ntfy 推送到 ntfy (binwiederhier/ntfy) 自建或官方 ntfy.sh。
// POST {server}/{topic}，消息体即正文，Title 走 Header；
// token 非空时按 Authorization: Bearer 鉴权。
func Ntfy(server, topic, token string) Notifier {
	server = strings.TrimRight(strings.TrimSpace(server), "/")
	topic = strings.TrimSpace(topic)
	token = strings.TrimSpace(token)
	return &ntfyNotifier{server: server, topic: topic, token: token, http: &http.Client{Timeout: 10 * time.Second}}
}

type ntfyNotifier struct {
	server string
	topic  string
	token  string
	http   *http.Client
}

func (n *ntfyNotifier) Name() string  { return "ntfy" }
func (n *ntfyNotifier) Enabled() bool { return n != nil && n.server != "" && n.topic != "" }

func (n *ntfyNotifier) Send(ctx context.Context, m Message) error {
	if !n.Enabled() {
		return fmt.Errorf("ntfy not configured")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, n.server+"/"+n.topic, strings.NewReader(m.Text))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "text/plain; charset=utf-8")
	if m.Title != "" {
		req.Header.Set("Title", m.Title)
	}
	if n.token != "" {
		req.Header.Set("Authorization", "Bearer "+n.token)
	}
	resp, err := n.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("ntfy send: %d", resp.StatusCode)
	}
	return nil
}
