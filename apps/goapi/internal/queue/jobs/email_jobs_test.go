package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"

	"goapi/config"
	"goapi/internal/email"
	"goapi/internal/queue"
)

type recordingSender struct {
	mu   sync.Mutex
	msgs []email.Message
	err  error
}

func (r *recordingSender) Send(ctx context.Context, msg email.Message) error {
	_ = ctx
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.err != nil {
		return r.err
	}
	r.msgs = append(r.msgs, msg)
	return nil
}

func (r *recordingSender) last() (email.Message, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.msgs) == 0 {
		return email.Message{}, false
	}
	return r.msgs[len(r.msgs)-1], true
}

func TestRegisterEmailHandlers_VerificationDispatchesToSender(t *testing.T) {
	t.Parallel()
	sender := &recordingSender{}
	cfg := &config.Config{}
	reg := queue.NewRegistry()
	RegisterEmailHandlers(reg, sender, cfg)

	payload := json.RawMessage(`{"to":"user@example.com","raw_token":"raw-verify-token"}`)
	if err := reg.Dispatch(context.Background(), EmailSendVerification, payload); err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	msg, ok := sender.last()
	if !ok {
		t.Fatal("expected one send")
	}
	if msg.To != "user@example.com" {
		t.Fatalf("To = %q", msg.To)
	}
	if msg.Kind != email.KindVerification {
		t.Fatalf("Kind = %q", msg.Kind)
	}
	if msg.Subject != "Verify your email" {
		t.Fatalf("Subject = %q", msg.Subject)
	}
	if !strings.Contains(msg.TextBody, "raw-verify-token") {
		t.Fatalf("body should contain token, got %q", msg.TextBody)
	}
}

func TestRegisterEmailHandlers_PasswordResetDispatchesToSender(t *testing.T) {
	t.Parallel()
	sender := &recordingSender{}
	cfg := &config.Config{}
	reg := queue.NewRegistry()
	RegisterEmailHandlers(reg, sender, cfg)

	payload := json.RawMessage(`{"to":"reset@example.com","raw_token":"raw-reset-token"}`)
	if err := reg.Dispatch(context.Background(), EmailSendPasswordReset, payload); err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	msg, ok := sender.last()
	if !ok {
		t.Fatal("expected one send")
	}
	if msg.To != "reset@example.com" {
		t.Fatalf("To = %q", msg.To)
	}
	if msg.Kind != email.KindPasswordReset {
		t.Fatalf("Kind = %q", msg.Kind)
	}
	if msg.Subject != "Password reset" {
		t.Fatalf("Subject = %q", msg.Subject)
	}
	if !strings.Contains(msg.TextBody, "raw-reset-token") {
		t.Fatalf("body should contain token, got %q", msg.TextBody)
	}
}

func TestRegisterEmailHandlers_OrganizationInvitationDispatchesToSender(t *testing.T) {
	t.Parallel()
	sender := &recordingSender{}
	cfg := &config.Config{}
	reg := queue.NewRegistry()
	RegisterEmailHandlers(reg, sender, cfg)

	payload := json.RawMessage(`{"to":"inv@example.com","raw_token":"tok","organization_name":"Acme"}`)
	if err := reg.Dispatch(context.Background(), EmailSendOrganizationInvitation, payload); err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	msg, ok := sender.last()
	if !ok {
		t.Fatal("expected one send")
	}
	if msg.Kind != email.KindOrganizationInvitation {
		t.Fatalf("Kind = %q", msg.Kind)
	}
	if msg.To != "inv@example.com" {
		t.Fatalf("To = %q", msg.To)
	}
	if !strings.Contains(msg.TextBody, "tok") {
		t.Fatalf("body missing token: %q", msg.TextBody)
	}
}

func TestRegisterEmailHandlers_InvalidPayload(t *testing.T) {
	t.Parallel()
	sender := &recordingSender{}
	cfg := &config.Config{}
	reg := queue.NewRegistry()
	RegisterEmailHandlers(reg, sender, cfg)

	err := reg.Dispatch(context.Background(), EmailSendVerification, json.RawMessage(`{"to":""}`))
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, queue.ErrInvalidPayload) {
		t.Fatalf("expected ErrInvalidPayload wrap, got %v", err)
	}
	if _, ok := sender.last(); ok {
		t.Fatal("sender should not be called")
	}
}
