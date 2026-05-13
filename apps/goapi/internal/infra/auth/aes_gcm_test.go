package auth

import (
	"bytes"
	"testing"
)

func TestEncryptDecryptAESGCM_RoundTrip(t *testing.T) {
	t.Parallel()
	key := bytes.Repeat([]byte{9}, 32)
	plain := []byte("totp-secret-material")
	sealed, err := EncryptAESGCM(plain, key)
	if err != nil {
		t.Fatal(err)
	}
	out, err := DecryptAESGCM(sealed, key)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(out, plain) {
		t.Fatalf("got %q want %q", out, plain)
	}
}

func TestDecryptAESGCM_WrongKey(t *testing.T) {
	t.Parallel()
	key := bytes.Repeat([]byte{9}, 32)
	sealed, err := EncryptAESGCM([]byte("x"), key)
	if err != nil {
		t.Fatal(err)
	}
	wrong := bytes.Repeat([]byte{1}, 32)
	if _, err := DecryptAESGCM(sealed, wrong); err == nil {
		t.Fatal("expected error")
	}
}
