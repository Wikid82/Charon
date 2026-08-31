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

func remoteServerUptimeRouter(t *testing.T, wireUptime bool) (*gin.Engine, *gorm.DB) {
	t.Helper()
	db := handlers.OpenTestDB(t)
	require.NoError(t, db.AutoMigrate(
		&models.RemoteServer{}, &models.UptimeMonitor{}, &models.UptimeHeartbeat{},
		&models.UptimeHost{}, &models.Setting{}, &models.Notification{}, &models.NotificationProvider{},
	))

	// Cap the pool at one physical connection, mirroring production's
	// configurePool (internal/database/database.go). Create fires
	// `go SyncAndCheckForRemoteServer`, whose writes would otherwise contend for
	// SQLite's shared-cache table lock with the delete path and surface as
	// non-deterministic "database table is locked" 500s — noise unrelated to the
	// mutex-coordination behaviour these tests exercise. With one connection the
	// two paths serialise through Go's pool queue, exactly as in production.
	sqlDB, sqlErr := db.DB()
	require.NoError(t, sqlErr)
	sqlDB.SetMaxOpenConns(1)

	ns := services.NewNotificationService(db, nil)
	h := handlers.NewRemoteServerHandler(services.NewRemoteServerService(db), ns)
	if wireUptime {
		h.SetUptimeService(services.NewUptimeService(db, ns))
	}

	gin.SetMode(gin.TestMode)
	r := gin.New()
	g := r.Group("/api/v1/remote-servers")
	g.POST("", h.Create)
	g.PUT("/:uuid", h.Update)
	g.DELETE("/:uuid", h.Delete)
	g.GET("/:uuid", h.Get)
	return r, db
}

func createRemoteServer(t *testing.T, r *gin.Engine, db *gorm.DB, payload map[string]any) models.RemoteServer {
	t.Helper()
	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/remote-servers", bytes.NewBuffer(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	var out models.RemoteServer
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &out))
	// RemoteServer.ID is json:"-"; reload from the DB to get the real primary key.
	var full models.RemoteServer
	require.NoError(t, db.Where("uuid = ?", out.UUID).First(&full).Error)
	return full
}

func TestRemoteServerHandler_Create_DrivesTargetedMonitorSync(t *testing.T) {
	r, db := remoteServerUptimeRouter(t, true)

	srv := createRemoteServer(t, r, db, map[string]any{
		"name": "edge", "host": "127.0.0.1", "port": 1, "connection_type": "direct", "enabled": true,
	})

	require.Eventually(t, func() bool {
		var c int64
		db.Model(&models.UptimeMonitor{}).Where("remote_server_id = ?", srv.ID).Count(&c)
		return c == 1
	}, 15*time.Second, 25*time.Millisecond, "Create should spawn SyncAndCheckForRemoteServer")

	var mon models.UptimeMonitor
	require.NoError(t, db.Where("remote_server_id = ?", srv.ID).First(&mon).Error)
	assert.Equal(t, "tcp", mon.Type)
	assert.Equal(t, "127.0.0.1:1", mon.URL)
}

func TestRemoteServerHandler_Update_SyncsLinkedMonitor(t *testing.T) {
	r, db := remoteServerUptimeRouter(t, true)
	srv := createRemoteServer(t, r, db, map[string]any{
		"name": "edge", "host": "127.0.0.1", "port": 1, "connection_type": "direct", "enabled": true,
	})
	require.Eventually(t, func() bool {
		var c int64
		db.Model(&models.UptimeMonitor{}).Where("remote_server_id = ?", srv.ID).Count(&c)
		return c == 1
	}, 15*time.Second, 25*time.Millisecond)

	payload := map[string]any{"name": "edge-renamed", "host": "127.0.0.1", "port": 2, "connection_type": "direct", "enabled": true}
	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest(http.MethodPut, "/api/v1/remote-servers/"+srv.UUID, bytes.NewBuffer(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	require.Eventually(t, func() bool {
		var mon models.UptimeMonitor
		if err := db.Where("remote_server_id = ?", srv.ID).First(&mon).Error; err != nil {
			return false
		}
		return mon.Name == "edge-renamed" && mon.URL == "127.0.0.1:2"
	}, 15*time.Second, 25*time.Millisecond, "Update should spawn SyncMonitorForRemoteServer")
}

func TestRemoteServerHandler_Delete_CleansUpLinkedMonitors(t *testing.T) {
	r, db := remoteServerUptimeRouter(t, true)
	srv := createRemoteServer(t, r, db, map[string]any{
		"name": "edge", "host": "127.0.0.1", "port": 1, "connection_type": "direct", "enabled": true,
	})
	require.Eventually(t, func() bool {
		var c int64
		db.Model(&models.UptimeMonitor{}).Where("remote_server_id = ?", srv.ID).Count(&c)
		return c == 1
	}, 15*time.Second, 25*time.Millisecond)

	req, _ := http.NewRequest(http.MethodDelete, "/api/v1/remote-servers/"+srv.UUID, http.NoBody)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusNoContent, w.Code)

	var c int64
	require.NoError(t, db.Model(&models.UptimeMonitor{}).Where("remote_server_id = ?", srv.ID).Count(&c).Error)
	assert.Equal(t, int64(0), c, "Delete cleans up linked monitors inline before the server row is removed")
}

// TestRemoteServerHandler_Delete_RaceWithInFlightCheck exercises the window the
// fix closes: Create spawns `go SyncAndCheckForRemoteServer`, which creates the
// monitor row and then runs an inline check that writes a heartbeat + a monitor
// column update. Deleting the moment the row appears (while that check is still
// in flight) previously let the check re-touch / recreate the monitor after the
// delete's cleanup ran, leaving an orphan. Looping many iterations makes a
// regression fail reliably, especially under `-race`.
func TestRemoteServerHandler_Delete_RaceWithInFlightCheck(t *testing.T) {
	r, db := remoteServerUptimeRouter(t, true)

	const iterations = 25
	for i := 0; i < iterations; i++ {
		srv := createRemoteServer(t, r, db, map[string]any{
			"name": "edge", "host": "127.0.0.1", "port": 1, "connection_type": "direct", "enabled": true,
		})

		require.Eventually(t, func() bool {
			var c int64
			db.Model(&models.UptimeMonitor{}).Where("remote_server_id = ?", srv.ID).Count(&c)
			return c == 1
		}, 15*time.Second, 2*time.Millisecond, "iteration %d: Create should spawn the monitor sync", i)

		req, _ := http.NewRequest(http.MethodDelete, "/api/v1/remote-servers/"+srv.UUID, http.NoBody)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		require.Equal(t, http.StatusNoContent, w.Code)

		var monitors int64
		require.NoError(t, db.Model(&models.UptimeMonitor{}).Where("remote_server_id = ?", srv.ID).Count(&monitors).Error)
		require.Equalf(t, int64(0), monitors, "iteration %d: linked monitor survived / was resurrected by the in-flight check", i)

		// The delete must not settle into a resurrected monitor a beat later either.
		require.Neverf(t, func() bool {
			var c int64
			db.Model(&models.UptimeMonitor{}).Where("remote_server_id = ?", srv.ID).Count(&c)
			return c != 0
		}, 150*time.Millisecond, 10*time.Millisecond, "iteration %d: monitor reappeared after delete returned", i)

		var heartbeats int64
		require.NoError(t, db.Model(&models.UptimeHeartbeat{}).Count(&heartbeats).Error)
		require.Equalf(t, int64(0), heartbeats, "iteration %d: orphan heartbeat rows left behind", i)
	}
}

// TestRemoteServerHandler_Delete_BeforeInFlightCheckCreatesMonitor covers the
// other side of the same race: delete arrives while the SyncAndCheckForRemoteServer
// goroutine is still running and has not created the monitor yet. Because Delete
// now removes the RemoteServer row before purging monitors, that in-flight sync
// must observe the missing server row and create nothing.
func TestRemoteServerHandler_Delete_BeforeInFlightCheckCreatesMonitor(t *testing.T) {
	r, db := remoteServerUptimeRouter(t, true)

	for i := 0; i < 25; i++ {
		srv := createRemoteServer(t, r, db, map[string]any{
			"name": "edge", "host": "127.0.0.1", "port": 1, "connection_type": "direct", "enabled": true,
		})

		// No wait — delete immediately, racing the just-spawned sync goroutine.
		req, _ := http.NewRequest(http.MethodDelete, "/api/v1/remote-servers/"+srv.UUID, http.NoBody)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		require.Equal(t, http.StatusNoContent, w.Code)

		require.Neverf(t, func() bool {
			var c int64
			db.Model(&models.UptimeMonitor{}).Where("remote_server_id = ?", srv.ID).Count(&c)
			return c != 0
		}, 250*time.Millisecond, 10*time.Millisecond, "iteration %d: in-flight sync recreated the monitor after delete", i)
	}
}

func TestRemoteServerHandler_Create_NilUptimeService_NoPanic(t *testing.T) {
	r, db := remoteServerUptimeRouter(t, false)

	srv := createRemoteServer(t, r, db, map[string]any{
		"name": "edge", "host": "127.0.0.1", "port": 1, "connection_type": "direct", "enabled": true,
	})

	var c int64
	require.NoError(t, db.Model(&models.UptimeMonitor{}).Where("remote_server_id = ?", srv.ID).Count(&c).Error)
	assert.Equal(t, int64(0), c, "no uptime service wired -> no monitor, no panic")
}
