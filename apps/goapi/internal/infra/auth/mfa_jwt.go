package auth

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// MFAChallengeClaims is a dedicated JWT shape for step-up login; must not be accepted by ParseToken (access tokens).
type MFAChallengeClaims struct {
	jwt.RegisteredClaims
}

var (
	// ErrNotMFAChallengeToken indicates the JWT is not a valid MFA challenge token.
	ErrNotMFAChallengeToken = errors.New("not an MFA challenge token")
)

// CreateMFAChallengeToken issues a short-lived HS256 JWT binding user_id (sub) and jti (id claim).
func (m *Manager) CreateMFAChallengeToken(userID uuid.UUID, jti string, ttl time.Duration) (string, error) {
	if ttl < time.Minute {
		ttl = time.Minute
	}
	if ttl > 30*time.Minute {
		ttl = 30 * time.Minute
	}
	now := time.Now().UTC()
	claims := MFAChallengeClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID.String(),
			ID:        jti,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(m.jwtSecret)
}

// ParseMFAChallengeToken validates an MFA challenge JWT and returns user id + jti.
func (m *Manager) ParseMFAChallengeToken(tokenString string) (userID uuid.UUID, jti string, err error) {
	token, err := jwt.ParseWithClaims(tokenString, &MFAChallengeClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return m.jwtSecret, nil
	})
	if err != nil {
		return uuid.Nil, "", err
	}
	claims, ok := token.Claims.(*MFAChallengeClaims)
	if !ok || !token.Valid {
		return uuid.Nil, "", ErrNotMFAChallengeToken
	}
	if claims.Subject == "" || claims.ID == "" {
		return uuid.Nil, "", ErrNotMFAChallengeToken
	}
	uid, err := uuid.Parse(claims.Subject)
	if err != nil {
		return uuid.Nil, "", ErrNotMFAChallengeToken
	}
	return uid, claims.ID, nil
}
