package cryptotest

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/crypto"
)

func newService(t *testing.T) *crypto.EncryptionService {
	t.Helper()
	key, err := crypto.GenerateNewKey()
	require.NoError(t, err)
	svc, err := crypto.NewEncryptionService(key)
	require.NoError(t, err)
	return svc
}

func TestSingleKey_NilServiceIsNilCipher(t *testing.T) {
	assert.Nil(t, SingleKey(nil), "a nil service must yield a nil interface, not a typed nil")
}

func TestSingleKey_EncryptsWithVersionOneAndDecryptsIgnoringTag(t *testing.T) {
	cipher := SingleKey(newService(t))
	require.NotNil(t, cipher)

	ciphertext, version, err := cipher.EncryptWithCurrentKey([]byte("secret"))
	require.NoError(t, err)
	assert.Equal(t, 1, version)

	plaintext, err := cipher.DecryptWithVersion(ciphertext, 0)
	require.NoError(t, err)
	assert.Equal(t, "secret", string(plaintext))

	plaintext, err = cipher.DecryptWithVersion(ciphertext, 7)
	require.NoError(t, err, "the tag is ignored by a single key")
	assert.Equal(t, "secret", string(plaintext))
}

func TestSingleKey_ReportsErrorsFromTheService(t *testing.T) {
	cipher := SingleKey(newService(t))
	_, err := cipher.DecryptWithVersion("not-base64!", 1)
	require.Error(t, err)
	assert.ErrorContains(t, err, "single key decrypt")
}
