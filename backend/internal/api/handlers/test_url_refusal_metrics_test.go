package handlers

import (
	"net/http"
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/metrics"
	"github.com/Wikid82/charon/backend/internal/models"
)

// TestPublicURL_RefusedPrivateAddressIsCounted describes the test-url refusal: a private address is answered as
// unreachable with 200, and the refusal moves the SSRF series and the blocked validation series on /metrics.
// The counters are read before and after, and the test is not parallel.
func TestPublicURL_RefusedPrivateAddressIsCounted(t *testing.T) {
	db := OpenTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.Setting{}))
	router := roleStampRouter("admin")
	router.POST("/settings/test-url", NewSettingsHandler(db).TestPublicURL)

	ssrfBefore := testutil.ToFloat64(metrics.SSRFBlockCounter.WithLabelValues("private"))
	validationBefore := testutil.ToFloat64(metrics.URLValidationCounter.WithLabelValues("blocked", "private_ip"))

	w := serveJSON(t, router, http.MethodPost, "/settings/test-url", `{"url":"https://127.0.0.1/"}`)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), `"reachable":false`)
	assert.Equal(t, ssrfBefore+1, testutil.ToFloat64(metrics.SSRFBlockCounter.WithLabelValues("private")))
	assert.Equal(t, validationBefore+1, testutil.ToFloat64(metrics.URLValidationCounter.WithLabelValues("blocked", "private_ip")))
}
