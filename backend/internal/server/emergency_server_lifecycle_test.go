package server

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/api/handlers"
	"github.com/Wikid82/charon/backend/internal/config"
)

// TestEmergencyServer_WithCrowdsecLifecycleStoresInstanceOrNil pins the Tier 2 wiring: the server keeps the
// lifecycle instance it is given, and a typed-nil handler counts as not wired.
func TestEmergencyServer_WithCrowdsecLifecycleStoresInstanceOrNil(t *testing.T) {
	var typedNil *handlers.CrowdsecHandler
	live := handlers.NewCrowdsecHandler(nil, nil, "", t.TempDir(), nil)

	srv := NewEmergencyServerWithDeps(nil, config.EmergencyConfig{}, nil, nil, nil).WithCrowdsecLifecycle(live)
	require.True(t, srv.crowdsec == handlers.CrowdsecLifecycle(live), "the Tier 2 server must keep the shared instance")

	srv = NewEmergencyServerWithDeps(nil, config.EmergencyConfig{}, nil, nil, nil).WithCrowdsecLifecycle(handlers.CrowdsecLifecycle(typedNil))
	require.True(t, srv.crowdsec == nil, "a typed-nil lifecycle must count as not wired")
}
