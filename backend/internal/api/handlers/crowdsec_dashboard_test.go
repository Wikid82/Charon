package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
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

// ---------------------------------------------------------------------------
// Helper functions unit tests
// ---------------------------------------------------------------------------

func TestNormalizeRange(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "24h", normalizeRange(""))
	assert.Equal(t, "1h", normalizeRange("1h"))
	assert.Equal(t, "7d", normalizeRange("7d"))
	assert.Equal(t, "30d", normalizeRange("30d"))
}

func TestIntervalForRange_AllBranches(t *testing.T) {
	t.Parallel()
	tests := []struct {
		rangeStr string
		expected string
	}{
		{"1h", "5m"},
		{"6h", "15m"},
		{"24h", "1h"},
		{"", "1h"},
		{"7d", "6h"},
		{"30d", "1d"},
		{"unknown", "1h"},
	}
	for _, tc := range tests {
		assert.Equal(t, tc.expected, intervalForRange(tc.rangeStr), "intervalForRange(%q)", tc.rangeStr)
	}
}

func TestIntervalToStrftime_AllBranches(t *testing.T) {
	t.Parallel()
	tests := []struct {
		interval string
		contains string
	}{
		{"5m", "/ 5) * 5"},
		{"15m", "/ 15) * 15"},
		{"1h", "%H:00:00Z"},
		{"6h", "/ 6) * 6"},
		{"1d", "T00:00:00Z"},
		{"unknown", "%H:00:00Z"},
	}
	for _, tc := range tests {
		result := intervalToStrftime(tc.interval)
		assert.Contains(t, result, tc.contains, "intervalToStrftime(%q)", tc.interval)
	}
}

func TestValidInterval_AllBranches(t *testing.T) {
	t.Parallel()
	for _, v := range []string{"5m", "15m", "1h", "6h", "1d"} {
		assert.True(t, validInterval(v), "validInterval(%q)", v)
	}
	assert.False(t, validInterval("10m"))
	assert.False(t, validInterval(""))
}

// ---------------------------------------------------------------------------
// DashboardSummary edge cases
// ---------------------------------------------------------------------------

func TestDashboardSummary_EmptyRange(t *testing.T) {
	t.Parallel()
	_, r := setupDashboardHandler(t)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/crowdsec/dashboard/summary", http.NoBody)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, "24h", body["range"])
}

func TestDashboardSummary_7dRange(t *testing.T) {
	t.Parallel()
	_, r := setupDashboardHandler(t)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/crowdsec/dashboard/summary?range=7d", http.NoBody)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, "7d", body["range"])
	// 7d includes the 48h-old record
	total := body["total_decisions"].(float64)
	assert.Equal(t, float64(6), total)
}

func TestDashboardSummary_TrendNegative100(t *testing.T) {
	t.Parallel()

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
	// Only seed decisions in the PREVIOUS 1h period (nothing in current)
	for i := 0; i < 3; i++ {
		require.NoError(t, db.Create(&models.SecurityDecision{
			UUID: uuid.NewString(), Source: "crowdsec", Action: "block",
			IP: "192.168.1.1", Scenario: "crowdsecurity/http-probing",
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
	assert.Equal(t, -100.0, body["decisions_trend"])
}

// ---------------------------------------------------------------------------
// DashboardTimeline edge cases
// ---------------------------------------------------------------------------

func TestDashboardTimeline_Cached(t *testing.T) {
	t.Parallel()
	_, r := setupDashboardHandler(t)

	// First call populates cache
	w1 := httptest.NewRecorder()
	req1 := httptest.NewRequest(http.MethodGet, "/api/v1/admin/crowdsec/dashboard/timeline?range=24h", http.NoBody)
	r.ServeHTTP(w1, req1)
	assert.Equal(t, http.StatusOK, w1.Code)

	// Second call hits cache
	w2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodGet, "/api/v1/admin/crowdsec/dashboard/timeline?range=24h", http.NoBody)
	r.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusOK, w2.Code)
}

func TestDashboardTimeline_InvalidRange(t *testing.T) {
	t.Parallel()
	_, r := setupDashboardHandler(t)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/crowdsec/dashboard/timeline?range=99z&interval=1h", http.NoBody)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestDashboardTimeline_AllRangeIntervals(t *testing.T) {
	t.Parallel()
	_, r := setupDashboardHandler(t)

	ranges := []struct {
		rangeStr string
		interval string
	}{
		{"1h", "5m"},
		{"6h", "15m"},
		{"7d", "6h"},
		{"30d", "1d"},
	}

	for _, tc := range ranges {
		t.Run(tc.rangeStr, func(t *testing.T) {
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet,
				fmt.Sprintf("/api/v1/admin/crowdsec/dashboard/timeline?range=%s", tc.rangeStr), http.NoBody)
			r.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)
			var body map[string]interface{}
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
			assert.Equal(t, tc.interval, body["interval"])
		})
	}
}

// ---------------------------------------------------------------------------
// DashboardTopIPs edge cases
// ---------------------------------------------------------------------------

func TestDashboardTopIPs_InvalidRange(t *testing.T) {
	t.Parallel()
	_, r := setupDashboardHandler(t)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/crowdsec/dashboard/top-ips?range=bad", http.NoBody)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestDashboardTopIPs_BadLimit(t *testing.T) {
	t.Parallel()
	_, r := setupDashboardHandler(t)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/crowdsec/dashboard/top-ips?limit=abc", http.NoBody)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestDashboardTopIPs_NegativeLimit(t *testing.T) {
	t.Parallel()
	_, r := setupDashboardHandler(t)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/crowdsec/dashboard/top-ips?limit=-5", http.NoBody)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestDashboardTopIPs_Cached(t *testing.T) {
	t.Parallel()
	_, r := setupDashboardHandler(t)

	w1 := httptest.NewRecorder()
	req1 := httptest.NewRequest(http.MethodGet, "/api/v1/admin/crowdsec/dashboard/top-ips?range=24h&limit=10", http.NoBody)
	r.ServeHTTP(w1, req1)
	assert.Equal(t, http.StatusOK, w1.Code)

	w2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodGet, "/api/v1/admin/crowdsec/dashboard/top-ips?range=24h&limit=10", http.NoBody)
	r.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusOK, w2.Code)
}

// ---------------------------------------------------------------------------
// DashboardScenarios edge cases
// ---------------------------------------------------------------------------

func TestDashboardScenarios_InvalidRange(t *testing.T) {
	t.Parallel()
	_, r := setupDashboardHandler(t)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/crowdsec/dashboard/scenarios?range=bad", http.NoBody)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestDashboardScenarios_Cached(t *testing.T) {
	t.Parallel()
	_, r := setupDashboardHandler(t)

	w1 := httptest.NewRecorder()
	req1 := httptest.NewRequest(http.MethodGet, "/api/v1/admin/crowdsec/dashboard/scenarios?range=24h", http.NoBody)
	r.ServeHTTP(w1, req1)
	assert.Equal(t, http.StatusOK, w1.Code)

	w2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodGet, "/api/v1/admin/crowdsec/dashboard/scenarios?range=24h", http.NoBody)
	r.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusOK, w2.Code)
}

// ---------------------------------------------------------------------------
// ListAlerts edge cases
// ---------------------------------------------------------------------------

func TestListAlerts_BadLimit(t *testing.T) {
	t.Parallel()
	_, r := setupDashboardHandler(t)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/crowdsec/alerts?limit=abc", http.NoBody)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestListAlerts_LimitCap(t *testing.T) {
	t.Parallel()
	_, r := setupDashboardHandler(t)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/crowdsec/alerts?limit=999", http.NoBody)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestListAlerts_NegativeLimit(t *testing.T) {
	t.Parallel()
	_, r := setupDashboardHandler(t)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/crowdsec/alerts?limit=-1", http.NoBody)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestListAlerts_BadOffset(t *testing.T) {
	t.Parallel()
	_, r := setupDashboardHandler(t)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/crowdsec/alerts?offset=abc", http.NoBody)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestListAlerts_NegativeOffset(t *testing.T) {
	t.Parallel()
	_, r := setupDashboardHandler(t)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/crowdsec/alerts?offset=-5", http.NoBody)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestListAlerts_Cached(t *testing.T) {
	t.Parallel()
	_, r := setupDashboardHandler(t)

	w1 := httptest.NewRecorder()
	req1 := httptest.NewRequest(http.MethodGet, "/api/v1/admin/crowdsec/alerts?range=24h", http.NoBody)
	r.ServeHTTP(w1, req1)
	assert.Equal(t, http.StatusOK, w1.Code)

	w2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodGet, "/api/v1/admin/crowdsec/alerts?range=24h", http.NoBody)
	r.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusOK, w2.Code)
}

func TestListAlerts_ScenarioFilter(t *testing.T) {
	t.Parallel()
	_, r := setupDashboardHandler(t)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/crowdsec/alerts?scenario=crowdsecurity/http-probing", http.NoBody)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Contains(t, body, "source")
}

// ---------------------------------------------------------------------------
// ExportDecisions edge cases
// ---------------------------------------------------------------------------

func TestExportDecisions_InvalidRange(t *testing.T) {
	t.Parallel()
	_, r := setupDashboardHandler(t)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/crowdsec/decisions/export?range=bad", http.NoBody)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestExportDecisions_EmptyRange(t *testing.T) {
	t.Parallel()
	_, r := setupDashboardHandler(t)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/crowdsec/decisions/export?format=json", http.NoBody)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestExportDecisions_CSVWithSourceFilter(t *testing.T) {
	t.Parallel()
	_, r := setupDashboardHandler(t)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/crowdsec/decisions/export?format=csv&source=crowdsec&range=7d", http.NoBody)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "text/csv")
	body := w.Body.String()
	assert.Contains(t, body, "uuid,ip,action,source,scenario")
	// Verify all rows are crowdsec source
	assert.NotContains(t, body, ",waf,")
}

func TestExportDecisions_AllSources(t *testing.T) {
	t.Parallel()
	_, r := setupDashboardHandler(t)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/crowdsec/decisions/export?format=json&source=all&range=7d", http.NoBody)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var decisions []models.SecurityDecision
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &decisions))
	// Should include both crowdsec and waf sources
	assert.GreaterOrEqual(t, len(decisions), 2)
}

// ---------------------------------------------------------------------------
// LAPI integration paths via httptest server
// ---------------------------------------------------------------------------

func TestDashboardSummary_ActiveDecisions_LAPIReachable(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"id":"1"},{"id":"2"},{"id":"3"}]`))
	}))
	t.Cleanup(server.Close)

	db := OpenTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.SecurityDecision{}, &models.SecurityConfig{}, &models.Setting{}))

	require.NoError(t, db.Create(&models.SecurityConfig{
		UUID:           "default",
		Name:           "default",
		CrowdSecAPIURL: server.URL,
	}).Error)

	h := newTestCrowdsecHandler(t, db, &fakeExec{}, "/bin/false", t.TempDir())
	h.validateLAPIURL = func(raw string) (*url.URL, error) { return url.Parse(raw) }

	r := gin.New()
	g := r.Group("/api/v1")
	h.RegisterRoutes(g)

	now := time.Now().UTC()
	require.NoError(t, db.Create(&models.SecurityDecision{
		UUID: uuid.NewString(), Source: "crowdsec", Action: "block",
		IP: "10.0.0.1", Scenario: "test", CreatedAt: now.Add(-30 * time.Minute),
	}).Error)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/crowdsec/dashboard/summary?range=1h", http.NoBody)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, float64(3), body["active_decisions"])
}

func TestDashboardSummary_ActiveDecisions_LAPIBadStatus(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(server.Close)

	db := OpenTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.SecurityDecision{}, &models.SecurityConfig{}, &models.Setting{}))
	require.NoError(t, db.Create(&models.SecurityConfig{
		UUID: "default", Name: "default", CrowdSecAPIURL: server.URL,
	}).Error)

	h := newTestCrowdsecHandler(t, db, &fakeExec{}, "/bin/false", t.TempDir())
	h.validateLAPIURL = func(raw string) (*url.URL, error) { return url.Parse(raw) }

	r := gin.New()
	g := r.Group("/api/v1")
	h.RegisterRoutes(g)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/crowdsec/dashboard/summary?range=1h", http.NoBody)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, float64(-1), body["active_decisions"])
}

func TestDashboardSummary_ActiveDecisions_LAPIBadJSON(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`not-json`))
	}))
	t.Cleanup(server.Close)

	db := OpenTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.SecurityDecision{}, &models.SecurityConfig{}, &models.Setting{}))
	require.NoError(t, db.Create(&models.SecurityConfig{
		UUID: "default", Name: "default", CrowdSecAPIURL: server.URL,
	}).Error)

	h := newTestCrowdsecHandler(t, db, &fakeExec{}, "/bin/false", t.TempDir())
	h.validateLAPIURL = func(raw string) (*url.URL, error) { return url.Parse(raw) }

	r := gin.New()
	g := r.Group("/api/v1")
	h.RegisterRoutes(g)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/crowdsec/dashboard/summary?range=1h", http.NoBody)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, float64(-1), body["active_decisions"])
}

func TestListAlerts_LAPISuccess(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"id":"a1"},{"id":"a2"},{"id":"a3"},{"id":"a4"},{"id":"a5"}]`))
	}))
	t.Cleanup(server.Close)

	db := OpenTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.SecurityDecision{}, &models.SecurityConfig{}, &models.Setting{}))
	require.NoError(t, db.Create(&models.SecurityConfig{
		UUID: "default", Name: "default", CrowdSecAPIURL: server.URL,
	}).Error)

	h := newTestCrowdsecHandler(t, db, &fakeExec{}, "/bin/false", t.TempDir())
	h.validateLAPIURL = func(raw string) (*url.URL, error) { return url.Parse(raw) }

	r := gin.New()
	g := r.Group("/api/v1")
	h.RegisterRoutes(g)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/crowdsec/alerts?range=24h", http.NoBody)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, "lapi", body["source"])
	assert.Equal(t, float64(5), body["total"])
}

func TestListAlerts_LAPISuccessWithOffset(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"id":"a1"},{"id":"a2"},{"id":"a3"},{"id":"a4"},{"id":"a5"}]`))
	}))
	t.Cleanup(server.Close)

	db := OpenTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.SecurityDecision{}, &models.SecurityConfig{}, &models.Setting{}))
	require.NoError(t, db.Create(&models.SecurityConfig{
		UUID: "default", Name: "default", CrowdSecAPIURL: server.URL,
	}).Error)

	h := newTestCrowdsecHandler(t, db, &fakeExec{}, "/bin/false", t.TempDir())
	h.validateLAPIURL = func(raw string) (*url.URL, error) { return url.Parse(raw) }

	r := gin.New()
	g := r.Group("/api/v1")
	h.RegisterRoutes(g)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/crowdsec/alerts?range=24h&offset=3&limit=10", http.NoBody)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, "lapi", body["source"])
	assert.Equal(t, float64(5), body["total"])
	alerts := body["alerts"].([]interface{})
	assert.Equal(t, 2, len(alerts))
}

func TestListAlerts_LAPISuccessWithLargeOffset(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"id":"a1"},{"id":"a2"}]`))
	}))
	t.Cleanup(server.Close)

	db := OpenTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.SecurityDecision{}, &models.SecurityConfig{}, &models.Setting{}))
	require.NoError(t, db.Create(&models.SecurityConfig{
		UUID: "default", Name: "default", CrowdSecAPIURL: server.URL,
	}).Error)

	h := newTestCrowdsecHandler(t, db, &fakeExec{}, "/bin/false", t.TempDir())
	h.validateLAPIURL = func(raw string) (*url.URL, error) { return url.Parse(raw) }

	r := gin.New()
	g := r.Group("/api/v1")
	h.RegisterRoutes(g)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/crowdsec/alerts?range=24h&offset=100", http.NoBody)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, "lapi", body["source"])
	// offset >= len(rawAlerts) returns nil, which marshals as JSON null
	alerts := body["alerts"]
	if alerts != nil {
		assert.Equal(t, 0, len(alerts.([]interface{})))
	}
}

func TestListAlerts_LAPISuccessWithLimitSlicing(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"id":"a1"},{"id":"a2"},{"id":"a3"},{"id":"a4"},{"id":"a5"}]`))
	}))
	t.Cleanup(server.Close)

	db := OpenTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.SecurityDecision{}, &models.SecurityConfig{}, &models.Setting{}))
	require.NoError(t, db.Create(&models.SecurityConfig{
		UUID: "default", Name: "default", CrowdSecAPIURL: server.URL,
	}).Error)

	h := newTestCrowdsecHandler(t, db, &fakeExec{}, "/bin/false", t.TempDir())
	h.validateLAPIURL = func(raw string) (*url.URL, error) { return url.Parse(raw) }

	r := gin.New()
	g := r.Group("/api/v1")
	h.RegisterRoutes(g)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/crowdsec/alerts?range=24h&limit=2", http.NoBody)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, "lapi", body["source"])
	assert.Equal(t, float64(5), body["total"])
	alerts := body["alerts"].([]interface{})
	assert.Equal(t, 2, len(alerts))
}

func TestListAlerts_LAPIBadJSON(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`not-json`))
	}))
	t.Cleanup(server.Close)

	db := OpenTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.SecurityDecision{}, &models.SecurityConfig{}, &models.Setting{}))
	require.NoError(t, db.Create(&models.SecurityConfig{
		UUID: "default", Name: "default", CrowdSecAPIURL: server.URL,
	}).Error)

	h := newTestCrowdsecHandler(t, db, &fakeExec{}, "/bin/false", t.TempDir())
	h.validateLAPIURL = func(raw string) (*url.URL, error) { return url.Parse(raw) }

	r := gin.New()
	g := r.Group("/api/v1")
	h.RegisterRoutes(g)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/crowdsec/alerts?range=24h", http.NoBody)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	// Falls back to cscli
	assert.Equal(t, "cscli", body["source"])
}

func TestListAlerts_LAPIBadStatus(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	t.Cleanup(server.Close)

	db := OpenTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.SecurityDecision{}, &models.SecurityConfig{}, &models.Setting{}))
	require.NoError(t, db.Create(&models.SecurityConfig{
		UUID: "default", Name: "default", CrowdSecAPIURL: server.URL,
	}).Error)

	h := newTestCrowdsecHandler(t, db, &fakeExec{}, "/bin/false", t.TempDir())
	h.validateLAPIURL = func(raw string) (*url.URL, error) { return url.Parse(raw) }

	r := gin.New()
	g := r.Group("/api/v1")
	h.RegisterRoutes(g)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/crowdsec/alerts?range=24h", http.NoBody)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, "cscli", body["source"])
}

func TestListAlerts_LAPIWithScenarioFilter(t *testing.T) {
	t.Parallel()

	var capturedQuery string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"id":"a1"}]`))
	}))
	t.Cleanup(server.Close)

	db := OpenTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.SecurityDecision{}, &models.SecurityConfig{}, &models.Setting{}))
	require.NoError(t, db.Create(&models.SecurityConfig{
		UUID: "default", Name: "default", CrowdSecAPIURL: server.URL,
	}).Error)

	h := newTestCrowdsecHandler(t, db, &fakeExec{}, "/bin/false", t.TempDir())
	h.validateLAPIURL = func(raw string) (*url.URL, error) { return url.Parse(raw) }

	r := gin.New()
	g := r.Group("/api/v1")
	h.RegisterRoutes(g)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/crowdsec/alerts?range=24h&scenario=crowdsecurity/http-probing", http.NoBody)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, capturedQuery, "scenario=crowdsecurity")
}

func TestFetchAlertsCscli_ErrorExec(t *testing.T) {
	t.Parallel()

	db := OpenTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.SecurityDecision{}, &models.SecurityConfig{}, &models.Setting{}))

	h := &CrowdsecHandler{
		DB:        db,
		Executor:  &fakeExec{},
		CmdExec:   &mockCmdExecutor{output: nil, err: fmt.Errorf("cscli not found")},
		BinPath:   "/bin/false",
		DataDir:   t.TempDir(),
		dashCache: newDashboardCache(),
	}

	r := gin.New()
	g := r.Group("/api/v1")
	h.RegisterRoutes(g)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/crowdsec/alerts?range=24h", http.NoBody)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, "cscli", body["source"])
	assert.Equal(t, float64(0), body["total"])
}

func TestFetchAlertsCscli_ValidJSON(t *testing.T) {
	t.Parallel()

	db := OpenTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.SecurityDecision{}, &models.SecurityConfig{}, &models.Setting{}))

	h := &CrowdsecHandler{
		DB:        db,
		Executor:  &fakeExec{},
		CmdExec:   &mockCmdExecutor{output: []byte(`[{"id":"1"},{"id":"2"}]`), err: nil},
		BinPath:   "/bin/false",
		DataDir:   t.TempDir(),
		dashCache: newDashboardCache(),
	}

	r := gin.New()
	g := r.Group("/api/v1")
	h.RegisterRoutes(g)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/crowdsec/alerts?range=24h", http.NoBody)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, "cscli", body["source"])
	assert.Equal(t, float64(2), body["total"])
}
