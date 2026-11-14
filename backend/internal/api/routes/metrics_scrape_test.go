package routes

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/Wikid82/charon/backend/internal/metrics"
)

// TestMetricsScrape_ServesURLTestAndBlockedCounters describes the served /metrics output: the URL test collectors and the
// proxied-block counters appear once recorded, and the removed monitored series never appears.
func TestMetricsScrape_ServesURLTestAndBlockedCounters(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	db, err := gorm.Open(sqlite.Open(isolatedMemoryDSN(t)), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, Register(context.Background(), router, db, config.Config{JWTSecret: "test-secret"}))

	metrics.RecordURLValidation("blocked", "private_ip")
	metrics.RecordSSRFBlock("metadata")
	metrics.RecordURLTestDuration(0.05)
	metrics.RecordBlockedEntry("waf")
	metrics.RecordBlockedEntry("crowdsec")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/metrics", http.NoBody))

	require.Equal(t, http.StatusOK, w.Code)
	body := w.Body.String()
	for _, name := range []string{
		"charon_url_validation_total", "charon_ssrf_blocks_total", "charon_url_test_duration_seconds",
		"charon_waf_blocked_total", "charon_crowdsec_blocked_total",
	} {
		assert.Contains(t, body, name)
	}
	assert.NotContains(t, body, "charon_waf_monitored_total")
	assert.NotContains(t, body, `user_id=`, "the SSRF series carries no user label")
}
