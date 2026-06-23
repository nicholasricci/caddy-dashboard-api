package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	ScheduledTaskStatusRunning = "running"
	ScheduledTaskStatusSuccess = "success"
	ScheduledTaskStatusFailed  = "failed"

	ScheduledTaskTypeDiscoveryRun        = "discovery_run"
	ScheduledTaskTypeTokenCleanup        = "token_cleanup"
	ScheduledTaskTypeNodeHealthcheck     = "node_healthcheck"
	ScheduledTaskTypeUpstreamHealthcheck = "upstream_healthcheck"
)

var AllScheduledTaskTypes = []string{
	ScheduledTaskTypeDiscoveryRun,
	ScheduledTaskTypeTokenCleanup,
	ScheduledTaskTypeNodeHealthcheck,
	ScheduledTaskTypeUpstreamHealthcheck,
}

type ScheduledTask struct {
	ID             uuid.UUID       `gorm:"type:char(36);primaryKey" json:"id"`
	Name           string          `gorm:"size:120;not null;uniqueIndex" json:"name"`
	Description    string          `gorm:"size:512" json:"description,omitempty"`
	TaskType       string          `gorm:"size:64;not null;index" json:"task_type"`
	CronExpression string          `gorm:"size:120;not null" json:"cron_expression"`
	Config         json.RawMessage `gorm:"type:json" json:"config,omitempty" swaggertype:"object"`
	Enabled        bool            `gorm:"not null;default:true" json:"enabled"`
	LastRunAt      *time.Time      `json:"last_run_at,omitempty"`
	LastStatus     string          `gorm:"size:32" json:"last_status,omitempty"`
	LastError      string          `gorm:"size:1024" json:"last_error,omitempty"`
	CreatedAt      time.Time       `json:"created_at"`
	UpdatedAt      time.Time       `json:"updated_at"`
	DeletedAt      gorm.DeletedAt  `gorm:"index" json:"-"`
}

func (t *ScheduledTask) BeforeCreate(_ *gorm.DB) error {
	if t.ID == uuid.Nil {
		t.ID = uuid.New()
	}
	return nil
}

type ScheduledTaskLog struct {
	ID              uuid.UUID       `gorm:"type:char(36);primaryKey" json:"id"`
	ScheduledTaskID uuid.UUID       `gorm:"index;not null" json:"scheduled_task_id"`
	StartedAt       time.Time       `gorm:"not null" json:"started_at"`
	FinishedAt      *time.Time      `json:"finished_at,omitempty"`
	Status          string          `gorm:"size:32;not null" json:"status"`
	Error           string          `gorm:"size:2048" json:"error,omitempty"`
	Details         json.RawMessage `gorm:"type:json" json:"details,omitempty" swaggertype:"object"`
	CreatedAt       time.Time       `json:"created_at"`
}

func (l *ScheduledTaskLog) BeforeCreate(_ *gorm.DB) error {
	if l.ID == uuid.Nil {
		l.ID = uuid.New()
	}
	return nil
}

type ListScheduledTasksResponse struct {
	Items []ScheduledTask `json:"items"`
}

type ListScheduledTaskLogsResponse struct {
	Items []ScheduledTaskLog `json:"items"`
}

type CreateScheduledTaskInput struct {
	Name           string          `json:"name" binding:"required"`
	Description    string          `json:"description"`
	TaskType       string          `json:"task_type" binding:"required"`
	CronExpression string          `json:"cron_expression" binding:"required"`
	Config         json.RawMessage `json:"config,omitempty" swaggertype:"object"`
	Enabled        *bool           `json:"enabled"`
}

type UpdateScheduledTaskInput struct {
	Name           *string         `json:"name"`
	Description    *string         `json:"description"`
	TaskType       *string         `json:"task_type"`
	CronExpression *string         `json:"cron_expression"`
	Config         json.RawMessage `json:"config,omitempty" swaggertype:"object"`
	Enabled        *bool           `json:"enabled"`
}
