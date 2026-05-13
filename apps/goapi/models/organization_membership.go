package models

import (
	"time"

	"github.com/google/uuid"
)

// OrganizationMembership links a user to an organization with a tenant role.
type OrganizationMembership struct {
	OrganizationID uuid.UUID `gorm:"type:uuid;primaryKey" json:"organization_id"`
	UserID         uuid.UUID `gorm:"type:uuid;primaryKey" json:"user_id"`
	Role           string    `gorm:"size:20;not null" json:"role"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

func (OrganizationMembership) TableName() string {
	return "organization_memberships"
}
