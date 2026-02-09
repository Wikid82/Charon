package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/Wikid82/charon/backend/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSecurityHandler_GetGeoIPStatus_NotInitialized(t *testing.T) {
	gin.SetMode(gin.TestMode)

	h := NewSecurityHandler(config.SecurityConfig{}, nil, nil)
	r := gin.New()
	r.GET("/security/geoip/status", h.GetGeoIPStatus)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/security/geoip/status", http.NoBody)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var body map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, false, body["loaded"])
	assert.Equal(t, "GeoIP service not initialized", body["message"])
}

func TestSecurityHandler_GetGeoIPStatus_Initialized_NotLoaded(t *testing.T) {
	gin.SetMode(gin.TestMode)

	h := NewSecurityHandler(config.SecurityConfig{}, nil, nil)
	h.SetGeoIPService(&services.GeoIPService{})

	r := gin.New()
	r.GET("/security/geoip/status", h.GetGeoIPStatus)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/security/geoip/status", http.NoBody)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var body map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, false, body["loaded"])
	assert.Equal(t, "GeoIP service available", body["message"])
}

func TestSecurityHandler_ReloadGeoIP_NotInitialized(t *testing.T) {
	gin.SetMode(gin.TestMode)

	h := NewSecurityHandler(config.SecurityConfig{}, nil, nil)
	r := gin.New()
	r.POST("/security/geoip/reload", h.ReloadGeoIP)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/security/geoip/reload", http.NoBody)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestSecurityHandler_ReloadGeoIP_LoadError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	h := NewSecurityHandler(config.SecurityConfig{}, nil, nil)
	h.SetGeoIPService(&services.GeoIPService{}) // dbPath empty => Load() will error

	r := gin.New()
	r.POST("/security/geoip/reload", h.ReloadGeoIP)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/security/geoip/reload", http.NoBody)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "Failed to reload GeoIP database")
}

func TestSecurityHandler_LookupGeoIP_MissingIPAddress(t *testing.T) {
	gin.SetMode(gin.TestMode)

	h := NewSecurityHandler(config.SecurityConfig{}, nil, nil)
	r := gin.New()
	r.POST("/security/geoip/lookup", h.LookupGeoIP)

	payload := []byte(`{}`)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/security/geoip/lookup", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "ip_address is required")
}

func TestSecurityHandler_LookupGeoIP_ServiceUnavailable(t *testing.T) {
	gin.SetMode(gin.TestMode)

	h := NewSecurityHandler(config.SecurityConfig{}, nil, nil)
	h.SetGeoIPService(&services.GeoIPService{}) // present but not loaded

	r := gin.New()
	r.POST("/security/geoip/lookup", h.LookupGeoIP)

	payload, _ := json.Marshal(map[string]string{"ip_address": "8.8.8.8"})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/security/geoip/lookup", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	assert.Contains(t, w.Body.String(), "GeoIP service not available")
}
