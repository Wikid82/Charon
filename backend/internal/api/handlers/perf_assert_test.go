package handlers

import (
	"net/http"
	"net/http/httptest"
	"os"
	"sort"
	"testing"
	"time"
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/Wikid82/charon/backend/internal/models"
)

// quick helper to form float ms from duration
func ms(d time.Duration) float64 { return float64(d.Microseconds()) / 1000.0 }

// setupPerfDB - uses a file-backed sqlite to avoid concurrency panics in parallel tests
func setupPerfDB(t *testing.T) *gorm.DB {
	t.Helper()
	path := ":memory:?cache=shared&_journal_mode=WAL"
	db, err := gorm.Open(sqlite.Open(path), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.Setting{}, &models.SecurityDecision{}, &models.SecurityRuleSet{}, &models.SecurityConfig{}))
	return db
}

// thresholdFromEnv loads threshold from environment var as milliseconds
func thresholdFromEnv(envKey string, defaultMs float64) float64 {
	if v := os.Getenv(envKey); v != "" {
		// try parse as float
		if parsed, err := time.ParseDuration(v); err == nil {
			return ms(parsed)
		}
		// fallback try parse as number ms
		var f float64
		if _, err := fmt.Sscanf(v, "%f", &f); err == nil {
			return f
		}
	}
	return defaultMs
}

// gatherStats runs the request counts times and returns durations ms
func gatherStats(t *testing.T, req *http.Request, router http.Handler, counts int) []float64 {
	t.Helper()
	res := make([]float64, 0, counts)
	for i := 0; i < counts; i++ {
		w := httptest.NewRecorder()
		s := time.Now()
		router.ServeHTTP(w, req)
		d := time.Since(s)
		res = append(res, ms(d))
		if w.Code >= 500 {
			t.Fatalf("unexpected status: %d", w.Code)
		}
	}
	return res
}

// computePercentiles returns avg, p50, p95, p99, max
func computePercentiles(samples []float64) (avg, p50, p95, p99, max float64) {
	sort.Float64s(samples)
	var sum float64
	for _, s := range samples {
		sum += s
	}
	avg = sum / float64(len(samples))
	p := func(pct float64) float64 {
		idx := int(float64(len(samples))*pct)
		if idx < 0 { idx = 0 }
		if idx >= len(samples) { idx = len(samples)-1 }
		return samples[idx]
	}
	p50 = p(0.50)
	p95 = p(0.95)
	p99 = p(0.99)
	max = samples[len(samples)-1]
	return
}

func perfLogStats(t *testing.T, title string, samples []float64) {
	av, p50, p95, p99, max := computePercentiles(samples)
	t.Logf("%s - avg=%.3fms p50=%.3fms p95=%.3fms p99=%.3fms max=%.3fms", title, av, p50, p95, p99, max)
	// no assert by default, individual tests decide how to fail
}

func TestPerf_GetStatus_AssertThreshold(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	db := setupPerfDB(t)

	// seed settings to emulate production path
	_ = db.Create(&models.Setting{Key: "security.cerberus.enabled", Value: "true", Category: "security"})
	_ = db.Create(&models.Setting{Key: "security.waf.enabled", Value: "true", Category: "security"})
	cfg := config.SecurityConfig{CerberusEnabled: true}
	h := NewSecurityHandler(cfg, db, nil)

	router := gin.New()
	router.GET("/api/v1/security/status", h.GetStatus)

	counts := 500
	req := httptest.NewRequest("GET", "/api/v1/security/status", nil)
	samples := gatherStats(t, req, router, counts)
	avg, _, p95, _, max := computePercentiles(samples)
	// default thresholds ms
	thresholdP95 := 2.0 // 2ms per request
	if env := os.Getenv("PERF_MAX_MS_GETSTATUS_P95"); env != "" {
		if parsed, err := time.ParseDuration(env); err == nil { thresholdP95 = ms(parsed) }
	}
	// fail if p95 exceeds threshold
	t.Logf("GetStatus avg=%.3fms p95=%.3fms max=%.3fms", avg, p95, max)
	if p95 > thresholdP95 {
		t.Fatalf("GetStatus P95 (%.3fms) exceeds threshold %.3fms", p95, thresholdP95)
	}
}

func TestPerf_GetStatus_Parallel_AssertThreshold(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	db := setupPerfDB(t)
	cfg := config.SecurityConfig{CerberusEnabled: true}
	h := NewSecurityHandler(cfg, db, nil)

	router := gin.New()
	router.GET("/api/v1/security/status", h.GetStatus)

	n := 200
	samples := make(chan float64, n)
	var worker = func() {
		for i := 0; i < n; i++ {
			req := httptest.NewRequest("GET", "/api/v1/security/status", nil)
			w := httptest.NewRecorder()
			s := time.Now()
			router.ServeHTTP(w, req)
			d := time.Since(s)
			samples <- ms(d)
		}
	}

	// run 4 concurrent workers
	for k := 0; k < 4; k++ { go worker() }
	collected := make([]float64, 0, n*4)
	for i := 0; i < n*4; i++ { collected = append(collected, <-samples) }
	avg, _, p95, _, max := computePercentiles(collected)
	thresholdP95 := 5.0 // 5ms default
	if env := os.Getenv("PERF_MAX_MS_GETSTATUS_P95_PARALLEL"); env != "" {
		if parsed, err := time.ParseDuration(env); err == nil { thresholdP95 = ms(parsed) }
	}
	t.Logf("GetStatus Parallel avg=%.3fms p95=%.3fms max=%.3fms", avg, p95, max)
	if p95 > thresholdP95 {
		t.Fatalf("GetStatus Parallel P95 (%.3fms) exceeds threshold %.3fms", p95, thresholdP95)
	}
}

func TestPerf_ListDecisions_AssertThreshold(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	db := setupPerfDB(t)
	// seed decisions
	for i := 0; i < 1000; i++ {
		db.Create(&models.SecurityDecision{UUID: fmt.Sprintf("d-%d", i), Source: "test", Action: "block", IP: "192.168.1.1"})
	}
	cfg := config.SecurityConfig{}
	h := NewSecurityHandler(cfg, db, nil)

	router := gin.New()
	router.GET("/api/v1/security/decisions", h.ListDecisions)

	counts := 200
	req := httptest.NewRequest("GET", "/api/v1/security/decisions?limit=50", nil)
	samples := gatherStats(t, req, router, counts)
	avg, _, p95, _, max := computePercentiles(samples)
	thresholdP95 := 30.0 // 30ms default
	if env := os.Getenv("PERF_MAX_MS_LISTDECISIONS_P95"); env != "" {
		if parsed, err := time.ParseDuration(env); err == nil { thresholdP95 = ms(parsed) }
	}
	t.Logf("ListDecisions avg=%.3fms p95=%.3fms max=%.3fms", avg, p95, max)
	if p95 > thresholdP95 {
		t.Fatalf("ListDecisions P95 (%.3fms) exceeds threshold %.3fms", p95, thresholdP95)
	}
}
