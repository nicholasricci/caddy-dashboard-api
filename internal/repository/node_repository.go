package repository

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
	"github.com/nicholasricci/caddy-dashboard/internal/models"
	"gorm.io/gorm"
)

type NodeRepository struct {
	db *gorm.DB
}

func NewNodeRepository(db *gorm.DB) *NodeRepository {
	return &NodeRepository{db: db}
}

func (r *NodeRepository) List(ctx context.Context) ([]models.CaddyNode, error) {
	var nodes []models.CaddyNode
	err := r.db.WithContext(ctx).Order("created_at desc").Limit(100).Find(&nodes).Error
	return nodes, err
}

func (r *NodeRepository) ListPaginated(ctx context.Context, limit, offset int) ([]models.CaddyNode, int64, error) {
	var nodes []models.CaddyNode
	var total int64
	q := r.db.WithContext(ctx).Model(&models.CaddyNode{})
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := q.Order("created_at desc").Limit(limit).Offset(offset).Find(&nodes).Error
	return nodes, total, err
}

func (r *NodeRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.CaddyNode, error) {
	var node models.CaddyNode
	if err := r.db.WithContext(ctx).First(&node, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &node, nil
}

func (r *NodeRepository) Create(ctx context.Context, node *models.CaddyNode) error {
	return r.db.WithContext(ctx).Create(node).Error
}

func (r *NodeRepository) Update(ctx context.Context, node *models.CaddyNode) error {
	return r.db.WithContext(ctx).Save(node).Error
}

func (r *NodeRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Delete(&models.CaddyNode{}, "id = ?", id).Error
}

func (r *NodeRepository) UpsertByInstanceOrIP(ctx context.Context, node *models.CaddyNode) error {
	db := r.db.WithContext(ctx)
	if node.InstanceID != nil && *node.InstanceID != "" {
		var existing models.CaddyNode
		err := db.Where("instance_id = ?", *node.InstanceID).First(&existing).Error
		if err == nil {
			return r.updateExistingNode(db, &existing, node)
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
	}
	if node.PrivateIP != nil && *node.PrivateIP != "" {
		var existing models.CaddyNode
		err := db.Where("private_ip = ? AND region = ?", *node.PrivateIP, node.Region).First(&existing).Error
		if err == nil {
			return r.updateExistingNode(db, &existing, node)
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
	}
	return r.Create(ctx, node)
}

func (r *NodeRepository) updateExistingNode(db *gorm.DB, existing, incoming *models.CaddyNode) error {
	incoming.ID = existing.ID
	updates := map[string]any{
		"ssm_enabled": incoming.SSMEnabled,
		"region":      incoming.Region,
	}
	if strings.TrimSpace(incoming.Name) != "" {
		updates["name"] = incoming.Name
	}
	if incoming.InstanceID != nil && *incoming.InstanceID != "" {
		updates["instance_id"] = incoming.InstanceID
	}
	if incoming.PrivateIP != nil && *incoming.PrivateIP != "" {
		updates["private_ip"] = incoming.PrivateIP
	}
	if strings.TrimSpace(incoming.Status) != "" {
		updates["status"] = incoming.Status
	}
	if incoming.LastSeenAt != nil {
		updates["last_seen_at"] = incoming.LastSeenAt
	}
	if existing.DiscoveryConfigID == nil && incoming.DiscoveryConfigID != nil {
		updates["discovery_config_id"] = incoming.DiscoveryConfigID
	}
	return db.Model(existing).Updates(updates).Error
}
