package main

import (
	"context"
	"log"

	"github.com/nicholasricci/caddy-dashboard/internal/config"
	"github.com/nicholasricci/caddy-dashboard/internal/database"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config load failed: %v", err)
	}
	db, err := database.NewConnection(context.Background(), cfg.Database)
	if err != nil {
		log.Fatalf("db connect failed: %v", err)
	}
	if err := database.AutoMigrate(db); err != nil {
		log.Fatalf("automigrate failed: %v", err)
	}
	log.Printf("migrations completed")
}
