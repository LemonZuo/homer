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
	RefCount  int64     `gorm:"-" json:"ref_count"`
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
	RefCount     int64     `gorm:"-" json:"ref_count"`
}

func (ACMEAccount) TableName() string { return "acme_account" }

// ACMEDeployTarget 是证书部署目标的顶层抽象。
// kind 决定 driver（ssh / safeline / ...），endpoint/auth_json/config_json 由 driver 解释。
type ACMEDeployTarget struct {
	ID         int64     `gorm:"primaryKey;column:id" json:"id"`
	Name       string    `gorm:"column:name;size:64;uniqueIndex:uk_kind_name" json:"name"`
	Kind       string    `gorm:"column:kind;size:32;uniqueIndex:uk_kind_name;index" json:"kind"`
	Endpoint   string    `gorm:"column:endpoint;size:512" json:"endpoint"`
	AuthJSON   string    `gorm:"column:auth_json;type:text" json:"auth_json"`
	ConfigJSON string    `gorm:"column:config_json;type:text" json:"config_json"`
	Enabled    BoolFlag  `gorm:"column:enabled;type:varchar(1);default:'1';index" json:"enabled"`
	CreatedAt  time.Time `gorm:"column:created_at;autoCreateTime" json:"created_at"`
	UpdatedAt  time.Time `gorm:"column:updated_at;autoUpdateTime" json:"updated_at"`
}

func (ACMEDeployTarget) TableName() string { return "acme_deploy_target" }

// ACMEDeployConfig 是“某个域名部署到某个目标”的配置。
// config_json 是 driver 配置；state_json 存部署后回写状态，例如雷池 cert_id。
type ACMEDeployConfig struct {
	ID         int64     `gorm:"primaryKey;column:id" json:"id"`
	DomainID   int64     `gorm:"column:domain_id;index" json:"domain_id"`
	TargetID   int64     `gorm:"column:target_id;index" json:"target_id"`
	Kind       string    `gorm:"column:kind;size:32;index" json:"kind"`
	Name       string    `gorm:"column:name;size:64" json:"name"`
	ConfigJSON string    `gorm:"column:config_json;type:text" json:"config_json"`
	StateJSON  string    `gorm:"column:state_json;type:text" json:"state_json"`
	AutoDeploy BoolFlag  `gorm:"column:auto_deploy;type:varchar(1);default:'0'" json:"auto_deploy"`
	Enabled    BoolFlag  `gorm:"column:enabled;type:varchar(1);default:'1';index" json:"enabled"`
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
	Name       string    `gorm:"column:name;size:64;uniqueIndex" json:"name"`
	Username   string    `gorm:"column:username;size:128" json:"username"`
	AuthType   string    `gorm:"column:auth_type;size:16" json:"auth_type"` // password | key
	Password   string    `gorm:"column:password;type:text" json:"password"`
	PrivateKey string    `gorm:"column:private_key;type:text" json:"private_key"`
	Passphrase string    `gorm:"column:passphrase;type:text" json:"passphrase"`
	CreatedAt  time.Time `gorm:"column:created_at;autoCreateTime" json:"created_at"`
	UpdatedAt  time.Time `gorm:"column:updated_at;autoUpdateTime" json:"updated_at"`
	RefCount   int64     `gorm:"-" json:"ref_count"`
}

func (SSHCredential) TableName() string { return "ssh_credential" }

// ACMESSHTarget 是旧 HTTP/UI 兼容视图；实际持久化使用 ACMEDeployTarget。
// AuthSource:
//   - "inline"（默认）：使用 Username + AuthType + Password|PrivateKey|Passphrase
//   - "credential"：忽略 inline 字段，运行时从 ssh_credential 加载（按 CredentialID）
//
// BastionTargetID 可选：>0 表示先连这台 SSH 机器，再从它出网到当前目标（单跳，
// 不支持跳板机链）。落在 acme_deploy_target.config_json 里，不是独立列。
type ACMESSHTarget struct {
	ID              int64     `gorm:"primaryKey;column:id" json:"id"`
	Name            string    `gorm:"column:name;size:64;uniqueIndex" json:"name"`
	Host            string    `gorm:"column:host;size:255" json:"host"`
	Port            int       `gorm:"column:port;default:22" json:"port"`
	AuthSource      string    `gorm:"-" json:"auth_source"`       // inline | credential
	CredentialID    int64     `gorm:"-" json:"credential_id"`     // auth_source=credential 时使用
	BastionTargetID int64     `gorm:"-" json:"bastion_target_id"` // 0=直连；>0=经此 SSH 机器跳一次
	Username        string    `gorm:"column:username;size:128" json:"username"`
	AuthType        string    `gorm:"column:auth_type;size:16" json:"auth_type"` // password | key
	Password        string    `gorm:"column:password;type:text" json:"password"`
	PrivateKey      string    `gorm:"column:private_key;type:text" json:"private_key"`
	Passphrase      string    `gorm:"column:passphrase;type:text" json:"passphrase"`
	Enabled         BoolFlag  `gorm:"column:enabled;type:varchar(1);default:'1'" json:"enabled"`
	CreatedAt       time.Time `gorm:"column:created_at;autoCreateTime" json:"created_at"`
	UpdatedAt       time.Time `gorm:"column:updated_at;autoUpdateTime" json:"updated_at"`
}

// ACMESSHDeployConfig 是旧 HTTP/UI 兼容视图；实际持久化使用 ACMEDeployConfig。
type ACMESSHDeployConfig struct {
	ID            int64     `gorm:"primaryKey;column:id" json:"id"`
	DomainID      int64     `gorm:"column:domain_id;index" json:"domain_id"`
	TargetID      int64     `gorm:"column:target_id;index" json:"target_id"`
	Name          string    `gorm:"column:name;size:64" json:"name"`
	CertPath      string    `gorm:"column:cert_path;size:512" json:"cert_path"`
	KeyPath       string    `gorm:"column:key_path;size:512" json:"key_path"`
	ChainPath     string    `gorm:"column:chain_path;size:512" json:"chain_path"`
	FullchainPath string    `gorm:"column:fullchain_path;size:512" json:"fullchain_path"`
	DeployCommand string    `gorm:"column:deploy_command;type:text" json:"deploy_command"`
	AutoDeploy    BoolFlag  `gorm:"column:auto_deploy;type:varchar(1);default:'0'" json:"auto_deploy"`
	Enabled       BoolFlag  `gorm:"column:enabled;type:varchar(1);default:'1'" json:"enabled"`
	CreatedAt     time.Time `gorm:"column:created_at;autoCreateTime" json:"created_at"`
	UpdatedAt     time.Time `gorm:"column:updated_at;autoUpdateTime" json:"updated_at"`
}

// ACMESafelineTarget 是旧 HTTP/UI 兼容视图；实际持久化使用 ACMEDeployTarget。
type ACMESafelineTarget struct {
	ID            int64     `gorm:"primaryKey;column:id" json:"id"`
	Name          string    `gorm:"column:name;size:64;uniqueIndex" json:"name"`
	BaseURL       string    `gorm:"column:base_url;size:512" json:"base_url"`
	APIToken      string    `gorm:"column:api_token;type:text" json:"api_token"`
	SkipTLSVerify BoolFlag  `gorm:"column:skip_tls_verify;type:varchar(1);default:'0'" json:"skip_tls_verify"`
	Enabled       BoolFlag  `gorm:"column:enabled;type:varchar(1);default:'1'" json:"enabled"`
	CreatedAt     time.Time `gorm:"column:created_at;autoCreateTime" json:"created_at"`
	UpdatedAt     time.Time `gorm:"column:updated_at;autoUpdateTime" json:"updated_at"`
}

// ACMESafelineDeployConfig 是旧 HTTP/UI 兼容视图；实际持久化使用 ACMEDeployConfig。
type ACMESafelineDeployConfig struct {
	ID         int64     `gorm:"primaryKey;column:id" json:"id"`
	DomainID   int64     `gorm:"column:domain_id;index" json:"domain_id"`
	TargetID   int64     `gorm:"column:target_id;index" json:"target_id"`
	Name       string    `gorm:"column:name;size:64" json:"name"`
	CertID     int64     `gorm:"column:cert_id;default:0" json:"cert_id"`
	CertType   int       `gorm:"column:cert_type;default:2" json:"cert_type"`
	AutoDeploy BoolFlag  `gorm:"column:auto_deploy;type:varchar(1);default:'0'" json:"auto_deploy"`
	Enabled    BoolFlag  `gorm:"column:enabled;type:varchar(1);default:'1'" json:"enabled"`
	CreatedAt  time.Time `gorm:"column:created_at;autoCreateTime" json:"created_at"`
	UpdatedAt  time.Time `gorm:"column:updated_at;autoUpdateTime" json:"updated_at"`
}

// ACMEDomain 自动签发的域名配置。
// account_id 引用 ACMEAccount.ID；provider 引用 ACMECredential.Provider。
// san_providers 是可选的「按域名指定 DNS provider」覆盖表，JSON：{"b.com":"alidns"}；
// 未列出的域名走 provider（默认）。用于一张证书的域名跨多个 DNS 服务商的场景。
// cas_enabled 控制本域名的证书是否参与阿里云 CAS：开启后签发/续期自动上传，
// 手动上传按钮也只有开启时才能用；关闭则两条路径都被拦截。
type ACMEDomain struct {
	ID           int64     `gorm:"primaryKey;column:id" json:"id"`
	MainDomain   string    `gorm:"column:main_domain;size:255;index:idx_main_domain" json:"main_domain"`
	SanDomains   string    `gorm:"column:san_domains;size:1024" json:"san_domains"`
	AccountID    int64     `gorm:"column:account_id;index" json:"account_id"`
	Provider     string    `gorm:"column:provider;size:64" json:"provider"`
	SanProviders string    `gorm:"column:san_providers;size:1024" json:"san_providers"`
	CASEnabled   BoolFlag  `gorm:"column:cas_enabled;type:varchar(1);default:'0'" json:"cas_enabled"`
	Enabled      BoolFlag  `gorm:"column:enabled;type:varchar(1);default:'1'" json:"enabled"`
	CreatedAt    time.Time `gorm:"column:created_at;autoCreateTime" json:"created_at"`
	UpdatedAt    time.Time `gorm:"column:updated_at;autoUpdateTime" json:"updated_at"`
}

func (ACMEDomain) TableName() string { return "acme_domain" }

// ACMECert 一次签发产物。每个域名只保留最近一条；续期时直接 upsert。
type ACMECert struct {
	ID           int64      `gorm:"primaryKey;column:id" json:"id"`
	DomainID     int64      `gorm:"column:domain_id;uniqueIndex" json:"domain_id"`
	CertPEM      string     `gorm:"column:cert_pem;type:mediumtext" json:"-"`
	KeyPEM       string     `gorm:"column:key_pem;type:mediumtext" json:"-"`
	ChainPEM     string     `gorm:"column:chain_pem;type:mediumtext" json:"-"`
	FullchainPEM string     `gorm:"column:fullchain_pem;type:mediumtext" json:"-"`
	Serial       string     `gorm:"column:serial;size:128" json:"serial"`
	NotBefore    time.Time  `gorm:"column:not_before" json:"not_before"`
	NotAfter     time.Time  `gorm:"column:not_after" json:"not_after"`
	CASCertID    int64      `gorm:"column:cas_cert_id" json:"cas_cert_id"`
	Status       string     `gorm:"column:status;size:16" json:"status"` // active | revoked
	RevokedAt    *time.Time `gorm:"column:revoked_at" json:"revoked_at"`
	IssuedAt     time.Time  `gorm:"column:issued_at;autoCreateTime" json:"issued_at"`
}

func (ACMECert) TableName() string { return "acme_cert" }

// ACMEIssueTask 签发/续期任务流水。log_text 全量存最终日志，
// 任务运行期间通过 SSE 推送增量；任务结束后前端从 GET /tasks/:id 拿全文。
type ACMEIssueTask struct {
	ID         int64      `gorm:"primaryKey;column:id" json:"id"`
	DomainID   int64      `gorm:"column:domain_id;index" json:"domain_id"`
	MainDomain string     `gorm:"column:main_domain;size:255" json:"main_domain"`
	Kind       string     `gorm:"column:kind;size:32" json:"kind"`           // issue | renew | revoke | upload_cas | deploy_ssh | deploy_safeline | deploy
	Status     string     `gorm:"column:status;size:16;index" json:"status"` // pending | running | success | failed
	StartedAt  time.Time  `gorm:"column:started_at;autoCreateTime" json:"started_at"`
	FinishedAt *time.Time `gorm:"column:finished_at" json:"finished_at"`
	LogText    string     `gorm:"column:log_text;type:mediumtext" json:"log_text"`
	ErrorMsg   string     `gorm:"column:error_msg;size:1024" json:"error_msg"`
}

func (ACMEIssueTask) TableName() string { return "acme_issue_task" }
