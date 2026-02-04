package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// TestStartSyncsSettingsTable verifies that Start() updates the settings table.
func TestStartSyncsSettingsTable(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := OpenTestDB(t)

	// Migrate both SecurityConfig and Setting tables
	require.NoError(t, db.AutoMigrate(&models.SecurityConfig{}, &models.Setting{}))

	// Mock LAPI server for testKeyAgainstLAPI (returns 200 OK for any key)
	mockLAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"new": [], "deleted": []}`))
	}))
	defer mockLAPI.Close()

	// Create SecurityConfig with mock LAPI URL so testKeyAgainstLAPI uses it
	secCfg := models.SecurityConfig{
		UUID:           "test-uuid",
		Name:           "default",
		CrowdSecAPIURL: mockLAPI.URL,
	}
	require.NoError(t, db.Create(&secCfg).Error)

	tmpDir := t.TempDir()
	fe := &fakeExec{}
	h := newTestCrowdsecHandler(t, db, fe, "/bin/false", tmpDir)

	// Replace CmdExec to prevent LAPI wait loop - simulate LAPI ready
	h.CmdExec = &mockCommandExecutor{
		output: []byte("lapi is running"),
		err:    nil,
	}

	r := gin.New()
	g := r.Group("/api/v1")
	h.RegisterRoutes(g)

	// Verify settings table is initially empty
	var initialSetting models.Setting
	err := db.Where("key = ?", "security.crowdsec.enabled").First(&initialSetting).Error
	require.Error(t, err, "expected setting to not exist initially")

	// Start CrowdSec
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/crowdsec/start", http.NoBody)
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	// Verify setting was created/updated to "true"
	var setting models.Setting
	err = db.Where("key = ?", "security.crowdsec.enabled").First(&setting).Error
	require.NoError(t, err, "expected setting to be created after Start")
	require.Equal(t, "true", setting.Value)
	require.Equal(t, "security", setting.Category)
	require.Equal(t, "bool", setting.Type)

	// Also verify SecurityConfig was updated
	var cfg models.SecurityConfig
	err = db.First(&cfg).Error
	require.NoError(t, err, "expected SecurityConfig to exist")
	require.Equal(t, "local", cfg.CrowdSecMode)
	require.True(t, cfg.Enabled)
}

// TestStopSyncsSettingsTable verifies that Stop() updates the settings table.
func TestStopSyncsSettingsTable(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := OpenTestDB(t)

	// Migrate both SecurityConfig and Setting tables
	require.NoError(t, db.AutoMigrate(&models.SecurityConfig{}, &models.Setting{}))

	// Mock LAPI server for testKeyAgainstLAPI (returns 200 OK for any key)
	mockLAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"new": [], "deleted": []}`))
	}))
	defer mockLAPI.Close()

	// Create SecurityConfig with mock LAPI URL so testKeyAgainstLAPI uses it
	secCfg := models.SecurityConfig{
		UUID:           "test-uuid",
		Name:           "default",
		CrowdSecAPIURL: mockLAPI.URL,
	}
	require.NoError(t, db.Create(&secCfg).Error)

	tmpDir := t.TempDir()
	fe := &fakeExec{}
	h := newTestCrowdsecHandler(t, db, fe, "/bin/false", tmpDir)

	// Replace CmdExec to prevent LAPI wait loop - simulate LAPI ready
	h.CmdExec = &mockCommandExecutor{
		output: []byte("lapi is running"),
		err:    nil,
	}

	r := gin.New()
	g := r.Group("/api/v1")
	h.RegisterRoutes(g)

	// First start CrowdSec to create the settings
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/crowdsec/start", http.NoBody)
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	// Verify setting is "true" after start
	var settingAfterStart models.Setting
	err := db.Where("key = ?", "security.crowdsec.enabled").First(&settingAfterStart).Error
	require.NoError(t, err)
	require.Equal(t, "true", settingAfterStart.Value)

	// Now stop CrowdSec
	w2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/admin/crowdsec/stop", http.NoBody)
	r.ServeHTTP(w2, req2)
	require.Equal(t, http.StatusOK, w2.Code)

	// Verify setting was updated to "false"
	var settingAfterStop models.Setting
	err = db.Where("key = ?", "security.crowdsec.enabled").First(&settingAfterStop).Error
	require.NoError(t, err)
	require.Equal(t, "false", settingAfterStop.Value)

	// Also verify SecurityConfig was updated
	var cfg models.SecurityConfig
	err = db.First(&cfg).Error
	require.NoError(t, err)
	require.Equal(t, "disabled", cfg.CrowdSecMode)
	require.False(t, cfg.Enabled)
}

// TestStartAndStopStateConsistency verifies consistent state across Start/Stop cycles.
func TestStartAndStopStateConsistency(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := OpenTestDB(t)

	require.NoError(t, db.AutoMigrate(&models.SecurityConfig{}, &models.Setting{}))

	// Mock LAPI server for testKeyAgainstLAPI (returns 200 OK for any key)
	mockLAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"new": [], "deleted": []}`))
	}))
	defer mockLAPI.Close()

	// Create SecurityConfig with mock LAPI URL so testKeyAgainstLAPI uses it
	secCfg := models.SecurityConfig{
		UUID:           "test-uuid",
		Name:           "default",
		CrowdSecAPIURL: mockLAPI.URL,
	}
	require.NoError(t, db.Create(&secCfg).Error)

	tmpDir := t.TempDir()
	fe := &fakeExec{}
	h := newTestCrowdsecHandler(t, db, fe, "/bin/false", tmpDir)

	// Replace CmdExec to simulate LAPI ready immediately (for cscli bouncers list)
	h.CmdExec = &mockCommandExecutor{
		output: []byte("lapi is running"),
		err:    nil,
	}

	r := gin.New()
	g := r.Group("/api/v1")
	h.RegisterRoutes(g)

	// Perform multiple start/stop cycles
	for i := 0; i < 3; i++ {
		// Start
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/crowdsec/start", http.NoBody)
		r.ServeHTTP(w, req)
		require.Equal(t, http.StatusOK, w.Code, "cycle %d start", i)

		// Verify both tables are in sync
		var setting models.Setting
		err := db.Where("key = ?", "security.crowdsec.enabled").First(&setting).Error
		require.NoError(t, err, "cycle %d: setting should exist after start", i)
		require.Equal(t, "true", setting.Value, "cycle %d: setting should be true after start", i)

		var cfg models.SecurityConfig
		err = db.First(&cfg).Error
		require.NoError(t, err, "cycle %d: config should exist after start", i)
		require.Equal(t, "local", cfg.CrowdSecMode, "cycle %d: mode should be local after start", i)

		// Stop
		w2 := httptest.NewRecorder()
		req2 := httptest.NewRequest(http.MethodPost, "/api/v1/admin/crowdsec/stop", http.NoBody)
		r.ServeHTTP(w2, req2)
		require.Equal(t, http.StatusOK, w2.Code, "cycle %d stop", i)

		// Verify both tables are in sync
		err = db.Where("key = ?", "security.crowdsec.enabled").First(&setting).Error
		require.NoError(t, err, "cycle %d: setting should exist after stop", i)
		require.Equal(t, "false", setting.Value, "cycle %d: setting should be false after stop", i)

		err = db.First(&cfg).Error
		require.NoError(t, err, "cycle %d: config should exist after stop", i)
		require.Equal(t, "disabled", cfg.CrowdSecMode, "cycle %d: mode should be disabled after stop", i)
	}
}

// TestExistingSettingIsUpdated verifies that an existing setting is updated, not duplicated.
func TestExistingSettingIsUpdated(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := OpenTestDB(t)

	require.NoError(t, db.AutoMigrate(&models.SecurityConfig{}, &models.Setting{}))

	// Mock LAPI server for testKeyAgainstLAPI (returns 200 OK for any key)
	mockLAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"new": [], "deleted": []}`))
	}))
	defer mockLAPI.Close()

	// Create SecurityConfig with mock LAPI URL so testKeyAgainstLAPI uses it
	secCfg := models.SecurityConfig{
		UUID:           "test-uuid",
		Name:           "default",
		CrowdSecAPIURL: mockLAPI.URL,
	}
	require.NoError(t, db.Create(&secCfg).Error)

	// Pre-create a setting with a different value
	existingSetting := models.Setting{
		Key:      "security.crowdsec.enabled",
		Value:    "false",
		Category: "security",
		Type:     "bool",
	}
	require.NoError(t, db.Create(&existingSetting).Error)

	tmpDir := t.TempDir()
	fe := &fakeExec{}
	h := newTestCrowdsecHandler(t, db, fe, "/bin/false", tmpDir)

	// Replace CmdExec to prevent LAPI wait loop - simulate LAPI ready
	h.CmdExec = &mockCommandExecutor{
		output: []byte("lapi is running"),
		err:    nil,
	}

	r := gin.New()
	g := r.Group("/api/v1")
	h.RegisterRoutes(g)

	// Start CrowdSec
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/crowdsec/start", http.NoBody)
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	// Verify the existing setting was updated (not duplicated)
	var settings []models.Setting
	err := db.Where("key = ?", "security.crowdsec.enabled").Find(&settings).Error
	require.NoError(t, err)
	require.Len(t, settings, 1, "should not create duplicate settings")
	require.Equal(t, "true", settings[0].Value, "setting should be updated to true")
}

// fakeFailingExec simulates an executor that fails on Start.
type fakeFailingExec struct{}

func (f *fakeFailingExec) Start(ctx context.Context, binPath, configDir string) (int, error) {
	return 0, http.ErrAbortHandler
}

func (f *fakeFailingExec) Stop(ctx context.Context, configDir string) error {
	return nil
}

func (f *fakeFailingExec) Status(ctx context.Context, configDir string) (running bool, pid int, err error) {
	return false, 0, nil
}

// TestStartFailureRevertsSettings verifies that a failed Start reverts the settings.
func TestStartFailureRevertsSettings(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := OpenTestDB(t)

	require.NoError(t, db.AutoMigrate(&models.SecurityConfig{}, &models.Setting{}))

	tmpDir := t.TempDir()
	fe := &fakeFailingExec{}
	h := newTestCrowdsecHandler(t, db, fe, "/bin/false", tmpDir)

	r := gin.New()
	g := r.Group("/api/v1")
	h.RegisterRoutes(g)

	// Pre-create a setting with "false" to verify it's reverted
	existingSetting := models.Setting{
		Key:      "security.crowdsec.enabled",
		Value:    "false",
		Category: "security",
		Type:     "bool",
	}
	require.NoError(t, db.Create(&existingSetting).Error)

	// Try to start CrowdSec (this will fail)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/crowdsec/start", http.NoBody)
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusInternalServerError, w.Code)

	// Verify the setting was reverted to "false"
	var setting models.Setting
	err := db.Where("key = ?", "security.crowdsec.enabled").First(&setting).Error
	require.NoError(t, err)
	require.Equal(t, "false", setting.Value, "setting should be reverted to false on failure")
}

// TestStatusResponseFormat verifies the status endpoint response format.
func TestStatusResponseFormat(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := OpenTestDB(t)

	require.NoError(t, db.AutoMigrate(&models.SecurityConfig{}, &models.Setting{}))

	tmpDir := t.TempDir()
	fe := &fakeExec{}
	h := newTestCrowdsecHandler(t, db, fe, "/bin/false", tmpDir)

	r := gin.New()
	g := r.Group("/api/v1")
	h.RegisterRoutes(g)

	// Get status
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/crowdsec/status", http.NoBody)
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	var resp map[string]any
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	// Verify response contains expected fields
	require.Contains(t, resp, "running")
	require.Contains(t, resp, "pid")
	require.Contains(t, resp, "lapi_ready")
}
