package handlers

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/models"
)

func TestAccessListHandler_Update_UsageCheckFailureReportsErrorWithoutApply(t *testing.T) {
	mgr := &countingCaddyManager{}
	r, db := accessListApplyRouter(t, mgr)
	id := createAccessListForApply(t, db)
	attachToProxyHost(t, db, id)
	// Dropping the host table makes the usage count fail while the list update itself still succeeds.
	require.NoError(t, db.Migrator().DropTable(&models.ProxyHost{}))

	w := putAccessList(r, id, updatedWhitelistBody)

	require.Equal(t, http.StatusInternalServerError, w.Code)
	require.Contains(t, w.Body.String(), "saved, but applying the configuration failed")
	require.Zero(t, mgr.calls, "the proxy config is not applied when usage cannot be determined")
}
