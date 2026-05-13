package handlers

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"goapi/config"
	"goapi/models"
	"goapi/services"
)

// RequestUserInput holds login credentials.
type RequestUserInput struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

// RefreshTokenInput holds refresh-token payload.
type RefreshTokenInput struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

// LogoutInput holds logout payload.
type LogoutInput struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

// VerifyEmailInput holds a verification token when not passed as a query parameter.
type VerifyEmailInput struct {
	Token string `json:"token" binding:"required"`
}

// ResendVerificationInput requests another verification email.
type ResendVerificationInput struct {
	Email string `json:"email" binding:"required,email"`
}

// PasswordResetRequestInput starts a password reset (response is always generic).
type PasswordResetRequestInput struct {
	Email string `json:"email" binding:"required,email"`
}

// PasswordResetConfirmInput completes a password reset with a single-use token.
type PasswordResetConfirmInput struct {
	Token    string `json:"token" binding:"required"`
	Password string `json:"password" binding:"required,min=8,max=128"`
}

// SignupUserInput holds registration data.
type SignupUserInput struct {
	FirstName string `json:"first_name" binding:"required,min=1,max=50"`
	LastName  string `json:"last_name" binding:"required,min=1,max=50"`
	Email     string `json:"email" binding:"required,email,max=255"`
	Password  string `json:"password" binding:"required,min=8,max=128"`
	PhoneNum  string `json:"phone_number" binding:"omitempty,max=20"`
	// Role is ignored for privilege: self-signup is always "user". Sending "admin" returns 403.
	Role string `json:"role" binding:"omitempty,oneof=user admin"`
}

// ReturnAuthSuccess returns login/register JSON. Shape depends on config:
// token always includes jwt_token; api_key and user are optional (see AUTH_RESPONSE_* envs).
func ReturnAuthSuccess(c *gin.Context, status int, user *models.User, token *services.Authentication) {
	cfg := config.Get()
	tokenObj := gin.H{"jwt_token": token.JWTToken}
	if token.RefreshToken != "" {
		tokenObj["refresh_token"] = token.RefreshToken
	}
	if cfg.AuthResponse.IncludeAPIKey {
		tokenObj["api_key"] = token.ApiKey
	}
	body := gin.H{"token": tokenObj}
	if cfg.AuthResponse.IncludeUser {
		body["user"] = gin.H{
			"id":    user.ID,
			"email": user.Email,
		}
	}
	c.JSON(status, body)
}

// LoginUser authenticates a user and returns a JWT token.
// @Summary      Login user
// @Description  Authenticate a user with email and password, returns JWT token
// @Tags         authentication
// @Accept       json
// @Produce      json
// @Param        credentials  body      RequestUserInput  true  "Login credentials"
// @Success      200          {object}  map[string]interface{}  "Login successful"
// @Failure      400          {object}  map[string]string  "Invalid request"
// @Failure      401          {object}  map[string]string  "Invalid credentials"
// @Failure      500          {object}  map[string]string  "Server error"
// @Router       /login [post]
func LoginUser(authService services.AuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		var input RequestUserInput
		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		ctx := services.WithAuthRequestMeta(c.Request.Context(), services.AuthRequestMeta{
			UserAgent: c.Request.UserAgent(),
			IPAddress: c.ClientIP(),
		})
		result, err := authService.Login(ctx, input.Email, input.Password)
		if err != nil {
			handleServiceError(c, err)
			return
		}
		if result.MFARequired {
			c.JSON(http.StatusOK, gin.H{
				"mfa_required":        true,
				"mfa_challenge_token": result.MFAChallengeToken,
			})
			return
		}

		ReturnAuthSuccess(c, http.StatusOK, result.User, result.Auth)
	}
}

// SignupUser registers a new user. When outbound email is enabled, returns message + user without tokens until verify-email and login.
// @Summary      Register new user
// @Description  Create a new user account (always role user). Self-registration as admin is forbidden; admins are created by existing admins via POST /users. With EMAIL_ENABLED, response has message and user only; otherwise tokens are returned after implicit verification.
// @Tags         authentication
// @Accept       json
// @Produce      json
// @Param        user  body      SignupUserInput  true  "User registration data"
// @Success      201   {object}  map[string]interface{}  "Registration successful"
// @Failure      400   {object}  map[string]string  "Invalid request"
// @Failure      403   {object}  map[string]string  "Cannot self-register as admin"
// @Failure      409   {object}  map[string]string  "Email already registered"
// @Failure      500   {object}  map[string]string  "Server error"
// @Router       /register [post]
func SignupUser(authService services.AuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		var input SignupUserInput
		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		if err := services.ValidatePasswordStrength(input.Password); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		registerInput := &services.RegisterInput{
			FirstName: input.FirstName,
			LastName:  input.LastName,
			Email:     input.Email,
			Password:  input.Password,
			PhoneNum:  input.PhoneNum,
			Role:      input.Role,
		}

		ctx := services.WithAuthRequestMeta(c.Request.Context(), services.AuthRequestMeta{
			UserAgent: c.Request.UserAgent(),
			IPAddress: c.ClientIP(),
		})
		user, token, err := authService.Register(ctx, registerInput)
		if err != nil {
			handleServiceError(c, err)
			return
		}

		if token == nil {
			c.JSON(http.StatusCreated, gin.H{
				"message": "Registration successful. Check your email to verify your account before signing in.",
				"user": gin.H{
					"id":    user.ID,
					"email": user.Email,
				},
			})
			return
		}

		ReturnAuthSuccess(c, http.StatusCreated, user, token)
	}
}

// RefreshToken rotates refresh token and returns a new access token pair.
func RefreshToken(authService services.AuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		var input RefreshTokenInput
		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		ctx := services.WithAuthRequestMeta(c.Request.Context(), services.AuthRequestMeta{
			UserAgent: c.Request.UserAgent(),
			IPAddress: c.ClientIP(),
		})
		token, err := authService.RefreshToken(ctx, input.RefreshToken)
		if err != nil {
			handleServiceError(c, err)
			return
		}

		c.JSON(http.StatusOK, gin.H{"token": gin.H{
			"jwt_token":     token.JWTToken,
			"refresh_token": token.RefreshToken,
		}})
	}
}

// Logout revokes the refresh/session identified by refresh_token.
func Logout(authService services.AuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		var input LogoutInput
		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		ctx := services.WithAuthRequestMeta(c.Request.Context(), services.AuthRequestMeta{
			UserAgent: c.Request.UserAgent(),
			IPAddress: c.ClientIP(),
		})
		if err := authService.Logout(ctx, input.RefreshToken); err != nil {
			handleServiceError(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "logged out"})
	}
}

const (
	msgResendVerificationAck   = "If an account exists for that email and verification is required, instructions were sent."
	msgPasswordResetRequestAck = "If an account exists for that email, password reset instructions were sent."
)

// VerifyEmail consumes a single-use email verification token from ?token= or JSON body.
func VerifyEmail(authService services.AuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := strings.TrimSpace(c.Query("token"))
		if token == "" {
			var input VerifyEmailInput
			if err := c.ShouldBindJSON(&input); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "token is required (query ?token= or JSON {\"token\"})"})
				return
			}
			token = strings.TrimSpace(input.Token)
		}
		ctx := services.WithAuthRequestMeta(c.Request.Context(), services.AuthRequestMeta{
			UserAgent: c.Request.UserAgent(),
			IPAddress: c.ClientIP(),
		})
		if err := authService.VerifyEmail(ctx, token); err != nil {
			handleServiceError(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "Email verified successfully"})
	}
}

// ResendVerificationEmail issues a new verification email when allowed (generic HTTP shape).
func ResendVerificationEmail(authService services.AuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		var input ResendVerificationInput
		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		ctx := services.WithAuthRequestMeta(c.Request.Context(), services.AuthRequestMeta{
			UserAgent: c.Request.UserAgent(),
			IPAddress: c.ClientIP(),
		})
		_ = authService.ResendVerificationEmail(ctx, input.Email)
		c.JSON(http.StatusOK, gin.H{"message": msgResendVerificationAck})
	}
}

// RequestPasswordReset always returns the same success body (no email enumeration).
func RequestPasswordReset(authService services.AuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		var input PasswordResetRequestInput
		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		ctx := services.WithAuthRequestMeta(c.Request.Context(), services.AuthRequestMeta{
			UserAgent: c.Request.UserAgent(),
			IPAddress: c.ClientIP(),
		})
		_ = authService.RequestPasswordReset(ctx, input.Email)
		c.JSON(http.StatusOK, gin.H{"message": msgPasswordResetRequestAck})
	}
}

// ConfirmPasswordReset validates a reset token and sets a new password (revokes refresh sessions).
func ConfirmPasswordReset(authService services.AuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		var input PasswordResetConfirmInput
		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if err := services.ValidatePasswordStrength(input.Password); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		ctx := services.WithAuthRequestMeta(c.Request.Context(), services.AuthRequestMeta{
			UserAgent: c.Request.UserAgent(),
			IPAddress: c.ClientIP(),
		})
		if err := authService.ConfirmPasswordReset(ctx, input.Token, input.Password); err != nil {
			handleServiceError(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "Password updated successfully"})
	}
}
