package handlers

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"goapi/services"
)

// handleServiceError converts service errors to appropriate HTTP responses.
func handleServiceError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, services.ErrInvalidCredentials):
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid email or password"})
	case errors.Is(err, services.ErrEmailNotVerified):
		c.JSON(http.StatusForbidden, gin.H{"error": "Verify your email before signing in"})
	case errors.Is(err, services.ErrEmailExists):
		c.JSON(http.StatusConflict, gin.H{"error": "Email already registered"})
	case errors.Is(err, services.ErrInvalidRole):
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid role. Valid roles are: user, admin"})
	case errors.Is(err, services.ErrInsufficientPrivileges):
		c.JSON(http.StatusForbidden, gin.H{"error": "Insufficient privileges for this operation"})
	case errors.Is(err, services.ErrForbiddenAdminRegistration):
		c.JSON(http.StatusForbidden, gin.H{"error": "Cannot self-register as admin"})
	case errors.Is(err, services.ErrUserNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
	case errors.Is(err, services.ErrNoFieldsToUpdate):
		c.JSON(http.StatusBadRequest, gin.H{"error": "At least one field must be provided for update"})
	case errors.Is(err, services.ErrInvalidVerificationToken):
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid or expired verification token"})
	case errors.Is(err, services.ErrInvalidPasswordResetToken):
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid or expired password reset token"})
	case errors.Is(err, services.ErrInvalidRefreshToken),
		errors.Is(err, services.ErrExpiredRefreshToken),
		errors.Is(err, services.ErrRevokedRefreshToken),
		errors.Is(err, services.ErrRefreshTokenReuseDetected):
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired refresh token"})
	case errors.Is(err, services.ErrPasswordTooShort),
		errors.Is(err, services.ErrPasswordNoUpper),
		errors.Is(err, services.ErrPasswordNoLower),
		errors.Is(err, services.ErrPasswordNoNumber),
		errors.Is(err, services.ErrPasswordNoSpecial):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case errors.Is(err, services.ErrPasswordHashing):
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
	case errors.Is(err, services.ErrInvalidAvatarURL):
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid avatar URL"})
	case errors.Is(err, services.ErrInvalidTimezone):
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid timezone"})
	case errors.Is(err, services.ErrInvalidLocale):
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid locale format"})
	case errors.Is(err, services.ErrInvalidTheme):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case errors.Is(err, services.ErrInvalidNotificationPrefs):
		c.JSON(http.StatusBadRequest, gin.H{"error": "notification_preferences must be a JSON object"})
	case errors.Is(err, services.ErrInvalidExtraSettingsJSON):
		c.JSON(http.StatusBadRequest, gin.H{"error": "extra_settings must be a JSON object"})
	case errors.Is(err, services.ErrInvalidProfileField):
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid profile field"})
	case errors.Is(err, services.ErrMFAUnavailable),
		errors.Is(err, services.ErrMFAConfigurationUnavailable):
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "MFA is not available"})
	case errors.Is(err, services.ErrMFAAlreadyEnabled):
		c.JSON(http.StatusConflict, gin.H{"error": "MFA is already enabled"})
	case errors.Is(err, services.ErrMFANotEnabled),
		errors.Is(err, services.ErrMFANoPendingEnrollment):
		c.JSON(http.StatusBadRequest, gin.H{"error": "MFA enrollment is not active"})
	case errors.Is(err, services.ErrMFAInvalidCode),
		errors.Is(err, services.ErrMFAInvalidChallenge):
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid MFA code"})
	case errors.Is(err, services.ErrInvalidCurrentPassword):
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid password"})
	case errors.Is(err, services.ErrOAuthUnavailable):
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "OAuth is not configured"})
	case errors.Is(err, services.ErrOAuthProviderDisabled):
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "OAuth provider is not enabled"})
	case errors.Is(err, services.ErrOAuthInvalidProvider):
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid OAuth provider"})
	case errors.Is(err, services.ErrOAuthEmailNotVerified):
		c.JSON(http.StatusForbidden, gin.H{"error": "Your OAuth account email is not verified with the provider. Verify it there, then try again."})
	case errors.Is(err, services.ErrOAuthEmailConflict):
		c.JSON(http.StatusForbidden, gin.H{"error": "That email is already registered to another account."})
	case errors.Is(err, services.ErrOAuthLinkEmailMismatch):
		c.JSON(http.StatusForbidden, gin.H{"error": "OAuth account email does not match this user account."})
	case errors.Is(err, services.ErrOAuthIdentityAlreadyLinked):
		c.JSON(http.StatusForbidden, gin.H{"error": "That OAuth identity is already linked to a different account."})
	case errors.Is(err, services.ErrOAuthExchangeInvalid):
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid oauth exchange code"})
	case errors.Is(err, services.ErrOrganizationNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "Organization not found"})
	case errors.Is(err, services.ErrOrganizationSlugExists):
		c.JSON(http.StatusConflict, gin.H{"error": "Organization slug is already in use"})
	case errors.Is(err, services.ErrInvalidOrganizationSlug):
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid organization slug"})
	case errors.Is(err, services.ErrInvalidOrganizationName):
		c.JSON(http.StatusBadRequest, gin.H{"error": "Organization name is required"})
	case errors.Is(err, services.ErrOrganizationInvitationExpired):
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invitation has expired"})
	case errors.Is(err, services.ErrOrganizationInvitationAccepted):
		c.JSON(http.StatusConflict, gin.H{"error": "Invitation already used"})
	case errors.Is(err, services.ErrInvalidOrganizationInvitation):
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid invitation token"})
	case errors.Is(err, services.ErrOrganizationInvitationEmailMismatch):
		c.JSON(http.StatusForbidden, gin.H{"error": "Invitation email does not match your account"})
	case errors.Is(err, services.ErrInvalidInvitationRole):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case errors.Is(err, services.ErrInvalidInvitationEmail):
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid invitation email"})
	case errors.Is(err, services.ErrAlreadyOrganizationMember):
		c.JSON(http.StatusConflict, gin.H{"error": "User is already a member of this organization"})
	case errors.Is(err, services.ErrCannotRemoveLastOwner):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case errors.Is(err, services.ErrNotOrganizationMember):
		c.JSON(http.StatusNotFound, gin.H{"error": "User is not a member of this organization"})
	case errors.Is(err, services.ErrOrganizationNoteNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "Note not found"})
	case errors.Is(err, services.ErrInvalidOrganizationNoteBody):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case errors.Is(err, services.ErrInvalidOrganizationAPIKeyName):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case errors.Is(err, services.ErrInvalidOrganizationAPIKeyScopes):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case errors.Is(err, services.ErrInvalidOrganizationAPIKey):
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid API key"})
	case errors.Is(err, services.ErrOrganizationAPIKeyNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "API key not found"})
	case errors.Is(err, services.ErrWebhooksUnavailable):
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Organization webhooks are not configured"})
	case errors.Is(err, services.ErrInvalidWebhookURL):
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid webhook URL"})
	case errors.Is(err, services.ErrInvalidWebhookEvents):
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid webhook events"})
	case errors.Is(err, services.ErrOrganizationWebhookNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "Webhook not found"})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
	}
}
