package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type RefreshToken struct {
	ID        uuid.UUID      `gorm:"type:char(36);primaryKey" json:"id"`
	UserID    uuid.UUID      `gorm:"type:char(36);index;not null" json:"user_id"`
	TokenHash string         `gorm:"size:64;index;not null" json:"-"`
	ExpiresAt time.Time      `gorm:"index;not null" json:"expires_at"`
	RevokedAt *time.Time     `gorm:"index" json:"revoked_at,omitempty"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

func (r *RefreshToken) BeforeCreate(_ *gorm.DB) error {
	if r.ID == uuid.Nil {
		r.ID = uuid.New()
	}
	return nil
}
