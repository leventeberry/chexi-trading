package models

import (
	"time"

	"github.com/google/uuid"
)

// Session stores hashed refresh token state for secure session lifecycle management.
type Session struct {
	ID                uuid.UUID  `gorm:"type:uuid;primaryKey" json:"id"`
	UserID            uuid.UUID  `gorm:"type:uuid;not null;index" json:"user_id"`
	TokenHash         string     `gorm:"size:128;not null;uniqueIndex" json:"-"`
	IssuedAt          time.Time  `gorm:"not null" json:"issued_at"`
	ExpiresAt         time.Time  `gorm:"not null;index" json:"expires_at"`
	RevokedAt         *time.Time `gorm:"index" json:"revoked_at,omitempty"`
	ReplacedBySession *uuid.UUID `gorm:"type:uuid" json:"replaced_by_session,omitempty"`
	RevokeReason      string     `gorm:"size:64" json:"revoke_reason,omitempty"`
	UserAgent         string     `gorm:"size:512" json:"user_agent,omitempty"`
	IPAddress         string     `gorm:"size:64" json:"ip_address,omitempty"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
}

// TableName overrides default pluralization.
func (Session) TableName() string {
	return "auth_sessions"
}
