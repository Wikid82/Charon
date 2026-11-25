package main

import (
	"github.com/Wikid82/charon/backend/internal/caddy"
	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/internal/services"
)

// upstreamNameManager is the part of the caddy manager the upstream name answerer is wired into.
type upstreamNameManager interface {
	SetUpstreamResolver(r caddy.UpstreamResolver)
}

// startUpstreamNames starts the answerer and gives its addresses to the manager. A start
// failure is logged and leaves the manager without a resolver: forward target names are
// then checked only when a host is saved. It returns nil in that case.
func startUpstreamNames(mgr upstreamNameManager, start func() (*services.UpstreamNameServer, error)) *services.UpstreamNameServer {
	server, err := start()
	if err != nil {
		logger.Log().WithError(err).Error("Upstream name checks are not available; forward target names are checked only when saved")
		return nil
	}
	mgr.SetUpstreamResolver(server.Resolver())
	return server
}
