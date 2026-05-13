package auth

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"io"
)

const aesGCMNonceSize = 12

// EncryptAESGCM encrypts plaintext with AES-256-GCM. Output is nonce || ciphertext||tag.
func EncryptAESGCM(plaintext, key []byte) ([]byte, error) {
	if len(key) != 32 {
		return nil, errors.New("AES-256-GCM requires a 32-byte key")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, aesGCMNonceSize)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	// Nonce is random (ReadFull above). G407 flags any `nonce` variable; this is not a hardcoded IV.
	// #nosec G407 -- IV is filled from crypto/rand immediately before Seal.
	ciphertext := gcm.Seal(nil, nonce, plaintext, nil)
	out := make([]byte, 0, aesGCMNonceSize+len(ciphertext))
	out = append(out, nonce...)
	out = append(out, ciphertext...)
	return out, nil
}

// DecryptAESGCM decrypts data produced by EncryptAESGCM.
func DecryptAESGCM(sealed, key []byte) ([]byte, error) {
	if len(key) != 32 {
		return nil, errors.New("AES-256-GCM requires a 32-byte key")
	}
	if len(sealed) < aesGCMNonceSize+gcmOverheadMin {
		return nil, errors.New("ciphertext too short")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := sealed[:aesGCMNonceSize]
	ct := sealed[aesGCMNonceSize:]
	return gcm.Open(nil, nonce, ct, nil)
}

const gcmOverheadMin = 16 // GCM tag
