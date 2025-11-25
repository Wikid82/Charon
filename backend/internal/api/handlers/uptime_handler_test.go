package handlers_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/Wikid82/CaddyProxyManagerPlus/backend/internal/api/handlers"
	"github.com/Wikid82/CaddyProxyManagerPlus/backend/internal/models"
	"github.com/Wikid82/CaddyProxyManagerPlus/backend/internal/services"
)

func setupUptimeHandlerTest(t *testing.T) (*gin.Engine, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.UptimeMonitor{}, &models.UptimeHeartbeat{}, &models.NotificationProvider{}, &models.Notification{}))

	ns := services.NewNotificationService(db)
	service := services.NewUptimeService(db, ns)
	handler := handlers.NewUptimeHandler(service)

	r := gin.Default()
	api := r.Group("/api/v1")
	uptime := api.Group("/uptime")
	uptime.GET("", handler.List)
	uptime.GET("/:id/history", handler.GetHistory)
	uptime.PUT("/:id", handler.Update)

	return r, db
}

func TestUptimeHandler_List(t *testing.T) {
	r, db := setupUptimeHandlerTest(t)

	// Seed Monitor
	monitor := models.UptimeMonitor{
		ID:   "monitor-1",
		Name: "Test Monitor",
		Type: "http",
		URL:  "http://example.com",
	}
	db.Create(&monitor)

	req, _ := http.NewRequest("GET", "/api/v1/uptime", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var list []models.UptimeMonitor
	err := json.Unmarshal(w.Body.Bytes(), &list)
	require.NoError(t, err)
	assert.Len(t, list, 1)
	assert.Equal(t, "Test Monitor", list[0].Name)
}

func TestUptimeHandler_GetHistory(t *testing.T) {
	r, db := setupUptimeHandlerTest(t)

	// Seed Monitor and Heartbeats
	monitorID := "monitor-1"
	monitor := models.UptimeMonitor{
		ID:   monitorID,
		Name: "Test Monitor",
	}
	db.Create(&monitor)

	db.Create(&models.UptimeHeartbeat{
		MonitorID: monitorID,
		Status:    "up",
		Latency:   10,
		CreatedAt: time.Now().Add(-1 * time.Minute),
	})
	db.Create(&models.UptimeHeartbeat{
		MonitorID: monitorID,
		Status:    "down",
		Latency:   0,
		CreatedAt: time.Now(),
	})

	req, _ := http.NewRequest("GET", "/api/v1/uptime/"+monitorID+"/history", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var history []models.UptimeHeartbeat
	err := json.Unmarshal(w.Body.Bytes(), &history)
	require.NoError(t, err)
	assert.Len(t, history, 2)
	// Should be ordered by created_at desc
	assert.Equal(t, "down", history[0].Status)
}

func TestUptimeHandler_Update(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		r, db := setupUptimeHandlerTest(t)

		monitorID := "monitor-update"
		monitor := models.UptimeMonitor{
			ID:         monitorID,
			Name:       "Original Name",
			Interval:   30,
			MaxRetries: 3,
		}
		db.Create(&monitor)

		updates := map[string]interface{}{
			"interval":    60,
			"max_retries": 5,
		}
		body, _ := json.Marshal(updates)

		req, _ := http.NewRequest("PUT", "/api/v1/uptime/"+monitorID, bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var result models.UptimeMonitor
		err := json.Unmarshal(w.Body.Bytes(), &result)
		require.NoError(t, err)
		assert.Equal(t, 60, result.Interval)
		assert.Equal(t, 5, result.MaxRetries)
	})

	t.Run("invalid_json", func(t *testing.T) {
		r, _ := setupUptimeHandlerTest(t)

		req, _ := http.NewRequest("PUT", "/api/v1/uptime/monitor-1", bytes.NewBuffer([]byte("invalid")))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("not_found", func(t *testing.T) {
		r, _ := setupUptimeHandlerTest(t)

		updates := map[string]interface{}{
			"interval": 60,
		}
		body, _ := json.Marshal(updates)

		req, _ := http.NewRequest("PUT", "/api/v1/uptime/nonexistent", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})
}
