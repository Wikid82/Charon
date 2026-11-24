package services

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/crypto"
)

func TestEncryptSecret_NilCipherReportsMissingKeyWithoutPanic(t *testing.T) {
	var cipher crypto.VersionedCipher

	ciphertext, keyVersion, err := encryptSecret(cipher, []byte("secret"))

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrEncryptionFailed)
	assert.ErrorIs(t, err, ErrEncryptionKeyMissing)
	assert.Empty(t, ciphertext)
	assert.Zero(t, keyVersion)
}

func TestEncryptSecret_ReturnsCiphertextAndVersion(t *testing.T) {
	key, err := crypto.GenerateNewKey()
	require.NoError(t, err)
	svc, err := crypto.NewEncryptionService(key)
	require.NoError(t, err)

	ciphertext, keyVersion, err := encryptSecret(crypto.SingleKey(svc), []byte("secret"))

	require.NoError(t, err)
	assert.NotEmpty(t, ciphertext)
	assert.Equal(t, 1, keyVersion)
}

func TestDecryptSecret_NilCipherReportsMissingKey(t *testing.T) {
	var cipher crypto.VersionedCipher

	plaintext, err := decryptSecret(cipher, "ciphertext", 1)

	assert.ErrorIs(t, err, ErrEncryptionKeyMissing)
	assert.Nil(t, plaintext)
}

func TestDecryptSecret_RoundTripsThroughCipher(t *testing.T) {
	key, err := crypto.GenerateNewKey()
	require.NoError(t, err)
	svc, err := crypto.NewEncryptionService(key)
	require.NoError(t, err)
	cipher := crypto.SingleKey(svc)

	ciphertext, keyVersion, err := encryptSecret(cipher, []byte("round trip"))
	require.NoError(t, err)

	plaintext, err := decryptSecret(cipher, ciphertext, keyVersion)
	require.NoError(t, err)
	assert.Equal(t, "round trip", string(plaintext))
}
