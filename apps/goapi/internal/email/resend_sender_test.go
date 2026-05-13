package email

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"goapi/config"
)

func TestResendSender_Send_Success(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %q", r.Method)
		}
		if r.Header.Get("Authorization") != "Bearer re_key" {
			t.Fatalf("Authorization = %q", r.Header.Get("Authorization"))
		}
		b, _ := io.ReadAll(r.Body)
		var m map[string]any
		if err := json.Unmarshal(b, &m); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if m["from"] != "from@example.com" {
			t.Fatalf("from = %v", m["from"])
		}
		if to, ok := m["to"].([]any); !ok || len(to) != 1 || to[0] != "a@b.com" {
			t.Fatalf("to = %v", m["to"])
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"x"}`))
	}))
	defer srv.Close()

	s := NewResendSender("re_key", "from@example.com", config.EnvironmentDevelopment, "")
	s.apiURL = srv.URL
	err := s.Send(context.Background(), Message{
		To:       "a@b.com",
		Subject:  "subj",
		TextBody: "hello",
		Kind:     KindVerification,
	})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
}

func TestResendSender_Send_RedirectInDev(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		var m map[string]any
		_ = json.Unmarshal(b, &m)
		if to, ok := m["to"].([]any); !ok || len(to) != 1 || to[0] != "inbox@example.com" {
			t.Fatalf("expected redirect to inbox, to = %v", m["to"])
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"x"}`))
	}))
	defer srv.Close()

	s := NewResendSender("re_key", "from@example.com", config.EnvironmentDevelopment, "inbox@example.com")
	s.apiURL = srv.URL
	err := s.Send(context.Background(), Message{To: "other@b.com", Subject: "s", TextBody: "b", Kind: KindVerification})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
}

func TestResendSender_Send_HTTPError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`{"message":"invalid"}`))
	}))
	defer srv.Close()

	s := NewResendSender("re_key", "from@example.com", config.EnvironmentDevelopment, "")
	s.apiURL = srv.URL
	err := s.Send(context.Background(), Message{To: "a@b.com", Subject: "s", TextBody: "b", Kind: KindVerification})
	if err == nil {
		t.Fatal("expected error")
	}
}
