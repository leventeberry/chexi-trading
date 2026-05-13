package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"goapi/internal/events"
	"goapi/internal/rbac"
	"goapi/models"
)

func TestOrganizationTenantMiddleware_APIKeyOrgMismatch(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	orgID := uuid.New()
	otherOrg := uuid.New()
	keyUser := uuid.New()
	repo := &fakeOrganizationRepository{
		org: &models.Organization{ID: orgID, Slug: "acme", Name: "Acme"},
	}

	r := gin.New()
	r.Use(func(c *gin.Context) {
		ctx := events.WithAPIKeyTenantPrincipal(c.Request.Context(), events.APIKeyTenantPrincipal{
			OrganizationID:  otherOrg,
			KeyID:           uuid.New(),
			Scopes:          []string{rbac.OrgScopeRead},
			CreatedByUserID: keyUser,
		})
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	})
	r.Use(OrganizationTenantMiddleware(repo, OrganizationTenantOptions{ParamName: "id"}))
	r.GET("/orgs/:id/x", func(c *gin.Context) { c.Status(http.StatusOK) })

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/orgs/"+orgID.String()+"/x", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestRequireOrgAdmin_rejectsAPIKeyTenant(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.Use(func(c *gin.Context) {
		tnt := events.OrganizationTenant{
			OrganizationID: uuid.New(),
			APIKeyAuth:     true,
			APIKeyID:       uuid.New(),
			APIScopes:      []string{rbac.OrgScopeWrite},
		}
		c.Set(GinKeyOrganizationTenant, tnt)
		c.Next()
	})
	r.Use(RequireOrgAdmin())
	r.GET("/x", func(c *gin.Context) { c.Status(http.StatusOK) })

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for API key on RequireOrgAdmin, got %d", w.Code)
	}
}

func TestRequireOrgAccessRead_allowsReadScopeAPIKey(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.Use(func(c *gin.Context) {
		tnt := events.OrganizationTenant{
			OrganizationID: uuid.New(),
			APIKeyAuth:     true,
			APIScopes:      []string{rbac.OrgScopeRead},
		}
		c.Set(GinKeyOrganizationTenant, tnt)
		c.Next()
	})
	r.Use(RequireOrgAccessRead())
	r.GET("/x", func(c *gin.Context) { c.Status(http.StatusOK) })

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestRequireOrgAccessWrite_rejectsReadOnlyAPIKey(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.Use(func(c *gin.Context) {
		tnt := events.OrganizationTenant{
			OrganizationID: uuid.New(),
			APIKeyAuth:     true,
			APIScopes:      []string{rbac.OrgScopeRead},
		}
		c.Set(GinKeyOrganizationTenant, tnt)
		c.Next()
	})
	r.Use(RequireOrgAccessWrite())
	r.POST("/x", func(c *gin.Context) { c.Status(http.StatusOK) })

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/x", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}
}
