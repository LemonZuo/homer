package model

import "time"

// ACMECredential lego DNS provider 凭证。
// envs 存 JSON：{"<LEGO_ENV_KEY>":"<value>", ...}，键名要和 lego 文档里该 provider
// 期望的环境变量名一致（例如 alidns 用 ALICLOUD_ACCESS_KEY / ALICLOUD_SECRET_KEY）。
type ACMECredential struct {
	ID        int64     `gorm:"primaryKey;column:id" json:"id"`
	Provider  string    `gorm:"column:provider;size:64;uniqueIndex" json:"provider"`
	EnvsJSON  string    `gorm:"column:envs_json;type:text" json:"envs_json"`
	CreatedAt time.Time `gorm:"column:created_at;autoCreateTime" json:"created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at;autoUpdateTime" json:"updated_at"`
}

func (ACMECredential) TableName() string { return "acme_credential" }

// ACMEAccount ACME CA 账号配置。Let's Encrypt / ZeroSSL / 自定义 ACME directory 都存在库里，
// 域名按 account_id 选择签发方，避免全局 env 只能固定一个 CA。
type ACMEAccount struct {
	ID           int64     `gorm:"primaryKey;column:id" json:"id"`
	Name         string    `gorm:"column:name;size:64;uniqueIndex" json:"name"`
	CA           string    `gorm:"column:ca;size:32" json:"ca"`
	DirectoryURL string    `gorm:"column:directory_url;size:512" json:"directory_url"`
	Email        string    `gorm:"column:email;size:255" json:"email"`
	EABKID       string    `gorm:"column:eab_kid;size:255" json:"eab_kid"`
	EABHMAC      string    `gorm:"column:eab_hmac;size:512" json:"eab_hmac"`
	Enabled      BoolFlag  `gorm:"column:enabled;type:varchar(1);default:'1'" json:"enabled"`
	CreatedAt    time.Time `gorm:"column:created_at;autoCreateTime" json:"created_at"`
	UpdatedAt    time.Time `gorm:"column:updated_at;autoUpdateTime" json:"updated_at"`
}

func (ACMEAccount) TableName() string { return "acme_account" }

// ACMEDomain 自动签发的域名配置。
// account_id 引用 ACMEAccount.ID；provider 引用 ACMECredential.Provider。
type ACMEDomain struct {
	ID         int64     `gorm:"primaryKey;column:id" json:"id"`
	MainDomain string    `gorm:"column:main_domain;size:255;uniqueIndex" json:"main_domain"`
	SanDomains string    `gorm:"column:san_domains;size:1024" json:"san_domains"`
	AccountID  int64     `gorm:"column:account_id;index" json:"account_id"`
	Provider   string    `gorm:"column:provider;size:64" json:"provider"`
	Enabled    BoolFlag  `gorm:"column:enabled;type:varchar(1);default:'1'" json:"enabled"`
	CreatedAt  time.Time `gorm:"column:created_at;autoCreateTime" json:"created_at"`
	UpdatedAt  time.Time `gorm:"column:updated_at;autoUpdateTime" json:"updated_at"`
}

func (ACMEDomain) TableName() string { return "acme_domain" }

// ACMECert 一次签发产物。每个域名只保留最近一条；续期时直接 upsert。
type ACMECert struct {
	ID           int64     `gorm:"primaryKey;column:id" json:"id"`
	DomainID     int64     `gorm:"column:domain_id;uniqueIndex" json:"domain_id"`
	CertPEM      string    `gorm:"column:cert_pem;type:mediumtext" json:"-"`
	KeyPEM       string    `gorm:"column:key_pem;type:mediumtext" json:"-"`
	ChainPEM     string    `gorm:"column:chain_pem;type:mediumtext" json:"-"`
	FullchainPEM string    `gorm:"column:fullchain_pem;type:mediumtext" json:"-"`
	Serial       string    `gorm:"column:serial;size:128" json:"serial"`
	NotBefore    time.Time `gorm:"column:not_before" json:"not_before"`
	NotAfter     time.Time `gorm:"column:not_after" json:"not_after"`
	CASCertID    int64     `gorm:"column:cas_cert_id" json:"cas_cert_id"`
	IssuedAt     time.Time `gorm:"column:issued_at;autoCreateTime" json:"issued_at"`
}

func (ACMECert) TableName() string { return "acme_cert" }

// ACMEIssueTask 签发/续期任务流水。log_text 全量存最终日志，
// 任务运行期间通过 SSE 推送增量；任务结束后前端从 GET /tasks/:id 拿全文。
type ACMEIssueTask struct {
	ID         int64      `gorm:"primaryKey;column:id" json:"id"`
	DomainID   int64      `gorm:"column:domain_id;index" json:"domain_id"`
	MainDomain string     `gorm:"column:main_domain;size:255" json:"main_domain"`
	Kind       string     `gorm:"column:kind;size:16" json:"kind"`           // issue | renew
	Status     string     `gorm:"column:status;size:16;index" json:"status"` // pending | running | success | failed
	StartedAt  time.Time  `gorm:"column:started_at;autoCreateTime" json:"started_at"`
	FinishedAt *time.Time `gorm:"column:finished_at" json:"finished_at"`
	LogText    string     `gorm:"column:log_text;type:mediumtext" json:"log_text"`
	ErrorMsg   string     `gorm:"column:error_msg;size:1024" json:"error_msg"`
}

func (ACMEIssueTask) TableName() string { return "acme_issue_task" }
