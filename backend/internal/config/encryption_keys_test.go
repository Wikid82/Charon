package config

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// clearEncryptionEnv unsets every encryption key variable so each test starts
// from a known state, whatever the developer's shell exports.
func clearEncryptionEnv(t *testing.T) {
	t.Helper()
	for _, name := range []string{
		"CHARON_ENCRYPTION_KEY",
		"CHARON_ENCRYPTION_KEY_NEXT",
		"ENCRYPTION_KEY",
		"CERBERUS_ENCRYPTION_KEY",
	} {
		t.Setenv(name, "")
	}
	for i := 0; i <= 11; i++ {
		t.Setenv(legacyKeyEnvName(i), "")
	}
}

func legacyKeyEnvName(slot int) string {
	return "CHARON_ENCRYPTION_KEY_V" + strconv.Itoa(slot)
}

func TestLoadEncryptionKeys_ReadsCurrentNextAndLegacySlots(t *testing.T) {
	clearEncryptionEnv(t)
	t.Setenv("CHARON_ENCRYPTION_KEY", "current-key")
	t.Setenv("CHARON_ENCRYPTION_KEY_NEXT", "next-key")
	t.Setenv(legacyKeyEnvName(1), "legacy-one")
	t.Setenv(legacyKeyEnvName(10), "legacy-ten")

	keys := LoadEncryptionKeys()

	assert.Equal(t, "current-key", keys.Current)
	assert.Equal(t, "next-key", keys.Next)
	assert.Equal(t, map[int]string{1: "legacy-one", 10: "legacy-ten"}, keys.Legacy)
}

func TestLoadEncryptionKeys_EmptyWhenUnset(t *testing.T) {
	clearEncryptionEnv(t)

	keys := LoadEncryptionKeys()

	assert.Empty(t, keys.Current)
	assert.Empty(t, keys.Next)
	assert.Empty(t, keys.Legacy)
}

func TestLoadEncryptionKeys_IgnoresUnprefixedAliases(t *testing.T) {
	clearEncryptionEnv(t)
	t.Setenv("ENCRYPTION_KEY", "alias-one")
	t.Setenv("CERBERUS_ENCRYPTION_KEY", "alias-two")

	keys := LoadEncryptionKeys()

	assert.Empty(t, keys.Current, "only CHARON_ENCRYPTION_KEY may supply the current key")
}

func TestLoadEncryptionKeys_IgnoresOutOfRangeAndBlankSlots(t *testing.T) {
	clearEncryptionEnv(t)
	t.Setenv(legacyKeyEnvName(0), "slot-zero")
	t.Setenv(legacyKeyEnvName(11), "slot-eleven")
	t.Setenv(legacyKeyEnvName(2), "")

	keys := LoadEncryptionKeys()

	assert.Empty(t, keys.Legacy)
}

func TestLoad_ExposesCurrentEncryptionKeyAsAlias(t *testing.T) {
	clearEncryptionEnv(t)
	t.Setenv("CHARON_ENCRYPTION_KEY", "current-key")
	t.Setenv("CHARON_ENCRYPTION_KEY_NEXT", "next-key")

	cfg, err := Load()
	require.NoError(t, err)

	assert.Equal(t, "current-key", cfg.EncryptionKey)
	assert.Equal(t, "current-key", cfg.EncryptionKeys.Current)
	assert.Equal(t, "next-key", cfg.EncryptionKeys.Next)
}
