package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/nicholasricci/caddy-dashboard/internal/models"
	"gorm.io/gorm"
)

type SnapshotRepository struct {
	db *gorm.DB
}

func NewSnapshotRepository(db *gorm.DB) *SnapshotRepository {
	return &SnapshotRepository{db: db}
}

func (r *SnapshotRepository) Create(ctx context.Context, s *models.CaddySnapshot) error {
	return r.db.WithContext(ctx).Create(s).Error
}

func (r *SnapshotRepository) ListByNodeID(ctx context.Context, nodeID uuid.UUID) ([]models.CaddySnapshot, error) {
	var snapshots []models.CaddySnapshot
	err := r.db.WithContext(ctx).
		Where("node_id = ?", nodeID).
		Order("applied_at desc").
		Find(&snapshots).Error
	return snapshots, err
}
