package database

import (
	"fmt"

	"github.com/nicholasricci/caddy-dashboard/internal/config"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func NewConnection(cfg config.DatabaseConfig) (*gorm.DB, error) {
	dsn := fmt.Sprintf(
		"%s:%s@tcp(%s:%d)/%s?%s",
		cfg.User, cfg.Password, cfg.Host, cfg.Port, cfg.Name, cfg.Params,
	)
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("open mariadb connection: %w", err)
	}
	return db, nil
}
