package models

import (
	"encoding/json"
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

type DiscoveryConfig struct {
	ID         uuid.UUID       `gorm:"type:char(36);primaryKey" json:"id"`
	Name       string          `gorm:"size:120;not null" json:"name"`
	Method     string          `gorm:"size:32;not null;default:aws_tag;index" json:"method"`
	Region     string          `gorm:"size:32;not null;index" json:"region"`
	TagKey     string          `gorm:"size:120;not null" json:"tag_key"`
	TagValue   string          `gorm:"size:255;not null" json:"tag_value"`
	Parameters json.RawMessage `gorm:"type:json" json:"parameters,omitempty" swaggertype:"object"`
	Enabled    bool            `gorm:"not null;default:true" json:"enabled"`
	CreatedAt  time.Time       `json:"created_at"`
	UpdatedAt  time.Time       `json:"updated_at"`
	DeletedAt  gorm.DeletedAt  `gorm:"index" json:"-"`
}

func (d *DiscoveryConfig) BeforeCreate(_ *gorm.DB) error {
	if d.ID == uuid.Nil {
		d.ID = uuid.New()
	}
	return nil
}
