package handlers

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/Wikid82/charon/backend/internal/crowdsec"
)

// MockCommandExecutor implements handlers.CommandExecutor and crowdsec.CommandExecutor
type MockCommandExecutor struct {
	mock.Mock
}

func (m *MockCommandExecutor) Execute(ctx context.Context, name string, args ...string) ([]byte, error) {
	call := m.Called(ctx, name, args)
	return call.Get(0).([]byte), call.Error(1)
}

func (m *MockCommandExecutor) ExecuteWithEnv(ctx context.Context, name string, args []string, env map[string]string) ([]byte, error) {
	call := m.Called(ctx, name, args, env)
	return call.Get(0).([]byte), call.Error(1)
}

// TestConsoleEnrollMissingKey covers the "enrollment_key required" branch
func TestConsoleEnrollMissingKey(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockExec := new(MockCommandExecutor)

	// Create real service
	consoleSvc := crowdsec.NewConsoleEnrollmentService(nil, mockExec, "/tmp", "")

	h := &CrowdsecHandler{
		Console: consoleSvc,
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	c.Request, _ = http.NewRequest("POST", "/enroll", bytes.NewBufferString(`{"agent_name": "test-agent"}`))
	c.Request.Header.Set("Content-Type", "application/json")

	t.Setenv("FEATURE_CROWDSEC_CONSOLE_ENROLLMENT", "1")

	h.ConsoleEnroll(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "enrollment_key required")
}

// TestGetCachedPreset_ValidationAndMiss covers path param validation empty check (if any) and cache miss
func TestGetCachedPreset_ValidationAndMiss(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tmpDir := t.TempDir()
	cache, _ := crowdsec.NewHubCache(tmpDir, time.Hour)
	mockExec := new(MockCommandExecutor)
	hubSvc := crowdsec.NewHubService(mockExec, cache, tmpDir)

	h := &CrowdsecHandler{
		Hub:     hubSvc,
		Console: nil,
	}

	t.Setenv("FEATURE_CERBERUS_ENABLED", "1")

	w := httptest.NewRecorder()
	_, r := gin.CreateTestContext(w)
	r.GET("/api/v1/presets/:slug", h.GetCachedPreset)

	req, _ := http.NewRequest(http.MethodGet, "/api/v1/presets/valid-slug", nil)
	r.ServeHTTP(w, req)

	// Expect 404 on cache miss
	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Contains(t, w.Body.String(), "cache miss")
}

func TestGetCachedPreset_SlugRequired(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &CrowdsecHandler{}
	t.Setenv("FEATURE_CERBERUS_ENABLED", "1")

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	// Manually set params with empty slug
	c.Params = []gin.Param{{Key: "slug", Value: "   "}}
	c.Request = httptest.NewRequest("GET", "/api", nil)

	tmpDir := t.TempDir()
	cache, _ := crowdsec.NewHubCache(tmpDir, time.Hour)
	h.Hub = crowdsec.NewHubService(&MockCommandExecutor{}, cache, tmpDir)

	h.GetCachedPreset(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "slug required")
}
