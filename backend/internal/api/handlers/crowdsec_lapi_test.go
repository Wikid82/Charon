package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestGetLAPIDecisions_FallbackToCscli(t *testing.T) {
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
