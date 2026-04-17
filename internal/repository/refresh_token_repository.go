package repository

import (
	"context"
	"time"

	"github.com/nicholasricci/caddy-dashboard/internal/models"
	"gorm.io/gorm"
)

type RefreshTokenRepository struct {
	db *gorm.DB
}

func NewRefreshTokenRepository(db *gorm.DB) *RefreshTokenRepository {
	return &RefreshTokenRepository{db: db}
}

func (r *RefreshTokenRepository) Create(ctx context.Context, token *models.RefreshToken) error {
	return r.db.WithContext(ctx).Create(token).Error
}

func (r *RefreshTokenRepository) GetActiveByHash(ctx context.Context, hash string) (*models.RefreshToken, error) {
	var out models.RefreshToken
	err := r.db.WithContext(ctx).
		Where("token_hash = ? AND revoked_at IS NULL AND expires_at > ?", hash, time.Now().UTC()).
		First(&out).Error
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func (r *RefreshTokenRepository) RevokeByHash(ctx context.Context, hash string) error {
	now := time.Now().UTC()
	return r.db.WithContext(ctx).Model(&models.RefreshToken{}).
		Where("token_hash = ? AND revoked_at IS NULL", hash).
		Update("revoked_at", &now).Error
}

func (r *RefreshTokenRepository) CleanupExpired(ctx context.Context) error {
	return r.db.WithContext(ctx).
		Where("expires_at <= ?", time.Now().UTC()).
		Delete(&models.RefreshToken{}).Error
}
