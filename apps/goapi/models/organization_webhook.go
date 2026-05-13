package models

import (
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
)

// OrganizationWebhook is an outbound webhook subscription for an organization.
type OrganizationWebhook struct {
	ID               uuid.UUID      `gorm:"type:uuid;primaryKey" json:"id"`
	OrganizationID   uuid.UUID      `gorm:"type:uuid;not null;index" json:"organization_id"`
	URL              string         `gorm:"type:text;not null" json:"url"`
	SecretCiphertext []byte         `gorm:"type:bytea;not null" json:"-"`
	SecretKeyVersion int            `gorm:"not null;default:1" json:"secret_key_version"`
	Events           pq.StringArray `gorm:"type:text[];not null" json:"events"`
	Enabled          bool           `gorm:"not null;default:true;index" json:"enabled"`
	CreatedByUserID  uuid.UUID      `gorm:"type:uuid;not null;index" json:"created_by_user_id"`
	CreatedAt        time.Time      `json:"created_at"`
	UpdatedAt        time.Time      `json:"updated_at"`
}

func (OrganizationWebhook) TableName() string {
	return "organization_webhooks"
}
