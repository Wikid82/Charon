package hecate

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/crypto"
	"github.com/Wikid82/charon/backend/internal/crypto/cryptotest"
	"github.com/Wikid82/charon/backend/internal/models"
)

// versionRecordingCipher wraps a cipher and records the key version each
// decrypt call was tagged with.
type versionRecordingCipher struct {
	crypto.VersionedCipher
	mu       sync.Mutex
	versions []int
}

func (c *versionRecordingCipher) DecryptWithVersion(ciphertext string, keyVersion int) ([]byte, error) {
	c.mu.Lock()
	c.versions = append(c.versions, keyVersion)
	c.mu.Unlock()
	return c.VersionedCipher.DecryptWithVersion(ciphertext, keyVersion)
}

// TestTunnelManager_StartTunnel_DecryptsWithStoredKeyVersion checks that the
// credentials are opened with the key version recorded on the tunnel row rather
// than an untagged placeholder.
func TestTunnelManager_StartTunnel_DecryptsWithStoredKeyVersion(t *testing.T) {
	db, encSvc := setupManagerTestDB(t)
	rec := &versionRecordingCipher{VersionedCipher: cryptotest.SingleKey(encSvc)}
	mgr := NewTunnelManager(db, rec)
	defer func() { _ = mgr.Stop() }()

	mgr.RegisterFactory(models.ProviderCloudflare, func(_ *models.TunnelConfig, _ string) (TunnelProvider, error) {
		return newMockProvider("keyed"), nil
	})

	creds, err := encSvc.Encrypt([]byte(`{"api_token":"test"}`))
	require.NoError(t, err)
	cfg := models.TunnelConfig{
		Name:                 "keyed-tunnel",
		Provider:             models.ProviderCloudflare,
		EncryptedCredentials: creds,
		KeyVersion:           3,
	}
	require.NoError(t, db.Create(&cfg).Error)

	require.NoError(t, mgr.StartTunnel(cfg.UUID))
	rec.mu.Lock()
	defer rec.mu.Unlock()
	assert.Equal(t, []int{3}, rec.versions)
}
