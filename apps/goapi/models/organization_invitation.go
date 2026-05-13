package models

import (
	"time"

	"github.com/google/uuid"
)

// OrganizationInvitation is a pending invite to join an organization (token stored hashed).
type OrganizationInvitation struct {
	ID              uuid.UUID  `gorm:"type:uuid;primaryKey" json:"id"`
	OrganizationID  uuid.UUID  `gorm:"type:uuid;not null;index" json:"organization_id"`
	Email           string     `gorm:"size:255;not null" json:"email"`
	Role            string     `gorm:"size:20;not null" json:"role"`
	TokenHash       string     `gorm:"size:64;not null;uniqueIndex" json:"-"`
	InvitedByUserID uuid.UUID  `gorm:"type:uuid;not null" json:"invited_by_user_id"`
	ExpiresAt       time.Time  `gorm:"not null" json:"expires_at"`
	AcceptedAt      *time.Time `json:"accepted_at,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

func (OrganizationInvitation) TableName() string {
	return "organization_invitations"
}
