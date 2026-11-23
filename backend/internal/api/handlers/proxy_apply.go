package handlers

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"

	"github.com/Wikid82/charon/backend/internal/api/middleware"
	"github.com/Wikid82/charon/backend/internal/caddy"
	"github.com/Wikid82/charon/backend/internal/util"
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

// hostConfigApplier is what the host handlers need from the proxy manager: the apply and the list of hosts the
// last apply left out.
type hostConfigApplier interface {
	ApplyConfig(ctx context.Context) error
	LeftOutHostUUIDs() map[string]struct{}
}

// applyOrRestore applies the proxy configuration on a detached, bounded context. When the apply is refused it runs
// restore, which puts the database back as it was before the request. A restore failure is logged, not returned.
// When the apply error says Caddy's own rollback also failed, it makes one best-effort re-apply after the restore,
// so the proxy reloads the restored state; that result is logged only. A nil mgr applies nothing and returns nil.
// The returned error is the apply error.
func applyOrRestore(ctx context.Context, mgr CaddyConfigManager, restore func() error) error {
	if mgr == nil {
		return nil
	}
	applyErr := applyProxyConfigDetached(ctx, mgr, proxyApplyTimeout)
	if applyErr == nil {
		return nil
	}
	if restoreErr := restore(); restoreErr != nil {
		logrus.WithError(restoreErr).WithField("operation", "host_restore").Error("Could not restore the previous host after a refused apply")
	}
	if errors.Is(applyErr, caddy.ErrRollbackFailed) {
		if reapplyErr := applyProxyConfigDetached(ctx, mgr, proxyApplyTimeout); reapplyErr != nil {
			logrus.WithError(reapplyErr).WithField("operation", "host_restore_reapply").Error("Re-apply after a failed rollback was refused")
		}
	}
	return applyErr
}

// respondProxyApplyFailed answers a refused apply (D-35-4): 500 with the proxy_apply_failed code. Administrators
// get the apply detail in the text; other callers get the fixed text only. The detail is always logged, sanitized.
// extra fields are merged into the body.
func respondProxyApplyFailed(c *gin.Context, err error, extra gin.H) {
	middleware.GetRequestLogger(c).
		WithField("apply_error", util.SanitizeForLog(err.Error())).
		Error("Error applying config")
	msg := "Failed to apply configuration (previous settings kept)"
	if isAdmin(c) {
		msg = fmt.Sprintf("%s: %s", msg, err.Error())
	}
	respondCoded(c, http.StatusInternalServerError, codeProxyApplyFailed, msg, extra)
}

// EmergencyResetReplyBudget bounds how long an emergency reset may take to answer: the wait for an apply lock held by
// another apply, the reset's own apply, its throttle wait and the writes. The reset extends its write deadline by this
// budget so the reply is not cut off by the Tier 2 server's write timeout.
const EmergencyResetReplyBudget = 2*proxyApplyTimeout + 15*time.Second
