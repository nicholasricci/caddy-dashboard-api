package database

import (
	"fmt"

	"github.com/nicholasricci/caddy-dashboard/internal/models"
	"gorm.io/gorm"
)

// BackfillCaddyNodes sets default transport for legacy rows (idempotent).
func BackfillCaddyNodes(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("nil db")
	}
	if !db.Migrator().HasTable("caddy_nodes") {
		return nil
	}
	if !db.Migrator().HasColumn(&models.CaddyNode{}, "transport") {
		return nil
	}
	return db.Exec(`UPDATE caddy_nodes SET transport = 'aws_ssm' WHERE transport IS NULL OR transport = ''`).Error
}
