package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// UpstreamProfileBinding maps a Caddy @id to the port used when registering an instance.
type UpstreamProfileBinding struct {
	ConfigID string `json:"config_id"`
	Port     int    `json:"port"`
}

// UpstreamProfile is an admin-defined standard registration: N (config_id, port) pairs for one discovery group.
type UpstreamProfile struct {
	ID                uuid.UUID       `gorm:"type:char(36);primaryKey" json:"id"`
	DiscoveryConfigID uuid.UUID       `gorm:"type:char(36);not null;index" json:"discovery_config_id"`
	Name              string          `gorm:"size:120;not null" json:"name"`
	Description       string          `gorm:"size:512" json:"description,omitempty"`
	Bindings          json.RawMessage `gorm:"type:json;not null" json:"bindings" swaggertype:"array,object"`
	CreatedAt         time.Time       `json:"created_at"`
	UpdatedAt         time.Time       `json:"updated_at"`
	DeletedAt         gorm.DeletedAt  `gorm:"index" json:"-"`
}

func (p *UpstreamProfile) BeforeCreate(_ *gorm.DB) error {
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	return nil
}

// RegisterUpstreamProfileTarget is one dial applied during profile-based registration.
type RegisterUpstreamProfileTarget struct {
	ConfigID string `json:"config_id"`
	Dial     string `json:"dial"`
}

// RegisterUpstreamProfileResponse is the result of POST /upstream-profiles/:id/register.
type RegisterUpstreamProfileResponse struct {
	UpstreamProfileID string                          `json:"upstream_profile_id"`
	DiscoveryConfigID string                          `json:"discovery_config_id"`
	SourceNodeID      string                          `json:"source_node_id"`
	Targets           []RegisterUpstreamProfileTarget `json:"targets"`
	Changed           bool                            `json:"changed"`
	DryRun            bool                            `json:"dry_run"`
	Mutate            *MutateUpstreamsResponse        `json:"mutate,omitempty"`
	Propagate         *PropagateConfigResponse        `json:"propagate,omitempty"`
}
