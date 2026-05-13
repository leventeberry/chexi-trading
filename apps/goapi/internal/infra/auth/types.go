package auth

import "github.com/golang-jwt/jwt/v5"

// Claims defines the JWT payload structure.
type Claims struct {
	ApiKey string `json:"api_key"`
	Role   string `json:"role"`
	jwt.RegisteredClaims
}

// TokenDetails holds generated token details for authenticated sessions.
type TokenDetails struct {
	ApiKey   string `json:"api_key"`
	JWTToken string `json:"jwt_token"`
}
