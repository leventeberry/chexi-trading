package models

import (
	"time"

	"github.com/google/uuid"
)

// OAuthExchangeCode is a one-time handoff from browser OAuth redirect to SPA JSON token exchange.
type OAuthExchangeCode struct {
	ID                uuid.UUID  `gorm:"type:uuid;primaryKey" json:"-"`
	CodeHash          string     `gorm:"size:64;uniqueIndex;not null" json:"-"`
	UserID            uuid.UUID  `gorm:"type:uuid;not null;index" json:"-"`
	ExpiresAt         time.Time  `gorm:"not null" json:"-"`
	UsedAt            *time.Time `json:"-"`
	MFAChallengeToken *string    `gorm:"type:text" json:"-"`
	CreatedAt         time.Time  `json:"-"`
}

func (OAuthExchangeCode) TableName() string {
	return "oauth_exchange_codes"
}
