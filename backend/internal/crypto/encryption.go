// Package crypto provides cryptographic services for sensitive data.
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
)

// cipherFactory creates block ciphers. Used for testing.
type cipherFactory func(key []byte) (cipher.Block, error)

// gcmFactory creates GCM ciphers. Used for testing.
type gcmFactory func(cipher cipher.Block) (cipher.AEAD, error)

// randReader provides random bytes. Used for testing.
type randReader func(b []byte) (n int, err error)

// EncryptionService provides AES-256-GCM encryption and decryption.
// The service is thread-safe and can be shared across goroutines.
type EncryptionService struct {
	key           []byte // 32 bytes for AES-256
	cipherFactory cipherFactory
	gcmFactory    gcmFactory
	randReader    randReader
}

// NewEncryptionService creates a new encryption service with the provided base64-encoded key.
// The key must be exactly 32 bytes (256 bits) when decoded.
func NewEncryptionService(keyBase64 string) (*EncryptionService, error) {
	key, err := base64.StdEncoding.DecodeString(keyBase64)
	if err != nil {
		return nil, fmt.Errorf("invalid base64 key: %w", err)
	}

	if len(key) != 32 {
		return nil, fmt.Errorf("invalid key length: expected 32 bytes, got %d bytes", len(key))
	}

	return &EncryptionService{
		key:           key,
		cipherFactory: aes.NewCipher,
		gcmFactory:    cipher.NewGCM,
		randReader:    rand.Read,
	}, nil
}

// Encrypt encrypts plaintext using AES-256-GCM and returns base64-encoded ciphertext.
// The nonce is randomly generated and prepended to the ciphertext.
func (s *EncryptionService) Encrypt(plaintext []byte) (string, error) {
	block, err := s.cipherFactory(s.key)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := s.gcmFactory(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}

	// Generate random nonce
	nonce := make([]byte, gcm.NonceSize())
	if _, err := s.randReader(nonce); err != nil {
		return "", fmt.Errorf("failed to generate nonce: %w", err)
	}

	// Encrypt and prepend nonce to ciphertext
	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)

	// Return base64-encoded result
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt decrypts base64-encoded ciphertext using AES-256-GCM.
// The nonce is expected to be prepended to the ciphertext.
func (s *EncryptionService) Decrypt(ciphertextB64 string) ([]byte, error) {
	ciphertext, err := base64.StdEncoding.DecodeString(ciphertextB64)
	if err != nil {
		return nil, fmt.Errorf("invalid base64 ciphertext: %w", err)
	}

	block, err := s.cipherFactory(s.key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := s.gcmFactory(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short: expected at least %d bytes, got %d bytes", nonceSize, len(ciphertext))
	}

	// Extract nonce and ciphertext
	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]

	// Decrypt
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("decryption failed: %w", err)
	}

	return plaintext, nil
}
