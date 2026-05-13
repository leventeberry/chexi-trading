package auth

import (
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

// PasswordCost defines the bcrypt hashing cost.
const PasswordCost = bcrypt.DefaultCost

// HashPassword generates a bcrypt hash of the given plaintext password.
func HashPassword(password string) (string, error) {
	hashed, err := bcrypt.GenerateFromPassword([]byte(password), PasswordCost)
	if err != nil {
		return "", fmt.Errorf("password hashing failed: %w", err)
	}
	return string(hashed), nil
}

// ComparePasswords checks whether plaintext password matches the stored bcrypt hash.
func ComparePasswords(hashedPassword, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password)) == nil
}
