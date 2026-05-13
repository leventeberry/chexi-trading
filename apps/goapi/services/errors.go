package services

import "errors"

// Service errors
var (
	ErrUserNotFound               = errors.New("user not found")
	ErrEmailExists                = errors.New("email already registered")
	ErrInvalidCredentials         = errors.New("invalid email or password")
	ErrEmailNotVerified           = errors.New("email address not verified")
	ErrInvalidRole                = errors.New("invalid role")
	ErrInsufficientPrivileges     = errors.New("insufficient privileges for this operation")
	ErrForbiddenAdminRegistration = errors.New("cannot self-register as admin")
	ErrPasswordHashing            = errors.New("failed to hash password")
	ErrNoFieldsToUpdate           = errors.New("at least one field must be provided for update")
	ErrTokenGeneration            = errors.New("failed to generate token")
	ErrSessionCreation            = errors.New("failed to create session")
	ErrInvalidRefreshToken        = errors.New("invalid refresh token")
	ErrExpiredRefreshToken        = errors.New("expired refresh token")
	ErrRevokedRefreshToken        = errors.New("revoked refresh token")
	ErrRefreshTokenReuseDetected  = errors.New("refresh token reuse detected")
	ErrInvalidVerificationToken   = errors.New("invalid or expired verification token")
	ErrInvalidPasswordResetToken  = errors.New("invalid or expired password reset token")
	ErrInvalidAvatarURL           = errors.New("invalid avatar_url")
	ErrInvalidTimezone            = errors.New("invalid timezone")
	ErrInvalidLocale              = errors.New("invalid locale")
	ErrInvalidTheme               = errors.New("invalid theme; use light, dark, or system")
	ErrInvalidNotificationPrefs   = errors.New("invalid notification_preferences JSON")
	ErrInvalidExtraSettingsJSON   = errors.New("invalid extra_settings JSON")
	ErrInvalidProfileField        = errors.New("invalid profile field value")

	// MFA / TOTP
	ErrMFAUnavailable              = errors.New("mfa service unavailable")
	ErrMFAConfigurationUnavailable = errors.New("mfa configuration unavailable")
	ErrMFAAlreadyEnabled           = errors.New("mfa already enabled")
	ErrMFANotEnabled               = errors.New("mfa not enabled or incomplete setup")
	ErrMFAInvalidCode              = errors.New("invalid mfa code")
	ErrMFAInvalidChallenge         = errors.New("invalid mfa challenge")
	ErrMFANoPendingEnrollment      = errors.New("no pending totp enrollment")
	ErrInvalidCurrentPassword      = errors.New("invalid current password")

	// OAuth
	ErrOAuthUnavailable           = errors.New("oauth is not available")
	ErrOAuthProviderDisabled      = errors.New("oauth provider is disabled")
	ErrOAuthInvalidProvider       = errors.New("invalid oauth provider")
	ErrOAuthInvalidState          = errors.New("invalid oauth state")
	ErrOAuthEmailNotVerified      = errors.New("oauth email not verified by provider")
	ErrOAuthEmailConflict         = errors.New("oauth email conflict with existing account")
	ErrOAuthLinkEmailMismatch     = errors.New("oauth email does not match account email")
	ErrOAuthIdentityAlreadyLinked = errors.New("oauth identity already linked to another account")
	ErrOAuthExchangeInvalid       = errors.New("invalid or expired oauth exchange code")

	// Organizations / multi-tenant
	ErrOrganizationNotFound    = errors.New("organization not found")
	ErrOrganizationSlugExists  = errors.New("organization slug is already in use")
	ErrInvalidOrganizationSlug = errors.New("invalid organization slug format")
	ErrInvalidOrganizationName = errors.New("organization name is required")

	ErrOrganizationInvitationExpired       = errors.New("organization invitation has expired")
	ErrOrganizationInvitationAccepted      = errors.New("organization invitation already accepted")
	ErrInvalidOrganizationInvitation       = errors.New("invalid organization invitation token")
	ErrOrganizationInvitationEmailMismatch = errors.New("invitation email does not match your account email")
	ErrInvalidInvitationRole               = errors.New("invalid invitation role; use admin or member")
	ErrAlreadyOrganizationMember           = errors.New("user is already a member of this organization")
	ErrCannotRemoveLastOwner               = errors.New("cannot remove the last owner from the organization")
	ErrInvalidInvitationEmail              = errors.New("invalid invitation email")
	ErrNotOrganizationMember               = errors.New("user is not a member of this organization")

	ErrOrganizationNoteNotFound    = errors.New("organization note not found")
	ErrInvalidOrganizationNoteBody = errors.New("title and body are required")

	ErrInvalidOrganizationAPIKey       = errors.New("invalid organization api key")
	ErrInvalidOrganizationAPIKeyScopes = errors.New("invalid organization api key scopes")
	ErrInvalidOrganizationAPIKeyName   = errors.New("organization api key name is required")
	ErrOrganizationAPIKeyNotFound      = errors.New("organization api key not found")

	ErrWebhooksUnavailable         = errors.New("organization webhooks are not configured")
	ErrInvalidWebhookURL           = errors.New("invalid webhook url")
	ErrInvalidWebhookEvents        = errors.New("invalid webhook events")
	ErrOrganizationWebhookNotFound = errors.New("organization webhook not found")
)
