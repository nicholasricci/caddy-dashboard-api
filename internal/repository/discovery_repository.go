package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/nicholasricci/caddy-dashboard/internal/models"
	"gorm.io/gorm"
)

type DiscoveryRepository struct {
	db *gorm.DB
}

func NewDiscoveryRepository(db *gorm.DB) *DiscoveryRepository {
	return &DiscoveryRepository{db: db}
}

func (r *DiscoveryRepository) List(ctx context.Context) ([]models.DiscoveryConfig, error) {
	var cfgs []models.DiscoveryConfig
	err := r.db.WithContext(ctx).Order("created_at desc").Find(&cfgs).Error
	return cfgs, err
}

func (r *DiscoveryRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.DiscoveryConfig, error) {
	var cfg models.DiscoveryConfig
	if err := r.db.WithContext(ctx).First(&cfg, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &cfg, nil
}

func (r *DiscoveryRepository) Create(ctx context.Context, cfg *models.DiscoveryConfig) error {
	return r.db.WithContext(ctx).Create(cfg).Error
}

func (r *DiscoveryRepository) Update(ctx context.Context, cfg *models.DiscoveryConfig) error {
	return r.db.WithContext(ctx).Save(cfg).Error
}

func (r *DiscoveryRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Delete(&models.DiscoveryConfig{}, "id = ?", id).Error
}
