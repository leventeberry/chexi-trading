package services

import (
	"encoding/json"

	"goapi/models"
)

// MeProfileDTO is the authenticated user's profile + settings bundle.
type MeProfileDTO struct {
	User     *models.User
	Settings models.UserSettings
}

// PatchMeProfileInput partial-updates /me (only non-nil pointers apply).
type PatchMeProfileInput struct {
	FirstName   *string
	LastName    *string
	DisplayName *string
	AvatarURL   *string
	PhoneNum    *string
	Timezone    *string
	Locale      *string

	Theme                     *string
	NotificationPreferences   *json.RawMessage
	MarketingEmailOptIn       *bool
	SecurityNotificationOptIn *bool
	ExtraSettings             *json.RawMessage
}

// CreateUserInput holds the data for creating a new user
type CreateUserInput struct {
	FirstName string
	LastName  string
	Email     string
	Password  string
	PhoneNum  string
	Role      string
}

// UpdateUserInput holds the data for updating a user
type UpdateUserInput struct {
	FirstName   *string
	LastName    *string
	DisplayName *string
	AvatarURL   *string
	Timezone    *string
	Locale      *string
	Email       *string
	Password    *string
	PhoneNum    *string
	Role        *string
}

// RegisterInput holds the data for user registration
type RegisterInput struct {
	FirstName string
	LastName  string
	Email     string
	Password  string
	PhoneNum  string
	Role      string
}

// PaginationParams holds pagination parameters
type PaginationParams struct {
	Page     int
	PageSize int
}

// Authentication holds generated API key and JWT token details.
type Authentication struct {
	ApiKey       string `json:"api_key"`
	JWTToken     string `json:"jwt_token"`
	RefreshToken string `json:"refresh_token,omitempty"`
}
