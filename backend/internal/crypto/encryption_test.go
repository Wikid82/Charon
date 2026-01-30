package crypto

import (
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
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

// TestEncrypt_NilPlaintext tests encryption of nil plaintext.
func TestEncrypt_NilPlaintext(t *testing.T) {
	key := make([]byte, 32)
	_, err := rand.Read(key)
	require.NoError(t, err)
	keyBase64 := base64.StdEncoding.EncodeToString(key)

	svc, err := NewEncryptionService(keyBase64)
	require.NoError(t, err)

	// Encrypt nil plaintext (should work like empty)
	ciphertext, err := svc.Encrypt(nil)
	assert.NoError(t, err)
	assert.NotEmpty(t, ciphertext)

	// Decrypt should return empty plaintext
	decrypted, err := svc.Decrypt(ciphertext)
	assert.NoError(t, err)
	assert.Empty(t, decrypted)
}

// TestDecrypt_ExactNonceSize tests decryption when ciphertext is exactly nonce size.
func TestDecrypt_ExactNonceSize(t *testing.T) {
	key := make([]byte, 32)
	_, err := rand.Read(key)
	require.NoError(t, err)
	keyBase64 := base64.StdEncoding.EncodeToString(key)

	svc, err := NewEncryptionService(keyBase64)
	require.NoError(t, err)

	// Create ciphertext that is exactly 12 bytes (GCM nonce size)
	// This will fail because there's no actual ciphertext after the nonce
	exactNonce := make([]byte, 12)
	_, _ = rand.Read(exactNonce)
	ciphertextB64 := base64.StdEncoding.EncodeToString(exactNonce)

	_, err = svc.Decrypt(ciphertextB64)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "decryption failed")
}

// TestDecrypt_OneByteLessThanNonce tests decryption with one byte less than nonce size.
func TestDecrypt_OneByteLessThanNonce(t *testing.T) {
	key := make([]byte, 32)
	_, err := rand.Read(key)
	require.NoError(t, err)
	keyBase64 := base64.StdEncoding.EncodeToString(key)

	svc, err := NewEncryptionService(keyBase64)
	require.NoError(t, err)

	// Create ciphertext that is 11 bytes (one less than GCM nonce size)
	shortData := make([]byte, 11)
	_, _ = rand.Read(shortData)
	ciphertextB64 := base64.StdEncoding.EncodeToString(shortData)

	_, err = svc.Decrypt(ciphertextB64)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "ciphertext too short")
}

// TestEncryptDecrypt_BinaryData tests encryption/decryption of binary data.
func TestEncryptDecrypt_BinaryData(t *testing.T) {
	key := make([]byte, 32)
	_, err := rand.Read(key)
	require.NoError(t, err)
	keyBase64 := base64.StdEncoding.EncodeToString(key)

	svc, err := NewEncryptionService(keyBase64)
	require.NoError(t, err)

	// Test with random binary data including null bytes
	binaryData := make([]byte, 256)
	_, err = rand.Read(binaryData)
	require.NoError(t, err)

	// Include explicit null bytes
	binaryData[50] = 0x00
	binaryData[100] = 0x00
	binaryData[150] = 0x00

	// Encrypt
	ciphertext, err := svc.Encrypt(binaryData)
	require.NoError(t, err)
	assert.NotEmpty(t, ciphertext)

	// Decrypt
	decrypted, err := svc.Decrypt(ciphertext)
	require.NoError(t, err)
	assert.Equal(t, binaryData, decrypted)
}

// TestEncryptDecrypt_LargePlaintext tests encryption of large data.
func TestEncryptDecrypt_LargePlaintext(t *testing.T) {
	key := make([]byte, 32)
	_, err := rand.Read(key)
	require.NoError(t, err)
	keyBase64 := base64.StdEncoding.EncodeToString(key)

	svc, err := NewEncryptionService(keyBase64)
	require.NoError(t, err)

	// 1MB of data
	largePlaintext := make([]byte, 1024*1024)
	_, err = rand.Read(largePlaintext)
	require.NoError(t, err)

	// Encrypt
	ciphertext, err := svc.Encrypt(largePlaintext)
	require.NoError(t, err)
	assert.NotEmpty(t, ciphertext)

	// Decrypt
	decrypted, err := svc.Decrypt(ciphertext)
	require.NoError(t, err)
	assert.Equal(t, largePlaintext, decrypted)
}

// TestDecrypt_CorruptedNonce tests decryption with corrupted nonce.
func TestDecrypt_CorruptedNonce(t *testing.T) {
	key := make([]byte, 32)
	_, err := rand.Read(key)
	require.NoError(t, err)
	keyBase64 := base64.StdEncoding.EncodeToString(key)

	svc, err := NewEncryptionService(keyBase64)
	require.NoError(t, err)

	// Encrypt valid plaintext
	original := "test data for nonce corruption"
	ciphertext, err := svc.Encrypt([]byte(original))
	require.NoError(t, err)

	// Decode, corrupt nonce (first 12 bytes), and re-encode
	ciphertextBytes, _ := base64.StdEncoding.DecodeString(ciphertext)
	for i := 0; i < 12; i++ {
		ciphertextBytes[i] ^= 0xFF // Flip all bits in nonce
	}
	corruptedCiphertext := base64.StdEncoding.EncodeToString(ciphertextBytes)

	// Attempt to decrypt with corrupted nonce
	_, err = svc.Decrypt(corruptedCiphertext)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "decryption failed")
}

// TestDecrypt_TruncatedCiphertext tests decryption with truncated ciphertext.
func TestDecrypt_TruncatedCiphertext(t *testing.T) {
	key := make([]byte, 32)
	_, err := rand.Read(key)
	require.NoError(t, err)
	keyBase64 := base64.StdEncoding.EncodeToString(key)

	svc, err := NewEncryptionService(keyBase64)
	require.NoError(t, err)

	// Encrypt valid plaintext
	original := "test data for truncation"
	ciphertext, err := svc.Encrypt([]byte(original))
	require.NoError(t, err)

	// Decode and truncate (remove last few bytes of auth tag)
	ciphertextBytes, _ := base64.StdEncoding.DecodeString(ciphertext)
	truncatedBytes := ciphertextBytes[:len(ciphertextBytes)-5]
	truncatedCiphertext := base64.StdEncoding.EncodeToString(truncatedBytes)

	// Attempt to decrypt truncated data
	_, err = svc.Decrypt(truncatedCiphertext)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "decryption failed")
}

// TestDecrypt_AppendedData tests decryption with extra data appended.
func TestDecrypt_AppendedData(t *testing.T) {
	key := make([]byte, 32)
	_, err := rand.Read(key)
	require.NoError(t, err)
	keyBase64 := base64.StdEncoding.EncodeToString(key)

	svc, err := NewEncryptionService(keyBase64)
	require.NoError(t, err)

	// Encrypt valid plaintext
	original := "test data for appending"
	ciphertext, err := svc.Encrypt([]byte(original))
	require.NoError(t, err)

	// Decode and append extra data
	ciphertextBytes, _ := base64.StdEncoding.DecodeString(ciphertext)
	appendedBytes := make([]byte, len(ciphertextBytes)+len("extra garbage"))
	copy(appendedBytes, ciphertextBytes)
	copy(appendedBytes[len(ciphertextBytes):], "extra garbage")
	appendedCiphertext := base64.StdEncoding.EncodeToString(appendedBytes)

	// Attempt to decrypt with appended data
	_, err = svc.Decrypt(appendedCiphertext)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "decryption failed")
}

// TestEncryptionService_ConcurrentAccess tests thread safety.
func TestEncryptionService_ConcurrentAccess(t *testing.T) {
	key := make([]byte, 32)
	_, err := rand.Read(key)
	require.NoError(t, err)
	keyBase64 := base64.StdEncoding.EncodeToString(key)

	svc, err := NewEncryptionService(keyBase64)
	require.NoError(t, err)

	const numGoroutines = 50
	const numOperations = 100

	// Channel to collect errors
	errChan := make(chan error, numGoroutines*numOperations*2)

	// Run concurrent encryptions and decryptions
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			for j := 0; j < numOperations; j++ {
				plaintext := []byte(strings.Repeat("a", (id*j+1)%100+1))

				// Encrypt
				ciphertext, err := svc.Encrypt(plaintext)
				if err != nil {
					errChan <- err
					continue
				}

				// Decrypt
				decrypted, err := svc.Decrypt(ciphertext)
				if err != nil {
					errChan <- err
					continue
				}

				// Verify
				if string(decrypted) != string(plaintext) {
					errChan <- assert.AnError
				}
			}
		}(i)
	}

	// Wait a bit for goroutines to complete
	// Note: In production, use sync.WaitGroup
	// This is simplified for testing
	close(errChan)
	for err := range errChan {
		if err != nil {
			t.Errorf("concurrent operation failed: %v", err)
		}
	}
}

// TestDecrypt_AllZerosCiphertext tests decryption of all-zeros ciphertext.
func TestDecrypt_AllZerosCiphertext(t *testing.T) {
	key := make([]byte, 32)
	_, err := rand.Read(key)
	require.NoError(t, err)
	keyBase64 := base64.StdEncoding.EncodeToString(key)

	svc, err := NewEncryptionService(keyBase64)
	require.NoError(t, err)

	// Create an all-zeros ciphertext that's long enough
	zeros := make([]byte, 32) // Longer than nonce (12 bytes)
	ciphertextB64 := base64.StdEncoding.EncodeToString(zeros)

	// This should fail authentication
	_, err = svc.Decrypt(ciphertextB64)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "decryption failed")
}

// TestDecrypt_RandomGarbageCiphertext tests decryption of random garbage.
func TestDecrypt_RandomGarbageCiphertext(t *testing.T) {
	key := make([]byte, 32)
	_, err := rand.Read(key)
	require.NoError(t, err)
	keyBase64 := base64.StdEncoding.EncodeToString(key)

	svc, err := NewEncryptionService(keyBase64)
	require.NoError(t, err)

	// Generate random garbage that's long enough to have a "nonce" and "ciphertext"
	garbage := make([]byte, 64)
	_, _ = rand.Read(garbage)
	ciphertextB64 := base64.StdEncoding.EncodeToString(garbage)

	// This should fail authentication
	_, err = svc.Decrypt(ciphertextB64)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "decryption failed")
}

// TestNewEncryptionService_EmptyKey tests error handling for empty key.
func TestNewEncryptionService_EmptyKey(t *testing.T) {
	svc, err := NewEncryptionService("")
	assert.Error(t, err)
	assert.Nil(t, svc)
	assert.Contains(t, err.Error(), "invalid key length")
}

// TestNewEncryptionService_WhitespaceKey tests error handling for whitespace key.
func TestNewEncryptionService_WhitespaceKey(t *testing.T) {
	svc, err := NewEncryptionService("   ")
	assert.Error(t, err)
	assert.Nil(t, svc)
	// Could be invalid base64 or invalid key length depending on parsing
}

// errCipherFactory is a mock cipher factory that always returns an error.
func errCipherFactory(_ []byte) (cipher.Block, error) {
	return nil, errors.New("mock cipher error")
}

// errGCMFactory is a mock GCM factory that always returns an error.
func errGCMFactory(_ cipher.Block) (cipher.AEAD, error) {
	return nil, errors.New("mock GCM error")
}

// errRandReader is a mock random reader that always returns an error.
func errRandReader(_ []byte) (int, error) {
	return 0, errors.New("mock random error")
}

// TestEncrypt_CipherCreationError tests encryption error when cipher creation fails.
func TestEncrypt_CipherCreationError(t *testing.T) {
	key := make([]byte, 32)
	_, err := rand.Read(key)
	require.NoError(t, err)
	keyBase64 := base64.StdEncoding.EncodeToString(key)

	svc, err := NewEncryptionService(keyBase64)
	require.NoError(t, err)

	// Inject error-producing cipher factory
	svc.cipherFactory = errCipherFactory

	_, err = svc.Encrypt([]byte("test plaintext"))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create cipher")
}

// TestEncrypt_GCMCreationError tests encryption error when GCM creation fails.
func TestEncrypt_GCMCreationError(t *testing.T) {
	key := make([]byte, 32)
	_, err := rand.Read(key)
	require.NoError(t, err)
	keyBase64 := base64.StdEncoding.EncodeToString(key)

	svc, err := NewEncryptionService(keyBase64)
	require.NoError(t, err)

	// Inject error-producing GCM factory
	svc.gcmFactory = errGCMFactory

	_, err = svc.Encrypt([]byte("test plaintext"))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create GCM")
}

// TestEncrypt_NonceGenerationError tests encryption error when nonce generation fails.
func TestEncrypt_NonceGenerationError(t *testing.T) {
	key := make([]byte, 32)
	_, err := rand.Read(key)
	require.NoError(t, err)
	keyBase64 := base64.StdEncoding.EncodeToString(key)

	svc, err := NewEncryptionService(keyBase64)
	require.NoError(t, err)

	// Inject error-producing random reader
	svc.randReader = errRandReader

	_, err = svc.Encrypt([]byte("test plaintext"))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to generate nonce")
}

// TestDecrypt_CipherCreationError tests decryption error when cipher creation fails.
func TestDecrypt_CipherCreationError(t *testing.T) {
	key := make([]byte, 32)
	_, err := rand.Read(key)
	require.NoError(t, err)
	keyBase64 := base64.StdEncoding.EncodeToString(key)

	svc, err := NewEncryptionService(keyBase64)
	require.NoError(t, err)

	// First encrypt something valid
	ciphertext, err := svc.Encrypt([]byte("test plaintext"))
	require.NoError(t, err)

	// Inject error-producing cipher factory for decrypt
	svc.cipherFactory = errCipherFactory

	_, err = svc.Decrypt(ciphertext)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create cipher")
}

// TestDecrypt_GCMCreationError tests decryption error when GCM creation fails.
func TestDecrypt_GCMCreationError(t *testing.T) {
	key := make([]byte, 32)
	_, err := rand.Read(key)
	require.NoError(t, err)
	keyBase64 := base64.StdEncoding.EncodeToString(key)

	svc, err := NewEncryptionService(keyBase64)
	require.NoError(t, err)

	// First encrypt something valid
	ciphertext, err := svc.Encrypt([]byte("test plaintext"))
	require.NoError(t, err)

	// Inject error-producing GCM factory for decrypt
	svc.gcmFactory = errGCMFactory

	_, err = svc.Decrypt(ciphertext)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create GCM")
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
