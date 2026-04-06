package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// ============================================
// Additional Coverage Tests for Quick Wins
// Target: Boost handlers coverage from 83.1% to 85%+
// ============================================

func TestUpdateAcquisitionConfigMissingContent(t *testing.T) {
	h := newTestCrowdsecHandler(t, OpenTestDB(t), &fakeExec{}, "/bin/false", t.TempDir())
	r := gin.New()
	g := r.Group("/api/v1")
	h.RegisterRoutes(g)

	// Send empty JSON
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/v1/admin/crowdsec/acquisition", strings.NewReader("{}"))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Contains(t, w.Body.String(), "content is required")
}

func TestUpdateAcquisitionConfigInvalidJSON(t *testing.T) {
	h := newTestCrowdsecHandler(t, OpenTestDB(t), &fakeExec{}, "/bin/false", t.TempDir())
	r := gin.New()
	g := r.Group("/api/v1")
	h.RegisterRoutes(g)

	// Send invalid JSON
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/v1/admin/crowdsec/acquisition", strings.NewReader("invalid json"))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGetLAPIDecisionsWithIPFilter(t *testing.T) {
	mockExec := &mockCommandExecutor{output: []byte(`[]`), err: nil}
	h := &CrowdsecHandler{
		CmdExec: mockExec,
		DataDir: t.TempDir(),
	}
	r := gin.New()
	r.GET("/decisions", h.GetLAPIDecisions)

	// Test with IP query parameter
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/decisions?ip=1.2.3.4", http.NoBody)
	r.ServeHTTP(w, req)

	// Should fallback to cscli-based ListDecisions
	require.Equal(t, http.StatusOK, w.Code)
}

func TestGetLAPIDecisionsWithScopeFilter(t *testing.T) {
	mockExec := &mockCommandExecutor{output: []byte(`[]`), err: nil}
	h := &CrowdsecHandler{
		CmdExec: mockExec,
		DataDir: t.TempDir(),
	}
	r := gin.New()
	r.GET("/decisions", h.GetLAPIDecisions)

	// Test with scope query parameter
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/decisions?scope=ip", http.NoBody)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
}

func TestGetLAPIDecisionsWithTypeFilter(t *testing.T) {
	mockExec := &mockCommandExecutor{output: []byte(`[]`), err: nil}
	h := &CrowdsecHandler{
		CmdExec: mockExec,
		DataDir: t.TempDir(),
	}
	r := gin.New()
	r.GET("/decisions", h.GetLAPIDecisions)

	// Test with type query parameter
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/decisions?type=ban", http.NoBody)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
}

func TestGetLAPIDecisionsWithMultipleFilters(t *testing.T) {
	mockExec := &mockCommandExecutor{output: []byte(`[]`), err: nil}
	h := &CrowdsecHandler{
		CmdExec: mockExec,
		DataDir: t.TempDir(),
	}
	r := gin.New()
	r.GET("/decisions", h.GetLAPIDecisions)

	// Test with multiple query parameters
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/decisions?ip=1.2.3.4&scope=ip&type=ban", http.NoBody)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
}
