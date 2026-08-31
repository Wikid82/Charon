package handlers_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/api/handlers"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/services"
)

func poolBackedUptimeRouter(t *testing.T) (*gin.Engine, *services.UptimeService) {
	t.Helper()
	db := handlers.OpenTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.UptimeMonitor{}, &models.UptimeHeartbeat{}, &models.UptimeHost{}, &models.RemoteServer{}, &models.NotificationProvider{}, &models.Notification{}))

	svc := services.NewUptimeService(db, services.NewNotificationService(db, nil))
	svc.Pool = services.NewUptimeWorkerPool(svc)

	h := handlers.NewUptimeHandler(svc)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/uptime/:id/check", h.CheckMonitor)
	return r, svc
}

func TestUptimeHandler_CheckMonitor_EnqueuesOntoPool(t *testing.T) {
	r, svc := poolBackedUptimeRouter(t)

	mon := models.UptimeMonitor{Name: "m", Type: "http", URL: "http://127.0.0.1:1", Enabled: true, Interval: 60}
	require.NoError(t, svc.DB.Create(&mon).Error)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/uptime/"+mon.ID+"/check", http.NoBody)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var body map[string]string
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, "Check enqueued", body["message"])
	assert.Equal(t, 1, svc.Pool.QueueDepth(), "job landed on the bounded pool queue")
}

func TestUptimeHandler_CheckMonitor_FullQueueReturns503(t *testing.T) {
	r, svc := poolBackedUptimeRouter(t)

	for svc.Pool.TryEnqueue(services.UptimeJob{Kind: services.JobMonitorCheck}) {
	}

	mon := models.UptimeMonitor{Name: "m", Type: "http", URL: "http://127.0.0.1:1", Enabled: true, Interval: 60}
	require.NoError(t, svc.DB.Create(&mon).Error)

	// Cancelled request context so Enqueue returns at once rather than blocking
	// its full 2s timeout.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	req := httptest.NewRequest(http.MethodPost, "/uptime/"+mon.ID+"/check", http.NoBody).WithContext(ctx)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	assert.Contains(t, w.Body.String(), "check queue is full")
}
