package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/nicholasricci/caddy-dashboard/internal/models"
	"gorm.io/gorm"
)

type DomainProfileRepository struct {
	db *gorm.DB
}

func NewDomainProfileRepository(db *gorm.DB) *DomainProfileRepository {
	return &DomainProfileRepository{db: db}
}

func (r *DomainProfileRepository) ListByDiscoveryConfigID(ctx context.Context, discoveryID uuid.UUID) ([]models.DomainProfile, error) {
	var profiles []models.DomainProfile
	err := r.db.WithContext(ctx).
		Where("discovery_config_id = ?", discoveryID).
		Order("name asc").
		Limit(200).
		Find(&profiles).Error
	return profiles, err
}

func (r *DomainProfileRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.DomainProfile, error) {
	var profile models.DomainProfile
	if err := r.db.WithContext(ctx).First(&profile, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &profile, nil
}

func (r *DomainProfileRepository) GetByIDs(ctx context.Context, ids []uuid.UUID) ([]models.DomainProfile, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var profiles []models.DomainProfile
	err := r.db.WithContext(ctx).Where("id IN ?", ids).Find(&profiles).Error
	return profiles, err
}

func (r *DomainProfileRepository) GetByNameAndDiscovery(ctx context.Context, discoveryID uuid.UUID, name string) (*models.DomainProfile, error) {
	var profile models.DomainProfile
	err := r.db.WithContext(ctx).
		Where("discovery_config_id = ? AND name = ?", discoveryID, name).
		First(&profile).Error
	if err != nil {
		return nil, err
	}
	return &profile, nil
}

func (r *DomainProfileRepository) Create(ctx context.Context, profile *models.DomainProfile) error {
	return r.db.WithContext(ctx).Create(profile).Error
}

func (r *DomainProfileRepository) Update(ctx context.Context, profile *models.DomainProfile) error {
	return r.db.WithContext(ctx).Save(profile).Error
}

func (r *DomainProfileRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Delete(&models.DomainProfile{}, "id = ?", id).Error
}
