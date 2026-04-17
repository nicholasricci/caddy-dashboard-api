package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type AuditLog struct {
	ID         uuid.UUID      `gorm:"type:char(36);primaryKey" json:"id"`
	Actor      string         `gorm:"size:120;index;not null" json:"actor"`
	Action     string         `gorm:"size:120;index;not null" json:"action"`
	Resource   string         `gorm:"size:64;index;not null" json:"resource"`
	ResourceID string         `gorm:"size:64;index" json:"resource_id,omitempty"`
	Payload    datatypes.JSON `gorm:"type:json" json:"payload,omitempty"`
	CreatedAt  time.Time      `gorm:"index" json:"created_at"`
	DeletedAt  gorm.DeletedAt `gorm:"index" json:"-"`
}

func (a *AuditLog) BeforeCreate(_ *gorm.DB) error {
	if a.ID == uuid.Nil {
		a.ID = uuid.New()
	}
	return nil
}
