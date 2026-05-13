package models

import (
	"time"

	"github.com/google/uuid"
)

// UserOAuthAccount binds an external IdP identity to a local user.
type UserOAuthAccount struct {
	ID             uuid.UUID `gorm:"type:uuid;primaryKey" json:"-"`
	UserID         uuid.UUID `gorm:"type:uuid;not null;index" json:"-"`
	Provider       string    `gorm:"size:32;not null;uniqueIndex:ux_oauth_provider_subject" json:"provider"`
	ProviderUserID string    `gorm:"size:255;not null;uniqueIndex:ux_oauth_provider_subject" json:"-"`
	Email          string    `gorm:"size:255;not null" json:"-"`
	EmailVerified  bool      `gorm:"not null" json:"-"`
	CreatedAt      time.Time `json:"-"`
	UpdatedAt      time.Time `json:"-"`
}

func (UserOAuthAccount) TableName() string {
	return "user_oauth_accounts"
}
