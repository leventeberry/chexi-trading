//go:build integration

package main

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"testing"
	"time"

	"goapi/internal/rbac"
	"goapi/models"
	"goapi/repositories"

	"gorm.io/gorm"
)

func uniqueTokenHash(label string) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s-%d-%d", label, time.Now().UnixNano(), time.Now().Nanosecond())))
	return hex.EncodeToString(sum[:])
}

func repoTestUser(t *testing.T, prefix string) *models.User {
	t.Helper()
	repo := repositories.NewUserRepository(testDB)
	u := &models.User{
		FirstName: "Tok",
		LastName:  "User",
		Email:     uniqueRepoUserEmail(prefix),
		PassHash:  "hash",
		Role:      rbac.RoleUser.String(),
	}
	if err := repo.Create(u); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = repo.Delete(u.ID) })
	return u
}

func TestAuthTokenRepository_EmailVerification_FindMark_FindAgainNotFound(t *testing.T) {
	u := repoTestUser(t, "ev1")
	repo := repositories.NewAuthTokenRepository(testDB)

	hash := uniqueTokenHash("ev")
	tok := &models.EmailVerificationToken{
		UserID:    u.ID,
		TokenHash: hash,
		ExpiresAt: time.Now().UTC().Add(time.Hour),
	}
	if err := repo.CreateEmailVerification(nil, tok); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		testDB.Exec(`DELETE FROM email_verification_tokens WHERE user_id = ?`, u.ID)
	})

	got, err := repo.FindActiveEmailVerificationByHash(nil, hash)
	if err != nil {
		t.Fatal(err)
	}
	if got.TokenHash != hash {
		t.Fatalf("hash mismatch")
	}

	now := time.Now().UTC()
	if err := repo.MarkEmailVerificationUsed(nil, got.ID, now); err != nil {
		t.Fatal(err)
	}
	_, err = repo.FindActiveEmailVerificationByHash(nil, hash)
	if !errors.Is(err, repositories.ErrTokenNotFound) {
		t.Fatalf("after mark used Find err = %v", err)
	}
	if err := repo.MarkEmailVerificationUsed(nil, got.ID, now); !errors.Is(err, repositories.ErrTokenNotFound) {
		t.Fatalf("second mark err = %v", err)
	}
}

func TestAuthTokenRepository_EmailVerification_FindActiveStillReturnsExpiredWhenUnused(t *testing.T) {
	// Repo does not filter expires_at — service layer rejects; lock current contract.
	u := repoTestUser(t, "evexp")
	repo := repositories.NewAuthTokenRepository(testDB)

	hash := uniqueTokenHash("exp")
	tok := &models.EmailVerificationToken{
		UserID:    u.ID,
		TokenHash: hash,
		ExpiresAt: time.Now().UTC().Add(-time.Hour),
	}
	if err := repo.CreateEmailVerification(nil, tok); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		testDB.Exec(`DELETE FROM email_verification_tokens WHERE user_id = ?`, u.ID)
	})

	got, err := repo.FindActiveEmailVerificationByHash(nil, hash)
	if err != nil {
		t.Fatal(err)
	}
	if !got.ExpiresAt.Before(time.Now().UTC()) {
		t.Fatal("expected expired row")
	}
}

func TestAuthTokenRepository_EmailVerification_DeleteUnusedForUser(t *testing.T) {
	u := repoTestUser(t, "delv")
	repo := repositories.NewAuthTokenRepository(testDB)

	h1 := uniqueTokenHash("d1")
	h2 := uniqueTokenHash("d2")
	if err := repo.CreateEmailVerification(nil, &models.EmailVerificationToken{
		UserID: u.ID, TokenHash: h1, ExpiresAt: time.Now().UTC().Add(time.Hour),
	}); err != nil {
		t.Fatal(err)
	}
	t2 := &models.EmailVerificationToken{UserID: u.ID, TokenHash: h2, ExpiresAt: time.Now().UTC().Add(time.Hour)}
	if err := repo.CreateEmailVerification(nil, t2); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		testDB.Exec(`DELETE FROM email_verification_tokens WHERE user_id = ?`, u.ID)
	})

	if err := repo.MarkEmailVerificationUsed(nil, t2.ID, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	if err := repo.DeleteUnusedEmailVerificationsForUser(nil, u.ID); err != nil {
		t.Fatal(err)
	}
	_, err := repo.FindActiveEmailVerificationByHash(nil, h1)
	if !errors.Is(err, repositories.ErrTokenNotFound) {
		t.Fatalf("unused token should be deleted err=%v", err)
	}
	_, err = repo.FindActiveEmailVerificationByHash(nil, h2)
	if !errors.Is(err, repositories.ErrTokenNotFound) {
		t.Fatalf("used token should remain non-active err=%v", err)
	}
}

func TestAuthTokenRepository_EmailVerification_LatestCreatedAt(t *testing.T) {
	u := repoTestUser(t, "latest-ev")
	repo := repositories.NewAuthTokenRepository(testDB)

	h1 := uniqueTokenHash("l1")
	h2 := uniqueTokenHash("l2")
	if err := repo.CreateEmailVerification(nil, &models.EmailVerificationToken{
		UserID: u.ID, TokenHash: h1, ExpiresAt: time.Now().UTC().Add(time.Hour),
	}); err != nil {
		t.Fatal(err)
	}
	time.Sleep(25 * time.Millisecond)
	if err := repo.CreateEmailVerification(nil, &models.EmailVerificationToken{
		UserID: u.ID, TokenHash: h2, ExpiresAt: time.Now().UTC().Add(time.Hour),
	}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		testDB.Exec(`DELETE FROM email_verification_tokens WHERE user_id = ?`, u.ID)
	})

	got, ok, err := repo.LatestEmailVerificationCreatedAt(u.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected latest row")
	}
	var newest time.Time
	if err := testDB.Raw(`
		SELECT MAX(created_at) FROM email_verification_tokens WHERE user_id = ?
	`, u.ID).Scan(&newest).Error; err != nil {
		t.Fatal(err)
	}
	if !got.Equal(newest) || got.IsZero() {
		t.Fatalf("Latest=%v want max=%v", got, newest)
	}
}

func TestAuthTokenRepository_EmailVerification_TxCommitVisible(t *testing.T) {
	u := repoTestUser(t, "tx-ev")
	repo := repositories.NewAuthTokenRepository(testDB)
	hash := uniqueTokenHash("tx")

	t.Cleanup(func() {
		testDB.Exec(`DELETE FROM email_verification_tokens WHERE user_id = ?`, u.ID)
	})

	tok := &models.EmailVerificationToken{
		UserID:    u.ID,
		TokenHash: hash,
		ExpiresAt: time.Now().UTC().Add(time.Hour),
	}
	err := testDB.Transaction(func(tx *gorm.DB) error {
		if err := repo.CreateEmailVerification(tx, tok); err != nil {
			return err
		}
		_, err := repo.FindActiveEmailVerificationByHash(tx, hash)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := repo.FindActiveEmailVerificationByHash(nil, hash); err != nil {
		t.Fatalf("after tx commit visible on default db err=%v", err)
	}
}

func TestAuthTokenRepository_PasswordReset_FindMark_FindAgainNotFound(t *testing.T) {
	u := repoTestUser(t, "pr1")
	repo := repositories.NewAuthTokenRepository(testDB)

	hash := uniqueTokenHash("pr")
	tok := &models.PasswordResetToken{
		UserID:    u.ID,
		TokenHash: hash,
		ExpiresAt: time.Now().UTC().Add(time.Hour),
	}
	if err := repo.CreatePasswordReset(nil, tok); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		testDB.Exec(`DELETE FROM password_reset_tokens WHERE user_id = ?`, u.ID)
	})

	got, err := repo.FindActivePasswordResetByHash(nil, hash)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.MarkPasswordResetUsed(nil, got.ID, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	_, err = repo.FindActivePasswordResetByHash(nil, hash)
	if !errors.Is(err, repositories.ErrTokenNotFound) {
		t.Fatalf("err = %v", err)
	}
}

func TestAuthTokenRepository_PasswordReset_FindActiveStillReturnsExpiredWhenUnused(t *testing.T) {
	u := repoTestUser(t, "prexp")
	repo := repositories.NewAuthTokenRepository(testDB)
	hash := uniqueTokenHash("pre")
	tok := &models.PasswordResetToken{
		UserID:    u.ID,
		TokenHash: hash,
		ExpiresAt: time.Now().UTC().Add(-time.Hour),
	}
	if err := repo.CreatePasswordReset(nil, tok); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		testDB.Exec(`DELETE FROM password_reset_tokens WHERE user_id = ?`, u.ID)
	})

	got, err := repo.FindActivePasswordResetByHash(nil, hash)
	if err != nil {
		t.Fatal(err)
	}
	if !got.ExpiresAt.Before(time.Now().UTC()) {
		t.Fatal("expected expired")
	}
}

func TestAuthTokenRepository_PasswordReset_DeleteUnusedForUser(t *testing.T) {
	u := repoTestUser(t, "prdel")
	repo := repositories.NewAuthTokenRepository(testDB)
	h1 := uniqueTokenHash("p1")
	h2 := uniqueTokenHash("p2")
	if err := repo.CreatePasswordReset(nil, &models.PasswordResetToken{
		UserID: u.ID, TokenHash: h1, ExpiresAt: time.Now().UTC().Add(time.Hour),
	}); err != nil {
		t.Fatal(err)
	}
	t2 := &models.PasswordResetToken{UserID: u.ID, TokenHash: h2, ExpiresAt: time.Now().UTC().Add(time.Hour)}
	if err := repo.CreatePasswordReset(nil, t2); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		testDB.Exec(`DELETE FROM password_reset_tokens WHERE user_id = ?`, u.ID)
	})

	if err := repo.MarkPasswordResetUsed(nil, t2.ID, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	if err := repo.DeleteUnusedPasswordResetsForUser(nil, u.ID); err != nil {
		t.Fatal(err)
	}
	_, err := repo.FindActivePasswordResetByHash(nil, h1)
	if !errors.Is(err, repositories.ErrTokenNotFound) {
		t.Fatalf("unused deleted err=%v", err)
	}
}

func TestAuthTokenRepository_PasswordReset_LatestCreatedAt(t *testing.T) {
	u := repoTestUser(t, "lpr")
	repo := repositories.NewAuthTokenRepository(testDB)
	h1 := uniqueTokenHash("q1")
	h2 := uniqueTokenHash("q2")
	if err := repo.CreatePasswordReset(nil, &models.PasswordResetToken{
		UserID: u.ID, TokenHash: h1, ExpiresAt: time.Now().UTC().Add(time.Hour),
	}); err != nil {
		t.Fatal(err)
	}
	time.Sleep(25 * time.Millisecond)
	if err := repo.CreatePasswordReset(nil, &models.PasswordResetToken{
		UserID: u.ID, TokenHash: h2, ExpiresAt: time.Now().UTC().Add(time.Hour),
	}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		testDB.Exec(`DELETE FROM password_reset_tokens WHERE user_id = ?`, u.ID)
	})

	got, ok, err := repo.LatestPasswordResetCreatedAt(u.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected latest")
	}
	var newest time.Time
	if err := testDB.Raw(`SELECT MAX(created_at) FROM password_reset_tokens WHERE user_id = ?`, u.ID).Scan(&newest).Error; err != nil {
		t.Fatal(err)
	}
	if !got.Equal(newest) {
		t.Fatalf("Latest=%v max=%v", got, newest)
	}
}

func TestAuthTokenRepository_PasswordReset_TxCommitVisible(t *testing.T) {
	u := repoTestUser(t, "tx-pr")
	repo := repositories.NewAuthTokenRepository(testDB)
	hash := uniqueTokenHash("txp")

	t.Cleanup(func() {
		testDB.Exec(`DELETE FROM password_reset_tokens WHERE user_id = ?`, u.ID)
	})

	tok := &models.PasswordResetToken{
		UserID:    u.ID,
		TokenHash: hash,
		ExpiresAt: time.Now().UTC().Add(time.Hour),
	}
	err := testDB.Transaction(func(tx *gorm.DB) error {
		if err := repo.CreatePasswordReset(tx, tok); err != nil {
			return err
		}
		_, err := repo.FindActivePasswordResetByHash(tx, hash)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.FindActivePasswordResetByHash(nil, hash); err != nil {
		t.Fatal(err)
	}
}
