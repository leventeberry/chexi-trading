package repositories

import "errors"

// Repository errors
var (
	ErrUserNotFound         = errors.New("user not found")
	ErrUserExists           = errors.New("user already exists")
	ErrTokenNotFound        = errors.New("token not found")
	ErrUserSettingsNotFound = errors.New("user settings not found")

	ErrOrganizationNotFound           = errors.New("organization not found")
	ErrOrganizationMembershipNotFound = errors.New("organization membership not found")

	ErrOrganizationInvitationNotFound = errors.New("organization invitation not found")

	ErrOrganizationNoteNotFound = errors.New("organization note not found")

	ErrTradePlanNotFound = errors.New("trade plan not found")

	ErrOrganizationAPIKeyNotFound = errors.New("organization api key not found")

	ErrOrganizationWebhookNotFound         = errors.New("organization webhook not found")
	ErrOrganizationWebhookDeliveryNotFound = errors.New("organization webhook delivery not found")
)
