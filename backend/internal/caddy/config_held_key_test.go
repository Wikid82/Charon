package caddy

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/crowdsec/bouncerkey"
)

// TestGetCrowdSecAPIKeyUsesHeldKey: a key that could not be saved is the key the proxy config carries, ahead of a
// different saved file and any environment variable.
func TestGetCrowdSecAPIKeyUsesHeldKey(t *testing.T) {
	clearCrowdSecKeyEnv(t)
	t.Setenv("CHARON_SECURITY_CROWDSEC_API_KEY", "env-key-held-caddy-0000000001")
	path := filepath.Join(t.TempDir(), "bouncer_key")
	useBouncerKeyFile(t, path)
	const held = "held-caddy-key-00000000000001"
	bouncerkey.Hold(path, held)
	t.Cleanup(func() { bouncerkey.Release(path) })

	require.Equal(t, held, getCrowdSecAPIKey())
}
