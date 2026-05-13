package email

import (
	"fmt"
	"net/url"
	"strings"

	"goapi/config"
)

// NewVerificationMessage builds a transactional verification email from config and raw token.
func NewVerificationMessage(cfg *config.Config, to, rawToken string) Message {
	q := url.QueryEscape(rawToken)
	body := VerificationBody(cfg, q, rawToken)
	return Message{
		To:       to,
		Subject:  "Verify your email",
		TextBody: body,
		Kind:     KindVerification,
	}
}

// VerificationBody returns the plaintext body for email verification (shared by HTTP handlers and queue workers).
func VerificationBody(cfg *config.Config, escapedToken, rawToken string) string {
	if cfg == nil {
		return fmt.Sprintf("Verify your email with POST /api/v1/verify-email and JSON body: {\"token\":\"%s\"}\n", rawToken)
	}
	base := strings.TrimRight(strings.TrimSpace(cfg.Email.AppPublicURL), "/")
	if base != "" {
		link := fmt.Sprintf("%s/api/v1/verify-email?token=%s", base, escapedToken)
		return fmt.Sprintf("Verify your email:\n%s\n\nYou can also POST JSON {\"token\":\"<token>\"} to %s/api/v1/verify-email\n", link, base)
	}
	return fmt.Sprintf("Verify your email with POST /api/v1/verify-email and JSON body: {\"token\":\"%s\"}\n", rawToken)
}

// NewPasswordResetMessage builds a password-reset transactional email from config and raw token.
func NewPasswordResetMessage(cfg *config.Config, to, rawToken string) Message {
	body := PasswordResetBody(cfg, rawToken)
	return Message{
		To:       to,
		Subject:  "Password reset",
		TextBody: body,
		Kind:     KindPasswordReset,
	}
}

// PasswordResetBody returns plaintext for password reset emails.
// NewOrganizationInvitationMessage builds an invite email with accept instructions.
func NewOrganizationInvitationMessage(cfg *config.Config, to, rawToken, organizationName string) Message {
	body := OrganizationInvitationBody(cfg, rawToken, organizationName)
	subject := "Organization invitation"
	if organizationName != "" {
		subject = fmt.Sprintf("Invitation to join %s", organizationName)
	}
	return Message{
		To:       to,
		Subject:  subject,
		TextBody: body,
		Kind:     KindOrganizationInvitation,
	}
}

// OrganizationInvitationBody returns plaintext for org invites.
func OrganizationInvitationBody(cfg *config.Config, rawToken, organizationName string) string {
	var base string
	if cfg != nil {
		base = strings.TrimRight(strings.TrimSpace(cfg.Email.AppPublicURL), "/")
	}
	orgLine := ""
	if organizationName != "" {
		orgLine = fmt.Sprintf("Organization: %s\n", organizationName)
	}
	if base != "" {
		return fmt.Sprintf("%sYou have been invited to join an organization.\n\nAccept with POST %s/api/v1/organizations/invitations/accept\nAuthorization: Bearer <your session token>\nJSON: {\"token\":\"%s\"}\n", orgLine, base, rawToken)
	}
	return fmt.Sprintf("%sAccept invitation with POST /api/v1/organizations/invitations/accept and JSON {\"token\":\"%s\"}\n", orgLine, rawToken)
}

func PasswordResetBody(cfg *config.Config, rawToken string) string {
	var base string
	if cfg != nil {
		base = strings.TrimRight(strings.TrimSpace(cfg.Email.AppPublicURL), "/")
	}
	if base != "" {
		return fmt.Sprintf("Reset your password:\nPOST %s/api/v1/password-reset/confirm\nJSON: {\"token\":\"%s\",\"password\":\"new-password\"}\n", base, rawToken)
	}
	return fmt.Sprintf("Reset your password using POST /api/v1/password-reset/confirm with {\"token\":\"%s\",\"password\":\"new-password\"}\n", rawToken)
}
