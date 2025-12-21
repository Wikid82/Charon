package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestGetLAPIDecisions_FallbackToCscli(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	// Create handler with mock executor
	handler := &CrowdsecHandler{
		CmdExec: &mockCommandExecutor{output: []byte(`[]`), err: nil},
		DataDir: t.TempDir(),
	}

	router.GET("/admin/crowdsec/decisions/lapi", handler.GetLAPIDecisions)

	// This test will fallback to cscli since localhost:8080 LAPI is not running
	req := httptest.NewRequest(http.MethodGet, "/admin/crowdsec/decisions/lapi", http.NoBody)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Should return success (from cscli fallback)
	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]any
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	// Should have decisions array (empty from mock)
	_, hasDecisions := response["decisions"]
	assert.True(t, hasDecisions)
}

func TestGetLAPIDecisions_EmptyResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	// Create handler with mock executor that returns empty array
	handler := &CrowdsecHandler{
		CmdExec: &mockCommandExecutor{output: []byte(`[]`), err: nil},
		DataDir: t.TempDir(),
	}

	router.GET("/admin/crowdsec/decisions/lapi", handler.GetLAPIDecisions)

	req := httptest.NewRequest(http.MethodGet, "/admin/crowdsec/decisions/lapi", http.NoBody)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Will fallback to cscli which returns empty
	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]any
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	// Should have decisions array (may be empty)
	_, hasDecisions := response["decisions"]
	assert.True(t, hasDecisions)
}

func TestCheckLAPIHealth_Handler(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	handler := &CrowdsecHandler{
		CmdExec: &mockCommandExecutor{output: []byte(`[]`), err: nil},
		DataDir: t.TempDir(),
	}

	router.GET("/admin/crowdsec/lapi/health", handler.CheckLAPIHealth)

	req := httptest.NewRequest(http.MethodGet, "/admin/crowdsec/lapi/health", http.NoBody)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]any
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)

	// Should have healthy field
	_, hasHealthy := response["healthy"]
	assert.True(t, hasHealthy)

	// Should have lapi_url field
	_, hasURL := response["lapi_url"]
	assert.True(t, hasURL)
}

func TestGetLAPIKey_FromEnv(t *testing.T) {
	// Save and restore original env
	original := os.Getenv("CROWDSEC_API_KEY")
	defer func() {
		if original != "" {
			_ = os.Setenv("CROWDSEC_API_KEY", original)
		} else {
			_ = os.Unsetenv("CROWDSEC_API_KEY")
		}
	}()

	// Set test value
	_ = os.Setenv("CROWDSEC_API_KEY", "test-key-123")

	key := getLAPIKey()
	assert.Equal(t, "test-key-123", key)
}

func TestGetLAPIKey_Empty(t *testing.T) {
	// Save and restore original env vars
	envVars := []string{
		"CROWDSEC_API_KEY",
		"CROWDSEC_BOUNCER_API_KEY",
		"CERBERUS_SECURITY_CROWDSEC_API_KEY",
		"CHARON_SECURITY_CROWDSEC_API_KEY",
		"CPM_SECURITY_CROWDSEC_API_KEY",
	}

	originals := make(map[string]string)
	for _, key := range envVars {
		originals[key] = os.Getenv(key)
		_ = os.Unsetenv(key)
	}
	defer func() {
		for key, val := range originals {
			if val != "" {
				_ = os.Setenv(key, val)
			}
		}
	}()

	key := getLAPIKey()
	assert.Empty(t, key)
}
