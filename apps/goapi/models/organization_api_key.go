package models

import (
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
)

// OrganizationAPIKey stores a hashed organization-scoped API key (never the raw secret).
type OrganizationAPIKey struct {
	ID              uuid.UUID      `gorm:"type:uuid;primaryKey" json:"id"`
	OrganizationID  uuid.UUID      `gorm:"type:uuid;not null;index" json:"organization_id"`
	Name            string         `gorm:"size:255;not null" json:"name"`
	KeyPrefix       string         `gorm:"size:32;not null;index" json:"key_prefix"`
	KeyHash         string         `gorm:"size:64;not null;uniqueIndex" json:"-"`
	Scopes          pq.StringArray `gorm:"type:text[];not null" json:"scopes"`
	CreatedByUserID uuid.UUID      `gorm:"type:uuid;not null;index" json:"created_by_user_id"`
	LastUsedAt      *time.Time     `json:"last_used_at,omitempty"`
	RevokedAt       *time.Time     `json:"revoked_at,omitempty"`
	ExpiresAt       *time.Time     `json:"expires_at,omitempty"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
}

func (OrganizationAPIKey) TableName() string {
	return "organization_api_keys"
}
