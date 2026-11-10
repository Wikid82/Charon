package caddy

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/config"
)

// TestManager_ApplyConfig_ReadsGeoIPStateAtEachApply pins that the GeoIP database state is read on every apply, so a
// database loaded or reloaded after startup reaches the next generated config.
func TestManager_ApplyConfig_ReadsGeoIPStateAtEachApply(t *testing.T) {
	db := openEnforcementApplyDB(t)
	manager := NewManager(&switchableCaddyClient{}, db, t.TempDir(), "", false, config.SecurityConfig{CerberusEnabled: true})

	reads := 0
	manager.SetGeoIPState(func() (string, bool) {
		reads++
		return testGeoIPDBPath, reads > 1
	})

	require.NoError(t, manager.ApplyConfig(context.Background()))
	require.NoError(t, manager.ApplyConfig(context.Background()))
	require.Equal(t, 2, reads, "one read per apply")
}
