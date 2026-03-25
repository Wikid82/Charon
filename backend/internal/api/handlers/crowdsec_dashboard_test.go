package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupDashboardHandler creates a CrowdsecHandler with an in-memory DB seeded with decisions.
func setupDashboardHandler(t *testing.T) (*CrowdsecHandler, *gin.Engine) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	db := OpenTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.SecurityDecision{}, &models.SecurityConfig{}, &models.Setting{}))

	h := &CrowdsecHandler{
		DB:        db,
		Executor:  &fakeExec{},
		CmdExec:   &fastCmdExec{},
		BinPath:   "/bin/false",
		DataDir:   t.TempDir(),
		dashCache: newDashboardCache(),
	}

	seedDashboardData(t, h)

	r := gin.New()
	g := r.Group("/api/v1")
	h.RegisterRoutes(g)
	return h, r
}

// seedDashboardData inserts representative records for testing.
func seedDashboardData(t *testing.T, h *CrowdsecHandler) {
	t.Helper()
	now := time.Now().UTC()

	decisions := []models.SecurityDecision{
		{UUID: uuid.NewString(), Source: "crowdsec", Action: "block", IP: "10.0.0.1", Scenario: "crowdsecurity/http-probing", Country: "US", CreatedAt: now.Add(-1 * time.Hour)},
		{UUID: uuid.NewString(), Source: "crowdsec", Action: "block", IP: "10.0.0.1", Scenario: "crowdsecurity/http-probing", Country: "US", CreatedAt: now.Add(-2 * time.Hour)},
		{UUID: uuid.NewString(), Source: "crowdsec", Action: "challenge", IP: "10.0.0.2", Scenario: "crowdsecurity/ssh-bf", Country: "DE", CreatedAt: now.Add(-3 * time.Hour)},
		{UUID: uuid.NewString(), Source: "crowdsec", Action: "block", IP: "10.0.0.3", Scenario: "crowdsecurity/http-probing", Country: "FR", CreatedAt: now.Add(-5 * time.Hour)},
		{UUID: uuid.NewString(), Source: "crowdsec", Action: "block", IP: "10.0.0.4", Scenario: "crowdsecurity/http-bad-user-agent", Country: "", CreatedAt: now.Add(-10 * time.Hour)},
		// Old record outside 24h but within 7d
		{UUID: uuid.NewString(), Source: "crowdsec", Action: "block", IP: "10.0.0.5", Scenario: "crowdsecurity/http-probing", Country: "JP", CreatedAt: now.Add(-48 * time.Hour)},
		// Non-crowdsec source
		{UUID: uuid.NewString(), Source: "waf", Action: "block", IP: "10.0.0.99", Scenario: "waf-rule", Country: "CN", CreatedAt: now.Add(-1 * time.Hour)},
	}

	for _, d := range decisions {
		require.NoError(t, h.DB.Create(&d).Error)
	}
}

func TestParseTimeRange(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input string
		valid bool
	}{
		{"1h", true},
		{"6h", true},
		{"24h", true},
		{"7d", true},
		{"30d", true},
		{"", true},
		{"2h", false},
		{"1w", false},
		{"invalid", false},
	}

	for _, tc := range tests {
		t.Run(fmt.Sprintf("range_%s", tc.input), func(t *testing.T) {
			_, err := parseTimeRange(tc.input)
			if tc.valid {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
		})
	}
}

func TestParseTimeRange_DefaultEmpty(t *testing.T) {
	t.Parallel()
	result, err := parseTimeRange("")
	require.NoError(t, err)
	expected := time.Now().UTC().Add(-24 * time.Hour)
	assert.InDelta(t, expected.UnixMilli(), result.UnixMilli(), 1000)
}

func TestDashboardSummary_OK(t *testing.T) {
	t.Parallel()
	_, r := setupDashboardHandler(t)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/crowdsec/dashboard/summary?range=24h", http.NoBody)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))

	assert.Contains(t, body, "total_decisions")
	assert.Contains(t, body, "active_decisions")
	assert.Contains(t, body, "unique_ips")
	assert.Contains(t, body, "top_scenario")
	assert.Contains(t, body, "decisions_trend")
	assert.Contains(t, body, "range")
	assert.Contains(t, body, "generated_at")
	assert.Equal(t, "24h", body["range"])

	// 5 crowdsec decisions within 24h (excludes 48h-old one)
	total := body["total_decisions"].(float64)
	assert.Equal(t, float64(5), total)

	// 4 unique crowdsec IPs within 24h
	assert.Equal(t, float64(4), body["unique_ips"].(float64))

	// LAPI unreachable in test => -1
	assert.Equal(t, float64(-1), body["active_decisions"].(float64))
}

func TestDashboardSummary_InvalidRange(t *testing.T) {
	t.Parallel()
	_, r := setupDashboardHandler(t)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/crowdsec/dashboard/summary?range=99z", http.NoBody)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestDashboardSummary_Cached(t *testing.T) {
	t.Parallel()
	_, r := setupDashboardHandler(t)

	// First call populates cache
	w1 := httptest.NewRecorder()
	req1 := httptest.NewRequest(http.MethodGet, "/api/v1/admin/crowdsec/dashboard/summary?range=24h", http.NoBody)
	r.ServeHTTP(w1, req1)
	assert.Equal(t, http.StatusOK, w1.Code)

	// Second call should hit cache
	w2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodGet, "/api/v1/admin/crowdsec/dashboard/summary?range=24h", http.NoBody)
	r.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusOK, w2.Code)
}

func TestDashboardTimeline_OK(t *testing.T) {
	t.Parallel()
	_, r := setupDashboardHandler(t)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/crowdsec/dashboard/timeline?range=24h", http.NoBody)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))

	assert.Contains(t, body, "buckets")
	assert.Contains(t, body, "interval")
	assert.Equal(t, "1h", body["interval"])
	assert.Equal(t, "24h", body["range"])
}

func TestDashboardTimeline_CustomInterval(t *testing.T) {
	t.Parallel()
	_, r := setupDashboardHandler(t)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/crowdsec/dashboard/timeline?range=6h&interval=15m", http.NoBody)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, "15m", body["interval"])
}

func TestDashboardTimeline_InvalidInterval(t *testing.T) {
	t.Parallel()
	_, r := setupDashboardHandler(t)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/crowdsec/dashboard/timeline?interval=99m", http.NoBody)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestDashboardTopIPs_OK(t *testing.T) {
	t.Parallel()
	_, r := setupDashboardHandler(t)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/crowdsec/dashboard/top-ips?range=24h&limit=3", http.NoBody)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))

	ips := body["ips"].([]interface{})
	assert.LessOrEqual(t, len(ips), 3)
	// 10.0.0.1 has 2 hits, should be first
	if len(ips) > 0 {
		first := ips[0].(map[string]interface{})
		assert.Equal(t, "10.0.0.1", first["ip"])
		assert.Equal(t, float64(2), first["count"])
	}
}

func TestDashboardTopIPs_LimitCap(t *testing.T) {
	t.Parallel()
	_, r := setupDashboardHandler(t)

	// Limit > 50 should be capped
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/crowdsec/dashboard/top-ips?limit=100", http.NoBody)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestDashboardScenarios_OK(t *testing.T) {
	t.Parallel()
	_, r := setupDashboardHandler(t)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/crowdsec/dashboard/scenarios?range=24h", http.NoBody)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))

	assert.Contains(t, body, "scenarios")
	assert.Contains(t, body, "total")
	scenarios := body["scenarios"].([]interface{})
	assert.Greater(t, len(scenarios), 0)

	// Verify percentages sum to ~100
	var totalPct float64
	for _, s := range scenarios {
		sc := s.(map[string]interface{})
		totalPct += sc["percentage"].(float64)
		assert.Contains(t, sc, "name")
		assert.Contains(t, sc, "count")
	}
	assert.InDelta(t, 100.0, totalPct, 1.0)
}

func TestListAlerts_OK(t *testing.T) {
	t.Parallel()
	_, r := setupDashboardHandler(t)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/crowdsec/alerts?range=24h", http.NoBody)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))

	assert.Contains(t, body, "alerts")
	assert.Contains(t, body, "source")
	// Falls back to cscli which returns empty/error in test
	assert.Equal(t, "cscli", body["source"])
}

func TestListAlerts_InvalidRange(t *testing.T) {
	t.Parallel()
	_, r := setupDashboardHandler(t)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/crowdsec/alerts?range=invalid", http.NoBody)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestExportDecisions_CSV(t *testing.T) {
	t.Parallel()
	_, r := setupDashboardHandler(t)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/crowdsec/decisions/export?format=csv&range=24h", http.NoBody)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "text/csv")
	assert.Contains(t, w.Header().Get("Content-Disposition"), "attachment")
	assert.Contains(t, w.Body.String(), "uuid,ip,action,source,scenario")
}

func TestExportDecisions_JSON(t *testing.T) {
	t.Parallel()
	_, r := setupDashboardHandler(t)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/crowdsec/decisions/export?format=json&range=24h", http.NoBody)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "application/json")

	var decisions []models.SecurityDecision
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &decisions))
	assert.Greater(t, len(decisions), 0)
}

func TestExportDecisions_InvalidFormat(t *testing.T) {
	t.Parallel()
	_, r := setupDashboardHandler(t)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/crowdsec/decisions/export?format=xml", http.NoBody)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestExportDecisions_InvalidSource(t *testing.T) {
	t.Parallel()
	_, r := setupDashboardHandler(t)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/crowdsec/decisions/export?source=evil", http.NoBody)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestSanitizeCSVField(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input    string
		expected string
	}{
		{"normal", "normal"},
		{"=cmd", "'=cmd"},
		{"+cmd", "'+cmd"},
		{"-cmd", "'-cmd"},
		{"@cmd", "'@cmd"},
		{"\tcmd", "'\tcmd"},
		{"\rcmd", "'\rcmd"},
		{"", ""},
	}

	for _, tc := range tests {
		assert.Equal(t, tc.expected, sanitizeCSVField(tc.input))
	}
}

func TestDashboardCache_Invalidate(t *testing.T) {
	t.Parallel()
	cache := newDashboardCache()
	cache.Set("dashboard:summary:24h", "data1", 5*time.Minute)
	cache.Set("dashboard:timeline:24h", "data2", 5*time.Minute)
	cache.Set("other:key", "data3", 5*time.Minute)

	cache.Invalidate("dashboard")

	_, ok1 := cache.Get("dashboard:summary:24h")
	assert.False(t, ok1)

	_, ok2 := cache.Get("dashboard:timeline:24h")
	assert.False(t, ok2)

	_, ok3 := cache.Get("other:key")
	assert.True(t, ok3)
}

func TestDashboardCache_TTLExpiry(t *testing.T) {
	t.Parallel()
	cache := newDashboardCache()
	cache.Set("key", "value", 1*time.Millisecond)

	time.Sleep(5 * time.Millisecond)
	_, ok := cache.Get("key")
	assert.False(t, ok)
}

func TestDashboardCache_TTLExpiry_DeletesEntry(t *testing.T) {
	t.Parallel()
	cache := newDashboardCache()
	cache.Set("expired", "data", 1*time.Millisecond)

	time.Sleep(5 * time.Millisecond)
	_, ok := cache.Get("expired")
	assert.False(t, ok)

	cache.mu.Lock()
	_, stillPresent := cache.entries["expired"]
	cache.mu.Unlock()
	assert.False(t, stillPresent, "expired entry should be deleted from map")
}

func TestDashboardSummary_DecisionsTrend(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	db := OpenTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.SecurityDecision{}, &models.SecurityConfig{}, &models.Setting{}))

	h := &CrowdsecHandler{
		DB:        db,
		Executor:  &fakeExec{},
		CmdExec:   &fastCmdExec{},
		BinPath:   "/bin/false",
		DataDir:   t.TempDir(),
		dashCache: newDashboardCache(),
	}

	now := time.Now().UTC()
	// Seed 3 decisions in the current 1h period
	for i := 0; i < 3; i++ {
		require.NoError(t, db.Create(&models.SecurityDecision{
			UUID: uuid.NewString(), Source: "crowdsec", Action: "block",
			IP: "192.168.1.1", Scenario: "crowdsecurity/http-probing",
			CreatedAt: now.Add(-time.Duration(i+1) * time.Minute),
		}).Error)
	}
	// Seed 2 decisions in the previous 1h period
	for i := 0; i < 2; i++ {
		require.NoError(t, db.Create(&models.SecurityDecision{
			UUID: uuid.NewString(), Source: "crowdsec", Action: "block",
			IP: "192.168.1.2", Scenario: "crowdsecurity/http-probing",
			CreatedAt: now.Add(-1*time.Hour - time.Duration(i+1)*time.Minute),
		}).Error)
	}

	r := gin.New()
	g := r.Group("/api/v1")
	h.RegisterRoutes(g)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/crowdsec/dashboard/summary?range=1h", http.NoBody)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))

	// (3 - 2) / 2 * 100 = 50.0
	trend := body["decisions_trend"].(float64)
	assert.InDelta(t, 50.0, trend, 0.1)
}

func TestExportDecisions_SourceFilter(t *testing.T) {
	t.Parallel()
	_, r := setupDashboardHandler(t)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/crowdsec/decisions/export?format=json&range=7d&source=waf", http.NoBody)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var decisions []models.SecurityDecision
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &decisions))
	for _, d := range decisions {
		assert.Equal(t, "waf", d.Source)
	}
}
