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
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/api/handlers"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/services"
)

func setupUptimeHandlerTest(t *testing.T) (*gin.Engine, *gorm.DB) {
	t.Helper()
	db := handlers.OpenTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.UptimeMonitor{}, &models.UptimeHeartbeat{}, &models.UptimeHost{}, &models.RemoteServer{}, &models.NotificationProvider{}, &models.Notification{}, &models.ProxyHost{}))

	ns := services.NewNotificationService(db)
	service := services.NewUptimeService(db, ns)
	handler := handlers.NewUptimeHandler(service)

	r := gin.Default()
	api := r.Group("/api/v1")
	uptime := api.Group("/uptime")
	uptime.GET("", handler.List)
	uptime.POST("", handler.Create)
	uptime.GET(":id/history", handler.GetHistory)
	uptime.PUT(":id", handler.Update)
	uptime.DELETE(":id", handler.Delete)
	uptime.POST(":id/check", handler.CheckMonitor)
	uptime.POST("/sync", handler.Sync)

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

	req, _ := http.NewRequest("GET", "/api/v1/uptime", http.NoBody)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var list []models.UptimeMonitor
	err := json.Unmarshal(w.Body.Bytes(), &list)
	require.NoError(t, err)
	assert.Len(t, list, 1)
	assert.Equal(t, "Test Monitor", list[0].Name)
}

func TestUptimeHandler_Create(t *testing.T) {
	t.Run("success_http", func(t *testing.T) {
		r, db := setupUptimeHandlerTest(t)

		payload := map[string]any{
			"name":        "New HTTP Monitor",
			"url":         "https://example.com",
			"type":        "http",
			"interval":    120,
			"max_retries": 5,
		}
		body, _ := json.Marshal(payload)

		req, _ := http.NewRequest("POST", "/api/v1/uptime", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusCreated, w.Code)

		var result models.UptimeMonitor
		err := json.Unmarshal(w.Body.Bytes(), &result)
		require.NoError(t, err)
		assert.Equal(t, "New HTTP Monitor", result.Name)
		assert.Equal(t, "https://example.com", result.URL)
		assert.Equal(t, "http", result.Type)
		assert.Equal(t, 120, result.Interval)
		assert.Equal(t, 5, result.MaxRetries)
		assert.True(t, result.Enabled)
		assert.Equal(t, "pending", result.Status)
		assert.NotEmpty(t, result.ID)

		// Verify it's in the database
		var dbMonitor models.UptimeMonitor
		require.NoError(t, db.First(&dbMonitor, "id = ?", result.ID).Error)
		assert.Equal(t, "New HTTP Monitor", dbMonitor.Name)
	})

	t.Run("success_tcp", func(t *testing.T) {
		r, _ := setupUptimeHandlerTest(t)

		payload := map[string]any{
			"name": "New TCP Monitor",
			"url":  "example.com:8080",
			"type": "tcp",
		}
		body, _ := json.Marshal(payload)

		req, _ := http.NewRequest("POST", "/api/v1/uptime", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusCreated, w.Code)

		var result models.UptimeMonitor
		err := json.Unmarshal(w.Body.Bytes(), &result)
		require.NoError(t, err)
		assert.Equal(t, "New TCP Monitor", result.Name)
		assert.Equal(t, "example.com:8080", result.URL)
		assert.Equal(t, "tcp", result.Type)
		assert.Equal(t, 60, result.Interval)  // Default
		assert.Equal(t, 3, result.MaxRetries) // Default
	})

	t.Run("success_defaults", func(t *testing.T) {
		r, _ := setupUptimeHandlerTest(t)

		payload := map[string]any{
			"name": "Default Monitor",
			"url":  "https://example.com/health",
			"type": "https",
		}
		body, _ := json.Marshal(payload)

		req, _ := http.NewRequest("POST", "/api/v1/uptime", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusCreated, w.Code)

		var result models.UptimeMonitor
		err := json.Unmarshal(w.Body.Bytes(), &result)
		require.NoError(t, err)
		assert.Equal(t, 60, result.Interval)  // Default
		assert.Equal(t, 3, result.MaxRetries) // Default
	})

	t.Run("missing_name", func(t *testing.T) {
		r, _ := setupUptimeHandlerTest(t)

		payload := map[string]any{
			"url":  "https://example.com",
			"type": "http",
		}
		body, _ := json.Marshal(payload)

		req, _ := http.NewRequest("POST", "/api/v1/uptime", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("missing_url", func(t *testing.T) {
		r, _ := setupUptimeHandlerTest(t)

		payload := map[string]any{
			"name": "No URL Monitor",
			"type": "http",
		}
		body, _ := json.Marshal(payload)

		req, _ := http.NewRequest("POST", "/api/v1/uptime", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("missing_type", func(t *testing.T) {
		r, _ := setupUptimeHandlerTest(t)

		payload := map[string]any{
			"name": "No Type Monitor",
			"url":  "https://example.com",
		}
		body, _ := json.Marshal(payload)

		req, _ := http.NewRequest("POST", "/api/v1/uptime", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("invalid_type", func(t *testing.T) {
		r, _ := setupUptimeHandlerTest(t)

		payload := map[string]any{
			"name": "Invalid Type Monitor",
			"url":  "https://example.com",
			"type": "invalid",
		}
		body, _ := json.Marshal(payload)

		req, _ := http.NewRequest("POST", "/api/v1/uptime", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("invalid_json", func(t *testing.T) {
		r, _ := setupUptimeHandlerTest(t)

		req, _ := http.NewRequest("POST", "/api/v1/uptime", bytes.NewBuffer([]byte("invalid")))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("invalid_tcp_url", func(t *testing.T) {
		r, _ := setupUptimeHandlerTest(t)

		payload := map[string]any{
			"name": "Bad TCP Monitor",
			"url":  "not-host-port",
			"type": "tcp",
		}
		body, _ := json.Marshal(payload)

		req, _ := http.NewRequest("POST", "/api/v1/uptime", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})
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

	req, _ := http.NewRequest("GET", "/api/v1/uptime/"+monitorID+"/history", http.NoBody)
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

func TestUptimeHandler_CheckMonitor(t *testing.T) {
	r, db := setupUptimeHandlerTest(t)

	// Create monitor
	monitor := models.UptimeMonitor{ID: "check-mon-1", Name: "Check Monitor", Type: "http", URL: "http://example.com"}
	db.Create(&monitor)

	req, _ := http.NewRequest("POST", "/api/v1/uptime/check-mon-1/check", http.NoBody)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestUptimeHandler_CheckMonitor_NotFound(t *testing.T) {
	r, _ := setupUptimeHandlerTest(t)

	req, _ := http.NewRequest("POST", "/api/v1/uptime/nonexistent/check", http.NoBody)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
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

		updates := map[string]any{
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

		updates := map[string]any{
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

func TestUptimeHandler_DeleteAndSync(t *testing.T) {
	t.Run("delete monitor", func(t *testing.T) {
		r, db := setupUptimeHandlerTest(t)

		monitor := models.UptimeMonitor{ID: "mon-delete", Name: "ToDelete", Type: "http", URL: "http://example.com"}
		db.Create(&monitor)

		req, _ := http.NewRequest("DELETE", "/api/v1/uptime/mon-delete", http.NoBody)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		var m models.UptimeMonitor
		require.Error(t, db.First(&m, "id = ?", "mon-delete").Error)
	})

	t.Run("sync creates monitor for proxy host", func(t *testing.T) {
		r, db := setupUptimeHandlerTest(t)

		// Create a proxy host to be synced to an uptime monitor
		host := models.ProxyHost{UUID: "ph-up-1", Name: "Test Host", DomainNames: "sync.example.com", ForwardHost: "127.0.0.1", ForwardPort: 80, Enabled: true}
		db.Create(&host)

		req, _ := http.NewRequest("POST", "/api/v1/uptime/sync", http.NoBody)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		var monitors []models.UptimeMonitor
		db.Where("proxy_host_id = ?", host.ID).Find(&monitors)
		assert.Len(t, monitors, 1)
		assert.Equal(t, "Test Host", monitors[0].Name)
	})

	t.Run("update enabled via PUT", func(t *testing.T) {
		r, db := setupUptimeHandlerTest(t)

		monitor := models.UptimeMonitor{ID: "mon-enable", Name: "ToToggle", Type: "http", URL: "http://example.com", Enabled: true}
		db.Create(&monitor)

		updates := map[string]any{"enabled": false}
		body, _ := json.Marshal(updates)
		req, _ := http.NewRequest("PUT", "/api/v1/uptime/mon-enable", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		var result models.UptimeMonitor
		err := json.Unmarshal(w.Body.Bytes(), &result)
		require.NoError(t, err)
		assert.False(t, result.Enabled)
	})
}

func TestUptimeHandler_Sync_Success(t *testing.T) {
	r, _ := setupUptimeHandlerTest(t)

	req, _ := http.NewRequest("POST", "/api/v1/uptime/sync", http.NoBody)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var result map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &result)
	require.NoError(t, err)
	assert.Equal(t, "Sync started", result["message"])
}

func TestUptimeHandler_Delete_Error(t *testing.T) {
	r, db := setupUptimeHandlerTest(t)
	db.Exec("DROP TABLE IF EXISTS uptime_monitors")

	req, _ := http.NewRequest("DELETE", "/api/v1/uptime/nonexistent", http.NoBody)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestUptimeHandler_List_Error(t *testing.T) {
	r, db := setupUptimeHandlerTest(t)
	db.Exec("DROP TABLE IF EXISTS uptime_monitors")

	req, _ := http.NewRequest("GET", "/api/v1/uptime", http.NoBody)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestUptimeHandler_GetHistory_Error(t *testing.T) {
	r, db := setupUptimeHandlerTest(t)
	db.Exec("DROP TABLE IF EXISTS uptime_heartbeats")

	req, _ := http.NewRequest("GET", "/api/v1/uptime/monitor-1/history", http.NoBody)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}
