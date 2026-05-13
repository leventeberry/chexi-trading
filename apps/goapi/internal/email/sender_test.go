package email

import (
	"context"
	"testing"

	"goapi/config"
)

func TestFromConfig_ReturnsNoopForNilOrDisabled(t *testing.T) {
	t.Parallel()

	if _, ok := FromConfig(nil).(NoopSender); !ok {
		t.Fatal("FromConfig(nil) should return NoopSender")
	}

	cfg := &config.Config{}
	cfg.Email.Enabled = false
	if _, ok := FromConfig(cfg).(NoopSender); !ok {
		t.Fatal("FromConfig(disabled) should return NoopSender")
	}
}

func TestFromConfig_ReturnsLogSenderForEnabledProviders(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	cfg.Email.Enabled = true
	cfg.Email.Provider = "log"
	cfg.Email.From = "noreply@example.com"
	cfg.Environment = config.EnvironmentTest

	if _, ok := FromConfig(cfg).(*LogSender); !ok {
		t.Fatal("FromConfig(log) should return *LogSender")
	}

	cfg.Email.Provider = "not-implemented-provider"
	if _, ok := FromConfig(cfg).(*LogSender); !ok {
		t.Fatal("FromConfig(unknown) should fall back to *LogSender")
	}

	cfg.Email.Provider = "resend"
	cfg.Email.ResendAPIKey = ""
	if _, ok := FromConfig(cfg).(*LogSender); !ok {
		t.Fatal("FromConfig(resend without API key) should fall back to *LogSender")
	}

	cfg.Email.ResendAPIKey = "re_test_localonly"
	if _, ok := FromConfig(cfg).(*ResendSender); !ok {
		t.Fatal("FromConfig(resend with API key) should return *ResendSender")
	}
}

func TestLogSender_Send_NoPanicAcrossEnvironments(t *testing.T) {
	t.Parallel()

	msg := Message{
		To:       "user@example.com",
		Subject:  "subject",
		TextBody: "body",
		Kind:     KindVerification,
	}

	devSender := NewLogSender("noreply@example.com", config.EnvironmentDevelopment)
	if err := devSender.Send(context.Background(), msg); err != nil {
		t.Fatalf("dev Send() error = %v", err)
	}

	prodSender := NewLogSender("noreply@example.com", config.EnvironmentProduction)
	if err := prodSender.Send(context.Background(), msg); err != nil {
		t.Fatalf("prod Send() error = %v", err)
	}
}

func TestNoopSender_Send(t *testing.T) {
	t.Parallel()

	var s NoopSender
	if err := s.Send(context.Background(), Message{To: "x@example.com"}); err != nil {
		t.Fatalf("NoopSender.Send() error = %v", err)
	}
}
