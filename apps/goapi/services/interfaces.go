package services

import (
	"context"

	"github.com/google/uuid"
	"goapi/models"
)

// TradePlanService manages advisory manual trade plans for the authenticated user.
type TradePlanService interface {
	CreateTradePlan(ctx context.Context, userID uuid.UUID, input *CreateTradePlanInput) (*TradePlanDTO, error)
	ListTradePlans(ctx context.Context, userID uuid.UUID) ([]TradePlanDTO, error)
	GetTradePlan(ctx context.Context, userID, id uuid.UUID) (*TradePlanDTO, error)
}

// UserService defines the interface for user business logic
type UserService interface {
	CreateUser(ctx context.Context, input *CreateUserInput, actorID uuid.UUID, actorRole string) (*models.User, error)
	GetUserByID(ctx context.Context, id uuid.UUID, actorID uuid.UUID, actorRole string) (*models.User, error)
	GetUserByEmail(ctx context.Context, email string) (*models.User, error)
	GetAllUsers(ctx context.Context, actorID uuid.UUID, actorRole string) ([]models.User, error)
	GetAllUsersPaginated(ctx context.Context, params *PaginationParams, actorID uuid.UUID, actorRole string) ([]models.User, int64, error)
	UpdateUser(ctx context.Context, id uuid.UUID, input *UpdateUserInput, actorID uuid.UUID, actorRole string) (*models.User, error)
	DeleteUser(ctx context.Context, id uuid.UUID, actorID uuid.UUID, actorRole string) error
	ValidateRole(role string) bool

	GetMyProfile(ctx context.Context, actorID uuid.UUID) (*MeProfileDTO, error)
	PatchMyProfile(ctx context.Context, actorID uuid.UUID, input *PatchMeProfileInput) (*MeProfileDTO, error)
}

// OrganizationNoteService is a minimal org-scoped sub-resource (tenant pattern example).
type OrganizationNoteService interface {
	CreateOrganizationNote(ctx context.Context, orgID uuid.UUID, actorID uuid.UUID, input *CreateOrganizationNoteInput) (*OrganizationNoteDTO, error)
	ListOrganizationNotes(ctx context.Context, orgID uuid.UUID) ([]OrganizationNoteDTO, error)
	DeleteOrganizationNote(ctx context.Context, orgID uuid.UUID, noteID uuid.UUID) error
}

// OrganizationAPIKeyService manages org-scoped API keys (create/list/revoke and verification).
type OrganizationAPIKeyService interface {
	CreateOrganizationAPIKey(ctx context.Context, orgID uuid.UUID, actorID uuid.UUID, input *CreateOrganizationAPIKeyInput) (*OrganizationAPIKeyCreatedDTO, error)
	ListOrganizationAPIKeys(ctx context.Context, orgID uuid.UUID, actorID uuid.UUID) ([]OrganizationAPIKeyListDTO, error)
	RevokeOrganizationAPIKey(ctx context.Context, orgID uuid.UUID, keyID uuid.UUID, actorID uuid.UUID) error
	AuthenticateOrganizationAPIKey(ctx context.Context, rawKey string) (*OrganizationAPIKeyPrincipal, error)
}

// OrganizationWebhookService manages org-scoped outbound webhooks and emits domain events.
type OrganizationWebhookService interface {
	CreateOrganizationWebhook(ctx context.Context, orgID uuid.UUID, actorID uuid.UUID, input *CreateOrganizationWebhookInput) (*OrganizationWebhookCreatedDTO, error)
	ListOrganizationWebhooks(ctx context.Context, orgID uuid.UUID, actorID uuid.UUID) ([]OrganizationWebhookListDTO, error)
	UpdateOrganizationWebhook(ctx context.Context, orgID uuid.UUID, webhookID uuid.UUID, actorID uuid.UUID, input *UpdateOrganizationWebhookInput) (*OrganizationWebhookPatchDTO, error)
	DeleteOrganizationWebhook(ctx context.Context, orgID uuid.UUID, webhookID uuid.UUID, actorID uuid.UUID) error
	ListOrganizationWebhookDeliveries(ctx context.Context, orgID uuid.UUID, webhookID uuid.UUID, actorID uuid.UUID, limit int) ([]OrganizationWebhookDeliveryDTO, error)

	// EmitOrganizationWebhookEvent enqueues deliveries for all matching subscriptions (no HTTP in caller).
	EmitOrganizationWebhookEvent(ctx context.Context, orgID uuid.UUID, eventType string, payload map[string]interface{})
}

// OrganizationService manages tenant organizations and membership authorization.
type OrganizationService interface {
	// ResolveOrganizationRouteID parses :id as a UUID or resolves an organization by slug (case-insensitive).
	ResolveOrganizationRouteID(ctx context.Context, raw string) (uuid.UUID, error)
	CreateOrganization(ctx context.Context, actorID uuid.UUID, input *CreateOrganizationInput) (*OrganizationDTO, error)
	ListOrganizations(ctx context.Context, actorID uuid.UUID) ([]OrganizationDTO, error)
	GetOrganization(ctx context.Context, id uuid.UUID, actorID uuid.UUID) (*OrganizationDTO, error)
	UpdateOrganization(ctx context.Context, id uuid.UUID, actorID uuid.UUID, input *UpdateOrganizationInput) (*OrganizationDTO, error)
	ListOrganizationMembers(ctx context.Context, id uuid.UUID, actorID uuid.UUID) ([]OrganizationMemberDTO, error)

	CreateOrganizationInvitation(ctx context.Context, orgID uuid.UUID, actorID uuid.UUID, input *CreateOrganizationInvitationInput) (*OrganizationInvitationDTO, error)
	ListOrganizationInvitations(ctx context.Context, orgID uuid.UUID, actorID uuid.UUID) ([]OrganizationInvitationDTO, error)
	AcceptOrganizationInvitation(ctx context.Context, actorID uuid.UUID, rawToken string) error
	RemoveOrganizationMember(ctx context.Context, orgID uuid.UUID, actorID uuid.UUID, targetUserID uuid.UUID) error
}

// AuthService defines the interface for authentication business logic
type AuthService interface {
	Login(ctx context.Context, email, password string) (*LoginResult, error)
	Register(ctx context.Context, input *RegisterInput) (*models.User, *Authentication, error)
	RefreshToken(ctx context.Context, refreshToken string) (*Authentication, error)
	Logout(ctx context.Context, refreshToken string) error
	ValidateCredentials(email, password string) (*models.User, error)
	VerifyEmail(ctx context.Context, token string) error
	ResendVerificationEmail(ctx context.Context, email string) error
	RequestPasswordReset(ctx context.Context, email string) error
	ConfirmPasswordReset(ctx context.Context, token, newPassword string) error

	SetupTOTP(ctx context.Context, userID uuid.UUID) (*TOTPSetupResult, error)
	ConfirmTOTP(ctx context.Context, userID uuid.UUID, code string) (*TOTPConfirmResult, error)
	DisableTOTP(ctx context.Context, userID uuid.UUID, password string) error
	VerifyMFALogin(ctx context.Context, challengeToken, code string) (*models.User, *Authentication, error)

	OAuthAuthorizeURL(ctx context.Context, provider string, linkUserID *uuid.UUID) (authorizeURL string, err error)
	OAuthHandleCallback(ctx context.Context, provider, authCode, rawState string) (result *OAuthCallbackResult, err error)
	OAuthCompleteExchange(ctx context.Context, oauthCode string) (*LoginResult, error)
}
