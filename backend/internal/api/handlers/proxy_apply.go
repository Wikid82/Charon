package handlers

import (
	"context"
	"time"
)

// proxyApplyTimeout bounds one proxy configuration apply triggered by an admin request.
const proxyApplyTimeout = 30 * time.Second

// applyProxyConfigDetached applies the proxy configuration on a context that keeps the request's values but not
// its cancellation, so a client that leaves cannot cancel the apply or the rollback inside it. The apply is
// bounded by timeout. The error is returned unchanged; each caller decides how to answer it.
func applyProxyConfigDetached(ctx context.Context, mgr CaddyConfigManager, timeout time.Duration) error {
	applyCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), timeout)
	defer cancel()
	return mgr.ApplyConfig(applyCtx)
}

// EmergencyResetReplyBudget bounds how long an emergency reset may take to answer: the wait for an apply lock held by
// another apply, the reset's own apply, its throttle wait and the writes. The reset extends its write deadline by this
// budget so the reply is not cut off by the Tier 2 server's write timeout.
const EmergencyResetReplyBudget = 2*proxyApplyTimeout + 15*time.Second
