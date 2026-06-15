package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/services"
)

func openStatsTestDB(t *testing.T) *StatsHandler {
	t.Helper()
	db := OpenTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.RequestLog{}, &models.ProxyHost{}, &models.SSLCertificate{}))
	svc := services.NewStatsService(db)
	return NewStatsHandler(svc)
}

func setupStatsRouter(h *StatsHandler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/api/stats/summary", h.GetStatsSummary)
	r.GET("/api/stats/top-hosts", h.GetTopHosts)
	r.GET("/api/stats/status-distribution", h.GetStatusDistribution)
	r.GET("/api/stats/traffic-volume", h.GetTrafficVolume)
	r.GET("/api/stats/cert-expiry", h.GetCertExpiry)
	r.GET("/api/stats/requests", h.GetRequests)
	r.GET("/api/stats/health", h.GetStatsHealth)
	return r
}

// TestGetStatsSummary_Returns200WithCorrectShape — test 1
func TestGetStatsSummary_Returns200WithCorrectShape(t *testing.T) {
	h := openStatsTestDB(t)
	r := setupStatsRouter(h)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/stats/summary", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var body map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Contains(t, body, "requests_last_24h")
	assert.Contains(t, body, "requests_last_7d")
	assert.Contains(t, body, "requests_last_30d")
}

// TestGetTopHosts_ValidPeriod_Returns200 — test 2
func TestGetTopHosts_ValidPeriod_Returns200(t *testing.T) {
	h := openStatsTestDB(t)
	r := setupStatsRouter(h)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/stats/top-hosts?period=24h", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// TestGetTopHosts_InvalidPeriod_Returns400 — test 3
func TestGetTopHosts_InvalidPeriod_Returns400(t *testing.T) {
	h := openStatsTestDB(t)
	r := setupStatsRouter(h)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/stats/top-hosts?period=invalid", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var body map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Contains(t, body, "error")
}

// TestGetStatusDistribution_InvalidPeriod_Returns400 — test 4
func TestGetStatusDistribution_InvalidPeriod_Returns400(t *testing.T) {
	h := openStatsTestDB(t)
	r := setupStatsRouter(h)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/stats/status-distribution?period=bad", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestGetTrafficVolume_InvalidBucket_Returns400 — test 5
func TestGetTrafficVolume_InvalidBucket_Returns400(t *testing.T) {
	h := openStatsTestDB(t)
	r := setupStatsRouter(h)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/stats/traffic-volume?bucket=1w", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestGetCertExpiry_WithinDaysZero_Returns400 — test 6
func TestGetCertExpiry_WithinDaysZero_Returns400(t *testing.T) {
	h := openStatsTestDB(t)
	r := setupStatsRouter(h)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/stats/cert-expiry?within_days=0", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestGetCertExpiry_WithinDays366_Returns400 — test 7
func TestGetCertExpiry_WithinDays366_Returns400(t *testing.T) {
	h := openStatsTestDB(t)
	r := setupStatsRouter(h)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/stats/cert-expiry?within_days=366", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestGetStatsHealth_Returns200WithDroppedCount — test 8
func TestGetStatsHealth_Returns200WithDroppedCount(t *testing.T) {
	db := OpenTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.RequestLog{}))
	svc := services.NewStatsService(db)
	ingester := services.NewStatsIngester(db)
	h := NewStatsHandlerWithIngester(svc, ingester)

	r := setupStatsRouter(h)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/stats/health", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var body map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Contains(t, body, "dropped_count")
}

// TestGetRequests_ValidBucket_Returns200 — extra coverage
func TestGetRequests_ValidBucket_Returns200(t *testing.T) {
	h := openStatsTestDB(t)
	r := setupStatsRouter(h)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/stats/requests?bucket=1h", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// TestGetRequests_InvalidBucket_Returns400 — extra coverage
func TestGetRequests_InvalidBucket_Returns400(t *testing.T) {
	h := openStatsTestDB(t)
	r := setupStatsRouter(h)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/stats/requests?bucket=bad", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestGetTopHosts_LimitCap_SilentlyCapAt50 — limit capping test
func TestGetTopHosts_LimitCap_SilentlyCapAt50(t *testing.T) {
	h := openStatsTestDB(t)
	r := setupStatsRouter(h)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/stats/top-hosts?period=24h&limit=100", nil)
	r.ServeHTTP(w, req)

	// Should not return 400 — limit is silently capped
	assert.Equal(t, http.StatusOK, w.Code)
}

// TestGetCertExpiry_ValidWithinDays_Returns200 — valid range passes
func TestGetCertExpiry_ValidWithinDays_Returns200(t *testing.T) {
	h := openStatsTestDB(t)
	r := setupStatsRouter(h)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/stats/cert-expiry?within_days=30", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// openStatsTestDBWithClosedDB returns a handler whose underlying DB is closed,
// causing all service calls to return errors.
func openStatsTestDBWithClosedDB(t *testing.T) *StatsHandler {
	t.Helper()
	db := OpenTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.RequestLog{}, &models.ProxyHost{}, &models.SSLCertificate{}))
	svc := services.NewStatsService(db)
	// Close the underlying SQL connection to force errors on every query.
	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())
	return NewStatsHandler(svc)
}

// TestGetStatsSummary_DBError_Returns500 covers the error path in GetStatsSummary.
func TestGetStatsSummary_DBError_Returns500(t *testing.T) {
	h := openStatsTestDBWithClosedDB(t)
	r := setupStatsRouter(h)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/stats/summary", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// TestGetTopHosts_DBError_Returns500 covers the error path in GetTopHosts.
func TestGetTopHosts_DBError_Returns500(t *testing.T) {
	h := openStatsTestDBWithClosedDB(t)
	r := setupStatsRouter(h)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/stats/top-hosts?period=24h", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// TestGetStatusDistribution_DBError_Returns500 covers the error path in GetStatusDistribution.
func TestGetStatusDistribution_DBError_Returns500(t *testing.T) {
	h := openStatsTestDBWithClosedDB(t)
	r := setupStatsRouter(h)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/stats/status-distribution?period=24h", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// TestGetTrafficVolume_DBError_Returns500 covers the error path in GetTrafficVolume.
func TestGetTrafficVolume_DBError_Returns500(t *testing.T) {
	h := openStatsTestDBWithClosedDB(t)
	r := setupStatsRouter(h)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/stats/traffic-volume?bucket=1h", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// TestGetCertExpiry_NonIntegerWithinDays_Returns400 covers the strconv.Atoi error path.
func TestGetCertExpiry_NonIntegerWithinDays_Returns400(t *testing.T) {
	h := openStatsTestDB(t)
	r := setupStatsRouter(h)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/stats/cert-expiry?within_days=abc", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Contains(t, body, "error")
}

// TestGetCertExpiry_DBError_Returns500 covers the error path in GetCertExpiry.
func TestGetCertExpiry_DBError_Returns500(t *testing.T) {
	h := openStatsTestDBWithClosedDB(t)
	r := setupStatsRouter(h)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/stats/cert-expiry?within_days=30", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// TestGetRequests_DBError_Returns500 covers the error path in GetRequests.
func TestGetRequests_DBError_Returns500(t *testing.T) {
	h := openStatsTestDBWithClosedDB(t)
	r := setupStatsRouter(h)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/stats/requests?bucket=1h", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// TestGetTopHosts_InvalidLimitIgnored_Returns200 covers the limit parse error branch.
func TestGetTopHosts_InvalidLimitIgnored_Returns200(t *testing.T) {
	h := openStatsTestDB(t)
	r := setupStatsRouter(h)

	w := httptest.NewRecorder()
	// Non-integer limit is silently ignored, defaulting to 10.
	req, _ := http.NewRequest(http.MethodGet, "/api/stats/top-hosts?period=24h&limit=abc", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}
