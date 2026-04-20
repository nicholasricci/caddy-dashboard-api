package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type CaddyNode struct {
	ID                uuid.UUID      `gorm:"type:char(36);primaryKey" json:"id"`
	Name              string         `gorm:"size:120;not null" json:"name"`
	InstanceID        *string        `gorm:"size:64;index" json:"instance_id,omitempty"`
	PrivateIP         *string        `gorm:"size:64;index" json:"private_ip,omitempty"`
	Region            string         `gorm:"size:32;index;not null" json:"region"`
	DiscoveryConfigID *uuid.UUID     `gorm:"type:char(36);index" json:"discovery_config_id,omitempty"`
	SSMEnabled        bool           `gorm:"not null;default:true" json:"ssm_enabled"`
	Status            string         `gorm:"size:32;not null;default:'unknown'" json:"status"`
	LastSeenAt        *time.Time     `json:"last_seen_at,omitempty"`
	CreatedAt         time.Time      `json:"created_at"`
	UpdatedAt         time.Time      `json:"updated_at"`
	DeletedAt         gorm.DeletedAt `gorm:"index" json:"-"`
}

func (n *CaddyNode) BeforeCreate(_ *gorm.DB) error {
	if n.ID == uuid.Nil {
		n.ID = uuid.New()
	}
	return nil
}
