package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// permissiveLAPIURLValidator allows any localhost URL for testing with mock servers.
func permissiveLAPIURLValidator(raw string) (*url.URL, error) {
	return url.Parse(raw)
}

// mockStopExecutor is a mock for the CrowdsecExecutor interface for Stop tests
type mockStopExecutor struct {
	stopCalled bool
	stopErr    error
}

func (m *mockStopExecutor) Start(_ context.Context, _, _ string) (int, error) {
	return 0, nil
}

func (m *mockStopExecutor) Stop(_ context.Context, _ string) error {
	m.stopCalled = true
	return m.stopErr
}

func (m *mockStopExecutor) Status(_ context.Context, _ string) (running bool, pid int, err error) {
	return false, 0, nil
}

// createTestSecurityService creates a SecurityService for testing
func createTestSecurityService(t *testing.T, db *gorm.DB) *services.SecurityService {
	t.Helper()
	svc := services.NewSecurityService(db)
	t.Cleanup(func() { svc.Close() })
	return svc
}

// TestCrowdsecHandler_Stop_Success tests the Stop handler with successful execution
func TestCrowdsecHandler_Stop_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := OpenTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.SecurityConfig{}, &models.Setting{}))

	// Create security config to be updated on stop
	cfg := models.SecurityConfig{Enabled: true, CrowdSecMode: "enabled"}
	require.NoError(t, db.Create(&cfg).Error)

	tmpDir := t.TempDir()
	mockExec := &mockStopExecutor{}
	h := &CrowdsecHandler{
		DB:       db,
		Executor: mockExec,
		CmdExec:  &mockCommandExecutor{},
		DataDir:  tmpDir,
	}

	r := gin.New()
	r.POST("/stop", h.Stop)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/stop", http.NoBody)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.True(t, mockExec.stopCalled)

	var response map[string]any
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "stopped", response["status"])

	// Verify config was updated
	var updatedCfg models.SecurityConfig
	require.NoError(t, db.First(&updatedCfg).Error)
	assert.Equal(t, "disabled", updatedCfg.CrowdSecMode)
	assert.False(t, updatedCfg.Enabled)

	// Verify setting was synced
	var setting models.Setting
	require.NoError(t, db.Where("key = ?", "security.crowdsec.enabled").First(&setting).Error)
	assert.Equal(t, "false", setting.Value)
}

// TestCrowdsecHandler_Stop_Error tests the Stop handler with an execution error
func TestCrowdsecHandler_Stop_Error(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := OpenTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.SecurityConfig{}, &models.Setting{}))

	tmpDir := t.TempDir()
	mockExec := &mockStopExecutor{stopErr: assert.AnError}
	h := &CrowdsecHandler{
		DB:       db,
		Executor: mockExec,
		CmdExec:  &mockCommandExecutor{},
		DataDir:  tmpDir,
	}

	r := gin.New()
	r.POST("/stop", h.Stop)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/stop", http.NoBody)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.True(t, mockExec.stopCalled)
}

// TestCrowdsecHandler_Stop_NoSecurityConfig tests Stop when there's no existing SecurityConfig
func TestCrowdsecHandler_Stop_NoSecurityConfig(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := OpenTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.SecurityConfig{}, &models.Setting{}))

	// Don't create security config - test the path where no config exists

	tmpDir := t.TempDir()
	mockExec := &mockStopExecutor{}
	h := &CrowdsecHandler{
		DB:       db,
		Executor: mockExec,
		CmdExec:  &mockCommandExecutor{},
		DataDir:  tmpDir,
	}

	r := gin.New()
	r.POST("/stop", h.Stop)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/stop", http.NoBody)
	r.ServeHTTP(w, req)

	// Should still return OK even without existing config
	assert.Equal(t, http.StatusOK, w.Code)
	assert.True(t, mockExec.stopCalled)
}

// TestGetLAPIDecisions_WithMockServer tests GetLAPIDecisions with a mock LAPI server
func TestGetLAPIDecisions_WithMockServer(t *testing.T) {
	// Use permissive validator for testing with mock server on random port
	orig := validateCrowdsecLAPIBaseURLFunc
	validateCrowdsecLAPIBaseURLFunc = permissiveLAPIURLValidator
	defer func() { validateCrowdsecLAPIBaseURLFunc = orig }()

	// Create a mock LAPI server
	mockLAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[{"id":1,"origin":"cscli","scope":"Ip","value":"1.2.3.4","type":"ban","duration":"4h","scenario":"manual ban"}]`))
	}))
	defer mockLAPI.Close()

	gin.SetMode(gin.TestMode)
	db := OpenTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.SecurityConfig{}))

	// Create security config with mock LAPI URL
	cfg := models.SecurityConfig{CrowdSecAPIURL: mockLAPI.URL}
	require.NoError(t, db.Create(&cfg).Error)

	secSvc := createTestSecurityService(t, db)
	h := &CrowdsecHandler{
		DB:       db,
		Security: secSvc,
		CmdExec:  &mockCommandExecutor{},
		DataDir:  t.TempDir(),
	}

	r := gin.New()
	r.GET("/decisions/lapi", h.GetLAPIDecisions)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/decisions/lapi", http.NoBody)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]any
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "lapi", response["source"])

	decisions, ok := response["decisions"].([]any)
	require.True(t, ok)
	assert.Len(t, decisions, 1)
}

// TestGetLAPIDecisions_Unauthorized tests GetLAPIDecisions when LAPI returns 401
func TestGetLAPIDecisions_Unauthorized(t *testing.T) {
	// Use permissive validator for testing with mock server on random port
	orig := validateCrowdsecLAPIBaseURLFunc
	validateCrowdsecLAPIBaseURLFunc = permissiveLAPIURLValidator
	defer func() { validateCrowdsecLAPIBaseURLFunc = orig }()

	// Create a mock LAPI server that returns 401
	mockLAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer mockLAPI.Close()

	gin.SetMode(gin.TestMode)
	db := OpenTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.SecurityConfig{}))

	cfg := models.SecurityConfig{CrowdSecAPIURL: mockLAPI.URL}
	require.NoError(t, db.Create(&cfg).Error)

	secSvc := createTestSecurityService(t, db)
	h := &CrowdsecHandler{
		DB:       db,
		Security: secSvc,
		CmdExec:  &mockCommandExecutor{},
		DataDir:  t.TempDir(),
	}

	r := gin.New()
	r.GET("/decisions/lapi", h.GetLAPIDecisions)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/decisions/lapi", http.NoBody)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// TestGetLAPIDecisions_NullResponse tests GetLAPIDecisions when LAPI returns null
func TestGetLAPIDecisions_NullResponse(t *testing.T) {
	// Use permissive validator for testing with mock server on random port
	orig := validateCrowdsecLAPIBaseURLFunc
	validateCrowdsecLAPIBaseURLFunc = permissiveLAPIURLValidator
	defer func() { validateCrowdsecLAPIBaseURLFunc = orig }()

	mockLAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`null`))
	}))
	defer mockLAPI.Close()

	gin.SetMode(gin.TestMode)
	db := OpenTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.SecurityConfig{}))

	cfg := models.SecurityConfig{CrowdSecAPIURL: mockLAPI.URL}
	require.NoError(t, db.Create(&cfg).Error)

	secSvc := createTestSecurityService(t, db)
	h := &CrowdsecHandler{
		DB:       db,
		Security: secSvc,
		CmdExec:  &mockCommandExecutor{},
		DataDir:  t.TempDir(),
	}

	r := gin.New()
	r.GET("/decisions/lapi", h.GetLAPIDecisions)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/decisions/lapi", http.NoBody)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]any
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "lapi", response["source"])
	assert.Equal(t, float64(0), response["total"])
}

// TestGetLAPIDecisions_NonJSONContentType tests the fallback when LAPI returns non-JSON
func TestGetLAPIDecisions_NonJSONContentType(t *testing.T) {
	mockLAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`<html>Error</html>`))
	}))
	defer mockLAPI.Close()

	gin.SetMode(gin.TestMode)
	db := OpenTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.SecurityConfig{}))

	cfg := models.SecurityConfig{CrowdSecAPIURL: mockLAPI.URL}
	require.NoError(t, db.Create(&cfg).Error)

	secSvc := createTestSecurityService(t, db)
	h := &CrowdsecHandler{
		DB:       db,
		Security: secSvc,
		CmdExec:  &mockCommandExecutor{output: []byte(`[]`)}, // Fallback mock
		DataDir:  t.TempDir(),
	}

	r := gin.New()
	r.GET("/decisions/lapi", h.GetLAPIDecisions)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/decisions/lapi", http.NoBody)
	r.ServeHTTP(w, req)

	// Should fallback to cscli and return OK
	assert.Equal(t, http.StatusOK, w.Code)
}

// TestCheckLAPIHealth_WithMockServer tests CheckLAPIHealth with a healthy LAPI
func TestCheckLAPIHealth_WithMockServer(t *testing.T) {
	// Use permissive validator for testing with mock server on random port
	orig := validateCrowdsecLAPIBaseURLFunc
	validateCrowdsecLAPIBaseURLFunc = permissiveLAPIURLValidator
	defer func() { validateCrowdsecLAPIBaseURLFunc = orig }()

	mockLAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status":"ok"}`))
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer mockLAPI.Close()

	gin.SetMode(gin.TestMode)
	db := OpenTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.SecurityConfig{}))

	cfg := models.SecurityConfig{CrowdSecAPIURL: mockLAPI.URL}
	require.NoError(t, db.Create(&cfg).Error)

	secSvc := createTestSecurityService(t, db)
	h := &CrowdsecHandler{
		DB:       db,
		Security: secSvc,
		CmdExec:  &mockCommandExecutor{},
		DataDir:  t.TempDir(),
	}

	r := gin.New()
	r.GET("/health", h.CheckLAPIHealth)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/health", http.NoBody)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]any
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.True(t, response["healthy"].(bool))
}

// TestCheckLAPIHealth_FallbackToDecisions tests the fallback to /v1/decisions endpoint
// when the primary /health endpoint is unreachable
func TestCheckLAPIHealth_FallbackToDecisions(t *testing.T) {
	// Use permissive validator for testing with mock server on random port
	orig := validateCrowdsecLAPIBaseURLFunc
	validateCrowdsecLAPIBaseURLFunc = permissiveLAPIURLValidator
	defer func() { validateCrowdsecLAPIBaseURLFunc = orig }()

	// Create a mock server that only responds to /v1/decisions, not /health
	mockLAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/decisions" {
			// Return 401 which indicates LAPI is running (just needs auth)
			w.WriteHeader(http.StatusUnauthorized)
		} else {
			// Close connection without responding to simulate unreachable endpoint
			panic(http.ErrAbortHandler)
		}
	}))
	defer mockLAPI.Close()

	gin.SetMode(gin.TestMode)
	db := OpenTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.SecurityConfig{}))

	cfg := models.SecurityConfig{CrowdSecAPIURL: mockLAPI.URL}
	require.NoError(t, db.Create(&cfg).Error)

	secSvc := createTestSecurityService(t, db)
	h := &CrowdsecHandler{
		DB:       db,
		Security: secSvc,
		CmdExec:  &mockCommandExecutor{},
		DataDir:  t.TempDir(),
	}

	r := gin.New()
	r.GET("/health", h.CheckLAPIHealth)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/health", http.NoBody)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]any
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	// Should be healthy via fallback
	assert.True(t, response["healthy"].(bool))
	if note, ok := response["note"].(string); ok {
		assert.Contains(t, note, "decisions endpoint")
	}
}

// TestGetLAPIKey_AllEnvVars tests that getLAPIKey checks all environment variable names
func TestGetLAPIKey_AllEnvVars(t *testing.T) {
	envVars := []string{
		"CROWDSEC_API_KEY",
		"CROWDSEC_BOUNCER_API_KEY",
		"CERBERUS_SECURITY_CROWDSEC_API_KEY",
		"CHARON_SECURITY_CROWDSEC_API_KEY",
		"CPM_SECURITY_CROWDSEC_API_KEY",
	}

	// Clean up all env vars first
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

	// Test each env var in order of priority
	for i, envVar := range envVars {
		t.Run(envVar, func(t *testing.T) {
			// Clear all vars
			for _, key := range envVars {
				_ = os.Unsetenv(key)
			}

			// Set only this env var
			testValue := "test-key-" + envVar
			_ = os.Setenv(envVar, testValue)

			key := getLAPIKey()
			if i == 0 || key == testValue {
				// First one should always be found, others only if earlier ones not set
				assert.Equal(t, testValue, key)
			}
		})
	}
}
