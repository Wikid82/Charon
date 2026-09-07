package services

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- CheckAll / enqueueAllChecks: drop-on-full + query-error branches ---

func TestUptimeService_EnqueueAllChecks_CountsDropsWhenQueueFull(t *testing.T) {
	t.Parallel()
	db := setupUptimeTestDB(t)
	svc := NewUptimeService(db, NewNotificationService(db, nil))
	pool, _ := newTestPool(t, db)
	svc.Pool = pool

	host := models.UptimeHost{Host: "10.2.2.2", Name: "h", Status: "up"}
	require.NoError(t, db.Create(&host).Error)
	seedMonitor(t, db, models.UptimeMonitor{Name: "a", Enabled: true, Interval: 60})
	seedMonitor(t, db, models.UptimeMonitor{Name: "b", Enabled: true, Interval: 60})

	// Saturate the bounded queue so every TryEnqueue fails.
	for i := 0; i < uptimeQueueCapacity; i++ {
		require.True(t, pool.TryEnqueue(UptimeJob{Kind: JobMonitorCheck}))
	}

	enqueued, dropped := svc.CheckAll()
	assert.Equal(t, 0, enqueued)
	assert.Equal(t, 3, dropped, "1 host + 2 monitors all dropped on a full queue")
}

func TestUptimeService_EnqueueAllChecks_SurvivesQueryErrors(t *testing.T) {
	t.Parallel()
	db := setupUptimeTestDB(t)
	svc := NewUptimeService(db, NewNotificationService(db, nil))
	pool, _ := newTestPool(t, db)
	svc.Pool = pool

	require.NoError(t, db.Migrator().DropTable(&models.UptimeHost{}))
	require.NoError(t, db.Migrator().DropTable(&models.UptimeMonitor{}))

	assert.NotPanics(t, func() {
		enqueued, dropped := svc.CheckAll()
		assert.Equal(t, 0, enqueued)
		assert.Equal(t, 0, dropped)
	})
}

func TestUptimeService_CheckAllInline_MonitorQueryError(t *testing.T) {
	t.Parallel()
	db := setupUptimeTestDB(t)
	svc := NewUptimeService(db, NewNotificationService(db, nil)) // no Pool -> inline path
	require.NoError(t, db.Migrator().DropTable(&models.UptimeMonitor{}))

	checked, dropped := svc.CheckAll()
	assert.Equal(t, 0, checked)
	assert.Equal(t, 0, dropped)
}

// --- CheckMonitor: pool-backed enqueue path ---

func TestUptimeService_CheckMonitor_EnqueuesWhenPoolWired(t *testing.T) {
	t.Parallel()
	db := setupUptimeTestDB(t)
	svc := NewUptimeService(db, NewNotificationService(db, nil))
	pool, _ := newTestPool(t, db)
	svc.Pool = pool

	svc.CheckMonitor(models.UptimeMonitor{ID: "m-enq", Type: "http", URL: "http://x"})

	jobs := drainJobs(pool)
	require.Len(t, jobs, 1)
	assert.True(t, jobs[0].Manual)
	assert.Equal(t, "m-enq", jobs[0].Monitor.ID)
}

// --- checkMonitor (inline): legacy retry default + pending seed + persist error ---

func TestUptimeService_CheckMonitorInline_LegacyDefaultsAndPersistError(t *testing.T) {
	t.Parallel()
	db := setupUptimeTestDB(t)
	svc := newTestUptimeService(t, db, NewNotificationService(db, nil))

	// Heartbeat insert will fail inside the ingester flush -> FlushResults error.
	require.NoError(t, db.Migrator().DropTable(&models.UptimeHeartbeat{}))

	// MaxRetries 0 -> legacy 3; Status "" -> "pending".
	assert.NotPanics(t, func() {
		svc.checkMonitor(models.UptimeMonitor{ID: "m-inline", Type: "tcp", URL: "127.0.0.1:9", MaxRetries: 0, Status: ""})
	})
}

// --- checkHost (inline): persist error branch ---

func TestUptimeService_CheckHostInline_PersistError(t *testing.T) {
	t.Parallel()
	db := setupUptimeTestDB(t)
	svc := newTestUptimeService(t, db, NewNotificationService(db, nil))

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer func() { _ = ln.Close() }()
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			_ = c.Close()
		}
	}()

	host := models.UptimeHost{Host: "127.0.0.1", Name: "h", Status: "pending"}
	require.NoError(t, db.Create(&host).Error)
	require.NoError(t, db.Create(&models.UptimeMonitor{
		UptimeHostID: &host.ID, Name: "m", Type: "tcp", URL: ln.Addr().String(), Enabled: true,
	}).Error)

	// The host UPDATE inside the ingester flush will fail.
	require.NoError(t, db.Migrator().DropTable(&models.UptimeHost{}))

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	assert.NotPanics(t, func() { svc.checkHost(ctx, &host) })
}

// --- NotifyMonitorDown / NotifyMonitorUp: thin interface adapters ---

func TestUptimeService_NotifyMonitorDownUp_Adapters(t *testing.T) {
	t.Parallel()
	db := setupUptimeTestDB(t)
	svc := newTestUptimeService(t, db, NewNotificationService(db, nil))

	mon := models.UptimeMonitor{ID: "m-notify", Name: "notify", URL: "http://n"}

	svc.NotifyMonitorDown(context.Background(), mon, "boom", "5m")
	svc.notificationMutex.Lock()
	_, queued := svc.pendingNotifications[""]
	svc.notificationMutex.Unlock()
	assert.True(t, queued, "NotifyMonitorDown routes into the batch queue")

	assert.NotPanics(t, func() {
		svc.NotifyMonitorUp(context.Background(), mon, "5m")
	})
	var recoveries []models.Notification
	require.NoError(t, db.Where("title LIKE ?", "%is UP%").Find(&recoveries).Error)
	assert.NotEmpty(t, recoveries, "NotifyMonitorUp sends a recovery notification")
}

// --- checkOrEnqueue: pool-backed path also freshens the parent host ---

func TestUptimeService_CheckOrEnqueue_PoolPathEnqueuesHostAndMonitor(t *testing.T) {
	t.Parallel()
	db := setupUptimeTestDB(t)
	svc := NewUptimeService(db, NewNotificationService(db, nil))
	pool, _ := newTestPool(t, db)
	svc.Pool = pool

	host := models.UptimeHost{Host: "10.3.3.3", Name: "h", Status: "up"}
	require.NoError(t, db.Create(&host).Error)
	mon := seedMonitor(t, db, models.UptimeMonitor{Name: "m", Enabled: true, Interval: 60, UptimeHostID: &host.ID})

	svc.checkOrEnqueue(mon)

	jobs := drainJobs(pool)
	require.Len(t, jobs, 2)
	assert.Equal(t, JobHostCheck, jobs[0].Kind)
	assert.Equal(t, JobMonitorCheck, jobs[1].Kind)
}

// --- remote-server sync: not-found + unbound-orthrus + load-error branches ---

func TestUptimeService_SyncAndCheckForRemoteServer_UnknownIDIsSilentNoop(t *testing.T) {
	t.Parallel()
	db := setupUptimeTestDB(t)
	svc := NewUptimeService(db, NewNotificationService(db, nil))

	assert.NotPanics(t, func() { svc.SyncAndCheckForRemoteServer(987654) })

	var count int64
	require.NoError(t, db.Model(&models.UptimeMonitor{}).Count(&count).Error)
	assert.Equal(t, int64(0), count)
}

func TestUptimeService_SyncMonitorForRemoteServer_UnknownIDReturnsError(t *testing.T) {
	t.Parallel()
	db := setupUptimeTestDB(t)
	svc := NewUptimeService(db, NewNotificationService(db, nil))

	err := svc.SyncMonitorForRemoteServer(987654)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "load remote server")
}

func TestUptimeService_SyncMonitorForRemoteServer_OrthrusUnbindsAfterCreation(t *testing.T) {
	t.Parallel()
	db := setupUptimeTestDB(t)
	svc := NewUptimeService(db, NewNotificationService(db, nil))

	uuid := "agent-bound-then-gone"
	server := models.RemoteServer{UUID: "rs-unbind", Name: "b", Host: "100.64.0.9", Enabled: true, ConnectionType: models.ConnectionTypeOrthrus, OrthrusAgentUUID: &uuid}
	require.NoError(t, db.Create(&server).Error)
	svc.SyncAndCheckForRemoteServer(server.ID) // creates the orthrus monitor

	var mon models.UptimeMonitor
	require.NoError(t, db.Where("remote_server_id = ?", server.ID).First(&mon).Error)

	// Agent UUID is cleared -> remoteServerMonitorTarget returns ok=false.
	require.NoError(t, db.Model(&models.RemoteServer{}).Where("id = ?", server.ID).Update("orthrus_agent_uuid", "").Error)

	assert.NoError(t, svc.SyncMonitorForRemoteServer(server.ID), "an unbound Orthrus agent is a no-op, not an error")
}

// --- UptimeSyncLoop: constructor + Run ticking / cancel / feature gate ---

func TestUptimeSyncLoop_RunTicksAndStops(t *testing.T) {
	t.Parallel()
	db := setupUptimeTestDB(t)
	svc := NewUptimeService(db, NewNotificationService(db, nil))

	loop := NewUptimeSyncLoop(svc)
	require.NotNil(t, loop)
	loop.tick = 3 * time.Millisecond

	// Feature enabled for the first stretch; SyncMonitors runs against a table
	// that has been dropped, so its error branch is exercised without panic.
	require.NoError(t, db.Migrator().DropTable(&models.UptimeMonitor{}))

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { loop.Run(ctx); close(done) }()
	time.Sleep(40 * time.Millisecond)

	// Flip the feature off so the continue branch runs too.
	require.NoError(t, db.Create(&models.Setting{Key: "feature.uptime.enabled", Value: "false", Type: "bool", Category: "feature"}).Error)
	time.Sleep(20 * time.Millisecond)

	cancel()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("UptimeSyncLoop.Run did not return after cancel")
	}
}
