package repository

import (
	"context"
	"strings"

	"github.com/nicholasricci/caddy-dashboard/internal/models"
	"gorm.io/gorm"
)

const actorLikeEscape = '!'

type AuditRepository struct {
	db *gorm.DB
}

func NewAuditRepository(db *gorm.DB) *AuditRepository {
	return &AuditRepository{db: db}
}

func (r *AuditRepository) Create(ctx context.Context, log *models.AuditLog) error {
	return r.db.WithContext(ctx).Create(log).Error
}

func (r *AuditRepository) ListPaginated(ctx context.Context, filter models.AuditListFilter, limit, offset int) ([]models.AuditLog, int64, error) {
	var out []models.AuditLog
	var total int64
	q := r.db.WithContext(ctx).Model(&models.AuditLog{})
	q = applyAuditListFilter(q, filter)
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := q.Order("created_at desc").Limit(limit).Offset(offset).Find(&out).Error
	return out, total, err
}

func applyAuditListFilter(q *gorm.DB, filter models.AuditListFilter) *gorm.DB {
	if filter.Action != "" {
		q = q.Where("action = ?", filter.Action)
	}
	if filter.Resource != "" {
		q = q.Where("resource = ?", filter.Resource)
	}
	if filter.Actor != "" {
		q = applyActorFilter(q, filter.Actor)
	}
	if filter.ResourceID != "" {
		q = q.Where("resource_id = ?", filter.ResourceID)
	}
	if filter.From != nil {
		q = q.Where("created_at >= ?", *filter.From)
	}
	if filter.To != nil {
		q = q.Where("created_at <= ?", *filter.To)
	}
	return q
}

func applyActorFilter(q *gorm.DB, actor string) *gorm.DB {
	if !strings.Contains(actor, "*") {
		return q.Where("actor = ?", actor)
	}
	return q.Where("actor LIKE ? ESCAPE '!'", actorPatternToLike(actor))
}

func actorPatternToLike(pattern string) string {
	var b strings.Builder
	b.Grow(len(pattern))
	for _, r := range pattern {
		switch r {
		case '*':
			b.WriteByte('%')
		case actorLikeEscape, '%', '_':
			b.WriteByte(actorLikeEscape)
			b.WriteRune(r)
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}
