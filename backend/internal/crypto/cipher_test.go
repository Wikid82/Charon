package crypto

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSingleKey_NilServiceIsNilCipher(t *testing.T) {
	assert.Nil(t, SingleKey(nil), "a nil service must yield a nil interface, not a typed nil")
}

func TestSingleKey_EncryptsWithVersionOneAndDecryptsIgnoringTag(t *testing.T) {
	key, err := GenerateNewKey()
	require.NoError(t, err)
	svc, err := NewEncryptionService(key)
	require.NoError(t, err)

	cipher := SingleKey(svc)
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

func TestRotationService_SatisfiesVersionedCipher(t *testing.T) {
	var _ VersionedCipher = (*RotationService)(nil)
}
