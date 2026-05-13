package services

import (
	"context"
	"errors"
	"testing"
)

func TestRefreshTokenRejectsInvalidInputs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		refreshToken string
		svc          *authService
		wantErr      error
	}{
		{
			name:         "empty token",
			refreshToken: "",
			svc:          &authService{},
			wantErr:      ErrInvalidRefreshToken,
		},
		{
			name:         "whitespace token",
			refreshToken: "   ",
			svc:          &authService{},
			wantErr:      ErrInvalidRefreshToken,
		},
		{
			name:         "nil db",
			refreshToken: "valid-looking-token",
			svc:          &authService{db: nil},
			wantErr:      ErrSessionCreation,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := tc.svc.RefreshToken(context.Background(), tc.refreshToken)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("RefreshToken() error = %v, want %v", err, tc.wantErr)
			}
		})
	}
}

func TestLogoutRejectsInvalidInputs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		refreshToken string
		svc          *authService
		wantErr      error
	}{
		{name: "empty token", refreshToken: "", svc: &authService{}, wantErr: ErrInvalidRefreshToken},
		{name: "whitespace token", refreshToken: "  ", svc: &authService{}, wantErr: ErrInvalidRefreshToken},
		{name: "nil db", refreshToken: "token", svc: &authService{}, wantErr: ErrSessionCreation},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := tc.svc.Logout(context.Background(), tc.refreshToken)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("Logout() error = %v, want %v", err, tc.wantErr)
			}
		})
	}
}

func TestRegisterRejectsInvalidAndAdminRolesBeforeRepoCalls(t *testing.T) {
	t.Parallel()

	s := &authService{}
	input := &RegisterInput{
		FirstName: "Test",
		LastName:  "User",
		Email:     "test@example.com",
		Password:  "Password1!",
		PhoneNum:  "123",
	}

	input.Role = "not-a-role"
	_, _, err := s.Register(context.Background(), input)
	if !errors.Is(err, ErrInvalidRole) {
		t.Fatalf("Register() invalid role error = %v, want %v", err, ErrInvalidRole)
	}

	input.Role = "admin"
	_, _, err = s.Register(context.Background(), input)
	if !errors.Is(err, ErrForbiddenAdminRegistration) {
		t.Fatalf("Register() admin self-registration error = %v, want %v", err, ErrForbiddenAdminRegistration)
	}
}
