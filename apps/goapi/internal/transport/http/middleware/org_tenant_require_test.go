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

func TestRequireOrgAdmin_allowsOwnerAndAdminRoles(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name       string
		role       string
		wantStatus int
	}{
		{name: "member_forbidden", role: rbac.OrgRoleMember, wantStatus: http.StatusForbidden},
		{name: "admin_ok", role: rbac.OrgRoleAdmin, wantStatus: http.StatusOK},
		{name: "owner_ok", role: rbac.OrgRoleOwner, wantStatus: http.StatusOK},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			r := gin.New()
			r.Use(func(c *gin.Context) {
				uid := uuid.New()
				tnt := events.OrganizationTenant{
					OrganizationID:   uuid.New(),
					EffectiveOrgRole: tc.role,
					Membership: &models.OrganizationMembership{
						OrganizationID: uuid.New(),
						UserID:         uid,
						Role:           tc.role,
					},
				}
				c.Set(GinKeyOrganizationTenant, tnt)
				c.Next()
			})
			r.Use(RequireOrgAdmin())
			r.GET("/x", func(c *gin.Context) { c.Status(http.StatusOK) })

			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/x", nil)
			r.ServeHTTP(w, req)
			if w.Code != tc.wantStatus {
				t.Fatalf("expected %d, got %d body=%s", tc.wantStatus, w.Code, w.Body.String())
			}
		})
	}
}

func TestRequireOrgOwner_rejectsGlobalAdminBypassSyntheticAdmin(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.Use(func(c *gin.Context) {
		tnt := events.OrganizationTenant{
			OrganizationID:    uuid.New(),
			EffectiveOrgRole:  rbac.OrgRoleAdmin,
			GlobalAdminBypass: true,
			Membership:        nil,
		}
		c.Set(GinKeyOrganizationTenant, tnt)
		c.Next()
	})
	r.Use(RequireOrgOwner())
	r.GET("/x", func(c *gin.Context) { c.Status(http.StatusOK) })

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for bypass on RequireOrgOwner, got %d", w.Code)
	}
}

func TestRequireOrgOwner_allowsOwnerMembership(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.Use(func(c *gin.Context) {
		oid := uuid.New()
		uid := uuid.New()
		tnt := events.OrganizationTenant{
			OrganizationID:   oid,
			EffectiveOrgRole: rbac.OrgRoleOwner,
			Membership: &models.OrganizationMembership{
				OrganizationID: oid,
				UserID:         uid,
				Role:           rbac.OrgRoleOwner,
			},
		}
		c.Set(GinKeyOrganizationTenant, tnt)
		c.Next()
	})
	r.Use(RequireOrgOwner())
	r.GET("/x", func(c *gin.Context) { c.Status(http.StatusOK) })

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
}
