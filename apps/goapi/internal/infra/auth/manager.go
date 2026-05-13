package auth

import (
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// Manager holds JWT configuration for signing and verifying tokens without globals.
type Manager struct {
	jwtSecret          []byte
	accessTokenMinutes int
}

// NewManager builds a Manager from explicit JWT settings (typically from config.Config.JWT).
func NewManager(secret string, accessTokenMinutes int) *Manager {
	if accessTokenMinutes < 1 {
		accessTokenMinutes = 15
	}
	return &Manager{
		jwtSecret:          []byte(secret),
		accessTokenMinutes: accessTokenMinutes,
	}
}

// CreateToken generates a new JWT token (and API key) for the given user ID and role.
func (m *Manager) CreateToken(userID uuid.UUID, role string) (*TokenDetails, error) {
	apiKey := uuid.NewString()
	now := time.Now().UTC()
	expiresAt := now.Add(time.Minute * time.Duration(m.accessTokenMinutes))

	claims := Claims{
		ApiKey: apiKey,
		Role:   role,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID.String(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedToken, err := token.SignedString(m.jwtSecret)
	if err != nil {
		return nil, err
	}

	return &TokenDetails{
		ApiKey:   apiKey,
		JWTToken: signedToken,
	}, nil
}

// ParseToken validates and parses a JWT string into claims.
func (m *Manager) ParseToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return m.jwtSecret, nil
	})
	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, jwt.ErrInvalidKey
	}
	// Reject MFA challenge JWTs and other tokens missing required access-token claims.
	if strings.TrimSpace(claims.ApiKey) == "" || strings.TrimSpace(claims.Role) == "" {
		return nil, jwt.ErrInvalidKey
	}
	return claims, nil
}
