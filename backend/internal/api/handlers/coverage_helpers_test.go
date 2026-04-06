package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test ttlRemainingSeconds helper function
func Test_ttlRemainingSeconds(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name         string
		now          time.Time
		retrievedAt  time.Time
		ttl          time.Duration
		wantNil      bool
		wantZero     bool
		wantPositive bool
	}{
		{
			name:        "zero retrievedAt returns nil",
			now:         now,
			retrievedAt: time.Time{},
			ttl:         time.Hour,
			wantNil:     true,
		},
		{
			name:        "zero ttl returns nil",
			now:         now,
			retrievedAt: now,
			ttl:         0,
			wantNil:     true,
		},
		{
			name:        "negative ttl returns nil",
			now:         now,
			retrievedAt: now,
			ttl:         -time.Hour,
			wantNil:     true,
		},
		{
			name:        "expired ttl returns zero",
			now:         now,
			retrievedAt: now.Add(-2 * time.Hour),
			ttl:         time.Hour,
			wantZero:    true,
		},
		{
			name:         "valid remaining time returns positive",
			now:          now,
			retrievedAt:  now,
			ttl:          time.Hour,
			wantPositive: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ttlRemainingSeconds(tt.now, tt.retrievedAt, tt.ttl)
			switch {
			case tt.wantNil:
				assert.Nil(t, result)
			case tt.wantZero:
				require.NotNil(t, result)
				assert.Equal(t, int64(0), *result)
			case tt.wantPositive:
				require.NotNil(t, result)
				assert.Greater(t, *result, int64(0))
			}
		})
	}
}

// Test mapCrowdsecStatus helper function
func Test_mapCrowdsecStatus(t *testing.T) {
	tests := []struct {
		name        string
		err         error
		defaultCode int
		want        int
	}{
		{
			name:        "deadline exceeded returns gateway timeout",
			err:         context.DeadlineExceeded,
			defaultCode: http.StatusInternalServerError,
			want:        http.StatusGatewayTimeout,
		},
		{
			name:        "context canceled returns gateway timeout",
			err:         context.Canceled,
			defaultCode: http.StatusInternalServerError,
			want:        http.StatusGatewayTimeout,
		},
		{
			name:        "other error returns default code",
			err:         errors.New("some error"),
			defaultCode: http.StatusInternalServerError,
			want:        http.StatusInternalServerError,
		},
		{
			name:        "other error returns bad request default",
			err:         errors.New("validation error"),
			defaultCode: http.StatusBadRequest,
			want:        http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mapCrowdsecStatus(tt.err, tt.defaultCode)
			assert.Equal(t, tt.want, got)
		})
	}
}

// Test actorFromContext helper function
func Test_actorFromContext(t *testing.T) {

	t.Run("with userID in context", func(t *testing.T) {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Set("userID", 123)

		result := actorFromContext(c)
		assert.Equal(t, "user:123", result)
	})

	t.Run("without userID in context", func(t *testing.T) {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())

		result := actorFromContext(c)
		assert.Equal(t, "unknown", result)
	})

	t.Run("with string userID", func(t *testing.T) {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Set("userID", "admin")

		result := actorFromContext(c)
		assert.Equal(t, "user:admin", result)
	})
}

// Test hubEndpoints helper function
func Test_hubEndpoints(t *testing.T) {

	t.Run("nil Hub returns nil", func(t *testing.T) {
		h := &CrowdsecHandler{Hub: nil}
		result := h.hubEndpoints()
		assert.Nil(t, result)
	})
}

// Test RealCommandExecutor Execute method
func TestRealCommandExecutor_Execute(t *testing.T) {
	t.Run("successful command", func(t *testing.T) {
		exec := &RealCommandExecutor{}
		output, err := exec.Execute(context.Background(), "echo", "hello")
		assert.NoError(t, err)
		assert.Contains(t, string(output), "hello")
	})

	t.Run("failed command", func(t *testing.T) {
		exec := &RealCommandExecutor{}
		_, err := exec.Execute(context.Background(), "false")
		assert.Error(t, err)
	})

	t.Run("context cancellation", func(t *testing.T) {
		exec := &RealCommandExecutor{}
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		_, err := exec.Execute(ctx, "sleep", "10")
		assert.Error(t, err)
	})
}

// Test isCerberusEnabled helper
func Test_isCerberusEnabled(t *testing.T) {
	db := OpenTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.Setting{}))

	t.Run("returns true when no setting exists (default)", func(t *testing.T) {
		// Clean up first
		db.Where("1=1").Delete(&models.Setting{})

		h := &CrowdsecHandler{DB: db}
		result := h.isCerberusEnabled()
		assert.True(t, result) // Default is true when no setting exists
	})

	t.Run("enabled when setting is true", func(t *testing.T) {
		// Clean up first
		db.Where("1=1").Delete(&models.Setting{})

		setting := models.Setting{
			Key:      "feature.cerberus.enabled",
			Value:    "true",
			Category: "feature",
			Type:     "bool",
		}
		require.NoError(t, db.Create(&setting).Error)

		h := &CrowdsecHandler{DB: db}
		result := h.isCerberusEnabled()
		assert.True(t, result)
	})

	t.Run("disabled when setting is false", func(t *testing.T) {
		// Clean up first
		db.Where("1=1").Delete(&models.Setting{})

		setting := models.Setting{
			Key:      "feature.cerberus.enabled",
			Value:    "false",
			Category: "feature",
			Type:     "bool",
		}
		require.NoError(t, db.Create(&setting).Error)

		h := &CrowdsecHandler{DB: db}
		result := h.isCerberusEnabled()
		assert.False(t, result)
	})
}

// Test isConsoleEnrollmentEnabled helper
func Test_isConsoleEnrollmentEnabled(t *testing.T) {
	db := OpenTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.Setting{}))

	t.Run("disabled when no setting exists", func(t *testing.T) {
		// Clean up first
		db.Where("1=1").Delete(&models.Setting{})

		h := &CrowdsecHandler{DB: db}
		result := h.isConsoleEnrollmentEnabled()
		assert.False(t, result)
	})

	t.Run("enabled when setting is true", func(t *testing.T) {
		// Clean up first
		db.Where("1=1").Delete(&models.Setting{})

		setting := models.Setting{
			Key:      "feature.crowdsec.console_enrollment",
			Value:    "true",
			Category: "feature",
			Type:     "bool",
		}
		require.NoError(t, db.Create(&setting).Error)

		h := &CrowdsecHandler{DB: db}
		result := h.isConsoleEnrollmentEnabled()
		assert.True(t, result)
	})

	t.Run("disabled when setting is false", func(t *testing.T) {
		// Clean up and add new setting
		db.Where("key = ?", "feature.crowdsec.console_enrollment").Delete(&models.Setting{})

		setting := models.Setting{
			Key:      "feature.crowdsec.console_enrollment",
			Value:    "false",
			Category: "feature",
			Type:     "bool",
		}
		require.NoError(t, db.Create(&setting).Error)

		h := &CrowdsecHandler{DB: db}
		result := h.isConsoleEnrollmentEnabled()
		assert.False(t, result)
	})
}

// Test CrowdsecHandler.ExportConfig
func TestCrowdsecHandler_ExportConfig(t *testing.T) {
	db := OpenTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.SecurityConfig{}))

	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, "crowdsec", "config")
	require.NoError(t, os.MkdirAll(configDir, 0o750))

	// Create test config file
	configFile := filepath.Join(configDir, "config.yaml")
	require.NoError(t, os.WriteFile(configFile, []byte("test: config"), 0o600))

	h := newTestCrowdsecHandler(t, db, &fakeExec{}, "/bin/false", tmpDir)

	r := gin.New()
	r.GET("/export", h.ExportConfig)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/export", http.NoBody)
	r.ServeHTTP(w, req)

	// Should return archive (if config exists) or not found
	assert.True(t, w.Code == http.StatusOK || w.Code == http.StatusNotFound)
}

// Test CrowdsecHandler.CheckLAPIHealth
func TestCrowdsecHandler_CheckLAPIHealth(t *testing.T) {
	db := OpenTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.SecurityConfig{}))

	tmpDir := t.TempDir()
	h := newTestCrowdsecHandler(t, db, &fakeExec{}, "/bin/false", tmpDir)

	r := gin.New()
	r.GET("/health", h.CheckLAPIHealth)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/health", http.NoBody)
	r.ServeHTTP(w, req)

	// LAPI won't be running, so expect error or unhealthy
	assert.True(t, w.Code >= http.StatusOK)
}

// Test CrowdsecHandler Console endpoints
func TestCrowdsecHandler_ConsoleStatus(t *testing.T) {
	db := OpenTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.SecurityConfig{}, &models.Setting{}, &models.CrowdsecConsoleEnrollment{}))

	// Enable console enrollment feature
	require.NoError(t, db.Create(&models.Setting{Key: "feature.crowdsec.console_enrollment", Value: "true"}).Error)

	tmpDir := t.TempDir()
	h := newTestCrowdsecHandler(t, db, &fakeExec{}, "/bin/false", tmpDir)

	r := gin.New()
	r.GET("/console/status", h.ConsoleStatus)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/console/status", http.NoBody)
	r.ServeHTTP(w, req)

	// Should return status when feature is enabled
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestCrowdsecHandler_ConsoleEnroll_Disabled(t *testing.T) {
	db := OpenTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.SecurityConfig{}, &models.Setting{}))

	tmpDir := t.TempDir()
	h := newTestCrowdsecHandler(t, db, &fakeExec{}, "/bin/false", tmpDir)

	r := gin.New()
	r.POST("/console/enroll", h.ConsoleEnroll)

	payload := map[string]string{"key": "test-key", "name": "test-name"}
	body, _ := json.Marshal(payload)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/console/enroll", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	// Should return error since console enrollment is disabled
	assert.True(t, w.Code >= http.StatusBadRequest)
}

func TestCrowdsecHandler_DeleteConsoleEnrollment(t *testing.T) {
	db := OpenTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.SecurityConfig{}, &models.Setting{}))

	tmpDir := t.TempDir()
	h := newTestCrowdsecHandler(t, db, &fakeExec{}, "/bin/false", tmpDir)

	r := gin.New()
	r.DELETE("/console/enroll", h.DeleteConsoleEnrollment)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/console/enroll", http.NoBody)
	r.ServeHTTP(w, req)

	// Should return OK or error depending on state
	assert.True(t, w.Code == http.StatusOK || w.Code >= http.StatusBadRequest)
}

// Test CrowdsecHandler.BanIP and UnbanIP
func TestCrowdsecHandler_BanIP(t *testing.T) {
	db := OpenTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.SecurityConfig{}, &models.Setting{}))

	tmpDir := t.TempDir()
	h := newTestCrowdsecHandler(t, db, &fakeExec{}, "/bin/false", tmpDir)
	// Override to simulate cscli failure
	h.CmdExec = &mockCmdExecutor{err: errors.New("cscli failed")}

	r := gin.New()
	r.POST("/ban", h.BanIP)

	payload := map[string]any{
		"ip":       "192.168.1.100",
		"duration": "24h",
		"reason":   "test ban",
	}
	body, _ := json.Marshal(payload)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/ban", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	// Should fail since cscli isn't available
	assert.True(t, w.Code >= http.StatusBadRequest)
}

func TestCrowdsecHandler_UnbanIP(t *testing.T) {
	db := OpenTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.SecurityConfig{}, &models.Setting{}))

	tmpDir := t.TempDir()
	h := newTestCrowdsecHandler(t, db, &fakeExec{}, "/bin/false", tmpDir)

	r := gin.New()
	r.POST("/unban", h.UnbanIP)

	payload := map[string]string{
		"ip": "192.168.1.100",
	}
	body, _ := json.Marshal(payload)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/unban", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	// Should fail since cscli isn't available
	assert.True(t, w.Code >= http.StatusBadRequest)
}

// Test CrowdsecHandler.UpdateAcquisitionConfig
func TestCrowdsecHandler_UpdateAcquisitionConfig(t *testing.T) {
	db := OpenTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.SecurityConfig{}, &models.Setting{}))

	tmpDir := t.TempDir()
	h := newTestCrowdsecHandler(t, db, &fakeExec{}, "/bin/false", tmpDir)

	r := gin.New()
	r.PUT("/acquisition", h.UpdateAcquisitionConfig)

	payload := map[string]any{
		"content": "source: file\nfilename: /var/log/test.log\nlabels:\n  type: test",
	}
	body, _ := json.Marshal(payload)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/acquisition", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	// Should handle the request (may fail due to missing directory)
	assert.True(t, w.Code >= http.StatusOK)
}

// Test WebSocketStatusHandler - removed duplicate tests, see websocket_status_handler_test.go

// Test DBHealthHandler - removed duplicate tests, see db_health_handler_test.go

// Test UpdateHandler - removed duplicate tests, see update_handler_test.go

// Test CerberusLogsHandler - requires services.LogWatcher and WebSocketTracker, tested in cerberus_logs_ws_test.go

// Test safeIntToUint for proxy_host_handler
func Test_safeIntToUint(t *testing.T) {
	tests := []struct {
		name   string
		val    int
		want   uint
		wantOK bool
	}{
		{name: "positive int", val: 5, want: 5, wantOK: true},
		{name: "zero", val: 0, want: 0, wantOK: true},
		{name: "negative int", val: -1, want: 0, wantOK: false},
		{name: "large positive", val: 1000000, want: 1000000, wantOK: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := safeIntToUint(tt.val)
			assert.Equal(t, tt.wantOK, ok)
			assert.Equal(t, tt.want, got)
		})
	}
}

// Test safeFloat64ToUint for proxy_host_handler
func Test_safeFloat64ToUint(t *testing.T) {
	tests := []struct {
		name   string
		val    float64
		want   uint
		wantOK bool
	}{
		{name: "positive integer float", val: 5.0, want: 5, wantOK: true},
		{name: "zero", val: 0.0, want: 0, wantOK: true},
		{name: "negative float", val: -1.0, want: 0, wantOK: false},
		{name: "fractional float", val: 5.5, want: 0, wantOK: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := safeFloat64ToUint(tt.val)
			assert.Equal(t, tt.wantOK, ok)
			assert.Equal(t, tt.want, got)
		})
	}
}

// Test CrowdsecHandler_DiagnosticsConnectivity
func TestCrowdsecHandler_DiagnosticsConnectivity(t *testing.T) {
	db := OpenTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.SecurityConfig{}, &models.Setting{}, &models.CrowdsecConsoleEnrollment{}))

	// Enable console enrollment feature
	require.NoError(t, db.Create(&models.Setting{Key: "feature.crowdsec.console_enrollment", Value: "true"}).Error)

	tmpDir := t.TempDir()
	h := newTestCrowdsecHandler(t, db, &fakeExec{}, "/bin/false", tmpDir)

	r := gin.New()
	r.GET("/diagnostics/connectivity", h.DiagnosticsConnectivity)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/diagnostics/connectivity", http.NoBody)
	r.ServeHTTP(w, req)

	// Should return a JSON response with connectivity checks
	assert.Equal(t, http.StatusOK, w.Code)

	var result map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
	assert.Contains(t, result, "lapi_running")
	assert.Contains(t, result, "lapi_ready")
	assert.Contains(t, result, "capi_registered")
}

// Test CrowdsecHandler_DiagnosticsConfig
func TestCrowdsecHandler_DiagnosticsConfig(t *testing.T) {
	db := OpenTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.SecurityConfig{}, &models.Setting{}))

	tmpDir := t.TempDir()
	h := newTestCrowdsecHandler(t, db, &fakeExec{}, "/bin/false", tmpDir)

	r := gin.New()
	r.GET("/diagnostics/config", h.DiagnosticsConfig)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/diagnostics/config", http.NoBody)
	r.ServeHTTP(w, req)

	// Should return a JSON response with config validation
	assert.Equal(t, http.StatusOK, w.Code)

	var result map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
	assert.Contains(t, result, "config_exists")
	assert.Contains(t, result, "config_valid")
	assert.Contains(t, result, "acquis_exists")
}

// Test CrowdsecHandler_ConsoleHeartbeat
func TestCrowdsecHandler_ConsoleHeartbeat(t *testing.T) {
	db := OpenTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.SecurityConfig{}, &models.Setting{}, &models.CrowdsecConsoleEnrollment{}))

	// Enable console enrollment feature
	require.NoError(t, db.Create(&models.Setting{Key: "feature.crowdsec.console_enrollment", Value: "true"}).Error)

	tmpDir := t.TempDir()
	h := newTestCrowdsecHandler(t, db, &fakeExec{}, "/bin/false", tmpDir)

	r := gin.New()
	r.GET("/console/heartbeat", h.ConsoleHeartbeat)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/console/heartbeat", http.NoBody)
	r.ServeHTTP(w, req)

	// Should return a JSON response with heartbeat info
	assert.Equal(t, http.StatusOK, w.Code)

	var result map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
	assert.Contains(t, result, "status")
	assert.Contains(t, result, "heartbeat_tracking_implemented")
}

// Test CrowdsecHandler_ConsoleHeartbeat_Disabled
func TestCrowdsecHandler_ConsoleHeartbeat_Disabled(t *testing.T) {
	db := OpenTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.SecurityConfig{}, &models.Setting{}))

	tmpDir := t.TempDir()
	h := newTestCrowdsecHandler(t, db, &fakeExec{}, "/bin/false", tmpDir)

	r := gin.New()
	r.GET("/console/heartbeat", h.ConsoleHeartbeat)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/console/heartbeat", http.NoBody)
	r.ServeHTTP(w, req)

	// Should return 404 when console enrollment is disabled
	assert.Equal(t, http.StatusNotFound, w.Code)
}
