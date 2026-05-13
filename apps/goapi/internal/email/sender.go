// Package email provides a minimal outbound mail abstraction with safe defaults for logs.
package email

import (
	"context"
	"strings"

	"goapi/config"
	"goapi/logger"
)

// Kind identifies transactional email type for metrics/logging (never log secrets).
const (
	KindVerification           = "verification"
	KindPasswordReset          = "password_reset"
	KindOrganizationInvitation = "organization_invitation"
)

// Message is a simple transactional email payload.
// TextBody may contain sensitive links; callers must not log it in staging/production.
type Message struct {
	To       string
	From     string
	Subject  string
	TextBody string
	Kind     string
}

// Sender sends transactional email.
type Sender interface {
	Send(ctx context.Context, msg Message) error
}

// LogSender logs email sends locally; in staging/production it omits body (may contain tokens).
type LogSender struct {
	from        string
	environment string
}

// NewLogSender returns a dev-oriented sender that writes to application logs.
func NewLogSender(from, environment string) *LogSender {
	return &LogSender{from: strings.TrimSpace(from), environment: strings.TrimSpace(environment)}
}

// Send implements Sender.
func (s *LogSender) Send(ctx context.Context, msg Message) error {
	_ = ctx
	from := msg.From
	if from == "" {
		from = s.from
	}
	if config.IsStagingOrProductionEnvironment(s.environment) {
		logger.Log.Info().
			Str("to", msg.To).
			Str("from", from).
			Str("kind", msg.Kind).
			Str("subject", msg.Subject).
			Msg("email send (body omitted in staging/production)")
		return nil
	}
	logger.Log.Info().
		Str("to", msg.To).
		Str("from", from).
		Str("kind", msg.Kind).
		Str("subject", msg.Subject).
		Str("body", msg.TextBody).
		Msg("email send (dev/test log sink)")
	return nil
}

// NoopSender drops messages (e.g. EMAIL_ENABLED=false).
type NoopSender struct{}

func (NoopSender) Send(ctx context.Context, msg Message) error {
	_ = ctx
	_ = msg
	return nil
}

// FromConfig returns the appropriate sender for local/dev vs future provider wiring.
func FromConfig(cfg *config.Config) Sender {
	if cfg == nil {
		return NoopSender{}
	}
	if !cfg.Email.Enabled {
		return NoopSender{}
	}
	switch strings.ToLower(strings.TrimSpace(cfg.Email.Provider)) {
	case "", "log":
		return NewLogSender(cfg.Email.From, cfg.Environment)
	case "resend":
		if strings.TrimSpace(cfg.Email.ResendAPIKey) == "" {
			logger.Log.Warn().Msg("EMAIL_PROVIDER=resend but RESEND_API_KEY is empty; using log sender")
			return NewLogSender(cfg.Email.From, cfg.Environment)
		}
		return NewResendSender(cfg.Email.ResendAPIKey, cfg.Email.From, cfg.Environment, cfg.Email.RedirectAllTo)
	default:
		logger.Log.Warn().Str("provider", cfg.Email.Provider).Msg("EMAIL_PROVIDER not implemented; using log sender")
		return NewLogSender(cfg.Email.From, cfg.Environment)
	}
}
