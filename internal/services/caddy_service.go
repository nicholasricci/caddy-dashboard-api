package services

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	caddysvc "github.com/nicholasricci/caddy-dashboard/internal/caddy"
	"github.com/nicholasricci/caddy-dashboard/internal/models"
	"github.com/nicholasricci/caddy-dashboard/internal/repository"
)

type CaddyService struct {
	caddySvc    *caddysvc.Service
	nodes       *repository.NodeRepository
	discoveries *repository.DiscoveryRepository
	snapshots   *repository.SnapshotRepository
}

func NewCaddyService(
	caddySvc *caddysvc.Service,
	nodes *repository.NodeRepository,
	discoveries *repository.DiscoveryRepository,
	snapshots *repository.SnapshotRepository,
) *CaddyService {
	return &CaddyService{
		caddySvc:    caddySvc,
		nodes:       nodes,
		discoveries: discoveries,
		snapshots:   snapshots,
	}
}

func (s *CaddyService) Sync(ctx context.Context, nodeID uuid.UUID, requestedBy string) error {
	return s.caddySvc.SyncNodeConfig(ctx, nodeID, requestedBy)
}

func (s *CaddyService) GetLiveConfig(ctx context.Context, nodeID uuid.UUID) (json.RawMessage, error) {
	return s.caddySvc.GetLiveConfig(ctx, nodeID)
}

func (s *CaddyService) Apply(ctx context.Context, nodeID uuid.UUID, payload json.RawMessage, requestedBy string) error {
	return s.caddySvc.ApplyConfig(ctx, nodeID, payload, requestedBy)
}

func (s *CaddyService) Reload(ctx context.Context, nodeID uuid.UUID) error {
	return s.caddySvc.Reload(ctx, nodeID)
}

func (s *CaddyService) ListSnapshots(ctx context.Context, nodeID uuid.UUID) ([]models.CaddySnapshot, error) {
	node, err := s.nodes.GetByID(ctx, nodeID)
	if err != nil {
		if repository.IsNotFound(err) {
			return nil, models.ErrNodeNotFound
		}
		return nil, err
	}
	if node.DiscoveryConfigID != nil {
		cfg, err := s.discoveries.GetByID(ctx, *node.DiscoveryConfigID)
		switch {
		case err == nil && cfg.SnapshotScope == models.SnapshotScopeGroup:
			return s.snapshots.ListByDiscoveryConfigID(ctx, cfg.ID)
		case err != nil && !repository.IsNotFound(err):
			return nil, err
		}
	}
	return s.snapshots.ListByNodeID(ctx, nodeID)
}

func (s *CaddyService) ListSnapshotsPaginated(ctx context.Context, nodeID uuid.UUID, limit, offset int) ([]models.CaddySnapshot, int64, error) {
	node, err := s.nodes.GetByID(ctx, nodeID)
	if err != nil {
		if repository.IsNotFound(err) {
			return nil, 0, models.ErrNodeNotFound
		}
		return nil, 0, err
	}
	if node.DiscoveryConfigID != nil {
		cfg, err := s.discoveries.GetByID(ctx, *node.DiscoveryConfigID)
		switch {
		case err == nil && cfg.SnapshotScope == models.SnapshotScopeGroup:
			return s.snapshots.ListByDiscoveryConfigIDPaginated(ctx, cfg.ID, limit, offset)
		case err != nil && !repository.IsNotFound(err):
			return nil, 0, err
		}
	}
	return s.snapshots.ListByNodeIDPaginated(ctx, nodeID, limit, offset)
}

func (s *CaddyService) ListDiscoverySnapshotsPaginated(ctx context.Context, discoveryConfigID uuid.UUID, limit, offset int) ([]models.CaddySnapshot, int64, error) {
	return s.snapshots.ListByDiscoveryConfigIDPaginated(ctx, discoveryConfigID, limit, offset)
}
