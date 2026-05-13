package handlers

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"goapi/models"
	"goapi/services"
)

const maxPatchMeJSONBytes = 1 << 20 // 1 MiB

func actorRoleFromGin(c *gin.Context) string {
	v, ok := c.Get("role")
	if !ok {
		return ""
	}
	s, _ := v.(string)
	return s
}

// actorFromGin returns the authenticated principal (JWT subject) and role. Identity must not be taken from request body.
func actorFromGin(c *gin.Context) (actorID uuid.UUID, actorRole string, ok bool) {
	v, exists := c.Get("userID")
	if !exists {
		return uuid.Nil, "", false
	}
	subject, typeOK := v.(string)
	if !typeOK || subject == "" {
		return uuid.Nil, "", false
	}
	id, err := uuid.Parse(subject)
	if err != nil {
		return uuid.Nil, "", false
	}
	return id, actorRoleFromGin(c), true
}

// CreateUserInput holds the data for creating a new user.
type CreateUserInput struct {
	FirstName string `json:"first_name" binding:"required,min=1,max=50"`
	LastName  string `json:"last_name" binding:"required,min=1,max=50"`
	Email     string `json:"email" binding:"required,email,max=255"`
	Password  string `json:"password" binding:"required,min=8,max=128"`
	PhoneNum  string `json:"phone_number" binding:"omitempty,max=20"`
	// Role defaults to user; only an admin JWT may set admin.
	Role string `json:"role" binding:"omitempty,oneof=user admin"`
}

// UpdateUserInput holds the data for updating a user.
type UpdateUserInput struct {
	FirstName   *string `json:"first_name" binding:"omitempty,min=1,max=50"`
	LastName    *string `json:"last_name" binding:"omitempty,min=1,max=50"`
	DisplayName *string `json:"display_name" binding:"omitempty,max=120"`
	AvatarURL   *string `json:"avatar_url" binding:"omitempty,max=2048"`
	Timezone    *string `json:"timezone" binding:"omitempty,max=64"`
	Locale      *string `json:"locale" binding:"omitempty,max=16"`
	Email       *string `json:"email" binding:"omitempty,email,max=255"`
	Password    *string `json:"password" binding:"omitempty,min=8,max=128"`
	PhoneNum    *string `json:"phone_number" binding:"omitempty,max=20"`
	// Setting role to admin requires an admin JWT.
	Role *string `json:"role" binding:"omitempty,oneof=user admin"`
}

// UserResponse represents a user in API responses.
type UserResponse struct {
	ID              uuid.UUID `json:"id"`
	FirstName       string    `json:"first_name"`
	LastName        string    `json:"last_name"`
	DisplayName     string    `json:"display_name,omitempty"`
	AvatarURL       string    `json:"avatar_url,omitempty"`
	Timezone        string    `json:"timezone,omitempty"`
	Locale          string    `json:"locale,omitempty"`
	Email           string    `json:"email"`
	PhoneNum        string    `json:"phone_number"`
	Role            string    `json:"role"`
	EmailVerifiedAt *string   `json:"email_verified_at,omitempty"`
	CreatedAt       string    `json:"created_at"`
	UpdatedAt       string    `json:"updated_at"`
}

func toUserResponse(user *models.User) *UserResponse {
	var verified *string
	if user.EmailVerifiedAt != nil {
		s := user.EmailVerifiedAt.Format(time.RFC3339)
		verified = &s
	}
	return &UserResponse{
		ID:              user.ID,
		FirstName:       user.FirstName,
		LastName:        user.LastName,
		DisplayName:     user.DisplayName,
		AvatarURL:       user.AvatarURL,
		Timezone:        user.Timezone,
		Locale:          user.Locale,
		Email:           user.Email,
		PhoneNum:        user.PhoneNum,
		Role:            user.Role,
		EmailVerifiedAt: verified,
		CreatedAt:       user.CreatedAt.Format(time.RFC3339),
		UpdatedAt:       user.UpdatedAt.Format(time.RFC3339),
	}
}

func toUserResponseList(users []models.User) []UserResponse {
	responses := make([]UserResponse, len(users))
	for i := range users {
		responses[i] = *toUserResponse(&users[i])
	}
	return responses
}

// PatchMeProfileBody is the allowed JSON "profile" object for PATCH /users/me.
type PatchMeProfileBody struct {
	FirstName   *string `json:"first_name,omitempty"`
	LastName    *string `json:"last_name,omitempty"`
	DisplayName *string `json:"display_name,omitempty"`
	AvatarURL   *string `json:"avatar_url,omitempty"`
	PhoneNum    *string `json:"phone_number,omitempty"`
	Timezone    *string `json:"timezone,omitempty"`
	Locale      *string `json:"locale,omitempty"`
}

// PatchMeSettingsBody is the allowed JSON "settings" object for PATCH /users/me.
type PatchMeSettingsBody struct {
	Theme                     *string         `json:"theme,omitempty"`
	NotificationPreferences   json.RawMessage `json:"notification_preferences,omitempty" swaggertype:"object"`
	MarketingEmailOptIn       *bool           `json:"marketing_email_opt_in,omitempty"`
	SecurityNotificationOptIn *bool           `json:"security_notification_opt_in,omitempty"`
	ExtraSettings             json.RawMessage `json:"extra_settings,omitempty" swaggertype:"object"`
}

// PatchMeBody is the PATCH /users/me request body (at least one of profile or settings should be sent).
type PatchMeBody struct {
	Profile  *PatchMeProfileBody  `json:"profile,omitempty"`
	Settings *PatchMeSettingsBody `json:"settings,omitempty"`
}

// MeSettingsResponse is the nested `settings` object in GET/PATCH /users/me responses.
type MeSettingsResponse struct {
	Theme                     string          `json:"theme"`
	NotificationPreferences   json.RawMessage `json:"notification_preferences" swaggertype:"object"`
	MarketingEmailOptIn       bool            `json:"marketing_email_opt_in"`
	SecurityNotificationOptIn bool            `json:"security_notification_opt_in"`
	ExtraSettings             json.RawMessage `json:"extra_settings" swaggertype:"object"`
}

// MeProfileResponse is the GET/PATCH /users/me JSON envelope for API documentation.
type MeProfileResponse struct {
	Profile  UserResponse       `json:"profile"`
	Settings MeSettingsResponse `json:"settings"`
}

func patchEnvelopeToServiceInput(env *PatchMeBody) *services.PatchMeProfileInput {
	in := &services.PatchMeProfileInput{}
	if env.Profile != nil {
		p := env.Profile
		in.FirstName = p.FirstName
		in.LastName = p.LastName
		in.DisplayName = p.DisplayName
		in.AvatarURL = p.AvatarURL
		in.PhoneNum = p.PhoneNum
		in.Timezone = p.Timezone
		in.Locale = p.Locale
	}
	if env.Settings != nil {
		s := env.Settings
		in.Theme = s.Theme
		if len(s.NotificationPreferences) > 0 {
			b := append([]byte(nil), s.NotificationPreferences...)
			rm := json.RawMessage(b)
			in.NotificationPreferences = &rm
		}
		in.MarketingEmailOptIn = s.MarketingEmailOptIn
		in.SecurityNotificationOptIn = s.SecurityNotificationOptIn
		if len(s.ExtraSettings) > 0 {
			b := append([]byte(nil), s.ExtraSettings...)
			rm := json.RawMessage(b)
			in.ExtraSettings = &rm
		}
	}
	return in
}

func meProfileToJSON(dto *services.MeProfileDTO) gin.H {
	u := dto.User
	s := dto.Settings
	var verified any
	if u.EmailVerifiedAt != nil {
		verified = u.EmailVerifiedAt.Format(time.RFC3339)
	}
	return gin.H{
		"profile": gin.H{
			"id":                u.ID,
			"first_name":        u.FirstName,
			"last_name":         u.LastName,
			"display_name":      u.DisplayName,
			"avatar_url":        u.AvatarURL,
			"phone_number":      u.PhoneNum,
			"timezone":          u.Timezone,
			"locale":            u.Locale,
			"email":             u.Email,
			"role":              u.Role,
			"email_verified_at": verified,
			"created_at":        u.CreatedAt.Format(time.RFC3339),
			"updated_at":        u.UpdatedAt.Format(time.RFC3339),
		},
		"settings": gin.H{
			"theme":                        s.Theme,
			"notification_preferences":     json.RawMessage(s.NotificationPreferences),
			"marketing_email_opt_in":       s.MarketingEmailOptIn,
			"security_notification_opt_in": s.SecurityNotificationOptIn,
			"extra_settings":               json.RawMessage(s.ExtraSettings),
		},
	}
}

// GetUsers retrieves all users with optional pagination.
// @Summary      Get all users
// @Description  Get a list of all users with optional pagination (requires authentication). Query parameters: page (default: 1), page_size (default: 10, max: 100)
// @Tags         users
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        page       query     int     false  "Page number (default: 1)"
// @Param        page_size  query     int     false  "Items per page (default: 10, max: 100)"
// @Success      200        {object}  map[string]interface{}  "Paginated users response"
// @Failure      400        {object}  map[string]string  "Invalid pagination parameters"
// @Failure      401        {object}  map[string]string  "Unauthorized"
// @Failure      403        {object}  map[string]string  "Forbidden"
// @Failure      500        {object}  map[string]string  "Server error"
// @Router       /admin/users [get]
func GetUsers(userService services.UserService) gin.HandlerFunc {
	return func(c *gin.Context) {
		actorID, actorRole, authed := actorFromGin(c)
		if !authed {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
			return
		}

		pageParam := c.Query("page")
		pageSizeParam := c.Query("page_size")

		if pageParam != "" || pageSizeParam != "" {
			page := 1
			pageSize := 10

			if pageParam != "" {
				parsedPage, err := strconv.Atoi(pageParam)
				if err != nil || parsedPage < 1 {
					c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid page parameter"})
					return
				}
				page = parsedPage
			}

			if pageSizeParam != "" {
				parsedPageSize, err := strconv.Atoi(pageSizeParam)
				if err != nil || parsedPageSize < 1 {
					c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid page_size parameter"})
					return
				}
				pageSize = parsedPageSize
			}

			params := &services.PaginationParams{
				Page:     page,
				PageSize: pageSize,
			}

			users, total, err := userService.GetAllUsersPaginated(c.Request.Context(), params, actorID, actorRole)
			if err != nil {
				handleServiceError(c, err)
				return
			}

			actualPageSize := pageSize
			if actualPageSize < 1 {
				actualPageSize = 10
			}
			if actualPageSize > 100 {
				actualPageSize = 100
			}

			c.JSON(http.StatusOK, gin.H{
				"data":        toUserResponseList(users),
				"total":       total,
				"page":        page,
				"page_size":   actualPageSize,
				"total_pages": (int(total) + actualPageSize - 1) / actualPageSize,
			})
			return
		}

		users, err := userService.GetAllUsers(c.Request.Context(), actorID, actorRole)
		if err != nil {
			handleServiceError(c, err)
			return
		}
		c.JSON(http.StatusOK, toUserResponseList(users))
	}
}

// GetMe returns the authenticated user's profile (JWT subject only).
// @Summary      Get current user
// @Description  Returns the profile and settings for the authenticated principal
// @Tags         users
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  handlers.MeProfileResponse
// @Failure      401  {object}  map[string]string  "Unauthorized"
// @Failure      403  {object}  map[string]string  "Forbidden"
// @Failure      404  {object}  map[string]string  "User not found"
// @Router       /users/me [get]
func GetMe(userService services.UserService) gin.HandlerFunc {
	return func(c *gin.Context) {
		actorID, _, authed := actorFromGin(c)
		if !authed {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
			return
		}

		dto, err := userService.GetMyProfile(c.Request.Context(), actorID)
		if err != nil {
			handleServiceError(c, err)
			return
		}

		c.JSON(http.StatusOK, meProfileToJSON(dto))
	}
}

// GetUser retrieves a specific user by ID.
// @Summary      Get user by ID
// @Description  Get a specific user by their ID (admin only)
// @Tags         users
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      string  true  "User ID (UUID)"
// @Success      200  {object}  models.User  "User object"
// @Failure      400  {object}  map[string]string  "Invalid user ID"
// @Failure      401  {object}  map[string]string  "Unauthorized"
// @Failure      403  {object}  map[string]string  "Forbidden"
// @Failure      404  {object}  map[string]string  "User not found"
// @Failure      500  {object}  map[string]string  "Server error"
// @Router       /admin/users/{id} [get]
func GetUser(userService services.UserService) gin.HandlerFunc {
	return func(c *gin.Context) {
		actorID, actorRole, authed := actorFromGin(c)
		if !authed {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
			return
		}

		idParam := c.Param("id")
		id, err := uuid.Parse(idParam)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
			return
		}

		user, err := userService.GetUserByID(c.Request.Context(), id, actorID, actorRole)
		if err != nil {
			handleServiceError(c, err)
			return
		}

		c.JSON(http.StatusOK, toUserResponse(user))
	}
}

// CreateUser creates a new user.
// @Summary      Create new user
// @Description  Create a new user account (admin only)
// @Tags         users
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        user  body      CreateUserInput  true  "User data"
// @Success      201   {object}  models.User  "Created user"
// @Failure      400   {object}  map[string]string  "Invalid request"
// @Failure      401   {object}  map[string]string  "Unauthorized"
// @Failure      403   {object}  map[string]string  "Insufficient privileges (e.g. non-admin assigning admin)"
// @Failure      409   {object}  map[string]string  "Email already registered"
// @Failure      500   {object}  map[string]string  "Server error"
// @Router       /admin/users [post]
func CreateUser(userService services.UserService) gin.HandlerFunc {
	return func(c *gin.Context) {
		actorID, actorRole, authed := actorFromGin(c)
		if !authed {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
			return
		}

		var input CreateUserInput
		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
			return
		}

		if err := services.ValidatePasswordStrength(input.Password); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		createInput := &services.CreateUserInput{
			FirstName: input.FirstName,
			LastName:  input.LastName,
			Email:     input.Email,
			Password:  input.Password,
			PhoneNum:  input.PhoneNum,
			Role:      input.Role,
		}

		user, err := userService.CreateUser(c.Request.Context(), createInput, actorID, actorRole)
		if err != nil {
			handleServiceError(c, err)
			return
		}

		c.JSON(http.StatusCreated, toUserResponse(user))
	}
}

// PatchMe updates profile and settings for the authenticated user only (no role/email/password).
// @Summary      Update current user profile and settings
// @Description  Send a JSON object with optional nested `profile` and/or `settings` keys only. Unknown fields are rejected (strict JSON decoding).
// @Tags         users
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body  body      handlers.PatchMeBody  true  "Partial profile/settings update"
// @Success      200   {object}  handlers.MeProfileResponse  "Updated profile and settings"
// @Failure      400   {object}  map[string]string             "Bad request"
// @Failure      401   {object}  map[string]string             "Unauthorized"
// @Failure      403   {object}  map[string]string             "Forbidden"
// @Failure      404   {object}  map[string]string             "User not found"
// @Failure      500   {object}  map[string]string             "Server error"
// @Router       /users/me [patch]
func PatchMe(userService services.UserService) gin.HandlerFunc {
	return func(c *gin.Context) {
		actorID, _, authed := actorFromGin(c)
		if !authed {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
			return
		}

		payload, err := io.ReadAll(io.LimitReader(c.Request.Body, maxPatchMeJSONBytes))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Unable to read request body"})
			return
		}
		dec := json.NewDecoder(bytes.NewReader(payload))
		dec.DisallowUnknownFields()
		var env PatchMeBody
		if err := dec.Decode(&env); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if dec.More() {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Request body must be a single JSON object"})
			return
		}

		svcIn := patchEnvelopeToServiceInput(&env)
		dto, err := userService.PatchMyProfile(c.Request.Context(), actorID, svcIn)
		if err != nil {
			handleServiceError(c, err)
			return
		}
		c.JSON(http.StatusOK, meProfileToJSON(dto))
	}
}

// UpdateUser updates an existing user.
// @Summary      Update user
// @Description  Update user information (admin only; partial updates supported)
// @Tags         users
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id    path      string           true  "User ID (UUID)"
// @Param        user  body      UpdateUserInput  true  "User update data"
// @Success      200   {object}  models.User  "Updated user"
// @Failure      400   {object}  map[string]string  "Invalid request"
// @Failure      401   {object}  map[string]string  "Unauthorized"
// @Failure      404   {object}  map[string]string  "User not found"
// @Failure      403   {object}  map[string]string  "Insufficient privileges (e.g. non-admin setting admin role)"
// @Failure      409   {object}  map[string]string  "Email already registered"
// @Failure      500   {object}  map[string]string  "Server error"
// @Router       /admin/users/{id} [put]
func UpdateUser(userService services.UserService) gin.HandlerFunc {
	return func(c *gin.Context) {
		actorID, actorRole, authed := actorFromGin(c)
		if !authed {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
			return
		}

		idParam := c.Param("id")
		id, err := uuid.Parse(idParam)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
			return
		}

		var input UpdateUserInput
		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		if input.Password != nil {
			if err := services.ValidatePasswordStrength(*input.Password); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
		}

		updateInput := &services.UpdateUserInput{
			FirstName:   input.FirstName,
			LastName:    input.LastName,
			DisplayName: input.DisplayName,
			AvatarURL:   input.AvatarURL,
			Timezone:    input.Timezone,
			Locale:      input.Locale,
			Email:       input.Email,
			Password:    input.Password,
			PhoneNum:    input.PhoneNum,
			Role:        input.Role,
		}

		user, err := userService.UpdateUser(c.Request.Context(), id, updateInput, actorID, actorRole)
		if err != nil {
			handleServiceError(c, err)
			return
		}

		c.JSON(http.StatusOK, toUserResponse(user))
	}
}

// DeleteUser deletes a user (admin only).
// @Summary      Delete user
// @Description  Delete a user by ID (requires admin role)
// @Tags         users
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      string  true  "User ID (UUID)"
// @Success      200  {object}  map[string]string  "User deleted successfully"
// @Failure      400  {object}  map[string]string  "Invalid user ID"
// @Failure      401  {object}  map[string]string  "Unauthorized"
// @Failure      403  {object}  map[string]string  "Insufficient permissions"
// @Failure      404  {object}  map[string]string  "User not found"
// @Failure      500  {object}  map[string]string  "Server error"
// @Router       /admin/users/{id} [delete]
func DeleteUser(userService services.UserService) gin.HandlerFunc {
	return func(c *gin.Context) {
		actorID, actorRole, authed := actorFromGin(c)
		if !authed {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
			return
		}

		idParam := c.Param("id")
		id, err := uuid.Parse(idParam)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
			return
		}

		err = userService.DeleteUser(c.Request.Context(), id, actorID, actorRole)
		if err != nil {
			handleServiceError(c, err)
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "User deleted successfully"})
	}
}
