package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// DomainProfileBinding maps a Caddy @id and optional match indexes for domain registration.
type DomainProfileBinding struct {
	ConfigID     string `json:"config_id"`
	MatchIndexes []int  `json:"match_indexes,omitempty"`
}

// DomainProfile is an admin-defined standard for M2M domain registration on a discovery group.
type DomainProfile struct {
	ID                uuid.UUID       `gorm:"type:char(36);primaryKey" json:"id"`
	DiscoveryConfigID uuid.UUID       `gorm:"type:char(36);not null;index" json:"discovery_config_id"`
	Name              string          `gorm:"size:120;not null" json:"name"`
	Description       string          `gorm:"size:512" json:"description,omitempty"`
	Bindings          json.RawMessage `gorm:"type:json;not null" json:"bindings" swaggertype:"array,object"`
	CreatedAt         time.Time       `json:"created_at"`
	UpdatedAt         time.Time       `json:"updated_at"`
	DeletedAt         gorm.DeletedAt  `gorm:"index" json:"-"`
}

func (p *DomainProfile) BeforeCreate(_ *gorm.DB) error {
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	return nil
}

// RegisterDomainProfileTarget is one host mutation applied during profile-based registration.
type RegisterDomainProfileTarget struct {
	ConfigID     string   `json:"config_id"`
	MatchIndexes []int    `json:"match_indexes,omitempty"`
	Domains      []string `json:"domains"`
}

// RegisterDomainResponse is the result of POST /discovery/:id/register-domain.
type RegisterDomainResponse struct {
	DiscoveryConfigID string                   `json:"discovery_config_id"`
	SourceNodeID      string                   `json:"source_node_id"`
	Domains           []string                 `json:"domains"`
	Changed           bool                     `json:"changed"`
	DryRun            bool                     `json:"dry_run"`
	Mutate            *MutateDomainsResponse   `json:"mutate,omitempty"`
	Propagate         *PropagateConfigResponse `json:"propagate,omitempty"`
}

// RegisterDomainProfileResponse is the result of POST /domain-profiles/:id/register.
type RegisterDomainProfileResponse struct {
	DomainProfileID   string                        `json:"domain_profile_id"`
	DiscoveryConfigID string                        `json:"discovery_config_id"`
	SourceNodeID      string                        `json:"source_node_id"`
	Targets           []RegisterDomainProfileTarget `json:"targets"`
	Changed           bool                          `json:"changed"`
	DryRun            bool                          `json:"dry_run"`
	Mutate            *MutateDomainsResponse        `json:"mutate,omitempty"`
	Propagate         *PropagateConfigResponse      `json:"propagate,omitempty"`
}
