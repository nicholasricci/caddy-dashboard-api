package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type CaddySnapshot struct {
	ID                uuid.UUID      `gorm:"type:char(36);primaryKey" json:"id"`
	NodeID            *uuid.UUID     `gorm:"type:char(36);index:idx_snapshot_node_applied_at" json:"node_id,omitempty"`
	DiscoveryConfigID *uuid.UUID     `gorm:"type:char(36);index:idx_snapshot_discovery_applied_at" json:"discovery_config_id,omitempty"`
	Config            datatypes.JSON `gorm:"type:json;not null" json:"config"`
	AppliedBy         string         `gorm:"size:120;not null" json:"applied_by"`
	AppliedAt         time.Time      `gorm:"not null;index:idx_snapshot_node_applied_at;index:idx_snapshot_discovery_applied_at" json:"applied_at"`
	CreatedAt         time.Time      `json:"created_at"`
	UpdatedAt         time.Time      `json:"updated_at"`
}

func (s *CaddySnapshot) BeforeCreate(_ *gorm.DB) error {
	if s.ID == uuid.Nil {
		s.ID = uuid.New()
	}
	return nil
}
