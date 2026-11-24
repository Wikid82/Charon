package crypto

import (
	"bytes"
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/internal/models"
)

// captureLogs routes the shared logger to a buffer for the rest of the test.
func captureLogs(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	logger.Init(false, &buf)
	t.Cleanup(func() { logger.Init(false, os.Stdout) })
	return &buf
}

func mustKey(t *testing.T) string {
	t.Helper()
	key, err := GenerateNewKey()
	require.NoError(t, err)
	return key
}

func TestNewRotationServiceWithKeys_RequiresCurrentKey(t *testing.T) {
	db := setupTestDB(t)

	_, err := NewRotationServiceWithKeys(db, config.EncryptionKeys{Next: mustKey(t)})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "CHARON_ENCRYPTION_KEY is required")
}

func TestNewRotationServiceWithKeys_UsesInjectedLegacySlots(t *testing.T) {
	db := setupTestDB(t)
	legacy := mustKey(t)

	rs, err := NewRotationServiceWithKeys(db, config.EncryptionKeys{
		Current: mustKey(t),
		Legacy:  map[int]string{3: legacy},
	})
	require.NoError(t, err)

	assert.Equal(t, []int{1, 3}, rs.keyVersions)

	legacySvc, err := NewEncryptionService(legacy)
	require.NoError(t, err)
	ciphertext, err := legacySvc.Encrypt([]byte("legacy secret"))
	require.NoError(t, err)

	plaintext, err := rs.DecryptWithVersion(ciphertext, 3)
	require.NoError(t, err)
	assert.Equal(t, "legacy secret", string(plaintext))
}

func TestNewRotationServiceWithKeys_IgnoresProcessEnvironment(t *testing.T) {
	db := setupTestDB(t)
	t.Setenv("CHARON_ENCRYPTION_KEY", mustKey(t))
	injected := mustKey(t)

	rs, err := NewRotationServiceWithKeys(db, config.EncryptionKeys{Current: injected})
	require.NoError(t, err)

	currentSvc, err := NewEncryptionService(injected)
	require.NoError(t, err)
	ciphertext, err := currentSvc.Encrypt([]byte("injected"))
	require.NoError(t, err)
	plaintext, err := rs.DecryptWithVersion(ciphertext, 1)
	require.NoError(t, err)
	assert.Equal(t, "injected", string(plaintext))
}

func TestNewRotationServiceWithKeys_InvalidLegacyKeyWarnsWithoutKeyText(t *testing.T) {
	db := setupTestDB(t)
	logs := captureLogs(t)
	const badKey = "not-a-valid-key-material"

	rs, err := NewRotationServiceWithKeys(db, config.EncryptionKeys{
		Current: mustKey(t),
		Legacy:  map[int]string{4: badKey},
	})
	require.NoError(t, err, "a bad legacy slot must not stop startup")

	assert.Equal(t, []int{1}, rs.keyVersions, "the bad slot is skipped")
	assert.Contains(t, logs.String(), `"slot":4`)
	assert.Contains(t, logs.String(), "legacy encryption key")
	assert.NotContains(t, logs.String(), badKey)
}

func TestNewRotationService_ReadsEnvironmentThroughConfig(t *testing.T) {
	db := setupTestDB(t)
	legacy := mustKey(t)
	t.Setenv("CHARON_ENCRYPTION_KEY", mustKey(t))
	t.Setenv("CHARON_ENCRYPTION_KEY_NEXT", mustKey(t))
	t.Setenv("CHARON_ENCRYPTION_KEY_V2", legacy)

	rs, err := NewRotationService(db)
	require.NoError(t, err)

	assert.NotNil(t, rs.nextKey)
	assert.Equal(t, []int{1, 2}, rs.keyVersions)
}

func TestDecryptWithVersion_FallbackLogsVersionsWithoutKeyMaterial(t *testing.T) {
	db := setupTestDB(t)
	logs := captureLogs(t)
	legacy := mustKey(t)

	rs, err := NewRotationServiceWithKeys(db, config.EncryptionKeys{
		Current: mustKey(t),
		Legacy:  map[int]string{3: legacy},
	})
	require.NoError(t, err)

	legacySvc, err := NewEncryptionService(legacy)
	require.NoError(t, err)
	const secret = "fallback secret"
	ciphertext, err := legacySvc.Encrypt([]byte(secret))
	require.NoError(t, err)

	// Tagged as version 1 (current), so the decrypt falls back to slot 3.
	plaintext, err := rs.DecryptWithVersion(ciphertext, 1)
	require.NoError(t, err)
	assert.Equal(t, secret, string(plaintext))

	assert.Contains(t, logs.String(), "decrypted with version 3 but was tagged as version 1")
	assert.NotContains(t, logs.String(), legacy)
	assert.NotContains(t, logs.String(), ciphertext)
	assert.NotContains(t, logs.String(), secret)
}

func TestRotateAllCredentials_FailedProviderLogsNameWithoutCiphertext(t *testing.T) {
	db := setupTestDB(t)
	logs := captureLogs(t)
	const brokenCiphertext = "broken-ciphertext-value"

	require.NoError(t, db.Create(&models.DNSProvider{
		Name:                 "broken-provider",
		ProviderType:         "cloudflare",
		CredentialsEncrypted: brokenCiphertext,
		KeyVersion:           1,
	}).Error)

	rs, err := NewRotationServiceWithKeys(db, config.EncryptionKeys{
		Current: mustKey(t),
		Next:    mustKey(t),
	})
	require.NoError(t, err)

	result, err := rs.RotateAllCredentials(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, result.FailureCount)

	assert.Contains(t, logs.String(), "broken-provider")
	assert.NotContains(t, logs.String(), brokenCiphertext)
}
