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

	"github.com/Wikid82/charon/backend/internal/models"
)

func setupFlagsDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open in-memory sqlite: %v", err)
	}
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
	req := httptest.NewRequest(http.MethodGet, "/api/v1/feature-flags", nil)
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
