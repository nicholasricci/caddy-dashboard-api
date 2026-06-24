package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/nicholasricci/caddy-dashboard/internal/models"
	"gorm.io/gorm"
)

type ScheduledTaskRepository struct {
	db *gorm.DB
}

func NewScheduledTaskRepository(db *gorm.DB) *ScheduledTaskRepository {
	return &ScheduledTaskRepository{db: db}
}

func (r *ScheduledTaskRepository) List(ctx context.Context) ([]models.ScheduledTask, error) {
	var tasks []models.ScheduledTask
	err := r.db.WithContext(ctx).Order("name ASC").Find(&tasks).Error
	return tasks, err
}

func (r *ScheduledTaskRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.ScheduledTask, error) {
	var task models.ScheduledTask
	err := r.db.WithContext(ctx).First(&task, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &task, nil
}

func (r *ScheduledTaskRepository) ListEnabled(ctx context.Context) ([]models.ScheduledTask, error) {
	var tasks []models.ScheduledTask
	err := r.db.WithContext(ctx).Where("enabled = ?", true).Order("name ASC").Find(&tasks).Error
	return tasks, err
}

func (r *ScheduledTaskRepository) Create(ctx context.Context, task *models.ScheduledTask) error {
	return r.db.WithContext(ctx).Create(task).Error
}

func (r *ScheduledTaskRepository) Update(ctx context.Context, task *models.ScheduledTask) error {
	return r.db.WithContext(ctx).Save(task).Error
}

func (r *ScheduledTaskRepository) UpdateLastRun(ctx context.Context, id uuid.UUID, at time.Time, status, errMsg string) error {
	return r.db.WithContext(ctx).Model(&models.ScheduledTask{}).Where("id = ?", id).Updates(map[string]any{
		"last_run_at": at,
		"last_status": status,
		"last_error":  errMsg,
	}).Error
}

func (r *ScheduledTaskRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Delete(&models.ScheduledTask{}, "id = ?", id).Error
}

func (r *ScheduledTaskRepository) Toggle(ctx context.Context, id uuid.UUID, enabled bool) error {
	return r.db.WithContext(ctx).Model(&models.ScheduledTask{}).Where("id = ?", id).Update("enabled", enabled).Error
}

func (r *ScheduledTaskRepository) DB() *gorm.DB {
	return r.db
}
