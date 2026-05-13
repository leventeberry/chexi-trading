package events

import (
	"context"

	"github.com/google/uuid"
	"goapi/models"
)

type requestIDCtxKey struct{}

type actorUserCtxKey struct{}

type orgTenantCtxKey struct{}

type apiKeyTenantPrincipalCtxKey struct{}

// APIKeyTenantPrincipal is set when the request authenticated with an organization API key.
type APIKeyTenantPrincipal struct {
	OrganizationID  uuid.UUID
	KeyID           uuid.UUID
	Scopes          []string
	CreatedByUserID uuid.UUID
}

// WithAPIKeyTenantPrincipal attaches a verified org API key identity to ctx.
func WithAPIKeyTenantPrincipal(parent context.Context, p APIKeyTenantPrincipal) context.Context {
	return context.WithValue(parent, apiKeyTenantPrincipalCtxKey{}, p)
}

// APIKeyTenantPrincipalFromContext returns the API key principal when present.
func APIKeyTenantPrincipalFromContext(ctx context.Context) (APIKeyTenantPrincipal, bool) {
	v, ok := ctx.Value(apiKeyTenantPrincipalCtxKey{}).(APIKeyTenantPrincipal)
	return v, ok
}

// OrganizationTenant holds resolved tenant identity for org-scoped handlers (set by middleware).
type OrganizationTenant struct {
	OrganizationID uuid.UUID
	// Membership is nil when AllowGlobalAdminBypass allowed access without membership.
	Membership *models.OrganizationMembership
	// EffectiveOrgRole is the membership role, or a synthetic admin role when GlobalAdminBypass is true.
	EffectiveOrgRole  string
	GlobalAdminBypass bool
	// APIKeyAuth is true when OrganizationTenantMiddleware resolved the tenant via an org API key.
	APIKeyAuth bool
	APIKeyID   uuid.UUID
	APIScopes  []string
	// APIKeyCreatedByUserID is set when APIKeyAuth; used to attribute writes (e.g. notes) to a real users row.
	APIKeyCreatedByUserID uuid.UUID
}

// WithOrganizationTenant stores tenant metadata on ctx for downstream services/handlers.
func WithOrganizationTenant(parent context.Context, t OrganizationTenant) context.Context {
	return context.WithValue(parent, orgTenantCtxKey{}, t)
}

// OrganizationTenantFromContext returns tenant metadata when middleware set it.
func OrganizationTenantFromContext(ctx context.Context) (OrganizationTenant, bool) {
	v, ok := ctx.Value(orgTenantCtxKey{}).(OrganizationTenant)
	return v, ok
}

// WithRequestID returns ctx storing request correlation ID (from middleware).
func WithRequestID(parent context.Context, id string) context.Context {
	return context.WithValue(parent, requestIDCtxKey{}, id)
}

// RequestIDFromContext returns the request ID if present.
func RequestIDFromContext(ctx context.Context) string {
	s, _ := ctx.Value(requestIDCtxKey{}).(string)
	return s
}

// WithActorUserID stores the authenticated user id for audit actors (middleware).
func WithActorUserID(parent context.Context, userID uuid.UUID) context.Context {
	return context.WithValue(parent, actorUserCtxKey{}, userID)
}

// ActorUserIDFromContext returns the authenticated user id when middleware set it.
func ActorUserIDFromContext(ctx context.Context) (uuid.UUID, bool) {
	v, ok := ctx.Value(actorUserCtxKey{}).(uuid.UUID)
	return v, ok
}
