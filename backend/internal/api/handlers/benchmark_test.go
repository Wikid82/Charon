package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/Wikid82/charon/backend/internal/models"
)

// setupBenchmarkDB creates an in-memory SQLite database for benchmarks
func setupBenchmarkDB(b *testing.B) *gorm.DB {
	b.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		b.Fatal(err)
	}
	if err := db.AutoMigrate(
		&models.SecurityConfig{},
		&models.SecurityRuleSet{},
		&models.SecurityDecision{},
		&models.SecurityAudit{},
		&models.Setting{},
		&models.ProxyHost{},
		&models.AccessList{},
		&models.User{},
	); err != nil {
		b.Fatal(err)
	}
	return db
}

// =============================================================================
// SECURITY HANDLER BENCHMARKS
// =============================================================================

func BenchmarkSecurityHandler_GetStatus(b *testing.B) {
	gin.SetMode(gin.ReleaseMode)
	db := setupBenchmarkDB(b)

	// Seed settings
	settings := []models.Setting{
		{Key: "security.cerberus.enabled", Value: "true", Category: "security"},
		{Key: "security.waf.enabled", Value: "true", Category: "security"},
		{Key: "security.rate_limit.enabled", Value: "true", Category: "security"},
		{Key: "security.crowdsec.enabled", Value: "true", Category: "security"},
		{Key: "security.acl.enabled", Value: "true", Category: "security"},
	}
	for _, s := range settings {
		db.Create(&s)
	}

	cfg := config.SecurityConfig{CerberusEnabled: true}
	h := NewSecurityHandler(cfg, db, nil)

	router := gin.New()
	router.GET("/api/v1/security/status", h.GetStatus)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("GET", "/api/v1/security/status", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			b.Fatalf("unexpected status: %d", w.Code)
		}
	}
}

func BenchmarkSecurityHandler_GetStatus_NoSettings(b *testing.B) {
	gin.SetMode(gin.ReleaseMode)
	db := setupBenchmarkDB(b)

	cfg := config.SecurityConfig{CerberusEnabled: true}
	h := NewSecurityHandler(cfg, db, nil)

	router := gin.New()
	router.GET("/api/v1/security/status", h.GetStatus)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("GET", "/api/v1/security/status", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			b.Fatalf("unexpected status: %d", w.Code)
		}
	}
}

func BenchmarkSecurityHandler_ListDecisions(b *testing.B) {
	gin.SetMode(gin.ReleaseMode)
	db := setupBenchmarkDB(b)

	// Seed some decisions
	for i := 0; i < 100; i++ {
		db.Create(&models.SecurityDecision{
			UUID:   "test-uuid-" + string(rune(i)),
			Source: "test",
			Action: "block",
			IP:     "192.168.1.1",
		})
	}

	cfg := config.SecurityConfig{}
	h := NewSecurityHandler(cfg, db, nil)

	router := gin.New()
	router.GET("/api/v1/security/decisions", h.ListDecisions)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("GET", "/api/v1/security/decisions?limit=50", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			b.Fatalf("unexpected status: %d", w.Code)
		}
	}
}

func BenchmarkSecurityHandler_ListRuleSets(b *testing.B) {
	gin.SetMode(gin.ReleaseMode)
	db := setupBenchmarkDB(b)

	// Seed some rulesets
	for i := 0; i < 10; i++ {
		db.Create(&models.SecurityRuleSet{
			UUID:    "ruleset-uuid-" + string(rune(i)),
			Name:    "Ruleset " + string(rune('A'+i)),
			Content: "SecRule REQUEST_URI \"@contains /admin\" \"id:1000,phase:1,deny\"",
			Mode:    "blocking",
		})
	}

	cfg := config.SecurityConfig{}
	h := NewSecurityHandler(cfg, db, nil)

	router := gin.New()
	router.GET("/api/v1/security/rulesets", h.ListRuleSets)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("GET", "/api/v1/security/rulesets", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			b.Fatalf("unexpected status: %d", w.Code)
		}
	}
}

func BenchmarkSecurityHandler_UpsertRuleSet(b *testing.B) {
	gin.SetMode(gin.ReleaseMode)
	db := setupBenchmarkDB(b)

	cfg := config.SecurityConfig{}
	h := NewSecurityHandler(cfg, db, nil)

	router := gin.New()
	router.POST("/api/v1/security/rulesets", h.UpsertRuleSet)

	payload := map[string]interface{}{
		"name":    "bench-ruleset",
		"content": "SecRule REQUEST_URI \"@contains /admin\" \"id:1000,phase:1,deny\"",
		"mode":    "blocking",
	}
	body, _ := json.Marshal(payload)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("POST", "/api/v1/security/rulesets", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			b.Fatalf("unexpected status: %d", w.Code)
		}
	}
}

func BenchmarkSecurityHandler_CreateDecision(b *testing.B) {
	gin.SetMode(gin.ReleaseMode)
	db := setupBenchmarkDB(b)

	cfg := config.SecurityConfig{}
	h := NewSecurityHandler(cfg, db, nil)

	router := gin.New()
	router.POST("/api/v1/security/decisions", h.CreateDecision)

	payload := map[string]interface{}{
		"ip":      "192.168.1.100",
		"action":  "block",
		"details": "benchmark test",
	}
	body, _ := json.Marshal(payload)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("POST", "/api/v1/security/decisions", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			b.Fatalf("unexpected status: %d", w.Code)
		}
	}
}

func BenchmarkSecurityHandler_GetConfig(b *testing.B) {
	gin.SetMode(gin.ReleaseMode)
	db := setupBenchmarkDB(b)

	// Seed a config
	db.Create(&models.SecurityConfig{
		Name:            "default",
		Enabled:         true,
		AdminWhitelist:  "192.168.1.0/24",
		WAFMode:         "block",
		RateLimitEnable: true,
		RateLimitBurst:  10,
	})

	cfg := config.SecurityConfig{}
	h := NewSecurityHandler(cfg, db, nil)

	router := gin.New()
	router.GET("/api/v1/security/config", h.GetConfig)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("GET", "/api/v1/security/config", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			b.Fatalf("unexpected status: %d", w.Code)
		}
	}
}

func BenchmarkSecurityHandler_UpdateConfig(b *testing.B) {
	gin.SetMode(gin.ReleaseMode)
	db := setupBenchmarkDB(b)

	cfg := config.SecurityConfig{}
	h := NewSecurityHandler(cfg, db, nil)

	router := gin.New()
	router.PUT("/api/v1/security/config", h.UpdateConfig)

	payload := map[string]interface{}{
		"name":                "default",
		"enabled":             true,
		"rate_limit_enable":   true,
		"rate_limit_burst":    10,
		"rate_limit_requests": 100,
	}
	body, _ := json.Marshal(payload)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("PUT", "/api/v1/security/config", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			b.Fatalf("unexpected status: %d", w.Code)
		}
	}
}

// =============================================================================
// PARALLEL BENCHMARKS (Concurrency Testing)
// =============================================================================

func BenchmarkSecurityHandler_GetStatus_Parallel(b *testing.B) {
	gin.SetMode(gin.ReleaseMode)
	db := setupBenchmarkDB(b)

	settings := []models.Setting{
		{Key: "security.cerberus.enabled", Value: "true", Category: "security"},
		{Key: "security.waf.enabled", Value: "true", Category: "security"},
	}
	for _, s := range settings {
		db.Create(&s)
	}

	cfg := config.SecurityConfig{CerberusEnabled: true}
	h := NewSecurityHandler(cfg, db, nil)

	router := gin.New()
	router.GET("/api/v1/security/status", h.GetStatus)

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			req := httptest.NewRequest("GET", "/api/v1/security/status", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			if w.Code != http.StatusOK {
				b.Fatalf("unexpected status: %d", w.Code)
			}
		}
	})
}

func BenchmarkSecurityHandler_ListDecisions_Parallel(b *testing.B) {
	gin.SetMode(gin.ReleaseMode)
	// Use file-based SQLite with WAL mode for parallel testing
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared&_journal_mode=WAL"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		b.Fatal(err)
	}
	if err := db.AutoMigrate(&models.SecurityDecision{}, &models.SecurityAudit{}); err != nil {
		b.Fatal(err)
	}

	for i := 0; i < 100; i++ {
		db.Create(&models.SecurityDecision{
			UUID:   "test-uuid-" + string(rune(i)),
			Source: "test",
			Action: "block",
			IP:     "192.168.1.1",
		})
	}

	cfg := config.SecurityConfig{}
	h := NewSecurityHandler(cfg, db, nil)

	router := gin.New()
	router.GET("/api/v1/security/decisions", h.ListDecisions)

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			req := httptest.NewRequest("GET", "/api/v1/security/decisions?limit=50", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			if w.Code != http.StatusOK {
				b.Fatalf("unexpected status: %d", w.Code)
			}
		}
	})
}

// =============================================================================
// MEMORY PRESSURE BENCHMARKS
// =============================================================================

func BenchmarkSecurityHandler_LargeRuleSetContent(b *testing.B) {
	gin.SetMode(gin.ReleaseMode)
	db := setupBenchmarkDB(b)

	cfg := config.SecurityConfig{}
	h := NewSecurityHandler(cfg, db, nil)

	router := gin.New()
	router.POST("/api/v1/security/rulesets", h.UpsertRuleSet)

	// 100KB ruleset content (under 2MB limit)
	largeContent := ""
	for i := 0; i < 1000; i++ {
		largeContent += "SecRule REQUEST_URI \"@contains /path" + string(rune(i)) + "\" \"id:" + string(rune(1000+i)) + ",phase:1,deny\"\n"
	}

	payload := map[string]interface{}{
		"name":    "large-ruleset",
		"content": largeContent,
		"mode":    "blocking",
	}
	body, _ := json.Marshal(payload)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("POST", "/api/v1/security/rulesets", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			b.Fatalf("unexpected status: %d", w.Code)
		}
	}
}

func BenchmarkSecurityHandler_ManySettingsLookups(b *testing.B) {
	gin.SetMode(gin.ReleaseMode)
	db := setupBenchmarkDB(b)

	// Seed many settings
	for i := 0; i < 100; i++ {
		db.Create(&models.Setting{
			Key:      "setting.key." + string(rune(i)),
			Value:    "value",
			Category: "misc",
		})
	}
	// Security settings
	settings := []models.Setting{
		{Key: "security.cerberus.enabled", Value: "true", Category: "security"},
		{Key: "security.waf.enabled", Value: "true", Category: "security"},
		{Key: "security.rate_limit.enabled", Value: "true", Category: "security"},
		{Key: "security.crowdsec.enabled", Value: "true", Category: "security"},
		{Key: "security.crowdsec.mode", Value: "local", Category: "security"},
		{Key: "security.crowdsec.api_url", Value: "http://localhost:8080", Category: "security"},
		{Key: "security.acl.enabled", Value: "true", Category: "security"},
	}
	for _, s := range settings {
		db.Create(&s)
	}

	cfg := config.SecurityConfig{}
	h := NewSecurityHandler(cfg, db, nil)

	router := gin.New()
	router.GET("/api/v1/security/status", h.GetStatus)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("GET", "/api/v1/security/status", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			b.Fatalf("unexpected status: %d", w.Code)
		}
	}
}
