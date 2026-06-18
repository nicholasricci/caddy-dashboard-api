package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/nicholasricci/caddy-dashboard/internal/models"
	"gorm.io/gorm"
)

type APIKeyRepository struct {
	db *gorm.DB
}

func NewAPIKeyRepository(db *gorm.DB) *APIKeyRepository {
	return &APIKeyRepository{db: db}
}

func (r *APIKeyRepository) DB() *gorm.DB {
	return r.db
}

func (r *APIKeyRepository) List(ctx context.Context) ([]models.APIKey, error) {
	var keys []models.APIKey
	err := r.db.WithContext(ctx).Order("name asc").Limit(200).Find(&keys).Error
	return keys, err
}

func (r *APIKeyRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.APIKey, error) {
	var key models.APIKey
	if err := r.db.WithContext(ctx).First(&key, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &key, nil
}

func (r *APIKeyRepository) GetByKeyHash(ctx context.Context, keyHash string) (*models.APIKey, error) {
	var key models.APIKey
	if err := r.db.WithContext(ctx).First(&key, "key_hash = ?", keyHash).Error; err != nil {
		return nil, err
	}
	return &key, nil
}

func (r *APIKeyRepository) Create(ctx context.Context, key *models.APIKey) error {
	return r.db.WithContext(ctx).Create(key).Error
}

func (r *APIKeyRepository) Update(ctx context.Context, key *models.APIKey) error {
	return r.db.WithContext(ctx).Save(key).Error
}

func (r *APIKeyRepository) TouchLastUsed(ctx context.Context, id uuid.UUID, at time.Time) error {
	return r.db.WithContext(ctx).Model(&models.APIKey{}).Where("id = ?", id).Update("last_used_at", at).Error
}

func (r *APIKeyRepository) Revoke(ctx context.Context, id uuid.UUID, at time.Time) error {
	return r.db.WithContext(ctx).Model(&models.APIKey{}).Where("id = ?", id).Update("revoked_at", at).Error
}

func (r *APIKeyRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Delete(&models.APIKey{}, "id = ?", id).Error
}
