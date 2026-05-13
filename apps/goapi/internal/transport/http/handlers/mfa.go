package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"goapi/config"
	"goapi/services"
)

func mfaEncryptionOr503(c *gin.Context, cfg *config.Config) bool {
	if config.MFAEncryptionConfigured(cfg) {
		return true
	}
	c.JSON(http.StatusServiceUnavailable, gin.H{"error": "MFA is not configured on this server"})
	return false
}

// ConfirmTOTPInput completes TOTP enrollment with a time-based code.
type ConfirmTOTPInput struct {
	Code string `json:"code" binding:"required"`
}

// DisableTOTPInput requires the current password to turn off MFA.
type DisableTOTPInput struct {
	Password string `json:"password" binding:"required"`
}

// VerifyMFALoginInput exchanges a login MFA challenge for session tokens.
type VerifyMFALoginInput struct {
	MFAChallengeToken string `json:"mfa_challenge_token" binding:"required"`
	Code              string `json:"code" binding:"required"`
}

// SetupTOTP starts TOTP enrollment (authenticated).
func SetupTOTP(auth services.AuthService, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !mfaEncryptionOr503(c, cfg) {
			return
		}
		actorID, _, ok := actorFromGin(c)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Missing or invalid Authorization header"})
			return
		}
		res, err := auth.SetupTOTP(c.Request.Context(), actorID)
		if err != nil {
			handleServiceError(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"secret": res.Secret, "uri": res.URI})
	}
}

// ConfirmTOTP enables MFA after validating the pending enrollment code.
func ConfirmTOTP(auth services.AuthService, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !mfaEncryptionOr503(c, cfg) {
			return
		}
		var input ConfirmTOTPInput
		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		actorID, _, ok := actorFromGin(c)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Missing or invalid Authorization header"})
			return
		}
		res, err := auth.ConfirmTOTP(c.Request.Context(), actorID, input.Code)
		if err != nil {
			handleServiceError(c, err)
			return
		}
		body := gin.H{"message": "MFA enabled"}
		if len(res.RecoveryCodes) > 0 {
			body["recovery_codes"] = res.RecoveryCodes
		}
		c.JSON(http.StatusOK, body)
	}
}

// DisableTOTP disables MFA after password confirmation (authenticated).
func DisableTOTP(auth services.AuthService, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !mfaEncryptionOr503(c, cfg) {
			return
		}
		var input DisableTOTPInput
		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		actorID, _, ok := actorFromGin(c)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Missing or invalid Authorization header"})
			return
		}
		if err := auth.DisableTOTP(c.Request.Context(), actorID, input.Password); err != nil {
			handleServiceError(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "MFA disabled"})
	}
}

// VerifyMFALogin completes password+MFA login with a challenge token and TOTP code.
func VerifyMFALogin(auth services.AuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		var input VerifyMFALoginInput
		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		ctx := services.WithAuthRequestMeta(c.Request.Context(), services.AuthRequestMeta{
			UserAgent: c.Request.UserAgent(),
			IPAddress: c.ClientIP(),
		})
		user, token, err := auth.VerifyMFALogin(ctx, input.MFAChallengeToken, input.Code)
		if err != nil {
			handleServiceError(c, err)
			return
		}
		ReturnAuthSuccess(c, http.StatusOK, user, token)
	}
}
