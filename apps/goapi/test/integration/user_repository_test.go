//go:build integration

package main

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"goapi/internal/rbac"
	"goapi/models"
	"goapi/repositories"
)

func uniqueRepoUserEmail(prefix string) string {
	return fmt.Sprintf("%s-%d@repo-it.test", prefix, time.Now().UnixNano())
}

func TestUserRepository_CreateNormalizesEmail(t *testing.T) {
	repo := repositories.NewUserRepository(testDB)
	raw := "  MixedCase@Repo-IT.Test  "
	u := &models.User{
		FirstName: "A",
		LastName:  "B",
		Email:     raw,
		PassHash:  "hash",
		PhoneNum:  "",
		Role:      rbac.RoleUser.String(),
	}
	if err := repo.Create(u); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = repo.Delete(u.ID) })

	if u.Email != "mixedcase@repo-it.test" {
		t.Fatalf("in-memory email after Create: %q", u.Email)
	}
	var row models.User
	if err := testDB.First(&row, "id = ?", u.ID).Error; err != nil {
		t.Fatal(err)
	}
	if row.Email != "mixedcase@repo-it.test" {
		t.Fatalf("persisted email: %q", row.Email)
	}
}

func TestUserRepository_FindByEmail_CaseInsensitive(t *testing.T) {
	repo := repositories.NewUserRepository(testDB)
	u := &models.User{
		FirstName: "C",
		LastName:  "D",
		Email:     uniqueRepoUserEmail("case"),
		PassHash:  "h",
		Role:      rbac.RoleUser.String(),
	}
	if err := repo.Create(u); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = repo.Delete(u.ID) })

	want := u.Email // normalized lowercase
	fromUpper, err := repo.FindByEmail(stringsToUpperEmail(want))
	if err != nil {
		t.Fatal(err)
	}
	if fromUpper.ID != u.ID {
		t.Fatalf("FindByEmail upper query: got id %v", fromUpper.ID)
	}
}

func stringsToUpperEmail(s string) string {
	b := make([]byte, len(s))
	for i := range s {
		c := s[i]
		if c >= 'a' && c <= 'z' {
			b[i] = c - 32
		} else {
			b[i] = c
		}
	}
	return string(b)
}

func TestUserRepository_FindByEmail_MixedCaseStoredRow(t *testing.T) {
	repo := repositories.NewUserRepository(testDB)
	id := uuid.New()
	emailStored := "WeIrD@MiXeD.Repo-It"
	// Bypass Create normalization to assert LOWER() lookup on legacy-shaped row.
	res := testDB.Exec(`
		INSERT INTO users (id, first_name, last_name, email, pass_hash, phone_num, role, created_at, updated_at)
		VALUES (?, 'X', 'Y', ?, 'p', '', ?, NOW(), NOW())
	`, id, emailStored, rbac.RoleUser.String())
	if res.Error != nil {
		t.Fatal(res.Error)
	}
	t.Cleanup(func() { _ = repo.Delete(id) })

	found, err := repo.FindByEmail("weird@mixed.repo-it")
	if err != nil {
		t.Fatal(err)
	}
	if found.ID != id {
		t.Fatalf("FindByEmail: got %v want %v", found.ID, id)
	}
}

func TestUserRepository_FindByID_NotFound(t *testing.T) {
	repo := repositories.NewUserRepository(testDB)
	_, err := repo.FindByID(uuid.MustParse("aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"))
	if !errors.Is(err, repositories.ErrUserNotFound) {
		t.Fatalf("err = %v", err)
	}
}

func TestUserRepository_Delete_NotFound(t *testing.T) {
	repo := repositories.NewUserRepository(testDB)
	err := repo.Delete(uuid.MustParse("bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"))
	if !errors.Is(err, repositories.ErrUserNotFound) {
		t.Fatalf("err = %v", err)
	}
}

func TestUserRepository_Update_PersistsScalars(t *testing.T) {
	repo := repositories.NewUserRepository(testDB)
	u := &models.User{
		FirstName: "Old",
		LastName:  "Name",
		Email:     uniqueRepoUserEmail("upd"),
		PassHash:  "h",
		Role:      rbac.RoleUser.String(),
	}
	if err := repo.Create(u); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = repo.Delete(u.ID) })

	u.FirstName = "New"
	u.LastName = "Sur"
	if err := repo.Update(u); err != nil {
		t.Fatal(err)
	}
	got, err := repo.FindByID(u.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.FirstName != "New" || got.LastName != "Sur" {
		t.Fatalf("got %+v", got)
	}
}

func TestUserRepository_FindAllWithPagination_TotalsAndPages(t *testing.T) {
	repo := repositories.NewUserRepository(testDB)
	var baseline int64
	if err := testDB.Model(&models.User{}).Count(&baseline).Error; err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 3; i++ {
		u := &models.User{
			FirstName: "P",
			LastName:  fmt.Sprintf("%d", i),
			Email:     uniqueRepoUserEmail(fmt.Sprintf("pag%d", i)),
			PassHash:  "h",
			Role:      rbac.RoleUser.String(),
		}
		if err := repo.Create(u); err != nil {
			t.Fatal(err)
		}
		uid := u.ID
		t.Cleanup(func() { _ = repo.Delete(uid) })
	}

	page1, total, err := repo.FindAllWithPagination(1, 2)
	if err != nil {
		t.Fatal(err)
	}
	if total != baseline+3 {
		t.Fatalf("total = %d, want %d", total, baseline+3)
	}
	if len(page1) != 2 {
		t.Fatalf("page1 len = %d", len(page1))
	}

	page2, total2, err := repo.FindAllWithPagination(2, 2)
	if err != nil {
		t.Fatal(err)
	}
	if total2 != total {
		t.Fatalf("total mismatch page2")
	}
	remaining := int(total - 2)
	if remaining < 0 {
		remaining = 0
	}
	expectPage2 := remaining
	if expectPage2 > 2 {
		expectPage2 = 2
	}
	if len(page2) != expectPage2 {
		t.Fatalf("page2 len = %d, want %d (total=%d pageSize=2)", len(page2), expectPage2, total)
	}
}

func TestUserRepository_ExistsByEmail(t *testing.T) {
	repo := repositories.NewUserRepository(testDB)
	email := uniqueRepoUserEmail("exists")
	ok, err := repo.ExistsByEmail(email)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("unexpected exists before create")
	}

	u := &models.User{
		FirstName: "E",
		LastName:  "E",
		Email:     email,
		PassHash:  "h",
		Role:      rbac.RoleUser.String(),
	}
	if err := repo.Create(u); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = repo.Delete(u.ID) })

	ok, err = repo.ExistsByEmail(stringsToUpperEmail(email))
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected exists with different case query")
	}
}

func TestUserRepository_Delete_RemovesRow(t *testing.T) {
	repo := repositories.NewUserRepository(testDB)
	u := &models.User{
		FirstName: "D",
		LastName:  "D",
		Email:     uniqueRepoUserEmail("del"),
		PassHash:  "h",
		Role:      rbac.RoleUser.String(),
	}
	if err := repo.Create(u); err != nil {
		t.Fatal(err)
	}
	if err := repo.Delete(u.ID); err != nil {
		t.Fatal(err)
	}
	_, err := repo.FindByID(u.ID)
	if !errors.Is(err, repositories.ErrUserNotFound) {
		t.Fatalf("after delete FindByID err = %v", err)
	}
}

func TestUserRepository_ExistsAnyAdmin_MatchesManualCount(t *testing.T) {
	repo := repositories.NewUserRepository(testDB)
	var want int64
	if err := testDB.Model(&models.User{}).
		Where("role = ?", rbac.RoleAdmin.String()).
		Count(&want).Error; err != nil {
		t.Fatal(err)
	}
	got, err := repo.ExistsAnyAdmin()
	if err != nil {
		t.Fatal(err)
	}
	if got != (want > 0) {
		t.Fatalf("ExistsAnyAdmin=%v manual count=%d", got, want)
	}
}
