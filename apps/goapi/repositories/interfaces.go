package repositories

import (
	"time"

	"github.com/google/uuid"
	"goapi/models"
)

// OrganizationRepository persists organizations and tenant memberships.
type OrganizationRepository interface {
	CreateWithOwnerMembership(org *models.Organization, membership *models.OrganizationMembership) error
	FindByID(id uuid.UUID) (*models.Organization, error)
	// FindBySlugLower resolves an organization by case-insensitive slug match.
	FindBySlugLower(slug string) (*models.Organization, error)
	ExistsSlugLower(slug string) (bool, error)
	ExistsSlugLowerForOtherOrg(slug string, excludeOrgID uuid.UUID) (bool, error)
	ListByUserID(userID uuid.UUID) ([]models.Organization, error)
	Update(org *models.Organization) error
	FindMembership(orgID, userID uuid.UUID) (*models.OrganizationMembership, error)
	ListMembershipsForOrg(orgID uuid.UUID) ([]models.OrganizationMembership, error)

	DeletePendingInvitationsForEmail(orgID uuid.UUID, email string) error
	CreateInvitation(inv *models.OrganizationInvitation) error
	FindInvitationByTokenHash(tokenHash string) (*models.OrganizationInvitation, error)
	ListInvitationsByOrg(orgID uuid.UUID) ([]models.OrganizationInvitation, error)
	MarkInvitationAccepted(id uuid.UUID, at time.Time) error

	DeleteMembership(orgID, userID uuid.UUID) error
	CountMembersWithRole(orgID uuid.UUID, role string) (int64, error)
}

// OrganizationNoteRepository persists notes scoped to an organization (tenant sub-resource).
type OrganizationNoteRepository interface {
	Create(note *models.OrganizationNote) error
	ListByOrganization(orgID uuid.UUID) ([]models.OrganizationNote, error)
	DeleteByOrganizationAndID(orgID, noteID uuid.UUID) error
}

// OrganizationAPIKeyRepository persists organization API keys (secrets stored hashed only).
type OrganizationAPIKeyRepository interface {
	Create(row *models.OrganizationAPIKey) error
	ListByOrganization(orgID uuid.UUID) ([]models.OrganizationAPIKey, error)
	FindActiveByKeyHash(keyHash string) (*models.OrganizationAPIKey, error)
	Revoke(orgID, keyID uuid.UUID, at time.Time) error
	TouchLastUsed(id uuid.UUID, at time.Time) error
}

// OrganizationWebhookRepository persists org webhook subscriptions and delivery rows.
type OrganizationWebhookRepository interface {
	CreateWebhook(row *models.OrganizationWebhook) error
	UpdateWebhook(row *models.OrganizationWebhook) error
	DeleteWebhook(orgID, webhookID uuid.UUID) error
	ListWebhooksByOrganization(orgID uuid.UUID) ([]models.OrganizationWebhook, error)
	FindWebhookByOrganizationAndID(orgID, webhookID uuid.UUID) (*models.OrganizationWebhook, error)
	ListEnabledWebhooksForEvent(orgID uuid.UUID, eventType string) ([]models.OrganizationWebhook, error)
	// FindWebhookByID loads a webhook by primary key (worker use).
	FindWebhookByID(webhookID uuid.UUID) (*models.OrganizationWebhook, error)

	CreateDelivery(row *models.OrganizationWebhookDelivery) error
	UpdateDelivery(row *models.OrganizationWebhookDelivery) error
	FindDeliveryByID(id uuid.UUID) (*models.OrganizationWebhookDelivery, error)
	ListDeliveriesByWebhook(orgID, webhookID uuid.UUID, limit int) ([]models.OrganizationWebhookDelivery, error)
}

// UserRepository defines the interface for user data operations
type UserRepository interface {
	Create(user *models.User) error
	FindByID(id uuid.UUID) (*models.User, error)
	FindByEmail(email string) (*models.User, error)
	FindAll() ([]models.User, error)
	FindAllWithPagination(page, pageSize int) ([]models.User, int64, error)
	Update(user *models.User) error
	Delete(id uuid.UUID) error
	ExistsByEmail(email string) (bool, error)
	// ExistsAnyAdmin reports whether any user has role admin (bootstrap idempotency).
	ExistsAnyAdmin() (bool, error)
}

// UserSettingsRepository persists 1:1 user preferences.
type UserSettingsRepository interface {
	FindByUserID(userID uuid.UUID) (*models.UserSettings, error)
	Upsert(settings *models.UserSettings) error
}
