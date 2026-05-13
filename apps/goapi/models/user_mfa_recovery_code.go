package models

import (
	"time"

	"github.com/google/uuid"
)

// UserMFARecoveryCode stores a hashed one-time backup code (plaintext never persisted).
type UserMFARecoveryCode struct {
	ID        uuid.UUID  `gorm:"type:uuid;primaryKey" json:"-"`
	UserID    uuid.UUID  `gorm:"type:uuid;not null;uniqueIndex:ux_user_mfa_recovery_code" json:"-"`
	CodeHash  string     `gorm:"size:64;not null;uniqueIndex:ux_user_mfa_recovery_code" json:"-"`
	UsedAt    *time.Time `json:"-"`
	CreatedAt time.Time  `json:"-"`
}

func (UserMFARecoveryCode) TableName() string {
	return "user_mfa_recovery_codes"
}
