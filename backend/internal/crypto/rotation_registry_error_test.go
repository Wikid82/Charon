package crypto

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/config"
)

// TestNewRotationService_RegistryFailureIsUnavailable checks that a failure to read
// the key registry is marked with ErrKeyRegistryUnavailable. The database here has
// no key registry table, so registering the keys fails.
func TestNewRotationService_RegistryFailureIsUnavailable(t *testing.T) {
	db := openStoreRowsDB(t)
	const key = "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY="

	_, err := NewRotationService(db, config.EncryptionKeys{Current: key})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrKeyRegistryUnavailable)
}

// TestNewRotationService_KeyErrorIsNotUnavailable checks that a key that cannot be
// parsed is reported as a key problem, not as a registry failure.
func TestNewRotationService_KeyErrorIsNotUnavailable(t *testing.T) {
	db := openStoreRowsDB(t)

	_, err := NewRotationService(db, config.EncryptionKeys{Current: "not-base64!"})
	require.Error(t, err)
	assert.NotErrorIs(t, err, ErrKeyRegistryUnavailable)
}
