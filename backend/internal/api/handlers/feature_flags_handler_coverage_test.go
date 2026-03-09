package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/models"
)

func TestFeatureFlagsHandler_GetFlags_DBPrecedence(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupFlagsDB(t)

	// Set a flag in DB
	db.Create(&models.Setting{
		Key:      "feature.cerberus.enabled",
		Value:    "false",
		Type:     "bool",
		Category: "feature",
	})

	// Set env var that should be ignored (DB takes precedence)
	t.Setenv("FEATURE_CERBERUS_ENABLED", "true")

	h := NewFeatureFlagsHandler(db)
	r := gin.New()
	r.GET("/api/v1/feature-flags", h.GetFlags)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/feature-flags", http.NoBody)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var flags map[string]bool
	err := json.Unmarshal(w.Body.Bytes(), &flags)
	require.NoError(t, err)

	// DB value (false) should take precedence over env (true)
	assert.False(t, flags["feature.cerberus.enabled"])
}

func TestFeatureFlagsHandler_GetFlags_EnvFallback(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupFlagsDB(t)

	// Set env var (no DB value exists)
	t.Setenv("FEATURE_CERBERUS_ENABLED", "false")

	h := NewFeatureFlagsHandler(db)
	r := gin.New()
	r.GET("/api/v1/feature-flags", h.GetFlags)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/feature-flags", http.NoBody)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var flags map[string]bool
	err := json.Unmarshal(w.Body.Bytes(), &flags)
	require.NoError(t, err)

	// Env value should be used
	assert.False(t, flags["feature.cerberus.enabled"])
}

func TestFeatureFlagsHandler_GetFlags_EnvShortForm(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupFlagsDB(t)

	// Set short form env var (CERBERUS_ENABLED instead of FEATURE_CERBERUS_ENABLED)
	t.Setenv("CERBERUS_ENABLED", "false")

	h := NewFeatureFlagsHandler(db)
	r := gin.New()
	r.GET("/api/v1/feature-flags", h.GetFlags)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/feature-flags", http.NoBody)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var flags map[string]bool
	err := json.Unmarshal(w.Body.Bytes(), &flags)
	require.NoError(t, err)

	// Short form env value should be used
	assert.False(t, flags["feature.cerberus.enabled"])
}

func TestFeatureFlagsHandler_GetFlags_EnvNumeric(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupFlagsDB(t)

	// Set numeric env var (1/0 instead of true/false)
	t.Setenv("FEATURE_UPTIME_ENABLED", "0")

	h := NewFeatureFlagsHandler(db)
	r := gin.New()
	r.GET("/api/v1/feature-flags", h.GetFlags)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/feature-flags", http.NoBody)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var flags map[string]bool
	err := json.Unmarshal(w.Body.Bytes(), &flags)
	require.NoError(t, err)

	// "0" should be parsed as false
	assert.False(t, flags["feature.uptime.enabled"])
}

func TestFeatureFlagsHandler_GetFlags_DefaultTrue(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupFlagsDB(t)

	// No DB value, no env var - check defaults
	h := NewFeatureFlagsHandler(db)
	r := gin.New()
	r.GET("/api/v1/feature-flags", h.GetFlags)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/feature-flags", http.NoBody)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var flags map[string]bool
	err := json.Unmarshal(w.Body.Bytes(), &flags)
	require.NoError(t, err)

	// Cerberus defaults to false (OFF by default per diagnostic fix)
	assert.False(t, flags["feature.cerberus.enabled"])
	// Uptime defaults to true (no explicit default set)
	assert.True(t, flags["feature.uptime.enabled"])
}

func TestFeatureFlagsHandler_GetFlags_AllDefaultFlagsPresent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupFlagsDB(t)

	h := NewFeatureFlagsHandler(db)
	r := gin.New()
	r.GET("/api/v1/feature-flags", h.GetFlags)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/feature-flags", http.NoBody)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var flags map[string]bool
	err := json.Unmarshal(w.Body.Bytes(), &flags)
	require.NoError(t, err)

	// Ensure all default flags are present
	for _, key := range defaultFlags {
		_, ok := flags[key]
		assert.True(t, ok, "expected flag %s to be present", key)
	}
}

func TestFeatureFlagsHandler_UpdateFlags_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupFlagsDB(t)

	h := NewFeatureFlagsHandler(db)
	r := gin.New()
	r.PUT("/api/v1/feature-flags", h.UpdateFlags)

	payload := map[string]bool{
		"feature.cerberus.enabled": false,
		"feature.uptime.enabled":   true,
	}
	b, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/feature-flags", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	// Verify DB persistence
	var s1 models.Setting
	err := db.Where("key = ?", "feature.cerberus.enabled").First(&s1).Error
	require.NoError(t, err)
	assert.Equal(t, "false", s1.Value)
	assert.Equal(t, "bool", s1.Type)
	assert.Equal(t, "feature", s1.Category)

	var s2 models.Setting
	err = db.Where("key = ?", "feature.uptime.enabled").First(&s2).Error
	require.NoError(t, err)
	assert.Equal(t, "true", s2.Value)
}

func TestFeatureFlagsHandler_UpdateFlags_Upsert(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupFlagsDB(t)

	// Create existing setting
	db.Create(&models.Setting{
		Key:      "feature.cerberus.enabled",
		Value:    "true",
		Type:     "bool",
		Category: "feature",
	})

	h := NewFeatureFlagsHandler(db)
	r := gin.New()
	r.PUT("/api/v1/feature-flags", h.UpdateFlags)

	// Update existing setting
	payload := map[string]bool{
		"feature.cerberus.enabled": false,
	}
	b, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/feature-flags", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	// Verify update
	var s models.Setting
	err := db.Where("key = ?", "feature.cerberus.enabled").First(&s).Error
	require.NoError(t, err)
	assert.Equal(t, "false", s.Value)

	// Verify only one record exists
	var count int64
	db.Model(&models.Setting{}).Where("key = ?", "feature.cerberus.enabled").Count(&count)
	assert.Equal(t, int64(1), count)
}

func TestFeatureFlagsHandler_UpdateFlags_InvalidJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupFlagsDB(t)

	h := NewFeatureFlagsHandler(db)
	r := gin.New()
	r.PUT("/api/v1/feature-flags", h.UpdateFlags)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/feature-flags", bytes.NewReader([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestFeatureFlagsHandler_UpdateFlags_OnlyAllowedKeys(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupFlagsDB(t)

	h := NewFeatureFlagsHandler(db)
	r := gin.New()
	r.PUT("/api/v1/feature-flags", h.UpdateFlags)

	// Try to set a key not in defaultFlags
	payload := map[string]bool{
		"feature.cerberus.enabled": false,
		"feature.invalid.key":      true, // Should be ignored
	}
	b, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/feature-flags", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	// Verify allowed key was saved
	var s1 models.Setting
	err := db.Where("key = ?", "feature.cerberus.enabled").First(&s1).Error
	require.NoError(t, err)

	// Verify disallowed key was NOT saved
	var s2 models.Setting
	err = db.Where("key = ?", "feature.invalid.key").First(&s2).Error
	assert.Error(t, err)
}

func TestFeatureFlagsHandler_UpdateFlags_EmptyPayload(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupFlagsDB(t)

	h := NewFeatureFlagsHandler(db)
	r := gin.New()
	r.PUT("/api/v1/feature-flags", h.UpdateFlags)

	payload := map[string]bool{}
	b, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/feature-flags", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestFeatureFlagsHandler_GetFlags_DBValueVariants(t *testing.T) {
	tests := []struct {
		name     string
		dbValue  string
		expected bool
	}{
		{"lowercase true", "true", true},
		{"uppercase TRUE", "TRUE", true},
		{"mixed case True", "True", true},
		{"numeric 1", "1", true},
		{"yes", "yes", true},
		{"YES uppercase", "YES", true},
		{"lowercase false", "false", false},
		{"numeric 0", "0", false},
		{"no", "no", false},
		{"empty string", "", false},
		{"random string", "random", false},
		{"whitespace padded true", "  true  ", true},
		{"whitespace padded false", "  false  ", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			db := setupFlagsDB(t)

			// Set flag with test value
			db.Create(&models.Setting{
				Key:      "feature.cerberus.enabled",
				Value:    tt.dbValue,
				Type:     "bool",
				Category: "feature",
			})

			h := NewFeatureFlagsHandler(db)
			r := gin.New()
			r.GET("/api/v1/feature-flags", h.GetFlags)

			req := httptest.NewRequest(http.MethodGet, "/api/v1/feature-flags", http.NoBody)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			require.Equal(t, http.StatusOK, w.Code)

			var flags map[string]bool
			err := json.Unmarshal(w.Body.Bytes(), &flags)
			require.NoError(t, err)

			assert.Equal(t, tt.expected, flags["feature.cerberus.enabled"],
				"dbValue=%q should result in %v", tt.dbValue, tt.expected)
		})
	}
}

func TestFeatureFlagsHandler_GetFlags_EnvValueVariants(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		expected bool
	}{
		{"true string", "true", true},
		{"TRUE uppercase", "TRUE", true},
		{"1 numeric", "1", true},
		{"false string", "false", false},
		{"FALSE uppercase", "FALSE", false},
		{"0 numeric", "0", false},
		{"invalid value defaults to numeric check", "invalid", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			db := setupFlagsDB(t)

			// Set env var (no DB value)
			t.Setenv("FEATURE_CERBERUS_ENABLED", tt.envValue)

			h := NewFeatureFlagsHandler(db)
			r := gin.New()
			r.GET("/api/v1/feature-flags", h.GetFlags)

			req := httptest.NewRequest(http.MethodGet, "/api/v1/feature-flags", http.NoBody)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			require.Equal(t, http.StatusOK, w.Code)

			var flags map[string]bool
			err := json.Unmarshal(w.Body.Bytes(), &flags)
			require.NoError(t, err)

			assert.Equal(t, tt.expected, flags["feature.cerberus.enabled"],
				"envValue=%q should result in %v", tt.envValue, tt.expected)
		})
	}
}

func TestFeatureFlagsHandler_UpdateFlags_BoolValues(t *testing.T) {
	tests := []struct {
		name     string
		value    bool
		dbExpect string
	}{
		{"true", true, "true"},
		{"false", false, "false"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			db := setupFlagsDB(t)

			h := NewFeatureFlagsHandler(db)
			r := gin.New()
			r.PUT("/api/v1/feature-flags", h.UpdateFlags)

			payload := map[string]bool{
				"feature.cerberus.enabled": tt.value,
			}
			b, _ := json.Marshal(payload)

			req := httptest.NewRequest(http.MethodPut, "/api/v1/feature-flags", bytes.NewReader(b))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			require.Equal(t, http.StatusOK, w.Code)

			var s models.Setting
			err := db.Where("key = ?", "feature.cerberus.enabled").First(&s).Error
			require.NoError(t, err)
			assert.Equal(t, tt.dbExpect, s.Value)
		})
	}
}

func TestFeatureFlagsHandler_NewFeatureFlagsHandler(t *testing.T) {
	db := setupFlagsDB(t)
	h := NewFeatureFlagsHandler(db)

	assert.NotNil(t, h)
	assert.NotNil(t, h.DB)
	assert.Equal(t, db, h.DB)
}

func TestFeatureFlagsHandler_GetFlags_EmailFlagDefaultFalse(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupFlagsDB(t)

	h := NewFeatureFlagsHandler(db)
	r := gin.New()
	r.GET("/api/v1/feature-flags", h.GetFlags)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/feature-flags", http.NoBody)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var flags map[string]bool
	err := json.Unmarshal(w.Body.Bytes(), &flags)
	require.NoError(t, err)

	assert.False(t, flags["feature.notifications.service.email.enabled"])
}
