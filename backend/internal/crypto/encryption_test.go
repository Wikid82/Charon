package crypto

import (
	"crypto/rand"
	"encoding/base64"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewEncryptionService_ValidKey tests successful creation with valid 32-byte key.
func TestNewEncryptionService_ValidKey(t *testing.T) {
	// Generate a valid 32-byte key
	key := make([]byte, 32)
	_, err := rand.Read(key)
	require.NoError(t, err)
	keyBase64 := base64.StdEncoding.EncodeToString(key)

	svc, err := NewEncryptionService(keyBase64)
	assert.NoError(t, err)
	assert.NotNil(t, svc)
	assert.Equal(t, 32, len(svc.key))
}

// TestNewEncryptionService_InvalidBase64 tests error handling for invalid base64.
func TestNewEncryptionService_InvalidBase64(t *testing.T) {
	invalidBase64 := "not-valid-base64!@#$"

	svc, err := NewEncryptionService(invalidBase64)
	assert.Error(t, err)
	assert.Nil(t, svc)
	assert.Contains(t, err.Error(), "invalid base64 key")
}

// TestNewEncryptionService_WrongKeyLength tests error handling for incorrect key length.
func TestNewEncryptionService_WrongKeyLength(t *testing.T) {
	tests := []struct {
		name      string
		keyLength int
	}{
		{"16 bytes", 16},
		{"24 bytes", 24},
		{"31 bytes", 31},
		{"33 bytes", 33},
		{"0 bytes", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key := make([]byte, tt.keyLength)
			_, _ = rand.Read(key)
			keyBase64 := base64.StdEncoding.EncodeToString(key)

			svc, err := NewEncryptionService(keyBase64)
			assert.Error(t, err)
			assert.Nil(t, svc)
			assert.Contains(t, err.Error(), "invalid key length")
		})
	}
}

// TestEncryptDecrypt_RoundTrip tests that encrypt followed by decrypt returns original plaintext.
func TestEncryptDecrypt_RoundTrip(t *testing.T) {
	key := make([]byte, 32)
	_, err := rand.Read(key)
	require.NoError(t, err)
	keyBase64 := base64.StdEncoding.EncodeToString(key)

	svc, err := NewEncryptionService(keyBase64)
	require.NoError(t, err)

	tests := []struct {
		name      string
		plaintext string
	}{
		{"simple text", "Hello, World!"},
		{"with special chars", "P@ssw0rd!#$%^&*()"},
		{"json data", `{"api_token":"sk_test_12345","region":"us-east-1"}`},
		{"unicode", "こんにちは世界 🌍"},
		{"long text", strings.Repeat("Lorem ipsum dolor sit amet. ", 100)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Encrypt
			ciphertext, err := svc.Encrypt([]byte(tt.plaintext))
			require.NoError(t, err)
			assert.NotEmpty(t, ciphertext)

			// Verify ciphertext is base64
			_, err = base64.StdEncoding.DecodeString(ciphertext)
			assert.NoError(t, err)

			// Decrypt
			decrypted, err := svc.Decrypt(ciphertext)
			require.NoError(t, err)
			assert.Equal(t, tt.plaintext, string(decrypted))
		})
	}
}

// TestEncrypt_EmptyPlaintext tests encryption of empty plaintext.
func TestEncrypt_EmptyPlaintext(t *testing.T) {
	key := make([]byte, 32)
	_, err := rand.Read(key)
	require.NoError(t, err)
	keyBase64 := base64.StdEncoding.EncodeToString(key)

	svc, err := NewEncryptionService(keyBase64)
	require.NoError(t, err)

	// Encrypt empty plaintext
	ciphertext, err := svc.Encrypt([]byte{})
	assert.NoError(t, err)
	assert.NotEmpty(t, ciphertext)

	// Decrypt should return empty plaintext
	decrypted, err := svc.Decrypt(ciphertext)
	assert.NoError(t, err)
	assert.Empty(t, decrypted)
}

// TestDecrypt_InvalidCiphertext tests decryption error handling.
func TestDecrypt_InvalidCiphertext(t *testing.T) {
	key := make([]byte, 32)
	_, err := rand.Read(key)
	require.NoError(t, err)
	keyBase64 := base64.StdEncoding.EncodeToString(key)

	svc, err := NewEncryptionService(keyBase64)
	require.NoError(t, err)

	tests := []struct {
		name       string
		ciphertext string
		errorMsg   string
	}{
		{
			name:       "invalid base64",
			ciphertext: "not-valid-base64!@#$",
			errorMsg:   "invalid base64 ciphertext",
		},
		{
			name:       "too short",
			ciphertext: base64.StdEncoding.EncodeToString([]byte("short")),
			errorMsg:   "ciphertext too short",
		},
		{
			name:       "empty string",
			ciphertext: "",
			errorMsg:   "ciphertext too short",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.Decrypt(tt.ciphertext)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.errorMsg)
		})
	}
}

// TestDecrypt_TamperedCiphertext tests that tampered ciphertext is detected.
func TestDecrypt_TamperedCiphertext(t *testing.T) {
	key := make([]byte, 32)
	_, err := rand.Read(key)
	require.NoError(t, err)
	keyBase64 := base64.StdEncoding.EncodeToString(key)

	svc, err := NewEncryptionService(keyBase64)
	require.NoError(t, err)

	// Encrypt valid plaintext
	original := "sensitive data"
	ciphertext, err := svc.Encrypt([]byte(original))
	require.NoError(t, err)

	// Decode, tamper, and re-encode
	ciphertextBytes, _ := base64.StdEncoding.DecodeString(ciphertext)
	if len(ciphertextBytes) > 12 {
		ciphertextBytes[12] ^= 0xFF // Flip bits in the middle
	}
	tamperedCiphertext := base64.StdEncoding.EncodeToString(ciphertextBytes)

	// Attempt to decrypt tampered data
	_, err = svc.Decrypt(tamperedCiphertext)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "decryption failed")
}

// TestEncrypt_DifferentNonces tests that multiple encryptions produce different ciphertexts.
func TestEncrypt_DifferentNonces(t *testing.T) {
	key := make([]byte, 32)
	_, err := rand.Read(key)
	require.NoError(t, err)
	keyBase64 := base64.StdEncoding.EncodeToString(key)

	svc, err := NewEncryptionService(keyBase64)
	require.NoError(t, err)

	plaintext := []byte("test data")

	// Encrypt the same plaintext multiple times
	ciphertext1, err := svc.Encrypt(plaintext)
	require.NoError(t, err)

	ciphertext2, err := svc.Encrypt(plaintext)
	require.NoError(t, err)

	// Ciphertexts should be different (due to random nonces)
	assert.NotEqual(t, ciphertext1, ciphertext2)

	// But both should decrypt to the same plaintext
	decrypted1, err := svc.Decrypt(ciphertext1)
	require.NoError(t, err)
	assert.Equal(t, plaintext, decrypted1)

	decrypted2, err := svc.Decrypt(ciphertext2)
	require.NoError(t, err)
	assert.Equal(t, plaintext, decrypted2)
}

// TestDecrypt_WrongKey tests that decryption with wrong key fails.
func TestDecrypt_WrongKey(t *testing.T) {
	// Encrypt with first key
	key1 := make([]byte, 32)
	_, err := rand.Read(key1)
	require.NoError(t, err)
	keyBase64_1 := base64.StdEncoding.EncodeToString(key1)

	svc1, err := NewEncryptionService(keyBase64_1)
	require.NoError(t, err)

	plaintext := "secret message"
	ciphertext, err := svc1.Encrypt([]byte(plaintext))
	require.NoError(t, err)

	// Try to decrypt with different key
	key2 := make([]byte, 32)
	_, err = rand.Read(key2)
	require.NoError(t, err)
	keyBase64_2 := base64.StdEncoding.EncodeToString(key2)

	svc2, err := NewEncryptionService(keyBase64_2)
	require.NoError(t, err)

	_, err = svc2.Decrypt(ciphertext)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "decryption failed")
}

// BenchmarkEncrypt benchmarks encryption performance.
func BenchmarkEncrypt(b *testing.B) {
	key := make([]byte, 32)
	_, _ = rand.Read(key)
	keyBase64 := base64.StdEncoding.EncodeToString(key)

	svc, _ := NewEncryptionService(keyBase64)
	plaintext := []byte("This is a test plaintext message for benchmarking encryption performance.")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = svc.Encrypt(plaintext)
	}
}

// BenchmarkDecrypt benchmarks decryption performance.
func BenchmarkDecrypt(b *testing.B) {
	key := make([]byte, 32)
	_, _ = rand.Read(key)
	keyBase64 := base64.StdEncoding.EncodeToString(key)

	svc, _ := NewEncryptionService(keyBase64)
	plaintext := []byte("This is a test plaintext message for benchmarking decryption performance.")
	ciphertext, _ := svc.Encrypt(plaintext)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = svc.Decrypt(ciphertext)
	}
}
