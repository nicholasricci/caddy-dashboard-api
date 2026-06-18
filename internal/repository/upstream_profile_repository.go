package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/nicholasricci/caddy-dashboard/internal/models"
	"gorm.io/gorm"
)

type UpstreamProfileRepository struct {
	db *gorm.DB
}

func NewUpstreamProfileRepository(db *gorm.DB) *UpstreamProfileRepository {
	return &UpstreamProfileRepository{db: db}
}

func (r *UpstreamProfileRepository) ListByDiscoveryConfigID(ctx context.Context, discoveryID uuid.UUID) ([]models.UpstreamProfile, error) {
	var profiles []models.UpstreamProfile
	err := r.db.WithContext(ctx).
		Where("discovery_config_id = ?", discoveryID).
		Order("name asc").
		Limit(200).
		Find(&profiles).Error
	return profiles, err
}

func (r *UpstreamProfileRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.UpstreamProfile, error) {
	var profile models.UpstreamProfile
	if err := r.db.WithContext(ctx).First(&profile, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &profile, nil
}

func (r *UpstreamProfileRepository) GetByIDs(ctx context.Context, ids []uuid.UUID) ([]models.UpstreamProfile, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var profiles []models.UpstreamProfile
	err := r.db.WithContext(ctx).Where("id IN ?", ids).Find(&profiles).Error
	return profiles, err
}

func (r *UpstreamProfileRepository) GetByNameAndDiscovery(ctx context.Context, discoveryID uuid.UUID, name string) (*models.UpstreamProfile, error) {
	var profile models.UpstreamProfile
	err := r.db.WithContext(ctx).
		Where("discovery_config_id = ? AND name = ?", discoveryID, name).
		First(&profile).Error
	if err != nil {
		return nil, err
	}
	return &profile, nil
}

func (r *UpstreamProfileRepository) Create(ctx context.Context, profile *models.UpstreamProfile) error {
	return r.db.WithContext(ctx).Create(profile).Error
}

func (r *UpstreamProfileRepository) Update(ctx context.Context, profile *models.UpstreamProfile) error {
	return r.db.WithContext(ctx).Save(profile).Error
}

func (r *UpstreamProfileRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Delete(&models.UpstreamProfile{}, "id = ?", id).Error
}
