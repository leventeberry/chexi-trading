package services

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"goapi/cache"
	"goapi/internal/events"
	authinfra "goapi/internal/infra/auth"
	"goapi/internal/rbac"
	"goapi/logger"
	"goapi/models"
	"goapi/repositories"
)

// userService implements UserService interface
type userService struct {
	userRepo     repositories.UserRepository
	settingsRepo repositories.UserSettingsRepository
	cache        cache.Cache
	recorder     events.Recorder
}

// NewUserService creates a new instance of UserService
// Factory function for creating user service
func NewUserService(userRepo repositories.UserRepository, settingsRepo repositories.UserSettingsRepository, cacheClient cache.Cache, recorder events.Recorder) UserService {
	return &userService{
		userRepo:     userRepo,
		settingsRepo: settingsRepo,
		cache:        cacheClient,
		recorder:     recorder,
	}
}

func actorPtrFromCtx(ctx context.Context) *uuid.UUID {
	id, ok := events.ActorUserIDFromContext(ctx)
	if !ok {
		return nil
	}
	return &id
}

// authorizeSelfOrAdmin allows access when the actor is admin or the target user is the actor.
// Checked before loading the target to avoid IDOR and user-existence leaks on cross-user access.
func (s *userService) authorizeSelfOrAdmin(targetID, actorID uuid.UUID, actorRole string) error {
	if rbac.IsAdminRole(actorRole) {
		return nil
	}
	if actorID == uuid.Nil || targetID != actorID {
		return ErrInsufficientPrivileges
	}
	return nil
}

func (s *userService) authorizeAdminOnly(actorRole string) error {
	if !rbac.IsAdminRole(actorRole) {
		return ErrInsufficientPrivileges
	}
	return nil
}

// CreateUser creates a new user with business logic validation.
// actorRole is the JWT role of the caller; only admins may assign RoleAdmin.
func (s *userService) CreateUser(ctx context.Context, input *CreateUserInput, actorID uuid.UUID, actorRole string) (*models.User, error) {
	if err := s.authorizeAdminOnly(actorRole); err != nil {
		return nil, err
	}
	_ = actorID // reserved for audit extensions; actor is already on request context

	// Validate role
	if input.Role != "" && !IsValidRole(input.Role) {
		return nil, ErrInvalidRole
	}

	// Set default role
	role := input.Role
	if role == "" {
		role = rbac.RoleUser.String()
	}
	if rbac.IsAdminRole(role) && actorRole != rbac.RoleAdmin.String() {
		return nil, ErrInsufficientPrivileges
	}

	// Check if email already exists
	exists, err := s.userRepo.ExistsByEmail(input.Email)
	if err != nil {
		return nil, fmt.Errorf("failed to check email existence: %w", err)
	}
	if exists {
		return nil, ErrEmailExists
	}

	// Hash password
	hash, err := authinfra.HashPassword(input.Password)
	if err != nil {
		return nil, ErrPasswordHashing
	}

	// Create user model
	user := &models.User{
		FirstName: input.FirstName,
		LastName:  input.LastName,
		Email:     input.Email,
		PassHash:  hash,
		PhoneNum:  input.PhoneNum,
		Role:      role,
	}

	// Save to database
	if err := s.userRepo.Create(user); err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	// Store in cache after successful creation
	if err := s.cache.SetUserByID(ctx, user.ID, user, cache.UserCacheTTL); err != nil {
		logger.Log.Warn().Err(err).Str("user_id", user.ID.String()).Msg("Failed to cache user by ID")
	}
	if err := s.cache.SetUserByEmail(ctx, user.Email, user, cache.UserCacheTTL); err != nil {
		logger.Log.Warn().Err(err).Str("email", user.Email).Msg("Failed to cache user by email")
	}

	sub := user.ID.String()
	events.RecordSafe(s.recorder, ctx, events.Event{
		OccurredAt:  events.NowUTC(),
		EventType:   "user.created",
		ActorUserID: actorPtrFromCtx(ctx),
		Subject:     sub,
		Metadata: events.MetadataJSON(map[string]string{
			"email": user.Email,
			"role":  user.Role,
		}),
		RequestID: events.RequestIDFromContext(ctx),
	})

	return user, nil
}

// GetUserByID retrieves a user by ID using cache-aside pattern
// 1. Check cache first
// 2. If cache miss, query database
// 3. Store result in cache for future requests
func (s *userService) GetUserByID(ctx context.Context, id uuid.UUID, actorID uuid.UUID, actorRole string) (*models.User, error) {
	if err := s.authorizeSelfOrAdmin(id, actorID, actorRole); err != nil {
		return nil, err
	}

	// Try to get from cache first
	user, err := s.cache.GetUserByID(ctx, id)
	if err == nil {
		// Cache hit - return cached user
		return user, nil
	}

	// Cache miss or error - fallback to database
	if !errors.Is(err, cache.ErrCacheMiss) {
		logger.Log.Warn().Err(err).Str("user_id", id.String()).Msg("Cache error when fetching user by ID")
	}

	user, err = s.userRepo.FindByID(id)
	if err != nil {
		if errors.Is(err, repositories.ErrUserNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to get user by ID %s: %w", id, err)
	}

	// Store in cache for future requests (best effort - don't fail on cache error)
	if err := s.cache.SetUserByID(ctx, id, user, cache.UserCacheTTL); err != nil {
		logger.Log.Warn().Err(err).Str("user_id", id.String()).Msg("Failed to cache user by ID")
	}
	if err := s.cache.SetUserByEmail(ctx, user.Email, user, cache.UserCacheTTL); err != nil {
		logger.Log.Warn().Err(err).Str("email", user.Email).Msg("Failed to cache user by email")
	}

	return user, nil
}

// GetUserByEmail retrieves a user by email using cache-aside pattern
// 1. Check cache first
// 2. If cache miss, query database
// 3. Store result in cache for future requests
func (s *userService) GetUserByEmail(ctx context.Context, email string) (*models.User, error) {
	// Try to get from cache first
	user, err := s.cache.GetUserByEmail(ctx, email)
	if err == nil {
		// Cache hit - return cached user
		return user, nil
	}

	// Cache miss or error - fallback to database
	if !errors.Is(err, cache.ErrCacheMiss) {
		logger.Log.Warn().Err(err).Str("email", email).Msg("Cache error when fetching user by email")
	}

	user, err = s.userRepo.FindByEmail(email)
	if err != nil {
		if errors.Is(err, repositories.ErrUserNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to get user by email %s: %w", email, err)
	}

	// Store in cache for future requests (best effort - don't fail on cache error)
	if err := s.cache.SetUserByEmail(ctx, email, user, cache.UserCacheTTL); err != nil {
		logger.Log.Warn().Err(err).Str("email", email).Msg("Failed to cache user by email")
	}
	if err := s.cache.SetUserByID(ctx, user.ID, user, cache.UserCacheTTL); err != nil {
		logger.Log.Warn().Err(err).Str("user_id", user.ID.String()).Msg("Failed to cache user by ID")
	}

	return user, nil
}

// GetAllUsers retrieves all users
func (s *userService) GetAllUsers(ctx context.Context, actorID uuid.UUID, actorRole string) ([]models.User, error) {
	_ = actorID
	if err := s.authorizeAdminOnly(actorRole); err != nil {
		return nil, err
	}
	return s.userRepo.FindAll()
}

// GetAllUsersPaginated retrieves users with pagination support
func (s *userService) GetAllUsersPaginated(ctx context.Context, params *PaginationParams, actorID uuid.UUID, actorRole string) ([]models.User, int64, error) {
	_ = actorID
	if err := s.authorizeAdminOnly(actorRole); err != nil {
		return nil, 0, err
	}
	// Validate and set defaults
	page := params.Page
	if page < 1 {
		page = 1
	}

	pageSize := params.PageSize
	if pageSize < 1 {
		pageSize = 10 // Default page size
	}
	if pageSize > 100 {
		pageSize = 100 // Max page size to prevent abuse
	}

	users, total, err := s.userRepo.FindAllWithPagination(page, pageSize)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get paginated users: %w", err)
	}
	return users, total, nil
}

// UpdateUser updates a user with business logic validation
func (s *userService) UpdateUser(ctx context.Context, id uuid.UUID, input *UpdateUserInput, actorID uuid.UUID, actorRole string) (*models.User, error) {
	if err := s.authorizeSelfOrAdmin(id, actorID, actorRole); err != nil {
		return nil, err
	}

	// Get existing user
	user, err := s.userRepo.FindByID(id)
	if err != nil {
		if errors.Is(err, repositories.ErrUserNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to get user by ID %s for update: %w", id, err)
	}

	// Store old email for cache invalidation if email is being changed
	oldEmail := user.Email

	// Validate at least one field is being updated
	if input.FirstName == nil && input.LastName == nil && input.Email == nil &&
		input.Password == nil && input.PhoneNum == nil && input.Role == nil &&
		input.DisplayName == nil && input.AvatarURL == nil && input.Timezone == nil && input.Locale == nil {
		return nil, ErrNoFieldsToUpdate
	}

	// Update fields if provided
	if input.FirstName != nil {
		fn, err := SanitizeString(*input.FirstName, maxNameLen)
		if err != nil || fn == "" {
			return nil, ErrInvalidProfileField
		}
		user.FirstName = fn
	}
	if input.LastName != nil {
		ln, err := SanitizeString(*input.LastName, maxNameLen)
		if err != nil || ln == "" {
			return nil, ErrInvalidProfileField
		}
		user.LastName = ln
	}
	if input.DisplayName != nil {
		dn, err := SanitizeString(*input.DisplayName, maxDisplayNameLen)
		if err != nil {
			return nil, ErrInvalidProfileField
		}
		user.DisplayName = dn
	}
	if input.AvatarURL != nil {
		v := strings.TrimSpace(*input.AvatarURL)
		if v == "" {
			user.AvatarURL = ""
		} else {
			if err := ValidateHTTPSURL(v, maxAvatarURLLen); err != nil {
				return nil, ErrInvalidAvatarURL
			}
			user.AvatarURL = v
		}
	}
	if input.Timezone != nil {
		tz := strings.TrimSpace(*input.Timezone)
		if tz == "" {
			user.Timezone = ""
		} else {
			if err := ValidateIANATimezone(tz); err != nil {
				return nil, ErrInvalidTimezone
			}
			user.Timezone = tz
		}
	}
	if input.Locale != nil {
		loc := strings.TrimSpace(*input.Locale)
		if loc == "" {
			user.Locale = ""
		} else {
			if err := ValidateLocaleFormat(loc); err != nil {
				return nil, ErrInvalidLocale
			}
			user.Locale = loc
		}
	}
	if input.PhoneNum != nil {
		pn, err := SanitizeString(*input.PhoneNum, maxPhoneLen)
		if err != nil {
			return nil, ErrInvalidProfileField
		}
		user.PhoneNum = pn
	}

	// Handle email update with uniqueness check
	// Compare normalized emails to handle case differences
	if input.Email != nil {
		normalizedInputEmail := strings.ToLower(strings.TrimSpace(*input.Email))
		normalizedCurrentEmail := strings.ToLower(strings.TrimSpace(user.Email))
		if normalizedInputEmail != normalizedCurrentEmail {
			exists, err := s.userRepo.ExistsByEmail(*input.Email)
			if err != nil {
				return nil, fmt.Errorf("failed to check email existence for update: %w", err)
			}
			if exists {
				return nil, ErrEmailExists
			}
			user.Email = *input.Email // Repository will normalize on save
		}
	}

	// Handle role update with validation
	if input.Role != nil {
		if !IsValidRole(*input.Role) {
			return nil, ErrInvalidRole
		}
		if rbac.IsAdminRole(*input.Role) && actorRole != rbac.RoleAdmin.String() {
			return nil, ErrInsufficientPrivileges
		}
		user.Role = *input.Role
	}

	// Handle password update with hashing
	if input.Password != nil {
		hash, err := authinfra.HashPassword(*input.Password)
		if err != nil {
			return nil, ErrPasswordHashing
		}
		user.PassHash = hash
	}

	// Save updates
	if err := s.userRepo.Update(user); err != nil {
		return nil, fmt.Errorf("failed to update user ID %s: %w", id, err)
	}

	// Invalidate cache - delete old entries
	// If email changed, delete both old and new email keys
	if input.Email != nil && *input.Email != oldEmail {
		// Delete old email key
		s.cache.DeleteUserByEmail(ctx, oldEmail)
		// Delete ID key (will be repopulated on next read)
		s.cache.DeleteUserByID(ctx, id)
	} else {
		// Delete all cached entries for this user (both ID and email)
		s.cache.DeleteUser(ctx, id, user.Email)
	}

	// Store updated user in cache for future requests
	if err := s.cache.SetUserByID(ctx, user.ID, user, cache.UserCacheTTL); err != nil {
		logger.Log.Warn().Err(err).Str("user_id", user.ID.String()).Msg("Failed to cache updated user by ID")
	}
	if err := s.cache.SetUserByEmail(ctx, user.Email, user, cache.UserCacheTTL); err != nil {
		logger.Log.Warn().Err(err).Str("email", user.Email).Msg("Failed to cache updated user by email")
	}

	fields := updatedFieldNames(input)
	events.RecordSafe(s.recorder, ctx, events.Event{
		OccurredAt:  events.NowUTC(),
		EventType:   "user.updated",
		ActorUserID: actorPtrFromCtx(ctx),
		Subject:     id.String(),
		Metadata: events.MetadataJSON(map[string]interface{}{
			"fields":         fields,
			"target_user_id": id.String(),
		}),
		RequestID: events.RequestIDFromContext(ctx),
	})

	return user, nil
}

// DeleteUser deletes a user
func (s *userService) DeleteUser(ctx context.Context, id uuid.UUID, actorID uuid.UUID, actorRole string) error {
	_ = actorID
	if err := s.authorizeAdminOnly(actorRole); err != nil {
		return err
	}

	// Get user first to get email for cache invalidation
	user, err := s.userRepo.FindByID(id)
	if err != nil {
		if errors.Is(err, repositories.ErrUserNotFound) {
			return ErrUserNotFound
		}
		return err
	}

	email := user.Email

	// Delete from database
	err = s.userRepo.Delete(id)
	if err != nil {
		if errors.Is(err, repositories.ErrUserNotFound) {
			return ErrUserNotFound
		}
		return fmt.Errorf("failed to delete user ID %s: %w", id, err)
	}

	// Invalidate cache - delete all cached entries for this user
	s.cache.DeleteUser(ctx, id, email)

	events.RecordSafe(s.recorder, ctx, events.Event{
		OccurredAt:  events.NowUTC(),
		EventType:   "user.deleted",
		ActorUserID: actorPtrFromCtx(ctx),
		Subject:     id.String(),
		Metadata: events.MetadataJSON(map[string]interface{}{
			"email": email,
		}),
		RequestID: events.RequestIDFromContext(ctx),
	})

	return nil
}

func updatedFieldNames(input *UpdateUserInput) []string {
	var names []string
	if input.FirstName != nil {
		names = append(names, "first_name")
	}
	if input.LastName != nil {
		names = append(names, "last_name")
	}
	if input.Email != nil {
		names = append(names, "email")
	}
	if input.Password != nil {
		names = append(names, "password")
	}
	if input.PhoneNum != nil {
		names = append(names, "phone_number")
	}
	if input.DisplayName != nil {
		names = append(names, "display_name")
	}
	if input.AvatarURL != nil {
		names = append(names, "avatar_url")
	}
	if input.Timezone != nil {
		names = append(names, "timezone")
	}
	if input.Locale != nil {
		names = append(names, "locale")
	}
	if input.Role != nil {
		names = append(names, "role")
	}
	return names
}

// ValidateRole checks if a role is valid
// Uses the shared IsValidRole function for consistency
func (s *userService) ValidateRole(role string) bool {
	return IsValidRole(role)
}
