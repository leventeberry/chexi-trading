package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"goapi/internal/events"
	"goapi/internal/rbac"
	"goapi/models"
	"goapi/repositories"
)

const (
	// GinKeyOrganizationTenant stores events.OrganizationTenant after OrganizationTenantMiddleware.
	GinKeyOrganizationTenant = "organizationTenant"
)

// OrganizationTenantOptions configures OrganizationTenantMiddleware.
type OrganizationTenantOptions struct {
	// ParamName is the route parameter holding organization UUID or slug (default "id").
	ParamName string
	// AllowGlobalAdminBypass when true allows JWT role admin to proceed without org membership.
	// Must be opted in per route group — never implicit.
	AllowGlobalAdminBypass bool
}

// OrganizationTenantMiddleware resolves the organization from :id (UUID or slug), verifies membership,
// and stores tenant metadata on Gin and request context.
func OrganizationTenantMiddleware(repo repositories.OrganizationRepository, opts OrganizationTenantOptions) gin.HandlerFunc {
	param := opts.ParamName
	if param == "" {
		param = "id"
	}
	return func(c *gin.Context) {
		raw := c.Param(param)
		if raw == "" {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "Missing organization identifier"})
			return
		}

		var org *models.Organization
		var err error
		if oid, perr := uuid.Parse(raw); perr == nil {
			org, err = repo.FindByID(oid)
		} else {
			org, err = repo.FindBySlugLower(raw)
		}
		if err != nil {
			if err == repositories.ErrOrganizationNotFound {
				c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": "Organization not found"})
				return
			}
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
			return
		}

		apiKeyPrincipal, hasAPIKey := events.APIKeyTenantPrincipalFromContext(c.Request.Context())
		if hasAPIKey {
			if org.ID != apiKeyPrincipal.OrganizationID {
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "API key is not valid for this organization"})
				return
			}
			tenant := events.OrganizationTenant{
				OrganizationID:        org.ID,
				Membership:            nil,
				EffectiveOrgRole:      "",
				GlobalAdminBypass:     false,
				APIKeyAuth:            true,
				APIKeyID:              apiKeyPrincipal.KeyID,
				APIScopes:             append([]string(nil), apiKeyPrincipal.Scopes...),
				APIKeyCreatedByUserID: apiKeyPrincipal.CreatedByUserID,
			}
			c.Set(GinKeyOrganizationTenant, tenant)
			ctx := events.WithOrganizationTenant(c.Request.Context(), tenant)
			c.Request = c.Request.WithContext(ctx)
			c.Next()
			return
		}

		actorIDStr, okUser := c.Get("userID")
		if !okUser {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
			return
		}
		subject, typeOK := actorIDStr.(string)
		if !typeOK || subject == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
			return
		}
		actorID, err := uuid.Parse(subject)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
			return
		}

		roleStr, _ := c.Get("role")
		jwtRole, _ := roleStr.(string)

		var tenant events.OrganizationTenant
		tenant.OrganizationID = org.ID

		if opts.AllowGlobalAdminBypass && rbac.IsAdminRole(jwtRole) {
			tenant.GlobalAdminBypass = true
			tenant.Membership = nil
			tenant.EffectiveOrgRole = rbac.OrgRoleAdmin
		} else {
			m, err := repo.FindMembership(org.ID, actorID)
			if err != nil {
				if err == repositories.ErrOrganizationMembershipNotFound {
					c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "Not a member of this organization"})
					return
				}
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
				return
			}
			tenant.Membership = m
			tenant.EffectiveOrgRole = m.Role
		}

		c.Set(GinKeyOrganizationTenant, tenant)
		ctx := events.WithOrganizationTenant(c.Request.Context(), tenant)
		c.Request = c.Request.WithContext(ctx)

		c.Next()
	}
}

// OrganizationTenantFromGin returns tenant metadata set by OrganizationTenantMiddleware.
func OrganizationTenantFromGin(c *gin.Context) (events.OrganizationTenant, bool) {
	if v, ok := c.Get(GinKeyOrganizationTenant); ok {
		if t, ok := v.(events.OrganizationTenant); ok {
			return t, true
		}
	}
	return events.OrganizationTenant{}, false
}

// RequireOrgMember ensures the request completed tenant resolution with membership or explicit admin bypass.
func RequireOrgMember() gin.HandlerFunc {
	return func(c *gin.Context) {
		t, ok := OrganizationTenantFromGin(c)
		if !ok {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "Tenant context missing"})
			return
		}
		if t.Membership == nil && !t.GlobalAdminBypass {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "Not a member of this organization"})
			return
		}
		c.Next()
	}
}

// RequireOrgAccessRead allows JWT org members (or global admin bypass) or org API keys with org:read/org:write.
func RequireOrgAccessRead() gin.HandlerFunc {
	return func(c *gin.Context) {
		t, ok := OrganizationTenantFromGin(c)
		if !ok {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "Tenant context missing"})
			return
		}
		if t.GlobalAdminBypass {
			c.Next()
			return
		}
		if t.Membership != nil {
			c.Next()
			return
		}
		if t.APIKeyAuth && rbac.APIScopesContainRead(t.APIScopes) {
			c.Next()
			return
		}
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "Insufficient privileges for this operation"})
	}
}

// RequireOrgAccessWrite allows JWT org members (or global admin bypass) or org API keys with org:write.
func RequireOrgAccessWrite() gin.HandlerFunc {
	return func(c *gin.Context) {
		t, ok := OrganizationTenantFromGin(c)
		if !ok {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "Tenant context missing"})
			return
		}
		if t.GlobalAdminBypass {
			c.Next()
			return
		}
		if t.Membership != nil {
			c.Next()
			return
		}
		if t.APIKeyAuth && rbac.APIScopesContainWrite(t.APIScopes) {
			c.Next()
			return
		}
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "Insufficient privileges for this operation"})
	}
}

// RequireOrgAdmin requires owner or admin (tenant role), or synthetic admin from explicit global bypass.
func RequireOrgAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		t, ok := OrganizationTenantFromGin(c)
		if !ok {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "Tenant context missing"})
			return
		}
		if t.APIKeyAuth {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "Insufficient privileges for this operation"})
			return
		}
		if !rbac.OrgRoleCanManageOrganization(t.EffectiveOrgRole) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "Insufficient privileges for this operation"})
			return
		}
		c.Next()
	}
}

// RequireOrgOwner requires the tenant membership role owner (global admin bypass does not grant owner).
func RequireOrgOwner() gin.HandlerFunc {
	return func(c *gin.Context) {
		t, ok := OrganizationTenantFromGin(c)
		if !ok {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "Tenant context missing"})
			return
		}
		if t.GlobalAdminBypass {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "Insufficient privileges for this operation"})
			return
		}
		if t.Membership == nil || t.Membership.Role != rbac.OrgRoleOwner {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "Insufficient privileges for this operation"})
			return
		}
		c.Next()
	}
}
