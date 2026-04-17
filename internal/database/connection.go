package database

import (
	"context"
	"fmt"
	"time"

	"github.com/nicholasricci/caddy-dashboard/internal/config"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func NewConnection(ctx context.Context, cfg config.DatabaseConfig) (*gorm.DB, error) {
	dsn := fmt.Sprintf(
		"%s:%s@tcp(%s:%d)/%s?%s",
		cfg.User, cfg.Password, cfg.Host, cfg.Port, cfg.Name, cfg.Params,
	)
	var (
		db  *gorm.DB
		err error
	)
	backoff := cfg.ConnectBackoff
	if backoff <= 0 {
		backoff = 500 * time.Millisecond
	}
	retries := cfg.ConnectRetries
	if retries < 1 {
		retries = 1
	}
	for i := 0; i < retries; i++ {
		db, err = gorm.Open(mysql.Open(dsn), &gorm.Config{})
		if err == nil {
			break
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(backoff):
		}
		if backoff < 10*time.Second {
			backoff *= 2
		}
	}
	if err != nil {
		return nil, fmt.Errorf("open mariadb connection: %w", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("sql db handle: %w", err)
	}
	sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	sqlDB.SetConnMaxIdleTime(cfg.ConnMaxIdleTime)
	return db, nil
}
