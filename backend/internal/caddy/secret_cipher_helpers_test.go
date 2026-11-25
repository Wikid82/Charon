package caddy

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/crypto"
	"github.com/Wikid82/charon/backend/internal/crypto/cryptotest"
)

// testCaddyEncryptionKey is a valid base64 32-byte key used by test fixtures.
const testCaddyEncryptionKey = "MTIzNDU2Nzg5MDEyMzQ1Njc4OTAxMjM0NTY3ODkwMTI="

// testSecretCipher builds the single-key cipher the server hands to a Caddy
// manager, so tests drive DNS credential decryption without the environment.
func testSecretCipher(t *testing.T, keyB64 string) crypto.VersionedCipher {
	t.Helper()
	svc, err := crypto.NewEncryptionService(keyB64)
	require.NoError(t, err)
	return cryptotest.SingleKey(svc)
}
