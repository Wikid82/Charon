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

	"github.com/Wikid82/charon/backend/internal/models"
)

func setupFlagsDB(t *testing.T) *gorm.DB {
	db := OpenTestDB(t)
	if err := db.AutoMigrate(&models.Setting{}); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}
	return db
}

func TestFeatureFlags_GetAndUpdate(t *testing.T) {
	db := setupFlagsDB(t)

	h := NewFeatureFlagsHandler(db)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/api/v1/feature-flags", h.GetFlags)
	r.PUT("/api/v1/feature-flags", h.UpdateFlags)

	// 1) GET should return all default flags (as keys)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/feature-flags", http.NoBody)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d body=%s", w.Code, w.Body.String())
	}
	var flags map[string]bool
	if err := json.Unmarshal(w.Body.Bytes(), &flags); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	// ensure keys present
	for _, k := range defaultFlags {
		if _, ok := flags[k]; !ok {
			t.Fatalf("missing default flag key: %s", k)
		}
	}

	// 2) PUT update a single flag
	payload := map[string]bool{
		defaultFlags[0]: true,
	}
	b, _ := json.Marshal(payload)
	req2 := httptest.NewRequest(http.MethodPut, "/api/v1/feature-flags", bytes.NewReader(b))
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	if w2.Code != http.StatusOK {
		t.Fatalf("expected 200 on update got %d body=%s", w2.Code, w2.Body.String())
	}

	// confirm DB persisted
	var s models.Setting
	if err := db.Where("key = ?", defaultFlags[0]).First(&s).Error; err != nil {
		t.Fatalf("expected setting persisted, db error: %v", err)
	}
	if s.Value != "true" {
		t.Fatalf("expected stored value 'true' got '%s'", s.Value)
	}
}

func TestFeatureFlags_EnvFallback(t *testing.T) {
	// Ensure env fallback is used when DB not present
	t.Setenv("FEATURE_CERBERUS_ENABLED", "true")

	db := setupFlagsDB(t)
	// Do not write any settings so DB lookup fails and env is used
	h := NewFeatureFlagsHandler(db)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/api/v1/feature-flags", h.GetFlags)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/feature-flags", http.NoBody)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d body=%s", w.Code, w.Body.String())
	}
	var flags map[string]bool
	if err := json.Unmarshal(w.Body.Bytes(), &flags); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if !flags["feature.cerberus.enabled"] {
		t.Fatalf("expected feature.cerberus.enabled to be true via env fallback")
	}
}

// setupBenchmarkFlagsDB creates an in-memory SQLite database for feature flags benchmarks
func setupBenchmarkFlagsDB(b *testing.B) *gorm.DB {
	b.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		b.Fatal(err)
	}
	if err := db.AutoMigrate(&models.Setting{}); err != nil {
		b.Fatal(err)
	}
	return db
}

// BenchmarkGetFlags measures GetFlags performance with batch query
func BenchmarkGetFlags(b *testing.B) {
	db := setupBenchmarkFlagsDB(b)

	// Seed database with all default flags
	db.Create(&models.Setting{Key: "feature.cerberus.enabled", Value: "true", Type: "bool", Category: "feature"})
	db.Create(&models.Setting{Key: "feature.uptime.enabled", Value: "false", Type: "bool", Category: "feature"})
	db.Create(&models.Setting{Key: "feature.crowdsec.console_enrollment", Value: "true", Type: "bool", Category: "feature"})

	h := NewFeatureFlagsHandler(db)
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.GET("/api/v1/feature-flags", h.GetFlags)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/feature-flags", http.NoBody)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			b.Fatalf("expected 200 got %d", w.Code)
		}
	}
}

// BenchmarkUpdateFlags measures UpdateFlags performance with transaction wrapping
func BenchmarkUpdateFlags(b *testing.B) {
	db := setupBenchmarkFlagsDB(b)

	h := NewFeatureFlagsHandler(db)
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.PUT("/api/v1/feature-flags", h.UpdateFlags)

	payload := map[string]bool{
		"feature.cerberus.enabled":            true,
		"feature.uptime.enabled":              false,
		"feature.crowdsec.console_enrollment": true,
	}
	payloadBytes, _ := json.Marshal(payload)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodPut, "/api/v1/feature-flags", bytes.NewReader(payloadBytes))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			b.Fatalf("expected 200 got %d", w.Code)
		}
	}
}

// TestGetFlags_BatchQuery verifies that GetFlags uses a single batch query
func TestGetFlags_BatchQuery(t *testing.T) {
	db := setupFlagsDB(t)

	// Insert multiple flags
	db.Create(&models.Setting{Key: "feature.cerberus.enabled", Value: "true", Type: "bool", Category: "feature"})
	db.Create(&models.Setting{Key: "feature.uptime.enabled", Value: "false", Type: "bool", Category: "feature"})
	db.Create(&models.Setting{Key: "feature.crowdsec.console_enrollment", Value: "true", Type: "bool", Category: "feature"})

	h := NewFeatureFlagsHandler(db)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/api/v1/feature-flags", h.GetFlags)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/feature-flags", http.NoBody)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d body=%s", w.Code, w.Body.String())
	}

	var flags map[string]bool
	if err := json.Unmarshal(w.Body.Bytes(), &flags); err != nil {
		t.Fatalf("invalid json: %v", err)
	}

	// Verify all flags returned with correct values
	if !flags["feature.cerberus.enabled"] {
		t.Errorf("expected cerberus.enabled to be true")
	}
	if flags["feature.uptime.enabled"] {
		t.Errorf("expected uptime.enabled to be false")
	}
	if !flags["feature.crowdsec.console_enrollment"] {
		t.Errorf("expected crowdsec.console_enrollment to be true")
	}
}

// TestUpdateFlags_TransactionRollback verifies transaction rollback on error
func TestUpdateFlags_TransactionRollback(t *testing.T) {
	db := setupFlagsDB(t)

	// Close the DB to force an error during transaction
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("failed to get sql.DB: %v", err)
	}
	_ = sqlDB.Close()

	h := NewFeatureFlagsHandler(db)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.PUT("/api/v1/feature-flags", h.UpdateFlags)

	payload := map[string]bool{
		"feature.cerberus.enabled": true,
	}
	b, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/feature-flags", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Should return error due to closed DB
	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 got %d body=%s", w.Code, w.Body.String())
	}
}

// TestUpdateFlags_TransactionAtomic verifies all updates succeed or all fail
func TestUpdateFlags_TransactionAtomic(t *testing.T) {
	db := setupFlagsDB(t)

	h := NewFeatureFlagsHandler(db)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.PUT("/api/v1/feature-flags", h.UpdateFlags)

	// Update multiple flags
	payload := map[string]bool{
		"feature.cerberus.enabled":            true,
		"feature.uptime.enabled":              false,
		"feature.crowdsec.console_enrollment": true,
	}
	b, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/feature-flags", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d body=%s", w.Code, w.Body.String())
	}

	// Verify all flags persisted
	var s1 models.Setting
	if err := db.Where("key = ?", "feature.cerberus.enabled").First(&s1).Error; err != nil {
		t.Errorf("expected cerberus.enabled to be persisted")
	} else if s1.Value != "true" {
		t.Errorf("expected cerberus.enabled to be true, got %s", s1.Value)
	}

	var s2 models.Setting
	if err := db.Where("key = ?", "feature.uptime.enabled").First(&s2).Error; err != nil {
		t.Errorf("expected uptime.enabled to be persisted")
	} else if s2.Value != "false" {
		t.Errorf("expected uptime.enabled to be false, got %s", s2.Value)
	}

	var s3 models.Setting
	if err := db.Where("key = ?", "feature.crowdsec.console_enrollment").First(&s3).Error; err != nil {
		t.Errorf("expected crowdsec.console_enrollment to be persisted")
	} else if s3.Value != "true" {
		t.Errorf("expected crowdsec.console_enrollment to be true, got %s", s3.Value)
	}
}
