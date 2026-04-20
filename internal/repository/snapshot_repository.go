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
		Limit(100).
		Find(&snapshots).Error
	return snapshots, err
}

func (r *SnapshotRepository) ListByNodeIDPaginated(ctx context.Context, nodeID uuid.UUID, limit, offset int) ([]models.CaddySnapshot, int64, error) {
	var snapshots []models.CaddySnapshot
	var total int64
	q := r.db.WithContext(ctx).Model(&models.CaddySnapshot{}).Where("node_id = ?", nodeID)
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := q.Order("applied_at desc").Limit(limit).Offset(offset).Find(&snapshots).Error
	return snapshots, total, err
}

func (r *SnapshotRepository) ListByDiscoveryConfigID(ctx context.Context, discoveryConfigID uuid.UUID) ([]models.CaddySnapshot, error) {
	var snapshots []models.CaddySnapshot
	err := r.db.WithContext(ctx).
		Where("discovery_config_id = ? AND node_id IS NULL", discoveryConfigID).
		Order("applied_at desc").
		Limit(100).
		Find(&snapshots).Error
	return snapshots, err
}

func (r *SnapshotRepository) ListByDiscoveryConfigIDPaginated(ctx context.Context, discoveryConfigID uuid.UUID, limit, offset int) ([]models.CaddySnapshot, int64, error) {
	var snapshots []models.CaddySnapshot
	var total int64
	q := r.db.WithContext(ctx).Model(&models.CaddySnapshot{}).
		Where("discovery_config_id = ? AND node_id IS NULL", discoveryConfigID)
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := q.Order("applied_at desc").Limit(limit).Offset(offset).Find(&snapshots).Error
	return snapshots, total, err
}

// BackfillDiscoveryConfigIDs fills missing discovery_config_id on legacy snapshots using
// DBMS-agnostic GORM queries (pluck group configs, pluck nodes per config, update
// snapshots in batch per config). The operation is idempotent and safe to re-run.
// Soft-deleted configs/nodes are excluded by GORM's DeletedAt filter.
//
// The function is a no-op when required columns do not exist yet, so a binary
// deployed before schema migration does not fail at startup.
//
// Note: this is no longer a single atomic SQL statement. The function wraps all
// operations in a transaction for a consistent read snapshot. If a discovery
// config changes snapshot_scope while this runs, affected rows may still be
// backfilled with the previously observed scope. Re-running remains safe.
func (r *SnapshotRepository) BackfillDiscoveryConfigIDs(ctx context.Context) (int64, error) {
	if !r.db.Migrator().HasColumn(&models.CaddySnapshot{}, "discovery_config_id") {
		return 0, nil
	}
	if !r.db.Migrator().HasColumn(&models.CaddyNode{}, "discovery_config_id") {
		return 0, nil
	}
	if !r.db.Migrator().HasColumn(&models.DiscoveryConfig{}, "snapshot_scope") {
		return 0, nil
	}

	var totalUpdated int64
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var groupCfgIDs []uuid.UUID
		if err := tx.Model(&models.DiscoveryConfig{}).
			Where("snapshot_scope = ?", models.SnapshotScopeGroup).
			Pluck("id", &groupCfgIDs).Error; err != nil {
			return err
		}

		for _, cfgID := range groupCfgIDs {
			var nodeIDs []uuid.UUID
			if err := tx.Model(&models.CaddyNode{}).
				Where("discovery_config_id = ?", cfgID).
				Pluck("id", &nodeIDs).Error; err != nil {
				return err
			}
			// Keep this guard: some drivers produce invalid SQL for empty IN clauses.
			if len(nodeIDs) == 0 {
				continue
			}

			res := tx.Model(&models.CaddySnapshot{}).
				Where("discovery_config_id IS NULL AND node_id IN ?", nodeIDs).
				Update("discovery_config_id", cfgID)
			if res.Error != nil {
				return res.Error
			}
			totalUpdated += res.RowsAffected
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return totalUpdated, nil
}
