package database

import (
	"github.com/nicholasricci/caddy-dashboard/internal/models"
	"gorm.io/gorm"
)

func AutoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&models.CaddyNode{},
		&models.DiscoveryConfig{},
		&models.CaddySnapshot{},
		&models.User{},
		&models.RefreshToken{},
		&models.AuditLog{},
	)
}
