package services

import (
	"context"
	"encoding/json"

	"github.com/nicholasricci/caddy-dashboard/internal/models"
	"github.com/nicholasricci/caddy-dashboard/internal/repository"
	"gorm.io/datatypes"
)

type AuditService struct {
	repo *repository.AuditRepository
}

func NewAuditService(repo *repository.AuditRepository) *AuditService {
	return &AuditService{repo: repo}
}

func (s *AuditService) Record(ctx context.Context, actor, action, resource, resourceID string, payload any) error {
	var raw datatypes.JSON
	if payload != nil {
		b, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		raw = datatypes.JSON(b)
	}
	return s.repo.Create(ctx, &models.AuditLog{
		Actor:      actor,
		Action:     action,
		Resource:   resource,
		ResourceID: resourceID,
		Payload:    raw,
	})
}

func (s *AuditService) ListPaginated(ctx context.Context, limit, offset int) ([]models.AuditLog, int64, error) {
	return s.repo.ListPaginated(ctx, limit, offset)
}
