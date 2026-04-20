package testutil

import (
	"fmt"
	"testing"
	"time"

	"github.com/nicholasricci/caddy-dashboard/internal/models"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// NewTestDB returns an isolated sqlite database for repository tests.
func NewTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dsn := fmt.Sprintf("file:%s_%d?mode=memory&_pragma=foreign_keys(1)", t.Name(), time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sql.DB: %v", err)
	}
	// In-memory sqlite is per-connection, keep one shared connection per test DB.
	sqlDB.SetMaxOpenConns(1)

	migrateTestSchema(t, db)

	t.Cleanup(func() {
		_ = sqlDB.Close()
	})
	return db
}

func migrateTestSchema(t *testing.T, db *gorm.DB) {
	t.Helper()
	if err := db.AutoMigrate(
		&models.DiscoveryConfig{},
		&models.CaddyNode{},
		&models.CaddySnapshot{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}
}
