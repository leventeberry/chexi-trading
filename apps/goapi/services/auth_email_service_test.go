package services

import (
	"context"
	"errors"
	"testing"

	"goapi/repositories"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestVerifyEmailRejectsMissingInputs(t *testing.T) {
	t.Parallel()

	db := openSQLiteDBForTest(t)
	tokenRepo := repositories.NewAuthTokenRepository(db)

	tests := []struct {
		name     string
		rawToken string
		repo     repositories.AuthTokenRepository
		db       *gorm.DB
	}{
		{name: "empty token", rawToken: "", repo: tokenRepo, db: db},
		{name: "nil token repo", rawToken: "token", repo: nil, db: db},
		{name: "nil db", rawToken: "token", repo: tokenRepo, db: nil},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			s := &authService{tokenRepo: tc.repo, db: tc.db}
			err := s.VerifyEmail(context.Background(), tc.rawToken)
			if !errors.Is(err, ErrInvalidVerificationToken) {
				t.Fatalf("VerifyEmail() error = %v, want %v", err, ErrInvalidVerificationToken)
			}
		})
	}
}

func TestConfirmPasswordResetValidationBranches(t *testing.T) {
	t.Parallel()

	db := openSQLiteDBForTest(t)
	tokenRepo := repositories.NewAuthTokenRepository(db)

	tests := []struct {
		name        string
		rawToken    string
		newPassword string
		repo        repositories.AuthTokenRepository
		db          *gorm.DB
		wantErr     error
	}{
		{
			name:        "empty token",
			rawToken:    "",
			newPassword: "Password1!",
			repo:        tokenRepo,
			db:          db,
			wantErr:     ErrInvalidPasswordResetToken,
		},
		{
			name:        "nil token repo",
			rawToken:    "token",
			newPassword: "Password1!",
			repo:        nil,
			db:          db,
			wantErr:     ErrInvalidPasswordResetToken,
		},
		{
			name:        "weak password checked before lookup",
			rawToken:    "token",
			newPassword: "short",
			repo:        tokenRepo,
			db:          db,
			wantErr:     ErrPasswordTooShort,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			s := &authService{tokenRepo: tc.repo, db: tc.db}
			err := s.ConfirmPasswordReset(context.Background(), tc.rawToken, tc.newPassword)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("ConfirmPasswordReset() error = %v, want %v", err, tc.wantErr)
			}
		})
	}
}

func openSQLiteDBForTest(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	return db
}
