package models

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	DiscoveryMethodAWSTag   = "aws_tag"
	DiscoveryMethodAWSCIDR  = "aws_cidr"
	DiscoveryMethodAWSSSM   = "aws_ssm"
	DiscoveryMethodStaticIP = "static_ip"
)

type SnapshotScope string

const (
	SnapshotScopeNode  SnapshotScope = "node"
	SnapshotScopeGroup SnapshotScope = "group"
)

func (s SnapshotScope) String() string {
	return string(s)
}

func (s SnapshotScope) IsValid() bool {
	return s == SnapshotScopeNode || s == SnapshotScopeGroup
}

func ParseSnapshotScope(raw string) (SnapshotScope, error) {
	scope := SnapshotScope(strings.ToLower(strings.TrimSpace(raw)))
	if scope == "" {
		return SnapshotScopeNode, nil
	}
	if !scope.IsValid() {
		return "", fmt.Errorf("invalid snapshot_scope %q (allowed: node, group)", raw)
	}
	return scope, nil
}

type DiscoveryConfig struct {
	ID            uuid.UUID       `gorm:"type:char(36);primaryKey" json:"id"`
	Name          string          `gorm:"size:120;not null" json:"name"`
	Method        string          `gorm:"size:32;not null;default:aws_tag;index" json:"method"`
	Region        string          `gorm:"size:32;not null;index" json:"region"`
	TagKey        string          `gorm:"size:120;not null" json:"tag_key"`
	TagValue      string          `gorm:"size:255;not null" json:"tag_value"`
	Parameters    json.RawMessage `gorm:"type:json" json:"parameters,omitempty" swaggertype:"object"`
	SnapshotScope SnapshotScope   `gorm:"size:16;not null;default:node" json:"snapshot_scope" enums:"node,group" example:"node"`
	Enabled       bool            `gorm:"not null;default:true" json:"enabled"`
	CreatedAt     time.Time       `json:"created_at"`
	UpdatedAt     time.Time       `json:"updated_at"`
	DeletedAt     gorm.DeletedAt  `gorm:"index" json:"-"`
}

func (d *DiscoveryConfig) BeforeCreate(_ *gorm.DB) error {
	if d.ID == uuid.Nil {
		d.ID = uuid.New()
	}
	if d.SnapshotScope == "" {
		d.SnapshotScope = SnapshotScopeNode
	}
	return nil
}
