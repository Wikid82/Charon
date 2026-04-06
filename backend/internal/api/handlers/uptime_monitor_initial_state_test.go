package handlers_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wikid82/charon/backend/internal/api/handlers"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestUptimeMonitorInitialStatePending - CONTRACT TEST for Phase 2.1
// Verifies that newly created monitors start in "pending" state, not "down"
func TestUptimeMonitorInitialStatePending(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)

	// Migrate UptimeMonitor model
	_ = db.AutoMigrate(&models.UptimeMonitor{}, &models.UptimeHost{})

	// Create handler with service
	notificationService := services.NewNotificationService(db, nil)
	uptimeService := services.NewUptimeService(db, notificationService)

	// Test: Create a monitor via service
	monitor, err := uptimeService.CreateMonitor(
		"Test API Server",
		"https://api.example.com/health",
		"http",
		60,
		3,
	)

	// Verify: Monitor created successfully
	require.NoError(t, err)
	require.NotNil(t, monitor)

	// CONTRACT: Monitor MUST start in "pending" state
	t.Run("newly_created_monitor_status_is_pending", func(t *testing.T) {
		assert.Equal(t, "pending", monitor.Status, "new monitor should start with status='pending'")
	})

	// CONTRACT: FailureCount MUST be zero
	t.Run("newly_created_monitor_failure_count_is_zero", func(t *testing.T) {
		assert.Equal(t, 0, monitor.FailureCount, "new monitor should have failure_count=0")
	})

	// CONTRACT: LastCheck should be zero/null (no checks yet)
	t.Run("newly_created_monitor_last_check_is_null", func(t *testing.T) {
		assert.True(t, monitor.LastCheck.IsZero(), "new monitor should have null last_check")
	})

	// Verify: In database - status persisted correctly
	t.Run("database_contains_pending_status", func(t *testing.T) {
		var dbMonitor models.UptimeMonitor
		result := db.Where("id = ?", monitor.ID).First(&dbMonitor)
		require.NoError(t, result.Error)

		assert.Equal(t, "pending", dbMonitor.Status, "database monitor should have status='pending'")
		assert.Equal(t, 0, dbMonitor.FailureCount, "database monitor should have failure_count=0")
	})

	// Test: Verify API response includes pending status
	t.Run("api_response_includes_pending_status", func(t *testing.T) {
		handler := handlers.NewUptimeHandler(uptimeService)
		router := gin.New()
		router.POST("/api/v1/uptime/monitors", handler.Create)

		requestData := map[string]interface{}{
			"name":        "API Health Check",
			"url":         "https://api.test.com/health",
			"type":        "http",
			"interval":    60,
			"max_retries": 3,
		}
		body, _ := json.Marshal(requestData)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/api/v1/uptime/monitors", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusCreated, w.Code)

		var response models.UptimeMonitor
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Equal(t, "pending", response.Status, "API response should include status='pending'")
	})
}
