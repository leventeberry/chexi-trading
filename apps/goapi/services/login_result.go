package services

import "goapi/models"

// LoginResult is the outcome of password authentication (full session or MFA step-up).
type LoginResult struct {
	User              *models.User
	Auth              *Authentication // set when MFA is not required
	MFARequired       bool
	MFAChallengeToken string
}

// TOTPSetupResult is returned once from setup; do not log or persist the secret client-side beyond enrollment.
type TOTPSetupResult struct {
	Secret string
	URI    string
}

// TOTPConfirmResult may include one-time recovery codes (plaintext); only hashes are stored.
type TOTPConfirmResult struct {
	RecoveryCodes []string
}
