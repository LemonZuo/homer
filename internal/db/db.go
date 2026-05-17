package db

import (
	"strings"

	"github.com/LemonZuo/homer/internal/config"
	"github.com/LemonZuo/homer/internal/logx"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func New(cfg *config.Config) (*gorm.DB, error) {
	debug := strings.EqualFold(strings.TrimSpace(cfg.LogLevel), "debug")
	gdb, err := gorm.Open(mysql.Open(cfg.DSN()), &gorm.Config{
		Logger: logx.NewGormLogger(debug),
	})
	if err != nil {
		logx.Error("db connect failed", "host", cfg.DBHost, "port", cfg.DBPort, "db", cfg.DBName, "err", err)
		return nil, err
	}
	logx.Info("db connected", "host", cfg.DBHost, "port", cfg.DBPort, "db", cfg.DBName)
	return gdb, nil
}
