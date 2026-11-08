package notify

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/LemonZuo/homer/internal/logx"
	"github.com/LemonZuo/homer/internal/model"
	"github.com/LemonZuo/homer/internal/modulecontrol"
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
	ModuleACME      = "acme"
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
	{ModuleACME, "证书签发/部署失败告警"},
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
	{"ntfy", "ntfy", []string{"server", "topic", "token"}},
}

// Hub 按 DB 里的「通道 + 模块映射」在发送时解析通知，配置改动即时生效（无缓存）。
type Hub struct {
	db      *gorm.DB
	modules *modulecontrol.Manager
}

func NewHub(db *gorm.DB, modules ...*modulecontrol.Manager) *Hub {
	h := &Hub{db: db}
	if len(modules) > 0 {
		h.modules = modules[0]
	}
	return h
}

// For 返回某模块的扇出 Notifier；Send 时实时查 DB，运行时改配置立即生效。
func (h *Hub) For(module string) Notifier {
	return &hubNotifier{db: h.db, module: module, modules: h.modules}
}

type hubNotifier struct {
	db      *gorm.DB
	module  string
	modules *modulecontrol.Manager
}

func (n *hubNotifier) Name() string { return "hub:" + n.module }

// resolve 查出该模块绑定的启用通道。DB 错误原样返回：调用方不能把「查不到配置」
// 当成「没配通道」，否则 MySQL 瞬断时告警会被视为已处理而丢失。
func (n *hubNotifier) resolve() ([]Notifier, error) {
	var chs []model.NotifyChannel
	err := n.db.
		Joins("JOIN notify_binding ON notify_binding.channel_id = notify_channel.id").
		Where("notify_binding.module = ? AND notify_channel.enabled = ?", n.module, "1").
		Find(&chs).Error
	if err != nil {
		return nil, fmt.Errorf("查询 %s 通知通道失败：%w", n.module, err)
	}
	out := make([]Notifier, 0, len(chs))
	for _, ch := range chs {
		if nf := buildChannel(ch); nf != nil {
			out = append(out, nf)
		}
	}
	return out, nil
}

// Enabled 查库失败时返回 true：让调用方走 Send 拿到错误并按发送失败重试，而不是当成「未配通道」跳过。
func (n *hubNotifier) Enabled() bool {
	if modulecontrol.Known(n.module) && !n.modules.Enabled(n.module) {
		return false
	}
	chs, err := n.resolve()
	if err != nil {
		logx.Warn("notify resolve failed", "module", n.module, "err", err)
		return true
	}
	for _, nf := range chs {
		if nf.Enabled() {
			return true
		}
	}
	return false
}

func (n *hubNotifier) Send(ctx context.Context, m Message) error {
	if modulecontrol.Known(n.module) {
		finish, err := n.modules.Begin(n.module)
		if err != nil {
			return err
		}
		defer finish()
	}
	chs, err := n.resolve()
	if err != nil {
		return err
	}
	return Multi(chs...).Send(ctx, m)
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
	case "ntfy":
		return Retry(3, 2*time.Second, Ntfy(cfg["server"], cfg["topic"], cfg["token"]))
	default:
		return nil
	}
}
