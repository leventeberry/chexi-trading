package middleware

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"goapi/config"
	"goapi/internal/events"
)

type captureRecorder struct {
	last EventSnapshot
	err  error
}

// EventSnapshot is a shallow copy suitable for assertions.
type EventSnapshot struct {
	EventType   string
	RequestID   string
	ActorUserID *uuid.UUID
	Metadata    map[string]interface{}
}

func (c *captureRecorder) Record(ctx context.Context, e events.Event) error {
	var meta map[string]interface{}
	if len(e.Metadata) > 0 {
		_ = json.Unmarshal(e.Metadata, &meta)
	}
	var actorCopy *uuid.UUID
	if e.ActorUserID != nil {
		u := *e.ActorUserID
		actorCopy = &u
	}
	c.last = EventSnapshot{
		EventType:   e.EventType,
		RequestID:   e.RequestID,
		ActorUserID: actorCopy,
		Metadata:    meta,
	}
	return c.err
}

func auditEnabledConfig(mutatingOnly bool) *config.Config {
	cfg := &config.Config{}
	cfg.Audit.HTTPEnabled = true
	cfg.Audit.HTTPMutatingOnly = mutatingOnly
	return cfg
}

func TestHTTPEventAudit_Disabled_NoRecord(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	cases := []struct {
		name string
		cfg  *config.Config
	}{
		{name: "nil cfg", cfg: nil},
		{name: "enabled false", cfg: func() *config.Config {
			c := &config.Config{}
			c.Audit.HTTPEnabled = false
			return c
		}()},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cr := &captureRecorder{}
			w := httptest.NewRecorder()
			_, eng := gin.CreateTestContext(w)
			eng.Use(HTTPEventAudit(cr, tc.cfg))
			eng.POST("/x", func(c *gin.Context) { c.Status(http.StatusCreated) })

			req := httptest.NewRequest(http.MethodPost, "/x", nil)
			eng.ServeHTTP(w, req)

			if cr.last.EventType != "" {
				t.Fatalf("expected no record, got %#v", cr.last)
			}
		})
	}
}

func TestHTTPEventAudit_EnabledNilRecorder_NoPanic(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	_, eng := gin.CreateTestContext(w)
	eng.Use(HTTPEventAudit(nil, auditEnabledConfig(true)))
	eng.GET("/ok", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) })

	req := httptest.NewRequest(http.MethodGet, "/ok", nil)
	eng.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("code = %d", w.Code)
	}
}

func TestHTTPEventAudit_MutatingOnly_SkipsSafeMethods(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	safe := []string{http.MethodGet, http.MethodHead, http.MethodOptions}
	for _, m := range safe {
		m := m
		t.Run(m, func(t *testing.T) {
			t.Parallel()
			cr := &captureRecorder{}
			w := httptest.NewRecorder()
			_, eng := gin.CreateTestContext(w)
			eng.Use(HTTPEventAudit(cr, auditEnabledConfig(true)))
			eng.Handle(m, "/p", func(c *gin.Context) { c.Status(http.StatusOK) })

			req := httptest.NewRequest(m, "/p", nil)
			eng.ServeHTTP(w, req)

			if cr.last.EventType != "" {
				t.Fatalf("safe method %s should skip audit, recorded %#v", m, cr.last)
			}
		})
	}
}

func TestHTTPEventAudit_MutatingOnly_RecordsPost(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	cr := &captureRecorder{}
	w := httptest.NewRecorder()
	_, eng := gin.CreateTestContext(w)
	eng.Use(HTTPEventAudit(cr, auditEnabledConfig(true)))
	eng.POST("/save", func(c *gin.Context) { c.Status(http.StatusAccepted) })

	req := httptest.NewRequest(http.MethodPost, "/save", nil)
	eng.ServeHTTP(w, req)

	if cr.last.EventType != "http.request" {
		t.Fatalf("EventType = %q", cr.last.EventType)
	}
	sc, ok := cr.last.Metadata["status_code"].(float64)
	if !ok || int(sc) != http.StatusAccepted {
		t.Fatalf("status_code metadata = %#v", cr.last.Metadata["status_code"])
	}
	method, ok := cr.last.Metadata["method"].(string)
	if !ok || method != http.MethodPost {
		t.Fatalf("method = %#v", cr.last.Metadata["method"])
	}
}

func TestHTTPEventAudit_AllMethodsIncludesGET(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	cr := &captureRecorder{}
	w := httptest.NewRecorder()
	_, eng := gin.CreateTestContext(w)
	eng.Use(HTTPEventAudit(cr, auditEnabledConfig(false)))
	eng.GET("/g", func(c *gin.Context) { c.Status(http.StatusOK) })

	req := httptest.NewRequest(http.MethodGet, "/g", nil)
	eng.ServeHTTP(w, req)

	if cr.last.EventType != "http.request" {
		t.Fatal("GET should be recorded when HTTPMutatingOnly=false")
	}
}

func TestHTTPEventAudit_PathIncludesQuery(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	cr := &captureRecorder{}
	w := httptest.NewRecorder()
	_, eng := gin.CreateTestContext(w)
	eng.Use(HTTPEventAudit(cr, auditEnabledConfig(true)))
	eng.POST("/api", func(c *gin.Context) { c.Status(http.StatusOK) })

	req := httptest.NewRequest(http.MethodPost, "/api?foo=bar&x=1", nil)
	eng.ServeHTTP(w, req)

	p, _ := cr.last.Metadata["path"].(string)
	if !strings.Contains(p, "?") || !strings.Contains(p, "foo=bar") {
		t.Fatalf("path metadata = %q", p)
	}
}

func TestHTTPEventAudit_PathRedactsSensitiveQueryValues(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	cr := &captureRecorder{}
	w := httptest.NewRecorder()
	_, eng := gin.CreateTestContext(w)
	eng.Use(HTTPEventAudit(cr, auditEnabledConfig(true)))
	eng.POST("/oauth/callback", func(c *gin.Context) { c.Status(http.StatusOK) })

	req := httptest.NewRequest(http.MethodPost, "/oauth/callback?code=abc123&state=xyz&foo=bar", nil)
	eng.ServeHTTP(w, req)

	p, _ := cr.last.Metadata["path"].(string)
	if strings.Contains(p, "abc123") || strings.Contains(p, "xyz") {
		t.Fatalf("sensitive query values leaked in metadata path: %q", p)
	}
	if !strings.Contains(p, "code=%5Bredacted%5D") || !strings.Contains(p, "state=%5Bredacted%5D") {
		t.Fatalf("expected redacted query values, got %q", p)
	}
	if !strings.Contains(p, "foo=bar") {
		t.Fatalf("non-sensitive query value should remain, got %q", p)
	}
}

func TestHTTPEventAudit_PathRedactsTokenQueryParam(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	cr := &captureRecorder{}
	w := httptest.NewRecorder()
	_, eng := gin.CreateTestContext(w)
	eng.Use(HTTPEventAudit(cr, auditEnabledConfig(true)))
	eng.POST("/hook", func(c *gin.Context) { c.Status(http.StatusOK) })

	req := httptest.NewRequest(http.MethodPost, "/hook?token=never_store_this&trace=1", nil)
	eng.ServeHTTP(w, req)

	p, _ := cr.last.Metadata["path"].(string)
	if strings.Contains(p, "never_store_this") {
		t.Fatalf("token query leaked in metadata path: %q", p)
	}
	if !strings.Contains(p, "trace=1") {
		t.Fatalf("expected non-sensitive param preserved: %q", p)
	}
}

func TestHTTPEventAudit_RequestIDFromContext(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	cr := &captureRecorder{}
	w := httptest.NewRecorder()
	_, eng := gin.CreateTestContext(w)
	eng.Use(RequestID())
	eng.Use(HTTPEventAudit(cr, auditEnabledConfig(true)))
	eng.POST("/tracked", func(c *gin.Context) { c.Status(http.StatusOK) })

	req := httptest.NewRequest(http.MethodPost, "/tracked", nil)
	req.Header.Set("X-Request-ID", "upstream-rid-99")
	eng.ServeHTTP(w, req)

	if cr.last.RequestID != "upstream-rid-99" {
		t.Fatalf("RequestID on event = %q", cr.last.RequestID)
	}
}

func TestHTTPEventAudit_ActorFromUserIDString(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	uid := uuid.MustParse("33333333-3333-4333-8333-333333333333")
	cr := &captureRecorder{}
	w := httptest.NewRecorder()
	_, eng := gin.CreateTestContext(w)
	eng.Use(HTTPEventAudit(cr, auditEnabledConfig(true)))
	eng.DELETE("/d", func(c *gin.Context) {
		c.Set("userID", uid.String())
		c.Status(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodDelete, "/d", nil)
	eng.ServeHTTP(w, req)

	if cr.last.ActorUserID == nil || *cr.last.ActorUserID != uid {
		t.Fatalf("ActorUserID = %v", cr.last.ActorUserID)
	}
}

func TestHTTPEventAudit_ActorOnlyValidUUIDString(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name   string
		userID interface{}
		wantNi bool // want nil actor
	}{
		{name: "missing", userID: nil, wantNi: true},
		{name: "invalid string", userID: "not-a-uuid", wantNi: true},
		{name: "wrong type", userID: 42, wantNi: true},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cr := &captureRecorder{}
			w := httptest.NewRecorder()
			_, eng := gin.CreateTestContext(w)
			eng.Use(HTTPEventAudit(cr, auditEnabledConfig(true)))
			eng.PATCH("/z", func(c *gin.Context) {
				if tc.userID != nil {
					c.Set("userID", tc.userID)
				}
				c.Status(http.StatusOK)
			})

			req := httptest.NewRequest(http.MethodPatch, "/z", nil)
			eng.ServeHTTP(w, req)

			if tc.wantNi && cr.last.ActorUserID != nil {
				t.Fatalf("expected nil actor, got %v", cr.last.ActorUserID)
			}
		})
	}
}

func TestHTTPEventAudit_ContextActorNotUsed(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	id := uuid.MustParse("44444444-4444-4444-8444-444444444444")
	cr := &captureRecorder{}
	w := httptest.NewRecorder()
	_, eng := gin.CreateTestContext(w)
	eng.Use(HTTPEventAudit(cr, auditEnabledConfig(true)))
	eng.PUT("/only-ctx", func(c *gin.Context) {
		ctx := events.WithActorUserID(c.Request.Context(), id)
		c.Request = c.Request.WithContext(ctx)
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPut, "/only-ctx", nil)
	eng.ServeHTTP(w, req)

	if cr.last.ActorUserID != nil {
		t.Fatalf("middleware ignores context actor only; expected nil, got %v", cr.last.ActorUserID)
	}
}

func TestHTTPEventAudit_RecorderError_DoesNotChangeHTTPResponse(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	cr := &captureRecorder{err: errors.New("persist failed")}
	w := httptest.NewRecorder()
	_, eng := gin.CreateTestContext(w)
	eng.Use(HTTPEventAudit(cr, auditEnabledConfig(true)))
	eng.POST("/fine", func(c *gin.Context) {
		c.String(http.StatusOK, "done")
	})

	req := httptest.NewRequest(http.MethodPost, "/fine", strings.NewReader(""))
	req.Header.Set("Content-Type", "text/plain")
	eng.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 despite recorder error, got %d body=%s", w.Code, w.Body.String())
	}
	if body := w.Body.String(); body != "done" {
		t.Fatalf("body = %q", body)
	}
	if cr.last.EventType != "http.request" {
		t.Fatal("expected event attempted before RecordSafe swallowed error")
	}
}
