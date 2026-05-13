package models

import (
	"time"

	"github.com/google/uuid"
)

// EmailVerificationToken stores hashed single-use email verification tokens.
type EmailVerificationToken struct {
	ID        uuid.UUID  `gorm:"type:uuid;primaryKey" json:"id"`
	UserID    uuid.UUID  `gorm:"type:uuid;not null;index" json:"user_id"`
	TokenHash string     `gorm:"size:64;not null;uniqueIndex" json:"-"`
	ExpiresAt time.Time  `gorm:"not null;index" json:"expires_at"`
	UsedAt    *time.Time `gorm:"index" json:"used_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
}

// TableName overrides default pluralization.
func (EmailVerificationToken) TableName() string {
	return "email_verification_tokens"
}
