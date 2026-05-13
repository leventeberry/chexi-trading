package models

import (
	"time"

	"github.com/google/uuid"
)

// UserTOTPFactor stores encrypted TOTP material (confirmed + optional pending enrollment).
type UserTOTPFactor struct {
	ID                     uuid.UUID `gorm:"type:uuid;primaryKey" json:"-"`
	UserID                 uuid.UUID `gorm:"type:uuid;uniqueIndex;not null" json:"-"`
	EncryptedSecret        []byte    `gorm:"type:bytea" json:"-"`
	PendingEncryptedSecret []byte    `gorm:"type:bytea" json:"-"`
	CreatedAt              time.Time `json:"-"`
	UpdatedAt              time.Time `json:"-"`
}

func (UserTOTPFactor) TableName() string {
	return "user_totp_factors"
}
