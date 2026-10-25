package routes

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/caddy"
	"github.com/Wikid82/charon/backend/internal/config"
)

// The production wiring must hand the CrowdSec handler the proxy's Caddy manager, so Start can apply.
func TestCrowdsecHandler_ProductionWiringHasProxyApplier(t *testing.T) {
	mgr := caddy.NewManager(nil, nil, t.TempDir(), "", false, config.SecurityConfig{})

	h := newCrowdsecHandler(nil, nil, "/bin/false", t.TempDir(), mgr)

	require.True(t, h.HasProxyApplier())
}

func TestCrowdsecHandler_MissingManagerIsNotWired(t *testing.T) {
	h := newCrowdsecHandler(nil, nil, "/bin/false", t.TempDir(), nil)

	require.False(t, h.HasProxyApplier())
}
