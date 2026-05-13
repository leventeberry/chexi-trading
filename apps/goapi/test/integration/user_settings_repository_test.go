//go:build integration

package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"goapi/models"
	"goapi/repositories"
)

func TestUserSettingsRepository_FindByUserID_NotFound(t *testing.T) {
	u := repoTestUser(t, "sett-nf")
	repo := repositories.NewUserSettingsRepository(testDB)

	_, err := repo.FindByUserID(u.ID)
	if !errors.Is(err, repositories.ErrUserSettingsNotFound) {
		t.Fatalf("err = %v", err)
	}
}

func TestUserSettingsRepository_Upsert_InsertThenUpdateSingleRow(t *testing.T) {
	u := repoTestUser(t, "sett-up")
	repo := repositories.NewUserSettingsRepository(testDB)

	t.Cleanup(func() {
		testDB.Exec(`DELETE FROM user_settings WHERE user_id = ?`, u.ID)
	})

	s := &models.UserSettings{
		UserID: u.ID,
		Theme:  "light",
	}
	if err := repo.Upsert(s); err != nil {
		t.Fatal(err)
	}

	var count int64
	if err := testDB.Model(&models.UserSettings{}).
		Where("user_id = ?", u.ID).
		Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("row count = %d want 1", count)
	}

	got, err := repo.FindByUserID(u.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Theme != "light" {
		t.Fatalf("theme = %q", got.Theme)
	}

	s.Theme = "dark"
	s.MarketingEmailOptIn = true
	if err := repo.Upsert(s); err != nil {
		t.Fatal(err)
	}

	if err := testDB.Model(&models.UserSettings{}).
		Where("user_id = ?", u.ID).
		Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("after second upsert row count = %d", count)
	}

	got2, err := repo.FindByUserID(u.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got2.Theme != "dark" || !got2.MarketingEmailOptIn {
		t.Fatalf("updated settings %+v", got2)
	}
}

func TestUserSettingsRepository_Upsert_DefaultsForEmptyThemeAndNilJSON(t *testing.T) {
	u := repoTestUser(t, "sett-def")
	repo := repositories.NewUserSettingsRepository(testDB)

	t.Cleanup(func() {
		testDB.Exec(`DELETE FROM user_settings WHERE user_id = ?`, u.ID)
	})

	if err := repo.Upsert(&models.UserSettings{
		UserID:                    u.ID,
		Theme:                     "",
		NotificationPreferences:   nil,
		ExtraSettings:             nil,
		MarketingEmailOptIn:       false,
		SecurityNotificationOptIn: true,
	}); err != nil {
		t.Fatal(err)
	}

	got, err := repo.FindByUserID(u.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Theme != "system" {
		t.Fatalf("default theme = %q", got.Theme)
	}
	if !bytes.Equal(got.NotificationPreferences, []byte(`{}`)) {
		t.Fatalf("notification_preferences = %s", got.NotificationPreferences)
	}
	if !bytes.Equal(got.ExtraSettings, []byte(`{}`)) {
		t.Fatalf("extra_settings = %s", got.ExtraSettings)
	}
}

func TestUserSettingsRepository_Upsert_JSONMergeOnSecondWrite(t *testing.T) {
	u := repoTestUser(t, "sett-json")
	repo := repositories.NewUserSettingsRepository(testDB)
	t.Cleanup(func() {
		testDB.Exec(`DELETE FROM user_settings WHERE user_id = ?`, u.ID)
	})

	prefs1 := []byte(`{"a":true}`)
	if err := repo.Upsert(&models.UserSettings{
		UserID:                  u.ID,
		Theme:                   "dark",
		NotificationPreferences: prefs1,
		ExtraSettings:           json.RawMessage(`{"k":1}`),
	}); err != nil {
		t.Fatal(err)
	}

	prefs2 := []byte(`{"b":false}`)
	if err := repo.Upsert(&models.UserSettings{
		UserID:                  u.ID,
		Theme:                   "dark",
		NotificationPreferences: prefs2,
		ExtraSettings:           json.RawMessage(`{"k":2}`),
	}); err != nil {
		t.Fatal(err)
	}

	got, err := repo.FindByUserID(u.ID)
	if err != nil {
		t.Fatal(err)
	}
	var prefs map[string]interface{}
	if err := json.Unmarshal(got.NotificationPreferences, &prefs); err != nil {
		t.Fatal(err)
	}
	bVal, ok := prefs["b"].(bool)
	if !ok || bVal {
		t.Fatalf("preferences want b=false got %v (%#v)", prefs["b"], prefs)
	}
	var extra map[string]interface{}
	if err := json.Unmarshal(got.ExtraSettings, &extra); err != nil {
		t.Fatal(err)
	}
	k, ok := extra["k"].(float64)
	if !ok || int(k) != 2 {
		t.Fatalf("extra k = %v (%#v)", extra["k"], extra)
	}
}

func TestUserSettingsRepository_Upsert_UpdatesTimestampsEventually(t *testing.T) {
	u := repoTestUser(t, "sett-ts")
	repo := repositories.NewUserSettingsRepository(testDB)
	t.Cleanup(func() {
		testDB.Exec(`DELETE FROM user_settings WHERE user_id = ?`, u.ID)
	})

	if err := repo.Upsert(&models.UserSettings{UserID: u.ID, Theme: "light"}); err != nil {
		t.Fatal(err)
	}
	got1, err := repo.FindByUserID(u.ID)
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(50 * time.Millisecond)
	if err := repo.Upsert(&models.UserSettings{UserID: u.ID, Theme: "dark"}); err != nil {
		t.Fatal(err)
	}
	got2, err := repo.FindByUserID(u.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !got2.UpdatedAt.After(got1.UpdatedAt) {
		t.Fatalf("expected updated_at to advance: before=%v after=%v", got1.UpdatedAt, got2.UpdatedAt)
	}
}
