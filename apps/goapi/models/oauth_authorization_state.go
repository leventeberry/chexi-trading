package models

import (
	"time"

	"github.com/google/uuid"
)

// OAuthAuthorizationState holds PKCE verifier and optional link intent for one OAuth round-trip.
type OAuthAuthorizationState struct {
	ID           uuid.UUID  `gorm:"type:uuid;primaryKey" json:"-"`
	StateHash    string     `gorm:"size:64;uniqueIndex;not null" json:"-"`
	Provider     string     `gorm:"size:32;not null" json:"-"`
	CodeVerifier string     `gorm:"type:text;not null" json:"-"`
	LinkUserID   *uuid.UUID `gorm:"type:uuid" json:"-"`
	ExpiresAt    time.Time  `gorm:"not null" json:"-"`
	UsedAt       *time.Time `json:"-"`
	CreatedAt    time.Time  `json:"-"`
}

func (OAuthAuthorizationState) TableName() string {
	return "oauth_authorization_states"
}
