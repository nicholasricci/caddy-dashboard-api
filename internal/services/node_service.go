package services

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/nicholasricci/caddy-dashboard/internal/models"
	"github.com/nicholasricci/caddy-dashboard/internal/repository"
)

// nodeRepository is satisfied by *repository.NodeRepository; narrowed for tests.
type nodeRepository interface {
	List(ctx context.Context) ([]models.CaddyNode, error)
	ListPaginated(ctx context.Context, limit, offset int) ([]models.CaddyNode, int64, error)
	GetByID(ctx context.Context, id uuid.UUID) (*models.CaddyNode, error)
	Create(ctx context.Context, node *models.CaddyNode) error
	Update(ctx context.Context, node *models.CaddyNode) error
	Delete(ctx context.Context, id uuid.UUID) error
}

type NodeService struct {
	repo nodeRepository
}

func NewNodeService(repo nodeRepository) *NodeService {
	return &NodeService{repo: repo}
}

func (s *NodeService) List(ctx context.Context) ([]models.CaddyNode, error) {
	return s.repo.List(ctx)
}

func (s *NodeService) ListPaginated(ctx context.Context, limit, offset int) ([]models.CaddyNode, int64, error) {
	return s.repo.ListPaginated(ctx, limit, offset)
}

func (s *NodeService) Create(ctx context.Context, node *models.CaddyNode) error {
	if node.Status == "" {
		node.Status = "manual"
	}
	if node.LastSeenAt == nil {
		now := time.Now().UTC()
		node.LastSeenAt = &now
	}
	return s.repo.Create(ctx, node)
}

func (s *NodeService) Get(ctx context.Context, id uuid.UUID) (*models.CaddyNode, error) {
	node, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if repository.IsNotFound(err) {
			return nil, models.ErrNodeNotFound
		}
		return nil, err
	}
	return node, nil
}

func (s *NodeService) Update(ctx context.Context, node *models.CaddyNode) error {
	return s.repo.Update(ctx, node)
}

func (s *NodeService) Delete(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}
