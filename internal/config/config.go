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

	// 企业微信通知（生日提醒等使用）
	WeWorkCorpID  string
	WeWorkAgentID string
	WeWorkSecret  string
	WeWorkTagID   string

	// 阿里云 CDN（加速域名管理，与 CAS 用独立 AK/SK）
	AliyunCDNAccessKeyID     string
	AliyunCDNAccessKeySecret string

	// 调度
	BirthdayRemindCron string
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

		WeWorkCorpID:  env("WEWORK_CORP_ID", ""),
		WeWorkAgentID: env("WEWORK_AGENT_ID", ""),
		WeWorkSecret:  env("WEWORK_SECRET", ""),
		WeWorkTagID:   env("WEWORK_TAG_ID", ""),

		AliyunCDNAccessKeyID:     env("ALIYUN_CDN_ACCESS_KEY_ID", ""),
		AliyunCDNAccessKeySecret: env("ALIYUN_CDN_ACCESS_KEY_SECRET", ""),

		BirthdayRemindCron: env("BIRTHDAY_REMIND_CRON", "0 0 9 * * *"),
	}
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
