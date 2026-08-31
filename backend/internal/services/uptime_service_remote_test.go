package services

import (
	"testing"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSyncAndCheckForRemoteServer_CreatesTCPMonitorWithConfiguredDefaultInterval(t *testing.T) {
	t.Parallel()
	db := setupUptimeTestDB(t)
	require.NoError(t, db.Create(&models.Setting{Key: "uptime.default_interval_seconds", Value: "45", Type: "int", Category: "uptime"}).Error)
	svc := NewUptimeService(db, NewNotificationService(db, nil))

	server := models.RemoteServer{UUID: "rs-1", Name: "box", Host: "127.0.0.1", Port: 1, Enabled: true, ConnectionType: models.ConnectionTypeDirect}
	require.NoError(t, db.Create(&server).Error)

	svc.SyncAndCheckForRemoteServer(server.ID)

	var mon models.UptimeMonitor
	require.NoError(t, db.Where("remote_server_id = ?", server.ID).First(&mon).Error)
	assert.Equal(t, "tcp", mon.Type)
	assert.Equal(t, "127.0.0.1:1", mon.URL)
	assert.Equal(t, 45, mon.Interval, "auto-created monitor honours uptime.default_interval_seconds, never a hardcoded 60")
	assert.Equal(t, "box", mon.Name)
}

func TestSyncAndCheckForRemoteServer_HTTPSchemeProducesHTTPMonitor(t *testing.T) {
	t.Parallel()
	db := setupUptimeTestDB(t)
	svc := NewUptimeService(db, NewNotificationService(db, nil))

	server := models.RemoteServer{UUID: "rs-2", Name: "web", Host: "127.0.0.1", Port: 1, Scheme: "https", Enabled: true, ConnectionType: models.ConnectionTypeDirect}
	require.NoError(t, db.Create(&server).Error)

	svc.SyncAndCheckForRemoteServer(server.ID)

	var mon models.UptimeMonitor
	require.NoError(t, db.Where("remote_server_id = ?", server.ID).First(&mon).Error)
	assert.Equal(t, "https", mon.Type)
	assert.Equal(t, "https://127.0.0.1:1", mon.URL)
	assert.Equal(t, 60, mon.Interval, "no config row -> hardcoded default")
}

func TestSyncAndCheckForRemoteServer_OrthrusWithoutAgentUUID_SilentNoMonitor(t *testing.T) {
	t.Parallel()
	db := setupUptimeTestDB(t)
	svc := NewUptimeService(db, NewNotificationService(db, nil))

	server := models.RemoteServer{UUID: "rs-3", Name: "orphan", Host: "100.64.0.1", Port: 0, Enabled: true, ConnectionType: models.ConnectionTypeOrthrus}
	require.NoError(t, db.Create(&server).Error)

	svc.SyncAndCheckForRemoteServer(server.ID) // must not panic, must not error, must not create a row

	var count int64
	require.NoError(t, db.Model(&models.UptimeMonitor{}).Where("remote_server_id = ?", server.ID).Count(&count).Error)
	assert.Equal(t, int64(0), count, "an unbound Orthrus agent yields no monitor until the UUID binds")
}

func TestSyncAndCheckForRemoteServer_OrthrusWithAgentUUID_CreatesOrthrusMonitor(t *testing.T) {
	t.Parallel()
	db := setupUptimeTestDB(t)
	svc := NewUptimeService(db, NewNotificationService(db, nil))

	uuid := "agent-xyz"
	server := models.RemoteServer{UUID: "rs-4", Name: "bound", Host: "100.64.0.2", Enabled: true, ConnectionType: models.ConnectionTypeOrthrus, OrthrusAgentUUID: &uuid}
	require.NoError(t, db.Create(&server).Error)

	svc.SyncAndCheckForRemoteServer(server.ID)

	var mon models.UptimeMonitor
	require.NoError(t, db.Where("remote_server_id = ?", server.ID).First(&mon).Error)
	assert.Equal(t, "orthrus", mon.Type)
	assert.Equal(t, uuid, mon.URL)
}

func TestSyncAndCheckForRemoteServer_FeatureDisabled_Noop(t *testing.T) {
	t.Parallel()
	db := setupUptimeTestDB(t)
	require.NoError(t, db.Create(&models.Setting{Key: "feature.uptime.enabled", Value: "false", Type: "bool", Category: "feature"}).Error)
	svc := NewUptimeService(db, NewNotificationService(db, nil))

	server := models.RemoteServer{UUID: "rs-5", Name: "x", Host: "127.0.0.1", Port: 1, Enabled: true, ConnectionType: models.ConnectionTypeDirect}
	require.NoError(t, db.Create(&server).Error)

	svc.SyncAndCheckForRemoteServer(server.ID)

	var count int64
	require.NoError(t, db.Model(&models.UptimeMonitor{}).Count(&count).Error)
	assert.Equal(t, int64(0), count)
}

func TestSyncMonitorForRemoteServer_UpdatesExistingMonitor(t *testing.T) {
	t.Parallel()
	db := setupUptimeTestDB(t)
	svc := NewUptimeService(db, NewNotificationService(db, nil))

	server := models.RemoteServer{UUID: "rs-6", Name: "before", Host: "127.0.0.1", Port: 1, Enabled: true, ConnectionType: models.ConnectionTypeDirect}
	require.NoError(t, db.Create(&server).Error)
	svc.SyncAndCheckForRemoteServer(server.ID)

	// Mutate the server, then sync.
	server.Name = "after"
	server.Port = 2
	server.Enabled = false
	require.NoError(t, db.Save(&server).Error)

	require.NoError(t, svc.SyncMonitorForRemoteServer(server.ID))

	var mon models.UptimeMonitor
	require.NoError(t, db.Where("remote_server_id = ?", server.ID).First(&mon).Error)
	assert.Equal(t, "after", mon.Name)
	assert.Equal(t, "127.0.0.1:2", mon.URL)
	assert.False(t, mon.Enabled)
}

func TestSyncMonitorForRemoteServer_NoMonitor_NoError(t *testing.T) {
	t.Parallel()
	db := setupUptimeTestDB(t)
	svc := NewUptimeService(db, NewNotificationService(db, nil))

	server := models.RemoteServer{UUID: "rs-7", Name: "lonely", Host: "127.0.0.1", Port: 1, Enabled: true, ConnectionType: models.ConnectionTypeDirect}
	require.NoError(t, db.Create(&server).Error)

	assert.NoError(t, svc.SyncMonitorForRemoteServer(server.ID))
}

func TestCheckAll_WithPool_EnqueuesEnabledMonitorsAndHosts(t *testing.T) {
	t.Parallel()
	db := setupUptimeTestDB(t)
	svc := NewUptimeService(db, NewNotificationService(db, nil))
	pool, _ := newTestPool(t, db)
	svc.Pool = pool

	host := models.UptimeHost{Host: "10.1.1.1", Name: "h", Status: "up"}
	require.NoError(t, db.Create(&host).Error)
	seedMonitor(t, db, models.UptimeMonitor{Name: "a", Enabled: true, Interval: 60})
	seedMonitor(t, db, models.UptimeMonitor{Name: "b", Enabled: true, Interval: 60})
	seedMonitor(t, db, models.UptimeMonitor{Name: "off", Enabled: false, Interval: 60})

	enqueued, dropped := svc.CheckAll()

	assert.Equal(t, 0, dropped)
	assert.Equal(t, 3, enqueued, "1 host + 2 enabled monitors")

	jobs := drainJobs(pool)
	require.Len(t, jobs, 3)
}
