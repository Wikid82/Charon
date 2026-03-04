// Package crypto provides cryptographic services for sensitive data.
package crypto

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/Wikid82/charon/backend/internal/models"
	"gorm.io/gorm"
)

// RotationService manages encryption key rotation with multi-key version support.
// It supports loading multiple encryption keys from environment variables:
// - CHARON_ENCRYPTION_KEY: Current encryption key (version 1)
// - CHARON_ENCRYPTION_KEY_NEXT: Next key for rotation (becomes current after rotation)
// - CHARON_ENCRYPTION_KEY_V1 through CHARON_ENCRYPTION_KEY_V10: Legacy keys for decryption
//
// Zero-downtime rotation workflow:
// 1. Set CHARON_ENCRYPTION_KEY_NEXT with new key
// 2. Restart application (loads both keys)
// 3. Call RotateAllCredentials() to re-encrypt all credentials with NEXT key
// 4. Promote: NEXT → current, old current → V1
// 5. Restart application
type RotationService struct {
	db          *gorm.DB
	currentKey  *EncryptionService         // Current encryption key
	nextKey     *EncryptionService         // Next key for rotation (optional)
	legacyKeys  map[int]*EncryptionService // Legacy keys indexed by version
	keyVersions []int                      // Sorted list of available key versions
}

// RotationResult contains the outcome of a rotation operation.
type RotationResult struct {
	TotalProviders  int       `json:"total_providers"`
	SuccessCount    int       `json:"success_count"`
	FailureCount    int       `json:"failure_count"`
	FailedProviders []uint    `json:"failed_providers,omitempty"`
	Duration        string    `json:"duration"`
	NewKeyVersion   int       `json:"new_key_version"`
	StartedAt       time.Time `json:"started_at"`
	CompletedAt     time.Time `json:"completed_at"`
}

// RotationStatus describes the current state of encryption keys.
type RotationStatus struct {
	CurrentVersion            int         `json:"current_version"`
	NextKeyConfigured         bool        `json:"next_key_configured"`
	LegacyKeyCount            int         `json:"legacy_key_count"`
	LegacyKeyVersions         []int       `json:"legacy_key_versions"`
	ProvidersOnCurrentVersion int         `json:"providers_on_current_version"`
	ProvidersOnOlderVersions  int         `json:"providers_on_older_versions"`
	ProvidersByVersion        map[int]int `json:"providers_by_version"`
}

// NewRotationService creates a new key rotation service.
// It loads the current key and any legacy/next keys from environment variables.
func NewRotationService(db *gorm.DB) (*RotationService, error) {
	rs := &RotationService{
		db:         db,
		legacyKeys: make(map[int]*EncryptionService),
	}

	// Load current key (required)
	currentKeyB64 := os.Getenv("CHARON_ENCRYPTION_KEY")
	if currentKeyB64 == "" {
		return nil, fmt.Errorf("CHARON_ENCRYPTION_KEY is required")
	}

	currentKey, err := NewEncryptionService(currentKeyB64)
	if err != nil {
		return nil, fmt.Errorf("failed to load current encryption key: %w", err)
	}
	rs.currentKey = currentKey

	// Load next key (optional, used during rotation)
	nextKeyB64 := os.Getenv("CHARON_ENCRYPTION_KEY_NEXT")
	if nextKeyB64 != "" {
		nextKey, err := NewEncryptionService(nextKeyB64)
		if err != nil {
			return nil, fmt.Errorf("failed to load next encryption key: %w", err)
		}
		rs.nextKey = nextKey
	}

	// Load legacy keys V1 through V10 (optional, for backward compatibility)
	for i := 1; i <= 10; i++ {
		envKey := fmt.Sprintf("CHARON_ENCRYPTION_KEY_V%d", i)
		keyB64 := os.Getenv(envKey)
		if keyB64 == "" {
			continue
		}

		legacyKey, err := NewEncryptionService(keyB64)
		if err != nil {
			// Log warning but continue - this allows partial key configurations
			fmt.Printf("Warning: failed to load legacy key %s: %v\n", envKey, err)
			continue
		}
		rs.legacyKeys[i] = legacyKey
	}

	// Build sorted list of available key versions
	rs.keyVersions = []int{1} // Current key is always version 1
	for v := range rs.legacyKeys {
		rs.keyVersions = append(rs.keyVersions, v)
	}
	sort.Ints(rs.keyVersions)

	return rs, nil
}

// DecryptWithVersion decrypts ciphertext using the specified key version.
// It automatically falls back to older versions if the specified version fails.
func (rs *RotationService) DecryptWithVersion(ciphertextB64 string, version int) ([]byte, error) {
	// Try the specified version first
	plaintext, err := rs.tryDecryptWithVersion(ciphertextB64, version)
	if err == nil {
		return plaintext, nil
	}

	// If specified version failed, try falling back to other versions
	// This handles cases where KeyVersion may be incorrectly tracked
	for _, v := range rs.keyVersions {
		if v == version {
			continue // Already tried this one
		}
		plaintext, err = rs.tryDecryptWithVersion(ciphertextB64, v)
		if err == nil {
			// Successfully decrypted with a different version
			// Log this for audit purposes
			fmt.Printf("Warning: credential decrypted with version %d but was tagged as version %d\n", v, version)
			return plaintext, nil
		}
	}

	return nil, fmt.Errorf("failed to decrypt with version %d or any fallback version", version)
}

// tryDecryptWithVersion attempts decryption with a specific key version.
func (rs *RotationService) tryDecryptWithVersion(ciphertextB64 string, version int) ([]byte, error) {
	var encService *EncryptionService

	if version == 1 {
		encService = rs.currentKey
	} else if legacy, ok := rs.legacyKeys[version]; ok {
		encService = legacy
	} else {
		return nil, fmt.Errorf("encryption key version %d not available", version)
	}

	return encService.Decrypt(ciphertextB64)
}

// EncryptWithCurrentKey encrypts plaintext with the current (or next during rotation) key.
// Returns the ciphertext and the version number of the key used.
func (rs *RotationService) EncryptWithCurrentKey(plaintext []byte) (string, int, error) {
	// During rotation, use next key if available
	if rs.nextKey != nil {
		ciphertext, err := rs.nextKey.Encrypt(plaintext)
		if err != nil {
			return "", 0, fmt.Errorf("failed to encrypt with next key: %w", err)
		}
		return ciphertext, 2, nil // Next key becomes version 2
	}

	// Normal operation: use current key
	ciphertext, err := rs.currentKey.Encrypt(plaintext)
	if err != nil {
		return "", 0, fmt.Errorf("failed to encrypt with current key: %w", err)
	}
	return ciphertext, 1, nil
}

// RotateAllCredentials re-encrypts all DNS provider credentials with the next key.
// This operation is atomic per provider but not globally - failed providers can be retried.
// Returns detailed results including any failures.
func (rs *RotationService) RotateAllCredentials(ctx context.Context) (*RotationResult, error) {
	if rs.nextKey == nil {
		return nil, fmt.Errorf("CHARON_ENCRYPTION_KEY_NEXT not configured - cannot rotate")
	}

	startTime := time.Now()
	result := &RotationResult{
		NewKeyVersion:   2,
		StartedAt:       startTime,
		FailedProviders: []uint{},
	}

	// Fetch all DNS providers
	var providers []models.DNSProvider
	if err := rs.db.WithContext(ctx).Find(&providers).Error; err != nil {
		return nil, fmt.Errorf("failed to fetch providers: %w", err)
	}

	result.TotalProviders = len(providers)

	// Re-encrypt each provider's credentials
	for _, provider := range providers {
		if err := rs.rotateProviderCredentials(ctx, &provider); err != nil {
			result.FailureCount++
			result.FailedProviders = append(result.FailedProviders, provider.ID)
			fmt.Printf("Failed to rotate provider %d (%s): %v\n", provider.ID, provider.Name, err)
			continue
		}
		result.SuccessCount++
	}

	result.CompletedAt = time.Now()
	result.Duration = result.CompletedAt.Sub(startTime).String()

	return result, nil
}

// rotateProviderCredentials re-encrypts a single provider's credentials.
func (rs *RotationService) rotateProviderCredentials(ctx context.Context, provider *models.DNSProvider) error {
	// Decrypt with old key (using fallback mechanism)
	plaintext, err := rs.DecryptWithVersion(provider.CredentialsEncrypted, provider.KeyVersion)
	if err != nil {
		return fmt.Errorf("failed to decrypt credentials: %w", err)
	}

	// Validate that decrypted data is valid JSON
	var credentials map[string]string
	if unmarshalErr := json.Unmarshal(plaintext, &credentials); unmarshalErr != nil {
		return fmt.Errorf("invalid credential format after decryption: %w", unmarshalErr)
	}

	// Re-encrypt with next key
	newCiphertext, err := rs.nextKey.Encrypt(plaintext)
	if err != nil {
		return fmt.Errorf("failed to encrypt with next key: %w", err)
	}

	// Update provider record atomically
	updates := map[string]interface{}{
		"credentials_encrypted": newCiphertext,
		"key_version":           2, // Next key becomes version 2
		"updated_at":            time.Now(),
	}

	if err := rs.db.WithContext(ctx).Model(provider).Updates(updates).Error; err != nil {
		return fmt.Errorf("failed to update provider record: %w", err)
	}

	return nil
}

// GetStatus returns the current rotation status including key configuration and provider distribution.
func (rs *RotationService) GetStatus() (*RotationStatus, error) {
	status := &RotationStatus{
		CurrentVersion:     1,
		NextKeyConfigured:  rs.nextKey != nil,
		LegacyKeyCount:     len(rs.legacyKeys),
		LegacyKeyVersions:  []int{},
		ProvidersByVersion: make(map[int]int),
	}

	// Collect legacy key versions
	for v := range rs.legacyKeys {
		status.LegacyKeyVersions = append(status.LegacyKeyVersions, v)
	}
	sort.Ints(status.LegacyKeyVersions)

	// Count providers by key version
	var providers []models.DNSProvider
	if err := rs.db.Select("key_version").Find(&providers).Error; err != nil {
		return nil, fmt.Errorf("failed to count providers by version: %w", err)
	}

	for _, p := range providers {
		status.ProvidersByVersion[p.KeyVersion]++
		if p.KeyVersion == 1 {
			status.ProvidersOnCurrentVersion++
		} else {
			status.ProvidersOnOlderVersions++
		}
	}

	return status, nil
}

// ValidateKeyConfiguration checks all configured encryption keys for validity.
// Returns error if any key is invalid (wrong length, invalid base64, etc.).
func (rs *RotationService) ValidateKeyConfiguration() error {
	// Current key is already validated during NewRotationService()
	// Just verify it's still accessible
	if rs.currentKey == nil {
		return fmt.Errorf("current encryption key not loaded")
	}

	// Test encryption/decryption with current key
	testData := []byte("validation_test")
	ciphertext, err := rs.currentKey.Encrypt(testData)
	if err != nil {
		return fmt.Errorf("current key encryption test failed: %w", err)
	}
	plaintext, err := rs.currentKey.Decrypt(ciphertext)
	if err != nil {
		return fmt.Errorf("current key decryption test failed: %w", err)
	}
	if string(plaintext) != string(testData) {
		return fmt.Errorf("current key round-trip test failed")
	}

	// Validate next key if configured
	if rs.nextKey != nil {
		ciphertext, err := rs.nextKey.Encrypt(testData)
		if err != nil {
			return fmt.Errorf("next key encryption test failed: %w", err)
		}
		plaintext, err := rs.nextKey.Decrypt(ciphertext)
		if err != nil {
			return fmt.Errorf("next key decryption test failed: %w", err)
		}
		if string(plaintext) != string(testData) {
			return fmt.Errorf("next key round-trip test failed")
		}
	}

	// Validate legacy keys
	for version, legacyKey := range rs.legacyKeys {
		ciphertext, err := legacyKey.Encrypt(testData)
		if err != nil {
			return fmt.Errorf("legacy key V%d encryption test failed: %w", version, err)
		}
		plaintext, err := legacyKey.Decrypt(ciphertext)
		if err != nil {
			return fmt.Errorf("legacy key V%d decryption test failed: %w", version, err)
		}
		if string(plaintext) != string(testData) {
			return fmt.Errorf("legacy key V%d round-trip test failed", version)
		}
	}

	return nil
}

// GenerateNewKey generates a new random 32-byte encryption key and returns it as base64.
// This is a utility function for administrators to generate keys for rotation.
func GenerateNewKey() (string, error) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return "", fmt.Errorf("failed to generate random key: %w", err)
	}
	return base64.StdEncoding.EncodeToString(key), nil
}
