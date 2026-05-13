package jobs

import (
	"context"
	"encoding/json"
	"fmt"

	"goapi/config"
	"goapi/internal/email"
	"goapi/internal/queue"
)

// Job type identifiers for transactional email dispatch.
const (
	EmailSendVerification           = "email.send_verification"
	EmailSendPasswordReset          = "email.send_password_reset"
	EmailSendOrganizationInvitation = "email.send_organization_invitation"
)

type emailTargetPayload struct {
	To       string `json:"to"`
	RawToken string `json:"raw_token"`
}

type emailOrgInvitePayload struct {
	To               string `json:"to"`
	RawToken         string `json:"raw_token"`
	OrganizationName string `json:"organization_name"`
}

// RegisterEmailHandlers wires verification and password-reset jobs to the mail sender.
func RegisterEmailHandlers(reg *queue.Registry, mail email.Sender, cfg *config.Config) {
	reg.Register(EmailSendVerification, func(ctx context.Context, payload json.RawMessage) error {
		var p emailTargetPayload
		if err := json.Unmarshal(payload, &p); err != nil {
			return fmt.Errorf("%w: %v", queue.ErrInvalidPayload, err)
		}
		if p.To == "" || p.RawToken == "" {
			return fmt.Errorf("%w: missing to or raw_token", queue.ErrInvalidPayload)
		}
		msg := email.NewVerificationMessage(cfg, p.To, p.RawToken)
		return mail.Send(ctx, msg)
	})

	reg.Register(EmailSendPasswordReset, func(ctx context.Context, payload json.RawMessage) error {
		var p emailTargetPayload
		if err := json.Unmarshal(payload, &p); err != nil {
			return fmt.Errorf("%w: %v", queue.ErrInvalidPayload, err)
		}
		if p.To == "" || p.RawToken == "" {
			return fmt.Errorf("%w: missing to or raw_token", queue.ErrInvalidPayload)
		}
		msg := email.NewPasswordResetMessage(cfg, p.To, p.RawToken)
		return mail.Send(ctx, msg)
	})

	reg.Register(EmailSendOrganizationInvitation, func(ctx context.Context, payload json.RawMessage) error {
		var p emailOrgInvitePayload
		if err := json.Unmarshal(payload, &p); err != nil {
			return fmt.Errorf("%w: %v", queue.ErrInvalidPayload, err)
		}
		if p.To == "" || p.RawToken == "" {
			return fmt.Errorf("%w: missing to or raw_token", queue.ErrInvalidPayload)
		}
		msg := email.NewOrganizationInvitationMessage(cfg, p.To, p.RawToken, p.OrganizationName)
		return mail.Send(ctx, msg)
	})
}
