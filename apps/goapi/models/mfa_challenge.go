package models

import (
	"time"

	"github.com/google/uuid"
)

// MFAChallenge tracks a short-lived login step-up token (JTI) for replay resistance.
type MFAChallenge struct {
	ID        uuid.UUID  `gorm:"type:uuid;primaryKey" json:"-"`
	UserID    uuid.UUID  `gorm:"type:uuid;not null;index" json:"-"`
	JTIHash   string     `gorm:"size:64;uniqueIndex;not null" json:"-"`
	ExpiresAt time.Time  `json:"-"`
	UsedAt    *time.Time `json:"-"`
	CreatedAt time.Time  `json:"-"`
}

func (MFAChallenge) TableName() string {
	return "mfa_challenges"
}
