package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/config"
)

func TestSecurityHandler_GetRateLimitPresets(t *testing.T) {

	cfg := config.SecurityConfig{}
	handler := NewSecurityHandler(cfg, nil, nil)
	router := gin.New()
	router.GET("/security/rate-limit/presets", handler.GetRateLimitPresets)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/security/rate-limit/presets", http.NoBody)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]any
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	presets, ok := response["presets"].([]any)
	require.True(t, ok, "presets should be an array")
	require.Len(t, presets, 4, "should have 4 presets")

	// Verify preset structure
	expectedIDs := []string{"standard", "api", "login", "relaxed"}
	for i, p := range presets {
		preset := p.(map[string]any)
		assert.Equal(t, expectedIDs[i], preset["id"])
		assert.NotEmpty(t, preset["name"])
		assert.NotEmpty(t, preset["description"])
		assert.NotNil(t, preset["requests"])
		assert.NotNil(t, preset["window_sec"])
		assert.NotNil(t, preset["burst"])
	}
}

func TestSecurityHandler_GetRateLimitPresets_StandardPreset(t *testing.T) {

	cfg := config.SecurityConfig{}
	handler := NewSecurityHandler(cfg, nil, nil)
	router := gin.New()
	router.GET("/security/rate-limit/presets", handler.GetRateLimitPresets)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/security/rate-limit/presets", http.NoBody)
	router.ServeHTTP(w, req)

	var response map[string]any
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	presets := response["presets"].([]any)
	standardPreset := presets[0].(map[string]any)

	assert.Equal(t, "standard", standardPreset["id"])
	assert.Equal(t, "Standard Web", standardPreset["name"])
	assert.Equal(t, float64(100), standardPreset["requests"])
	assert.Equal(t, float64(60), standardPreset["window_sec"])
	assert.Equal(t, float64(20), standardPreset["burst"])
}

func TestSecurityHandler_GetRateLimitPresets_LoginPreset(t *testing.T) {

	cfg := config.SecurityConfig{}
	handler := NewSecurityHandler(cfg, nil, nil)
	router := gin.New()
	router.GET("/security/rate-limit/presets", handler.GetRateLimitPresets)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/security/rate-limit/presets", http.NoBody)
	router.ServeHTTP(w, req)

	var response map[string]any
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	presets := response["presets"].([]any)
	loginPreset := presets[2].(map[string]any)

	assert.Equal(t, "login", loginPreset["id"])
	assert.Equal(t, "Login Protection", loginPreset["name"])
	assert.Equal(t, float64(5), loginPreset["requests"])
	assert.Equal(t, float64(300), loginPreset["window_sec"])
	assert.Equal(t, float64(2), loginPreset["burst"])
}
