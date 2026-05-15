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
