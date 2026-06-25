package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/nicholasricci/caddy-dashboard/internal/models"
	"gorm.io/gorm"
)

type ScheduledTaskLogRepository struct {
	db *gorm.DB
}

func NewScheduledTaskLogRepository(db *gorm.DB) *ScheduledTaskLogRepository {
	return &ScheduledTaskLogRepository{db: db}
}

func (r *ScheduledTaskLogRepository) Create(ctx context.Context, log *models.ScheduledTaskLog) error {
	return r.db.WithContext(ctx).Create(log).Error
}

func (r *ScheduledTaskLogRepository) Update(ctx context.Context, log *models.ScheduledTaskLog) error {
	return r.db.WithContext(ctx).Save(log).Error
}

func (r *ScheduledTaskLogRepository) ListByTaskIDPaginated(
	ctx context.Context,
	taskID uuid.UUID,
	filter models.ScheduledTaskLogListFilter,
	limit, offset int,
) ([]models.ScheduledTaskLog, int64, error) {
	var logs []models.ScheduledTaskLog
	var total int64

	q := r.db.WithContext(ctx).Model(&models.ScheduledTaskLog{}).Where("scheduled_task_id = ?", taskID)
	q = applyScheduledTaskLogListFilter(q, filter)

	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := q.Order("started_at DESC").Limit(limit).Offset(offset).Find(&logs).Error
	return logs, total, err
}

func applyScheduledTaskLogListFilter(q *gorm.DB, filter models.ScheduledTaskLogListFilter) *gorm.DB {
	if filter.Status != "" {
		q = q.Where("status = ?", filter.Status)
	}
	if filter.From != nil {
		q = q.Where("started_at >= ?", *filter.From)
	}
	if filter.To != nil {
		q = q.Where("started_at <= ?", *filter.To)
	}
	return q
}

func (r *ScheduledTaskLogRepository) DeleteOlderThan(ctx context.Context, days int) error {
	return r.db.WithContext(ctx).
		Where("created_at < NOW() - INTERVAL ? DAY", days).
		Delete(&models.ScheduledTaskLog{}).Error
}

func (r *ScheduledTaskLogRepository) DB() *gorm.DB {
	return r.db
}
