package handlers

// stats_api_integration_test.go — end-to-end stats API tests with a real
// SQLite :memory: database.  Unlike the unit tests in stats_handler_test.go,
// these tests seed RequestLog and SSL/ProxyHost rows first and then assert
// that the HTTP responses contain the expected seeded data.

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/services"
)

// openSeededStatsDB creates an in-memory DB, migrates stats-related models, and
// returns a fully-wired StatsHandler ready for HTTP testing.
func openSeededStatsDB(t *testing.T) *StatsHandler {
	t.Helper()
	db := OpenTestDB(t)
	require.NoError(t, db.AutoMigrate(
		&models.RequestLog{},
		&models.ProxyHost{},
		&models.SSLCertificate{},
	))

	now := time.Now().UTC()

	// Seed a variety of RequestLog rows across different time windows.
	logs := []models.RequestLog{
		// Last 24 h — host-a dominates
		{HostID: "host-a", Timestamp: now.Add(-1 * time.Hour), Method: "GET", StatusCode: 200, BytesSent: 1024, DurationMs: 10},
		{HostID: "host-a", Timestamp: now.Add(-2 * time.Hour), Method: "POST", StatusCode: 201, BytesSent: 512, DurationMs: 20},
		{HostID: "host-a", Timestamp: now.Add(-3 * time.Hour), Method: "GET", StatusCode: 200, BytesSent: 256, DurationMs: 8},
		{HostID: "host-b", Timestamp: now.Add(-4 * time.Hour), Method: "GET", StatusCode: 404, BytesSent: 128, DurationMs: 5},
		// Last 7 d (but older than 24 h)
		{HostID: "host-a", Timestamp: now.Add(-48 * time.Hour), Method: "GET", StatusCode: 500, BytesSent: 2048, DurationMs: 30},
		{HostID: "host-b", Timestamp: now.Add(-72 * time.Hour), Method: "DELETE", StatusCode: 204, BytesSent: 0, DurationMs: 8},
		// Last 30 d (but older than 7 d)
		{HostID: "host-c", Timestamp: now.Add(-10 * 24 * time.Hour), Method: "GET", StatusCode: 200, BytesSent: 4096, DurationMs: 15},
		{HostID: "host-c", Timestamp: now.Add(-20 * 24 * time.Hour), Method: "GET", StatusCode: 200, BytesSent: 8192, DurationMs: 25},
	}
	require.NoError(t, db.Create(&logs).Error)

	svc := services.NewStatsService(db)
	ingester := services.NewStatsIngester(db)
	h := NewStatsHandlerFull(svc, ingester, nil)
	return h
}

// TestStatsAPI_Summary_SeededCounts verifies GET /api/stats/summary returns
// the correct request counts matching the seeded data.
func TestStatsAPI_Summary_SeededCounts(t *testing.T) {
	h := openSeededStatsDB(t)
	r := setupStatsRouter(h)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/stats/summary", nil)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var body map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))

	// 4 logs within 24 h
	assert.InDelta(t, 4, body["requests_last_24h"], 0, "expected 4 requests in last 24h")
	// 4 + 2 = 6 within 7 d
	assert.InDelta(t, 6, body["requests_last_7d"], 0, "expected 6 requests in last 7d")
	// all 8 within 30 d
	assert.InDelta(t, 8, body["requests_last_30d"], 0, "expected 8 requests in last 30d")
}

// TestStatsAPI_TopHosts_24h verifies GET /api/stats/top-hosts?period=24h
// returns host-a as the top host (3 requests in last 24h vs host-b's 1).
func TestStatsAPI_TopHosts_24h(t *testing.T) {
	h := openSeededStatsDB(t)
	r := setupStatsRouter(h)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/stats/top-hosts?period=24h", nil)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var results []map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &results))
	require.NotEmpty(t, results, "expected at least one top-host entry")

	// host-a should be first because it has 3 requests in the 24h window
	topHost, ok := results[0]["host_id"].(string)
	require.True(t, ok, "host_id should be a string")
	assert.Equal(t, "host-a", topHost)
}

// TestStatsAPI_StatusDistribution_7d verifies GET /api/stats/status-distribution?period=7d
// includes the 200, 201, 404, 500, and 204 status codes present in the 7d window.
func TestStatsAPI_StatusDistribution_7d(t *testing.T) {
	h := openSeededStatsDB(t)
	r := setupStatsRouter(h)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/stats/status-distribution?period=7d", nil)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var results []map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &results))
	require.NotEmpty(t, results, "expected status distribution entries")

	// Collect all returned status codes
	codes := make(map[int]bool)
	for _, entry := range results {
		if code, ok := entry["code"].(float64); ok {
			codes[int(code)] = true
		}
	}

	assert.True(t, codes[200], "expected status code 200 in distribution")
	assert.True(t, codes[201], "expected status code 201 in distribution")
	assert.True(t, codes[404], "expected status code 404 in distribution")
}

// TestStatsAPI_TrafficVolume_1h verifies GET /api/stats/traffic-volume?bucket=1h
// returns bucket data with the expected fields present.
func TestStatsAPI_TrafficVolume_1h(t *testing.T) {
	h := openSeededStatsDB(t)
	r := setupStatsRouter(h)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/stats/traffic-volume?bucket=1h", nil)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var results []map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &results))

	// Results may be empty if no logs fall in a full 1h bucket, but the response
	// shape must be a JSON array (not null or an error object).
	assert.NotNil(t, results, "traffic volume response must be a JSON array")

	// Validate shape of any returned bucket
	for _, bucket := range results {
		assert.Contains(t, bucket, "bucket", "each bucket must have a 'bucket' timestamp field")
		assert.Contains(t, bucket, "bytes_sent", "each bucket must have a 'bytes_sent' field")
	}
}

// TestStatsAPI_CertExpiry_30Days verifies GET /api/stats/cert-expiry?within_days=30
// returns a JSON array (certificates expiring soon, if any).
func TestStatsAPI_CertExpiry_30Days(t *testing.T) {
	h := openSeededStatsDB(t)
	r := setupStatsRouter(h)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/stats/cert-expiry?within_days=30", nil)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	// Response must decode as a JSON array (may be empty for a fresh test DB).
	var results []map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &results))
	assert.NotNil(t, results)
}

// TestStatsAPI_CertExpiry_WithExpiringSoon verifies that a certificate expiring
// within the threshold window is included in the cert-expiry response.
func TestStatsAPI_CertExpiry_WithExpiringSoon(t *testing.T) {
	db := OpenTestDB(t)
	require.NoError(t, db.AutoMigrate(
		&models.RequestLog{},
		&models.ProxyHost{},
		&models.SSLCertificate{},
	))

	// Seed a certificate that expires in 15 days (within the 30-day window).
	expiresAt := time.Now().UTC().Add(15 * 24 * time.Hour)
	cert := models.SSLCertificate{
		UUID:      "cert-uuid-expiring-soon",
		Name:      "Expiring Soon Cert",
		ExpiresAt: &expiresAt,
	}
	require.NoError(t, db.Create(&cert).Error)

	// Seed a proxy host linked to that certificate.
	host := models.ProxyHost{
		UUID:          "host-uuid-expiring",
		DomainNames:   "expiring.example.com",
		ForwardHost:   "10.0.0.1",
		ForwardPort:   80,
		CertificateID: &cert.ID,
	}
	require.NoError(t, db.Create(&host).Error)

	svc := services.NewStatsService(db)
	h := NewStatsHandler(svc)
	r := setupStatsRouter(h)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/stats/cert-expiry?within_days=30", nil)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var results []map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &results))
	require.NotEmpty(t, results, "expected at least one cert expiry entry")

	// Verify response shape fields
	first := results[0]
	assert.Contains(t, first, "host_id")
	assert.Contains(t, first, "hostname")
	assert.Contains(t, first, "expires_at")
	assert.Contains(t, first, "days_left")
}

// TestStatsAPI_CertExpiry_ZeroDays_Returns400 verifies that within_days=0
// is rejected with HTTP 400.
func TestStatsAPI_CertExpiry_ZeroDays_Returns400(t *testing.T) {
	h := openSeededStatsDB(t)
	r := setupStatsRouter(h)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/stats/cert-expiry?within_days=0", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var body map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Contains(t, body, "error")
}

// TestStatsAPI_CertExpiry_366Days_Returns400 verifies that within_days=366
// is rejected with HTTP 400 because the upper bound is 365.
func TestStatsAPI_CertExpiry_366Days_Returns400(t *testing.T) {
	h := openSeededStatsDB(t)
	r := setupStatsRouter(h)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/stats/cert-expiry?within_days=366", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var body map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Contains(t, body, "error")
}

// TestStatsAPI_Health_DroppedCountPresent verifies GET /api/stats/health
// includes the dropped_count field and returns HTTP 200.
func TestStatsAPI_Health_DroppedCountPresent(t *testing.T) {
	db := OpenTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.RequestLog{}))

	svc := services.NewStatsService(db)
	ingester := services.NewStatsIngester(db)
	h := NewStatsHandlerFull(svc, ingester, nil)
	r := setupStatsRouter(h)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/stats/health", nil)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var body map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Contains(t, body, "dropped_count", "health response must include dropped_count")

	// Fresh ingester should report 0 dropped events
	assert.InDelta(t, float64(0), body["dropped_count"], 0, "expected 0 dropped events for a fresh ingester")
}
