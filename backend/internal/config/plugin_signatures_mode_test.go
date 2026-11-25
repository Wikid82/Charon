package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLoadPluginSignatures_EmptyAllowlistIsStrictAndEmpty(t *testing.T) {
	t.Setenv("CHARON_PLUGIN_SIGNATURES", "{}")

	signatures := loadPluginSignatures()

	assert.NotNil(t, signatures)
	assert.Empty(t, signatures, "an empty allowlist permits no external plugins")
}
