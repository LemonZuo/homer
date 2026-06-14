package notify

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/LemonZuo/homer/internal/model"
	"github.com/LemonZuo/homer/internal/notify/email"
	"github.com/LemonZuo/homer/internal/notify/wework"
	"gorm.io/gorm"
)

// 模块名常量：notify_binding.module 取这些值。
const (
	ModuleBirthday  = "birthday"
	ModuleEvent     = "event"
	ModuleBypass    = "bypass"
	ModuleSchedAlrt = "scheduler_alert"
	ModuleUPS       = "ups"
	ModuleESXi      = "esxi"
)

// Modules 暴露给前端的模块清单（key + 中文名）。
var Modules = []struct {
	Key   string `json:"key"`
	Label string `json:"label"`
}{
	{ModuleBirthday, "生日提醒"},
	{ModuleEvent, "事项提醒"},
	{ModuleBypass, "Bypass 分流转发"},
	{ModuleSchedAlrt, "任务失败告警"},
	{ModuleUPS, "UPS 状态告警"},
	{ModuleESXi, "ESXi 状态告警"},
}

// ChannelTypes 各通道类型的字段 schema，供前端动态表单。
var ChannelTypes = []struct {
	Type   string   `json:"type"`
	Label  string   `json:"label"`
	Fields []string `json:"fields"`
}{
	{"wework", "企业微信", []string{"corp_id", "agent_id", "secret", "tag_id"}},
	{"email", "Resend 邮件", []string{"api_key", "from", "to"}},
	{"webhook", "Webhook", []string{"url"}},
	{"bark", "Bark", []string{"server", "device_key"}},
}

// Hub 按 DB 里的「通道 + 模块映射」在发送时解析通知，配置改动即时生效（无缓存）。
type Hub struct{ db *gorm.DB }

func NewHub(db *gorm.DB) *Hub { return &Hub{db: db} }

// For 返回某模块的扇出 Notifier；Send 时实时查 DB，运行时改配置立即生效。
func (h *Hub) For(module string) Notifier { return &hubNotifier{db: h.db, module: module} }

type hubNotifier struct {
	db     *gorm.DB
	module string
}

func (n *hubNotifier) Name() string { return "hub:" + n.module }

func (n *hubNotifier) resolve() []Notifier {
	var chs []model.NotifyChannel
	err := n.db.
		Joins("JOIN notify_binding ON notify_binding.channel_id = notify_channel.id").
		Where("notify_binding.module = ? AND notify_channel.enabled = ?", n.module, "1").
		Find(&chs).Error
	if err != nil {
		return nil
	}
	out := make([]Notifier, 0, len(chs))
	for _, ch := range chs {
		if nf := buildChannel(ch); nf != nil {
			out = append(out, nf)
		}
	}
	return out
}

func (n *hubNotifier) Enabled() bool {
	for _, nf := range n.resolve() {
		if nf.Enabled() {
			return true
		}
	}
	return false
}

func (n *hubNotifier) Send(ctx context.Context, m Message) error {
	return Multi(n.resolve()...).Send(ctx, m)
}

// buildChannel 把一条通道记录构造成带统一重试的 Notifier；类型未知或配置坏返回 nil。
func buildChannel(ch model.NotifyChannel) Notifier {
	cfg := map[string]string{}
	if strings.TrimSpace(ch.ConfigJSON) != "" {
		if json.Unmarshal([]byte(ch.ConfigJSON), &cfg) != nil {
			return nil
		}
	}
	switch ch.Type {
	case "wework":
		c := wework.New(cfg["corp_id"], cfg["agent_id"], cfg["secret"], cfg["tag_id"])
		return Retry(3, 2*time.Second, WeWork(c))
	case "email":
		return Retry(3, 2*time.Second, Email(email.NewResend(cfg["api_key"], cfg["from"]), cfg["to"]))
	case "webhook":
		return Retry(3, 2*time.Second, Webhook(cfg["url"]))
	case "bark":
		return Retry(3, 2*time.Second, Bark(cfg["server"], cfg["device_key"]))
	default:
		return nil
	}
}
