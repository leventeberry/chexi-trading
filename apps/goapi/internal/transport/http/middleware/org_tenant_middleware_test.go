package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"goapi/internal/events"
	"goapi/internal/rbac"
	"goapi/models"
	"goapi/repositories"
)

// fakeOrganizationRepository implements OrganizationRepository for middleware tests only.
type fakeOrganizationRepository struct {
	org        *models.Organization
	membership *models.OrganizationMembership
	memErr     error
}

func (f *fakeOrganizationRepository) CreateWithOwnerMembership(org *models.Organization, membership *models.OrganizationMembership) error {
	panic("not implemented")
}
func (f *fakeOrganizationRepository) FindByID(id uuid.UUID) (*models.Organization, error) {
	if f.org != nil && f.org.ID == id {
		return f.org, nil
	}
	return nil, repositories.ErrOrganizationNotFound
}
func (f *fakeOrganizationRepository) FindBySlugLower(slug string) (*models.Organization, error) {
	if f.org != nil && f.org.Slug == slug {
		return f.org, nil
	}
	return nil, repositories.ErrOrganizationNotFound
}
func (f *fakeOrganizationRepository) ExistsSlugLower(slug string) (bool, error) {
	panic("not implemented")
}
func (f *fakeOrganizationRepository) ExistsSlugLowerForOtherOrg(slug string, excludeOrgID uuid.UUID) (bool, error) {
	panic("not implemented")
}
func (f *fakeOrganizationRepository) ListByUserID(userID uuid.UUID) ([]models.Organization, error) {
	panic("not implemented")
}
func (f *fakeOrganizationRepository) Update(org *models.Organization) error { panic("not implemented") }
func (f *fakeOrganizationRepository) FindMembership(orgID, userID uuid.UUID) (*models.OrganizationMembership, error) {
	if f.memErr != nil {
		return nil, f.memErr
	}
	if f.membership != nil && f.membership.OrganizationID == orgID && f.membership.UserID == userID {
		return f.membership, nil
	}
	return nil, repositories.ErrOrganizationMembershipNotFound
}
func (f *fakeOrganizationRepository) ListMembershipsForOrg(orgID uuid.UUID) ([]models.OrganizationMembership, error) {
	panic("not implemented")
}
func (f *fakeOrganizationRepository) DeletePendingInvitationsForEmail(orgID uuid.UUID, email string) error {
	panic("not implemented")
}
func (f *fakeOrganizationRepository) CreateInvitation(inv *models.OrganizationInvitation) error {
	panic("not implemented")
}
func (f *fakeOrganizationRepository) FindInvitationByTokenHash(tokenHash string) (*models.OrganizationInvitation, error) {
	panic("not implemented")
}
func (f *fakeOrganizationRepository) ListInvitationsByOrg(orgID uuid.UUID) ([]models.OrganizationInvitation, error) {
	panic("not implemented")
}
func (f *fakeOrganizationRepository) MarkInvitationAccepted(id uuid.UUID, at time.Time) error {
	panic("not implemented")
}
func (f *fakeOrganizationRepository) DeleteMembership(orgID, userID uuid.UUID) error {
	panic("not implemented")
}
func (f *fakeOrganizationRepository) CountMembersWithRole(orgID uuid.UUID, role string) (int64, error) {
	panic("not implemented")
}

func TestOrganizationTenantMiddleware_SetsGinAndRequestContext(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	orgID := uuid.New()
	userID := uuid.New()
	repo := &fakeOrganizationRepository{
		org: &models.Organization{ID: orgID, Slug: "acme", Name: "Acme"},
		membership: &models.OrganizationMembership{
			OrganizationID: orgID,
			UserID:         userID,
			Role:           rbac.OrgRoleMember,
		},
	}

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("userID", userID.String())
		c.Set("role", "user")
		c.Next()
	})
	r.Use(OrganizationTenantMiddleware(repo, OrganizationTenantOptions{ParamName: "id"}))
	r.GET("/orgs/:id/ctx", func(c *gin.Context) {
		gt, ok := OrganizationTenantFromGin(c)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"err": "no gin tenant"})
			return
		}
		rt, ok := events.OrganizationTenantFromContext(c.Request.Context())
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"err": "no ctx tenant"})
			return
		}
		if gt.OrganizationID != rt.OrganizationID {
			c.JSON(http.StatusInternalServerError, gin.H{"err": "mismatch"})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"org_id":        gt.OrganizationID.String(),
			"effective":     gt.EffectiveOrgRole,
			"membership_ok": gt.Membership != nil,
		})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/orgs/"+orgID.String()+"/ctx", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatalf("json: %v", err)
	}
	if payload["effective"] != rbac.OrgRoleMember {
		t.Fatalf("role: %v", payload["effective"])
	}
}

func TestOrganizationTenantMiddleware_GlobalAdminBypass_NoMembership(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	orgID := uuid.New()
	adminID := uuid.New()
	repo := &fakeOrganizationRepository{
		org:    &models.Organization{ID: orgID, Slug: "solo", Name: "Solo"},
		memErr: repositories.ErrOrganizationMembershipNotFound,
	}

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("userID", adminID.String())
		c.Set("role", rbac.RoleAdmin.String())
		c.Next()
	})
	r.Use(OrganizationTenantMiddleware(repo, OrganizationTenantOptions{
		ParamName:              "id",
		AllowGlobalAdminBypass: true,
	}))
	r.GET("/orgs/:id/x", func(c *gin.Context) {
		tnt, _ := OrganizationTenantFromGin(c)
		if !tnt.GlobalAdminBypass {
			c.JSON(http.StatusExpectationFailed, gin.H{"err": "no bypass"})
			return
		}
		if tnt.Membership != nil {
			c.JSON(http.StatusExpectationFailed, gin.H{"err": "unexpected membership"})
			return
		}
		c.Status(http.StatusNoContent)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/orgs/"+orgID.String()+"/x", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d body=%s", w.Code, w.Body.String())
	}
}
