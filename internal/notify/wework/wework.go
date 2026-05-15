// Package wework 企业微信应用消息发送客户端。
// 仅实现「发文本消息」一个能力，供生日提醒等周期任务使用。
package wework

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

type Client struct {
	corpID  string
	agentID string
	secret  string
	tagID   string

	mu       sync.Mutex
	token    string
	tokenExp time.Time
}

func New(corpID, agentID, secret, tagID string) *Client {
	return &Client{corpID: corpID, agentID: agentID, secret: secret, tagID: tagID}
}

func (c *Client) Enabled() bool {
	return c.corpID != "" && c.agentID != "" && c.secret != ""
}

func (c *Client) getToken() (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.token != "" && time.Now().Before(c.tokenExp) {
		return c.token, nil
	}
	url := fmt.Sprintf("https://qyapi.weixin.qq.com/cgi-bin/gettoken?corpid=%s&corpsecret=%s", c.corpID, c.secret)
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var data struct {
		Errcode     int    `json:"errcode"`
		Errmsg      string `json:"errmsg"`
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return "", err
	}
	if data.Errcode != 0 {
		return "", fmt.Errorf("wework gettoken: %d %s", data.Errcode, data.Errmsg)
	}
	c.token = data.AccessToken
	c.tokenExp = time.Now().Add(time.Duration(max(60, data.ExpiresIn-60)) * time.Second)
	return c.token, nil
}

// SendText 发送一条文本消息。当配置了 tagID 时按标签下发，否则发给应用可见范围全部成员。
func (c *Client) SendText(content string) error {
	if !c.Enabled() {
		return fmt.Errorf("wework not configured")
	}
	token, err := c.getToken()
	if err != nil {
		return err
	}
	body := map[string]any{
		"agentid": c.agentID,
		"msgtype": "text",
		"text":    map[string]string{"content": content},
		"safe":    0,
	}
	if c.tagID != "" {
		body["totag"] = c.tagID
	} else {
		body["touser"] = "@all"
	}
	buf, _ := json.Marshal(body)
	url := fmt.Sprintf("https://qyapi.weixin.qq.com/cgi-bin/message/send?access_token=%s", token)
	resp, err := http.Post(url, "application/json", bytes.NewReader(buf))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	var data struct {
		Errcode int    `json:"errcode"`
		Errmsg  string `json:"errmsg"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return err
	}
	if data.Errcode != 0 {
		return fmt.Errorf("wework send: %d %s", data.Errcode, data.Errmsg)
	}
	return nil
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
