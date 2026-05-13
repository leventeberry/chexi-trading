package models

import (
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID              uuid.UUID  `gorm:"type:uuid;primaryKey" json:"id"`
	FirstName       string     `json:"first_name"`
	LastName        string     `json:"last_name"`
	DisplayName     string     `gorm:"size:120" json:"display_name,omitempty"`
	AvatarURL       string     `gorm:"size:2048" json:"avatar_url,omitempty"`
	Timezone        string     `gorm:"size:64" json:"timezone,omitempty"`
	Locale          string     `gorm:"size:16" json:"locale,omitempty"`
	Email           string     `gorm:"uniqueIndex;not null" json:"email"`
	PassHash        string     `json:"-"` // Excluded from JSON responses for security
	PhoneNum        string     `json:"phone_number"`
	Role            string     `json:"role"`
	EmailVerifiedAt *time.Time `json:"email_verified_at,omitempty"`
	TotpEnabled     bool       `gorm:"default:false" json:"totp_enabled,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}
