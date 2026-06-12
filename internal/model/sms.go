package model

import "time"

// 客户端安全措施模式，与 SmsForwarder Android「服务端设置-客户端安全措施」一致：
//
//	0 无       明文 JSON
//	1 签名     明文 JSON + HmacSHA256 sign 校验
//	2 RSA      请求体 RSA 公钥加密、响应用公钥解密（服务端持私钥）
//	3 SM4      请求/响应 SM4-CBC-PKCS7 + 固定 IV，hex 传输
const (
	SmsAuthNone = 0
	SmsAuthSign = 1
	SmsAuthRSA  = 2
	SmsAuthSM4  = 3
)

// SmsForwarder 一个「短信转发器」(SmsForwarder Android) 服务端配置。
// 支持配置多台、前端按需切换；不同安全模式所需密钥拆分到各自字段。
type SmsForwarder struct {
	ID             int64     `gorm:"primaryKey;column:id" json:"id"`
	Name           string    `gorm:"column:name;size:64;uniqueIndex;comment:转发器名称" json:"name"`
	ServerURL      string    `gorm:"column:server_url;size:512;comment:服务端地址" json:"server_url"`
	AuthMode       int       `gorm:"column:auth_mode;default:1;comment:安全模式" json:"auth_mode"`
	SignKey        string    `gorm:"column:sign_key;size:512;comment:签名密钥" json:"sign_key"`
	RSAPublicKey   string    `gorm:"column:rsa_public_key;type:text;comment:RSA 公钥" json:"rsa_public_key"`
	SM4Key         string    `gorm:"column:sm4_key;size:64;comment:SM4 密钥" json:"sm4_key"`
	TimeoutSeconds int       `gorm:"column:timeout_seconds;default:30;comment:超时秒数" json:"timeout_seconds"`
	Enabled        BoolFlag  `gorm:"column:enabled;type:varchar(1);default:'1';comment:是否启用" json:"enabled"`
	CreatedAt      time.Time `gorm:"column:created_at;autoCreateTime" json:"created_at"`
	UpdatedAt      time.Time `gorm:"column:updated_at;autoUpdateTime" json:"updated_at"`
}

func (SmsForwarder) TableName() string { return "sms_forwarder" }
