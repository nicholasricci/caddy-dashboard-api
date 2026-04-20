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

// BackfillDiscoveryConfigIDs fills missing discovery_config_id on legacy snapshots when:
// - snapshot is linked to a node;
// - node has discovery_config_id;
// - related discovery config uses group snapshot scope.
// The statement is idempotent and can be safely re-run. Soft-deleted configs/nodes are skipped.
// The function is a no-op when the discovery_config_id column does not yet exist on caddy_snapshots,
// so a binary deployed before the schema migration does not fail at startup.
// MySQL/MariaDB-specific multi-table UPDATE syntax.
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
	stmt := `
UPDATE caddy_snapshots s
JOIN caddy_nodes n ON s.node_id = n.id
JOIN discovery_configs d ON n.discovery_config_id = d.id
SET s.discovery_config_id = n.discovery_config_id
WHERE s.discovery_config_id IS NULL
  AND s.node_id IS NOT NULL
  AND n.discovery_config_id IS NOT NULL
  AND n.deleted_at IS NULL
  AND d.deleted_at IS NULL
  AND d.snapshot_scope = ?`
	res := r.db.WithContext(ctx).Exec(stmt, models.SnapshotScopeGroup)
	return res.RowsAffected, res.Error
}
