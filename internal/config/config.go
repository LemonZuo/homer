package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	DBHost     string
	DBPort     string
	DBUser     string
	DBPassword string
	DBName     string
	DBCharset  string
	ServerPort string

	// 日志级别：debug | info | warn | error（默认 info）。仅控制台输出，不落文件。
	LogLevel string

	// 阿里云 CDN（加速域名管理，与 CAS 用独立 AK/SK）
	AliyunCDNAccessKeyID     string
	AliyunCDNAccessKeySecret string

	// 阿里云 CAS 数字证书管理（与 CDN 用独立 AK/SK）
	AliyunCASAccessKeyID     string
	AliyunCASAccessKeySecret string

	// ACME 自动签发。CA 账号与 ZeroSSL EAB 信息存 acme_account 表。
	ACMEDataDir         string // ./data/acme
	ACMERenewBeforeDays int    // 剩余天数 ≤ 此值则续期
	ACMERenewCron       string // cron 表达式
	ACMEKeyType         string // 证书密钥类型：ec256 | ec384 | rsa2048 | rsa3072 | rsa4096 | rsa8192（默认 ec256）

	// 部署任务失败重试。仅作用于持久化部署配置触发的任务（手动单条 / 按域名 / 续期后自动）；
	// 临时部署（ad-hoc，无法重建配置）不重试。重试由 cron 择时拉起，不在任务内 sleep。
	ACMEDeployRetry           int    // 允许总执行次数（含首次），1=不重试
	ACMEDeployRetryBackoffSec int    // 退避基数秒，实际间隔 = backoff * 已执行次数
	ACMEDeployRetryCron       string // 扫描待重试任务的 cron

	// 后台任务 cron。
	BirthdayRemindCron string
	EventRemindCron    string

	// 任务连续失败达此次数才告警（防抖），默认 1（每次失败都告警）。
	SchedulerAlertFailThreshold int

	// UPS 监控。机器从 ups_host(enabled='1')取,凭证从 ups_ssh_credential 库选。
	// SampleCron 6 段表达式;UPSSSHTimeoutSec 是单机采样的整体超时;
	// RetentionDays 是 ups_sample 的保留天数,过期由 cleanup cron 清。
	UPSSampleCron    string
	UPSCleanupCron   string
	UPSRetentionDays int
	UPSSSHTimeoutSec int

	// ESXi 监控。机器从 esxi_host(enabled='1')取,凭证从 esxi_ssh_credential 库选。
	// 单机一轮要跑多次 esxcli/vsish/vim-cmd,EsxiSSHTimeoutSec 给得宽一点。
	EsxiSampleCron    string
	EsxiCleanupCron   string
	EsxiRetentionDays int
	EsxiSSHTimeoutSec int
}

func Load() *Config {
	_ = godotenv.Load()
	return &Config{
		DBHost:     env("DB_HOST", "127.0.0.1"),
		DBPort:     env("DB_PORT", "3306"),
		DBUser:     env("DB_USER", "root"),
		DBPassword: env("DB_PASSWORD", ""),
		DBName:     env("DB_NAME", ""),
		DBCharset:  env("DB_CHARSET", "utf8mb4"),
		ServerPort: normalizePort(env("SERVER_PORT", "8081")),

		LogLevel: env("LOG_LEVEL", "info"),

		AliyunCDNAccessKeyID:     env("ALIYUN_CDN_ACCESS_KEY_ID", ""),
		AliyunCDNAccessKeySecret: env("ALIYUN_CDN_ACCESS_KEY_SECRET", ""),

		AliyunCASAccessKeyID:     env("ALIYUN_CAS_ACCESS_KEY_ID", ""),
		AliyunCASAccessKeySecret: env("ALIYUN_CAS_ACCESS_KEY_SECRET", ""),

		ACMEDataDir:         env("ACME_DATA_DIR", "./data/acme"),
		ACMERenewBeforeDays: envInt("ACME_RENEW_BEFORE_DAYS", 30),
		ACMERenewCron:       env("ACME_RENEW_CRON", "0 0 3 * * *"),
		ACMEKeyType:         env("ACME_KEY_TYPE", "ec256"),

		ACMEDeployRetry:           envInt("ACME_DEPLOY_RETRY", 3),
		ACMEDeployRetryBackoffSec: envInt("ACME_DEPLOY_RETRY_BACKOFF_SEC", 10),
		ACMEDeployRetryCron:       env("ACME_DEPLOY_RETRY_CRON", "0 * * * * *"),

		BirthdayRemindCron: env("BIRTHDAY_REMIND_CRON", "0 0 9 * * *"),
		EventRemindCron:    env("EVENT_REMIND_CRON", "0 0 9 * * *"),

		SchedulerAlertFailThreshold: envInt("SCHEDULER_ALERT_FAIL_THRESHOLD", 1),

		UPSSampleCron:    env("UPS_SAMPLE_CRON", "*/30 * * * * *"),
		UPSCleanupCron:   env("UPS_CLEANUP_CRON", "0 0 4 * * *"),
		UPSRetentionDays: envInt("UPS_RETENTION_DAYS", 7),
		UPSSSHTimeoutSec: envInt("UPS_SSH_TIMEOUT_SEC", 5),

		EsxiSampleCron:    env("ESXI_SAMPLE_CRON", "*/30 * * * * *"),
		EsxiCleanupCron:   env("ESXI_CLEANUP_CRON", "0 0 4 * * *"),
		EsxiRetentionDays: envInt("ESXI_RETENTION_DAYS", 7),
		EsxiSSHTimeoutSec: envInt("ESXI_SSH_TIMEOUT_SEC", 30),
	}
}

func envInt(key string, def int) int {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	n := 0
	if _, err := fmt.Sscanf(v, "%d", &n); err != nil || n <= 0 {
		return def
	}
	return n
}

func (c *Config) DSN() string {
	return fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=%s&parseTime=True&loc=Local",
		c.DBUser, c.DBPassword, c.DBHost, c.DBPort, c.DBName, c.DBCharset)
}

func (c *Config) ListenAddr() string {
	return ":" + normalizePort(c.ServerPort)
}

func (c *Config) ListenURL() string {
	return "http://0.0.0.0:" + normalizePort(c.ServerPort)
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func normalizePort(port string) string {
	port = strings.TrimSpace(port)
	port = strings.TrimPrefix(port, ":")
	if port == "" {
		return "8081"
	}
	return port
}
