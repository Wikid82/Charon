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

	ns := services.NewNotificationService(db, nil)
	service := services.NewUptimeService(db, ns)
	handler := handlers.NewUptimeHandler(service)

	r := gin.Default()
	api := r.Group("/api/v1")
	uptime := api.Group("/uptime")
	uptime.GET("", handler.List)
	uptime.POST("", handler.Create)
	uptime.GET("/monitors/summary", handler.Summary)
	uptime.GET("/health", handler.Health)
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

// --- interval floor validation (Commit 2 / §3.6.3) ------------------------

func TestUptimeHandler_Create_IntervalFloor(t *testing.T) {
	postMonitor := func(r http.Handler, interval int) *httptest.ResponseRecorder {
		payload := map[string]any{
			"name":     "floor-test",
			"url":      "https://floor.example.com",
			"type":     "http",
			"interval": interval,
		}
		body, _ := json.Marshal(payload)
		req, _ := http.NewRequest("POST", "/api/v1/uptime", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w
	}

	t.Run("sub_floor_rejected", func(t *testing.T) {
		r, _ := setupUptimeHandlerTest(t)
		w := postMonitor(r, 10)
		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Contains(t, w.Body.String(), "at least 30 seconds")
	})

	t.Run("at_floor_accepted", func(t *testing.T) {
		r, _ := setupUptimeHandlerTest(t)
		w := postMonitor(r, 45)
		require.Equal(t, http.StatusCreated, w.Code)
		var m models.UptimeMonitor
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &m))
		assert.Equal(t, 45, m.Interval)
	})

	t.Run("zero_stored_as_default", func(t *testing.T) {
		r, _ := setupUptimeHandlerTest(t)
		w := postMonitor(r, 0)
		require.Equal(t, http.StatusCreated, w.Code)
		var m models.UptimeMonitor
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &m))
		assert.Equal(t, 60, m.Interval)
	})
}

func TestUptimeHandler_Update_IntervalFloor(t *testing.T) {
	r, db := setupUptimeHandlerTest(t)
	monitor := models.UptimeMonitor{ID: "mon-floor", Name: "m", Interval: 60, MaxRetries: 3}
	require.NoError(t, db.Create(&monitor).Error)

	body, _ := json.Marshal(map[string]any{"interval": 5})
	req, _ := http.NewRequest("PUT", "/api/v1/uptime/mon-floor", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "at least 30 seconds")

	// Unchanged in the DB.
	var after models.UptimeMonitor
	require.NoError(t, db.First(&after, "id = ?", "mon-floor").Error)
	assert.Equal(t, 60, after.Interval)
}

// --- Commit 7: batch summary, /uptime/health, history paging ----------------

func TestUptimeHandler_Summary(t *testing.T) {
	r, db := setupUptimeHandlerTest(t)

	m1 := models.UptimeMonitor{ID: "sum-b", Name: "Beta", Type: "http", URL: "http://b.example", Enabled: true, Interval: 30, Status: "up", Latency: 12}
	m2 := models.UptimeMonitor{ID: "sum-a", Name: "Alpha", Type: "tcp", URL: "tcp://a.example", Enabled: true, Interval: 60, Status: "down", Latency: 0}
	require.NoError(t, db.Create(&m1).Error)
	require.NoError(t, db.Create(&m2).Error)

	base := time.Now().Add(-1 * time.Hour)
	for i := 0; i < 4; i++ {
		status := "up"
		if i == 3 {
			status = "down"
		}
		require.NoError(t, db.Create(&models.UptimeHeartbeat{
			MonitorID: "sum-b", Status: status, Latency: int64(i), CreatedAt: base.Add(time.Duration(i) * time.Minute),
		}).Error)
	}

	req, _ := http.NewRequest("GET", "/api/v1/uptime/monitors/summary", http.NoBody)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var got []services.MonitorSummary
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	require.Len(t, got, 2)

	// Ordered by name ASC.
	assert.Equal(t, "Alpha", got[0].Name)
	assert.Equal(t, "Beta", got[1].Name)

	// Alpha has no heartbeats -> [] and nil uptime, status from the row.
	assert.Equal(t, "down", got[0].Status)
	assert.NotNil(t, got[0].RecentBeats)
	assert.Len(t, got[0].RecentBeats, 0)
	assert.Nil(t, got[0].Uptime24h)

	// Beta: 4 beats, chronological ASC, 75% uptime.
	require.Len(t, got[1].RecentBeats, 4)
	assert.True(t, got[1].RecentBeats[0].CreatedAt.Before(got[1].RecentBeats[3].CreatedAt))
	require.NotNil(t, got[1].Uptime24h)
	assert.InDelta(t, 75.0, *got[1].Uptime24h, 0.0001)
}

func TestUptimeHandler_Summary_BeatsClamp(t *testing.T) {
	base := time.Now().Add(-3 * time.Hour)

	cases := []struct {
		name  string
		id    string
		query string
		want  int
	}{
		{"above_cap", "clamp-cap", "?beats=999", 60},   // capped
		{"below_floor", "clamp-floor", "?beats=0", 1},  // floored
		{"unparseable", "clamp-nan", "?beats=abc", 30}, // unparseable -> default
		{"omitted", "clamp-omit", "", 30},              // omitted -> default
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Fresh handler per sub-test so the 30s cache from a prior call
			// cannot serve a stale series (the cache stores 60-wide and slices,
			// so this is belt-and-braces). The shared in-memory DB persists
			// across sub-tests, hence a distinct monitor id per case.
			r, db := setupUptimeHandlerTest(t)
			require.NoError(t, db.Create(&models.UptimeMonitor{ID: tc.id, Name: "Clamp", Type: "http", URL: "http://c.example", Enabled: true, Status: "up"}).Error)
			bb := make([]models.UptimeHeartbeat, 0, 70)
			for i := 0; i < 70; i++ {
				bb = append(bb, models.UptimeHeartbeat{MonitorID: tc.id, Status: "up", Latency: int64(i), CreatedAt: base.Add(time.Duration(i) * time.Minute)})
			}
			require.NoError(t, db.CreateInBatches(&bb, 50).Error)

			req, _ := http.NewRequest("GET", "/api/v1/uptime/monitors/summary"+tc.query, http.NoBody)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			require.Equal(t, http.StatusOK, w.Code)

			var got []services.MonitorSummary
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
			require.Len(t, got, 1)
			assert.Len(t, got[0].RecentBeats, tc.want)
		})
	}
}

func TestUptimeHandler_Health(t *testing.T) {
	r, _ := setupUptimeHandlerTest(t)

	req, _ := http.NewRequest("GET", "/api/v1/uptime/health", http.NoBody)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))

	for _, k := range []string{"heartbeats_dropped", "checks_enqueue_dropped", "queue_depth", "worker_pool_size"} {
		v, ok := body[k]
		require.Truef(t, ok, "missing key %q", k)
		assert.EqualValuesf(t, 0, v, "nil pool/ingester -> %q is 0", k)
	}
}

func TestUptimeHandler_GetHistory_LimitCap(t *testing.T) {
	r, db := setupUptimeHandlerTest(t)
	require.NoError(t, db.Create(&models.UptimeMonitor{ID: "hist-cap", Name: "Cap"}).Error)

	base := time.Now().Add(-10 * time.Hour)
	rows := make([]models.UptimeHeartbeat, 0, 600)
	for i := 0; i < 600; i++ {
		rows = append(rows, models.UptimeHeartbeat{MonitorID: "hist-cap", Status: "up", Latency: int64(i), CreatedAt: base.Add(time.Duration(i) * time.Second)})
	}
	require.NoError(t, db.CreateInBatches(&rows, 200).Error)

	req, _ := http.NewRequest("GET", "/api/v1/uptime/hist-cap/history?limit=99999", http.NoBody)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var history []models.UptimeHeartbeat
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &history))
	assert.Len(t, history, 500, "limit is hard-capped at 500")
}

func TestUptimeHandler_GetHistory_LimitDefaultAndNonPositive(t *testing.T) {
	r, db := setupUptimeHandlerTest(t)
	require.NoError(t, db.Create(&models.UptimeMonitor{ID: "hist-def", Name: "Def"}).Error)

	base := time.Now().Add(-5 * time.Hour)
	rows := make([]models.UptimeHeartbeat, 0, 80)
	for i := 0; i < 80; i++ {
		rows = append(rows, models.UptimeHeartbeat{MonitorID: "hist-def", Status: "up", Latency: int64(i), CreatedAt: base.Add(time.Duration(i) * time.Second)})
	}
	require.NoError(t, db.CreateInBatches(&rows, 40).Error)

	for _, q := range []string{"", "?limit=0", "?limit=-5", "?limit=abc"} {
		req, _ := http.NewRequest("GET", "/api/v1/uptime/hist-def/history"+q, http.NoBody)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		require.Equalf(t, http.StatusOK, w.Code, "query %q", q)
		var history []models.UptimeHeartbeat
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &history))
		assert.Lenf(t, history, 60, "query %q -> default limit 60", q)
	}
}

func TestUptimeHandler_GetHistory_BeforeCursor(t *testing.T) {
	r, db := setupUptimeHandlerTest(t)
	require.NoError(t, db.Create(&models.UptimeMonitor{ID: "hist-cur", Name: "Cur"}).Error)

	anchor := time.Now().Add(-1 * time.Hour).UTC().Truncate(time.Second)
	// 5 older, 5 newer than the anchor.
	for i := 1; i <= 5; i++ {
		require.NoError(t, db.Create(&models.UptimeHeartbeat{MonitorID: "hist-cur", Status: "up", CreatedAt: anchor.Add(-time.Duration(i) * time.Minute)}).Error)
		require.NoError(t, db.Create(&models.UptimeHeartbeat{MonitorID: "hist-cur", Status: "up", CreatedAt: anchor.Add(time.Duration(i) * time.Minute)}).Error)
	}

	req, _ := http.NewRequest("GET", "/api/v1/uptime/hist-cur/history?before="+anchor.Format(time.RFC3339), http.NoBody)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var history []models.UptimeHeartbeat
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &history))
	require.Len(t, history, 5, "only rows older than the cursor")
	for _, h := range history {
		assert.True(t, h.CreatedAt.Before(anchor), "row %s must be < cursor", h.CreatedAt)
	}
}

func TestUptimeHandler_GetHistory_BadBefore(t *testing.T) {
	r, db := setupUptimeHandlerTest(t)
	require.NoError(t, db.Create(&models.UptimeMonitor{ID: "hist-bad", Name: "Bad"}).Error)

	req, _ := http.NewRequest("GET", "/api/v1/uptime/hist-bad/history?before=not-a-timestamp", http.NoBody)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "RFC3339")
}

func TestUptimeHandler_Summary_DBError(t *testing.T) {
	r, db := setupUptimeHandlerTest(t)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())

	req, _ := http.NewRequest("GET", "/api/v1/uptime/monitors/summary", http.NoBody)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "Failed to build summary")
}

func TestUptimeHandler_Health_WithLivePool(t *testing.T) {
	db := handlers.OpenTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.UptimeMonitor{}, &models.UptimeHeartbeat{}, &models.UptimeHost{}, &models.RemoteServer{}, &models.NotificationProvider{}, &models.Notification{}))

	svc := services.NewUptimeService(db, services.NewNotificationService(db, nil))
	svc.Pool = services.NewUptimeWorkerPool(svc)
	h := handlers.NewUptimeHandler(svc)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/uptime/health", h.Health)

	req := httptest.NewRequest(http.MethodGet, "/uptime/health", http.NoBody)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.EqualValues(t, 30, body["worker_pool_size"], "live pool reports its construction size")
	assert.EqualValues(t, 0, body["queue_depth"])
	assert.EqualValues(t, 0, body["checks_enqueue_dropped"])
	assert.EqualValues(t, 0, body["heartbeats_dropped"])
}
