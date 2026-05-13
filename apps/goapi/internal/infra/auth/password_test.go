package auth

import "testing"

func TestHashAndComparePassword(t *testing.T) {
	t.Parallel()

	password := "StrongP@ssw0rd"
	hashed, err := HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword() error = %v", err)
	}
	if hashed == "" {
		t.Fatal("expected non-empty hash")
	}
	if hashed == password {
		t.Fatal("expected hashed value to differ from plaintext")
	}
	if !ComparePasswords(hashed, password) {
		t.Fatal("expected ComparePasswords() to accept correct password")
	}
	if ComparePasswords(hashed, "WrongPassword123!") {
		t.Fatal("expected ComparePasswords() to reject wrong password")
	}
}
