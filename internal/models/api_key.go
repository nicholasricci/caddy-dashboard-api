package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	APIKeyScopeRegisterUpstream = "register_upstream"
	APIKeyScopeRegisterDomain   = "register_domain"
	APIKeyPrefix                = "cdk_live_"
)

// AllAPIKeyScopes lists scopes recognized by the API today (extensible).
var AllAPIKeyScopes = []string{
	APIKeyScopeRegisterUpstream,
	APIKeyScopeRegisterDomain,
}

type APIKey struct {
	ID                        uuid.UUID       `gorm:"type:char(36);primaryKey" json:"id"`
	Name                      string          `gorm:"size:120;not null" json:"name"`
	KeyPrefix                 string          `gorm:"size:32;not null;index" json:"key_prefix"`
	KeyHash                   string          `gorm:"size:64;not null;uniqueIndex" json:"-"`
	Scopes                    json.RawMessage `gorm:"type:json;not null" json:"scopes" swaggertype:"array,string"`
	AllowedDiscoveryConfigIDs json.RawMessage `gorm:"type:json;not null" json:"allowed_discovery_config_ids" swaggertype:"array,string"`
	AllowedUpstreamProfileIDs json.RawMessage `gorm:"type:json" json:"allowed_upstream_profile_ids,omitempty" swaggertype:"array,string"`
	AllowedDomainProfileIDs   json.RawMessage `gorm:"type:json" json:"allowed_domain_profile_ids,omitempty" swaggertype:"array,string"`
	ExpiresAt                 *time.Time      `json:"expires_at,omitempty"`
	RevokedAt                 *time.Time      `json:"revoked_at,omitempty"`
	LastUsedAt                *time.Time      `json:"last_used_at,omitempty"`
	CreatedAt                 time.Time       `json:"created_at"`
	UpdatedAt                 time.Time       `json:"updated_at"`
	DeletedAt                 gorm.DeletedAt  `gorm:"index" json:"-"`
}

func (k *APIKey) BeforeCreate(_ *gorm.DB) error {
	if k.ID == uuid.Nil {
		k.ID = uuid.New()
	}
	return nil
}

// APIKeyListResponse wraps GET /api/v1/api-keys.
type APIKeyListResponse struct {
	Items []APIKey `json:"items"`
}

// APIKeyCreateResponse is returned once when an API key is created (includes the secret).
type APIKeyCreateResponse struct {
	APIKey APIKey `json:"api_key"`
	Secret string `json:"secret"`
}

// RegisterUpstreamResponse is the result of POST /discovery/:id/register-upstream.
type RegisterUpstreamResponse struct {
	DiscoveryConfigID string                   `json:"discovery_config_id"`
	SourceNodeID      string                   `json:"source_node_id"`
	Dial              string                   `json:"dial"`
	Changed           bool                     `json:"changed"`
	DryRun            bool                     `json:"dry_run"`
	Mutate            *MutateUpstreamsResponse `json:"mutate,omitempty"`
	Propagate         *PropagateConfigResponse `json:"propagate,omitempty"`
}
