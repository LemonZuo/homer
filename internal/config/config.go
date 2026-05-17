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

	// 后台任务 cron。
	BirthdayRemindCron string
	EventRemindCron    string

	// 任务连续失败达此次数才告警（防抖），默认 1（每次失败都告警）。
	SchedulerAlertFailThreshold int
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

		AliyunCDNAccessKeyID:     env("ALIYUN_CDN_ACCESS_KEY_ID", ""),
		AliyunCDNAccessKeySecret: env("ALIYUN_CDN_ACCESS_KEY_SECRET", ""),

		AliyunCASAccessKeyID:     env("ALIYUN_CAS_ACCESS_KEY_ID", ""),
		AliyunCASAccessKeySecret: env("ALIYUN_CAS_ACCESS_KEY_SECRET", ""),

		ACMEDataDir:         env("ACME_DATA_DIR", "./data/acme"),
		ACMERenewBeforeDays: envInt("ACME_RENEW_BEFORE_DAYS", 30),
		ACMERenewCron:       env("ACME_RENEW_CRON", "0 0 3 * * *"),

		BirthdayRemindCron: env("BIRTHDAY_REMIND_CRON", "0 0 9 * * *"),
		EventRemindCron:    env("EVENT_REMIND_CRON", "0 0 9 * * *"),

		SchedulerAlertFailThreshold: envInt("SCHEDULER_ALERT_FAIL_THRESHOLD", 1),
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
