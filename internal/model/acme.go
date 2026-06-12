package model

import "time"

// ACMECredential lego DNS provider 凭证。
// envs 存 JSON：{"<LEGO_ENV_KEY>":"<value>", ...}，键名要和 lego 文档里该 provider
// 期望的环境变量名一致（例如 alidns 用 ALICLOUD_ACCESS_KEY / ALICLOUD_SECRET_KEY）。
type ACMECredential struct {
	ID        int64     `gorm:"primaryKey;column:id" json:"id"`
	Provider  string    `gorm:"column:provider;size:64;uniqueIndex;comment:DNS provider" json:"provider"`
	EnvsJSON  string    `gorm:"column:envs_json;type:text;comment:环境变量 JSON" json:"envs_json"`
	CreatedAt time.Time `gorm:"column:created_at;autoCreateTime" json:"created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at;autoUpdateTime" json:"updated_at"`
	RefCount  int64     `gorm:"-" json:"ref_count"`
}

func (ACMECredential) TableName() string { return "acme_credential" }

// ACMEAccount ACME CA 账号配置。Let's Encrypt / ZeroSSL / 自定义 ACME directory 都存在库里，
// 域名按 account_id 选择签发方，避免全局 env 只能固定一个 CA。
type ACMEAccount struct {
	ID           int64     `gorm:"primaryKey;column:id" json:"id"`
	Name         string    `gorm:"column:name;size:64;uniqueIndex;comment:账号名称" json:"name"`
	CA           string    `gorm:"column:ca;size:32;comment:CA 类型" json:"ca"`
	DirectoryURL string    `gorm:"column:directory_url;size:512;comment:ACME directory URL" json:"directory_url"`
	Email        string    `gorm:"column:email;size:255;comment:注册邮箱" json:"email"`
	EABKID       string    `gorm:"column:eab_kid;size:255;comment:EAB KID" json:"eab_kid"`
	EABHMAC      string    `gorm:"column:eab_hmac;size:512;comment:EAB HMAC" json:"eab_hmac"`
	Enabled      BoolFlag  `gorm:"column:enabled;type:varchar(1);default:'1';comment:是否启用" json:"enabled"`
	CreatedAt    time.Time `gorm:"column:created_at;autoCreateTime" json:"created_at"`
	UpdatedAt    time.Time `gorm:"column:updated_at;autoUpdateTime" json:"updated_at"`
	RefCount     int64     `gorm:"-" json:"ref_count"`
}

func (ACMEAccount) TableName() string { return "acme_account" }

// ACMEDeployTarget 是证书部署目标的顶层抽象。
// kind 决定 driver（ssh / safeline / ...），endpoint/auth_json/config_json 由 driver 解释。
type ACMEDeployTarget struct {
	ID         int64     `gorm:"primaryKey;column:id" json:"id"`
	Name       string    `gorm:"column:name;size:64;uniqueIndex:uk_kind_name;comment:目标名称" json:"name"`
	Kind       string    `gorm:"column:kind;size:32;uniqueIndex:uk_kind_name;index;comment:目标类型" json:"kind"`
	Endpoint   string    `gorm:"column:endpoint;size:512;comment:目标地址" json:"endpoint"`
	AuthJSON   string    `gorm:"column:auth_json;type:text;comment:认证配置" json:"auth_json"`
	ConfigJSON string    `gorm:"column:config_json;type:text;comment:目标配置" json:"config_json"`
	Enabled    BoolFlag  `gorm:"column:enabled;type:varchar(1);default:'1';index;comment:是否启用" json:"enabled"`
	CreatedAt  time.Time `gorm:"column:created_at;autoCreateTime" json:"created_at"`
	UpdatedAt  time.Time `gorm:"column:updated_at;autoUpdateTime" json:"updated_at"`
}

func (ACMEDeployTarget) TableName() string { return "acme_deploy_target" }

// ACMEDeployConfig 是“某个域名部署到某个目标”的配置。
// config_json 是 driver 配置；state_json 存部署后回写状态，例如雷池 cert_id。
type ACMEDeployConfig struct {
	ID         int64     `gorm:"primaryKey;column:id" json:"id"`
	DomainID   int64     `gorm:"column:domain_id;index;comment:域名 ID" json:"domain_id"`
	TargetID   int64     `gorm:"column:target_id;index;comment:目标 ID" json:"target_id"`
	Kind       string    `gorm:"column:kind;size:32;index;comment:目标类型" json:"kind"`
	Name       string    `gorm:"column:name;size:64;comment:配置名称" json:"name"`
	ConfigJSON string    `gorm:"column:config_json;type:text;comment:部署配置" json:"config_json"`
	StateJSON  string    `gorm:"column:state_json;type:text;comment:部署状态" json:"state_json"`
	AutoDeploy BoolFlag  `gorm:"column:auto_deploy;type:varchar(1);default:'0';comment:自动部署" json:"auto_deploy"`
	Enabled    BoolFlag  `gorm:"column:enabled;type:varchar(1);default:'1';index;comment:是否启用" json:"enabled"`
	CreatedAt  time.Time `gorm:"column:created_at;autoCreateTime" json:"created_at"`
	UpdatedAt  time.Time `gorm:"column:updated_at;autoUpdateTime" json:"updated_at"`
}

func (ACMEDeployConfig) TableName() string { return "acme_deploy_config" }

// SSHCredential 可被多台机器复用的 SSH 登录身份（用户名 + 密码 / 私钥）。
// 引用关系不落 acme_deploy_target 列，而是写在 auth_json 里：
//
//	{"auth_source":"credential","credential_id":42}
//
// 这样 safeline 等其他 driver 无需感知此表。
type SSHCredential struct {
	ID         int64     `gorm:"primaryKey;column:id" json:"id"`
	Name       string    `gorm:"column:name;size:64;uniqueIndex;comment:凭证名称" json:"name"`
	Username   string    `gorm:"column:username;size:128;comment:用户名" json:"username"`
	AuthType   string    `gorm:"column:auth_type;size:16;comment:认证类型" json:"auth_type"`
	Password   string    `gorm:"column:password;type:text;comment:密码" json:"password"`
	PrivateKey string    `gorm:"column:private_key;type:text;comment:私钥" json:"private_key"`
	Passphrase string    `gorm:"column:passphrase;type:text;comment:私钥口令" json:"passphrase"`
	CreatedAt  time.Time `gorm:"column:created_at;autoCreateTime" json:"created_at"`
	UpdatedAt  time.Time `gorm:"column:updated_at;autoUpdateTime" json:"updated_at"`
	RefCount   int64     `gorm:"-" json:"ref_count"`
}

func (SSHCredential) TableName() string { return "ssh_credential" }

// ACMEDomain 自动签发的域名配置。
// account_id 引用 ACMEAccount.ID；provider 引用 ACMECredential.Provider。
// san_providers 是可选的「按域名指定 DNS provider」覆盖表，JSON：{"b.com":"alidns"}；
// 未列出的域名走 provider（默认）。用于一张证书的域名跨多个 DNS 服务商的场景。
type ACMEDomain struct {
	ID           int64     `gorm:"primaryKey;column:id" json:"id"`
	MainDomain   string    `gorm:"column:main_domain;size:255;index:idx_main_domain;comment:主域名" json:"main_domain"`
	SanDomains   string    `gorm:"column:san_domains;size:1024;comment:SAN 域名" json:"san_domains"`
	AccountID    int64     `gorm:"column:account_id;index;comment:账号 ID" json:"account_id"`
	Provider     string    `gorm:"column:provider;size:64;comment:DNS provider" json:"provider"`
	SanProviders string    `gorm:"column:san_providers;size:1024;comment:SAN provider 覆盖" json:"san_providers"`
	Enabled      BoolFlag  `gorm:"column:enabled;type:varchar(1);default:'1';comment:是否启用" json:"enabled"`
	CreatedAt    time.Time `gorm:"column:created_at;autoCreateTime" json:"created_at"`
	UpdatedAt    time.Time `gorm:"column:updated_at;autoUpdateTime" json:"updated_at"`
}

func (ACMEDomain) TableName() string { return "acme_domain" }

// ACMECert 一次签发产物。每个域名只保留最近一条；续期时直接 upsert。
type ACMECert struct {
	ID           int64      `gorm:"primaryKey;column:id" json:"id"`
	DomainID     int64      `gorm:"column:domain_id;uniqueIndex;comment:域名 ID" json:"domain_id"`
	CertPEM      string     `gorm:"column:cert_pem;type:mediumtext;comment:证书" json:"-"`
	KeyPEM       string     `gorm:"column:key_pem;type:mediumtext;comment:私钥" json:"-"`
	ChainPEM     string     `gorm:"column:chain_pem;type:mediumtext;comment:中间证书" json:"-"`
	FullchainPEM string     `gorm:"column:fullchain_pem;type:mediumtext;comment:完整链" json:"-"`
	Serial       string     `gorm:"column:serial;size:128;comment:序列号" json:"serial"`
	NotBefore    time.Time  `gorm:"column:not_before;comment:生效时间" json:"not_before"`
	NotAfter     time.Time  `gorm:"column:not_after;comment:到期时间" json:"not_after"`
	Status       string     `gorm:"column:status;size:16;comment:状态" json:"status"`
	RevokedAt    *time.Time `gorm:"column:revoked_at;comment:吊销时间" json:"revoked_at"`
	IssuedAt     time.Time  `gorm:"column:issued_at;autoCreateTime;comment:签发时间" json:"issued_at"`
}

func (ACMECert) TableName() string { return "acme_cert" }

// ACMEIssueTask 签发/续期任务流水。log_text 全量存最终日志，
// 任务运行期间通过 SSE 推送增量；任务结束后前端从 GET /tasks/:id 拿全文。
type ACMEIssueTask struct {
	ID         int64      `gorm:"primaryKey;column:id" json:"id"`
	DomainID   int64      `gorm:"column:domain_id;index;comment:域名 ID" json:"domain_id"`
	MainDomain string     `gorm:"column:main_domain;size:255;comment:主域名" json:"main_domain"`
	Kind       string     `gorm:"column:kind;size:32;comment:任务类型" json:"kind"`
	Status     string     `gorm:"column:status;size:16;index;comment:状态" json:"status"`
	StartedAt  time.Time  `gorm:"column:started_at;autoCreateTime;comment:开始时间" json:"started_at"`
	FinishedAt *time.Time `gorm:"column:finished_at;comment:结束时间" json:"finished_at"`
	LogText    string     `gorm:"column:log_text;type:mediumtext;comment:日志" json:"log_text"`
	ErrorMsg   string     `gorm:"column:error_msg;size:1024;comment:错误信息" json:"error_msg"`

	// 失败重试。仅持久化部署配置触发的部署任务参与重试（config_id>0）。
	// attempt=已执行次数，max_attempt=允许总次数（1=不重试），
	// next_retry_at=下次可重试时刻，由 acme-deploy-retry cron 扫描拉起。
	Attempt     int        `gorm:"column:attempt;not null;default:0;comment:已执行次数" json:"attempt"`
	MaxAttempt  int        `gorm:"column:max_attempt;not null;default:1;comment:允许总次数" json:"max_attempt"`
	ConfigID    int64      `gorm:"column:config_id;not null;default:0;comment:部署配置 ID" json:"config_id"`
	NextRetryAt *time.Time `gorm:"column:next_retry_at;index;comment:下次重试时刻" json:"next_retry_at"`
}

func (ACMEIssueTask) TableName() string { return "acme_issue_task" }
