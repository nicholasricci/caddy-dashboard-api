package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type CaddyNode struct {
	ID                uuid.UUID       `gorm:"type:char(36);primaryKey" json:"id"`
	Name              string          `gorm:"size:120;not null" json:"name"`
	InstanceID        *string         `gorm:"size:64;index" json:"instance_id,omitempty"`
	PrivateIP         *string         `gorm:"size:64;index" json:"private_ip,omitempty"`
	Region            *string         `gorm:"size:32;index" json:"region,omitempty"`
	Transport         string          `gorm:"size:32;not null;default:aws_ssm;index" json:"transport"`
	TransportConfig   json.RawMessage `gorm:"type:json" json:"transport_config,omitempty" swaggertype:"object"`
	DiscoveryConfigID *uuid.UUID      `gorm:"type:char(36);index" json:"discovery_config_id,omitempty"`
	// Deprecated: use Transport == TransportAWSSSM instead; kept for API compatibility.
	SSMEnabled bool           `gorm:"not null;default:true" json:"ssm_enabled"`
	Status     string         `gorm:"size:32;not null;default:'unknown'" json:"status"`
	LastSeenAt *time.Time     `json:"last_seen_at,omitempty"`
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
	DeletedAt  gorm.DeletedAt `gorm:"index" json:"-"`
}

func (n *CaddyNode) BeforeCreate(_ *gorm.DB) error {
	if n.ID == uuid.Nil {
		n.ID = uuid.New()
	}
	if n.Transport == "" {
		n.Transport = TransportAWSSSM
	}
	n.syncLegacySSMEnabled()
	return nil
}

func (n *CaddyNode) BeforeUpdate(_ *gorm.DB) error {
	n.syncLegacySSMEnabled()
	return nil
}

func (n *CaddyNode) syncLegacySSMEnabled() {
	n.SSMEnabled = n.EffectiveTransport() == TransportAWSSSM
}
