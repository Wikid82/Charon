package services

import (
	"testing"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- CreateMonitor: write-time interval resolution ---------------------------

func TestCreateMonitor_ClampsIntervalAtWriteTime(t *testing.T) {
	db := setupUptimeTestDB(t)
	svc := NewUptimeService(db, nil)

	t.Run("zero resolves to hardcoded default when no setting", func(t *testing.T) {
		m, err := svc.CreateMonitor("z", "http://z.example", "http", 0, 0)
		require.NoError(t, err)
		assert.Equal(t, 60, m.Interval)
	})

	t.Run("sub-floor is raised to 30", func(t *testing.T) {
		m, err := svc.CreateMonitor("low", "http://low.example", "http", 10, 0)
		require.NoError(t, err)
		assert.Equal(t, 30, m.Interval)
	})

	t.Run("valid value is stored verbatim", func(t *testing.T) {
		m, err := svc.CreateMonitor("ok", "http://ok.example", "http", 120, 0)
		require.NoError(t, err)
		assert.Equal(t, 120, m.Interval)
	})

	t.Run("zero resolves to the configured default", func(t *testing.T) {
		seedUptimeSetting(t, db, "uptime.default_interval_seconds", "45")
		svc.uptimeCfg.forceRefresh()
		m, err := svc.CreateMonitor("d", "http://d.example", "http", 0, 0)
		require.NoError(t, err)
		assert.Equal(t, 45, m.Interval)
	})
}

// --- UpdateMonitor: interval floor -----------------------------------------

func TestUpdateMonitor_RejectsSubFloorInterval(t *testing.T) {
	db := setupUptimeTestDB(t)
	svc := NewUptimeService(db, nil)

	m, err := svc.CreateMonitor("u", "http://u.example", "http", 60, 0)
	require.NoError(t, err)

	t.Run("float64 (JSON-decoded) below floor is rejected", func(t *testing.T) {
		_, err := svc.UpdateMonitor(m.ID, map[string]any{"interval": float64(5)})
		assert.ErrorIs(t, err, ErrIntervalTooLow)
	})

	t.Run("int below floor is rejected", func(t *testing.T) {
		_, err := svc.UpdateMonitor(m.ID, map[string]any{"interval": 29})
		assert.ErrorIs(t, err, ErrIntervalTooLow)
	})

	t.Run("at floor is accepted", func(t *testing.T) {
		updated, err := svc.UpdateMonitor(m.ID, map[string]any{"interval": 30})
		require.NoError(t, err)
		assert.Equal(t, 30, updated.Interval)
	})

	t.Run("above floor is accepted", func(t *testing.T) {
		updated, err := svc.UpdateMonitor(m.ID, map[string]any{"interval": float64(300)})
		require.NoError(t, err)
		assert.Equal(t, 300, updated.Interval)
	})
}

// --- S3: auto-create paths honour the configured default -------------------

func TestSyncMonitors_AutoCreateHonoursConfiguredDefault(t *testing.T) {
	db := setupUptimeTestDB(t)
	seedUptimeSetting(t, db, "uptime.default_interval_seconds", "45")
	svc := NewUptimeService(db, nil)

	require.NoError(t, db.Create(&models.ProxyHost{
		Name:        "app",
		DomainNames: "app.example.com",
		ForwardHost: "10.0.0.5",
		ForwardPort: 8080,
	}).Error)

	require.NoError(t, svc.SyncMonitors())

	var m models.UptimeMonitor
	require.NoError(t, db.Where("upstream_host = ?", "10.0.0.5").First(&m).Error)
	assert.Equal(t, 45, m.Interval, "proxy-host monitor must inherit uptime.default_interval_seconds, not a hardcoded 60")
}

func TestSyncMonitors_RemoteServerAutoCreateHonoursConfiguredDefault(t *testing.T) {
	db := setupUptimeTestDB(t)
	seedUptimeSetting(t, db, "uptime.default_interval_seconds", "45")
	svc := NewUptimeService(db, nil)

	require.NoError(t, db.Create(&models.RemoteServer{
		Name:    "edge",
		Host:    "192.168.9.9",
		Port:    22,
		Enabled: true,
	}).Error)

	require.NoError(t, svc.SyncMonitors())

	var m models.UptimeMonitor
	require.NoError(t, db.Where("upstream_host = ?", "192.168.9.9").First(&m).Error)
	assert.Equal(t, 45, m.Interval)
}

func TestSyncAndCheckForHost_AutoCreateHonoursConfiguredDefault(t *testing.T) {
	db := setupUptimeTestDB(t)
	seedUptimeSetting(t, db, "uptime.default_interval_seconds", "45")
	// Feature flag must be on for SyncAndCheckForHost to proceed.
	seedUptimeSetting(t, db, "feature.uptime.enabled", "true")
	svc := newTestUptimeService(t, db, NewNotificationService(db, nil))
	svc.uptimeCfg.forceRefresh()

	host := models.ProxyHost{
		Name:        "svc",
		DomainNames: "svc.example.com",
		ForwardHost: "10.1.2.3",
		ForwardPort: 9000,
	}
	require.NoError(t, db.Create(&host).Error)

	svc.SyncAndCheckForHost(host.ID)

	var m models.UptimeMonitor
	require.NoError(t, db.Where("proxy_host_id = ?", host.ID).First(&m).Error)
	assert.Equal(t, 45, m.Interval)
}
