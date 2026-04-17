package repository

import (
	"context"

	"github.com/nicholasricci/caddy-dashboard/internal/models"
	"gorm.io/gorm"
)

type AuditRepository struct {
	db *gorm.DB
}

func NewAuditRepository(db *gorm.DB) *AuditRepository {
	return &AuditRepository{db: db}
}

func (r *AuditRepository) Create(ctx context.Context, log *models.AuditLog) error {
	return r.db.WithContext(ctx).Create(log).Error
}

func (r *AuditRepository) ListPaginated(ctx context.Context, limit, offset int) ([]models.AuditLog, int64, error) {
	var out []models.AuditLog
	var total int64
	q := r.db.WithContext(ctx).Model(&models.AuditLog{})
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := q.Order("created_at desc").Limit(limit).Offset(offset).Find(&out).Error
	return out, total, err
}
