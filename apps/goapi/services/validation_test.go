package services

import (
	"errors"
	"testing"
)

func TestValidatePasswordStrength(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		wantErr error
	}{
		{name: "too short", input: "Aa1!", wantErr: ErrPasswordTooShort},
		{name: "missing uppercase", input: "password1!", wantErr: ErrPasswordNoUpper},
		{name: "missing lowercase", input: "PASSWORD1!", wantErr: ErrPasswordNoLower},
		{name: "missing number", input: "Password!", wantErr: ErrPasswordNoNumber},
		{name: "missing special", input: "Password1", wantErr: ErrPasswordNoSpecial},
		{name: "valid", input: "Password1!", wantErr: nil},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := ValidatePasswordStrength(tc.input)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("ValidatePasswordStrength(%q) error = %v, want %v", tc.input, err, tc.wantErr)
			}
		})
	}
}
