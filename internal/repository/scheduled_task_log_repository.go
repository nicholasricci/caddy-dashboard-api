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

func (r *ScheduledTaskLogRepository) ListByTaskID(ctx context.Context, taskID uuid.UUID) ([]models.ScheduledTaskLog, error) {
	var logs []models.ScheduledTaskLog
	err := r.db.WithContext(ctx).Where("scheduled_task_id = ?", taskID).Order("created_at DESC").Find(&logs).Error
	return logs, err
}

func (r *ScheduledTaskLogRepository) DeleteOlderThan(ctx context.Context, days int) error {
	return r.db.WithContext(ctx).
		Where("created_at < NOW() - INTERVAL ? DAY", days).
		Delete(&models.ScheduledTaskLog{}).Error
}

func (r *ScheduledTaskLogRepository) DB() *gorm.DB {
	return r.db
}
