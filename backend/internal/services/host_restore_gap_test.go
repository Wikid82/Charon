package services

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/models"
)

// seedUptimeMonitor adds a monitor linked to hostID, with one heartbeat, to a database that has the uptime tables.
func seedUptimeMonitor(t *testing.T, db *gorm.DB, hostID uint, monitorID string) {
	t.Helper()
	require.NoError(t, db.Create(&models.UptimeMonitor{ID: monitorID, ProxyHostID: &hostID, Name: "monitor"}).Error)
	require.NoError(t, db.Create(&models.UptimeHeartbeat{MonitorID: monitorID, Status: "up", CreatedAt: time.Now()}).Error)
}

func TestDeleteHostMonitors_ListFailureIsReported(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:list-failure?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	// A monitor table without the host link cannot be searched by host.
	require.NoError(t, db.Exec("CREATE TABLE uptime_monitors (id TEXT PRIMARY KEY)").Error)

	err = deleteHostMonitors(db, 1)

	assert.ErrorContains(t, err, "list monitors of proxy host 1")
}

func TestDeleteHostMonitors_NoMonitorsIsNoop(t *testing.T) {
	db := setupProxyHostTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.UptimeMonitor{}, &models.UptimeHeartbeat{}))

	assert.NoError(t, deleteHostMonitors(db, 42))
}

func TestDeleteHostMonitors_DeleteFailuresAreReported(t *testing.T) {
	for _, tc := range []struct {
		name, table, want string
	}{
		{name: "heartbeats", table: "uptime_heartbeats", want: "delete heartbeats of proxy host"},
		{name: "monitors", table: "uptime_monitors", want: "delete monitors of proxy host"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db := setupProxyHostTestDB(t)
			require.NoError(t, db.AutoMigrate(&models.UptimeMonitor{}, &models.UptimeHeartbeat{}))
			host, _ := seedRestoreHost(t, NewProxyHostService(db), "monitor-"+tc.name, "monitor-"+tc.name+".example.com")
			seedUptimeMonitor(t, db, host.ID, "mon-"+tc.name)
			refuseOn(t, db, "DELETE", tc.table)

			err := deleteHostMonitors(db, host.ID)

			assert.ErrorContains(t, err, tc.want)
		})
	}
}

func TestRemoveCreated_HeartbeatFailureKeepsTheHost(t *testing.T) {
	db := setupProxyHostTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.UptimeMonitor{}, &models.UptimeHeartbeat{}))
	service := NewProxyHostService(db)
	host, written := seedRestoreHost(t, service, "remove-hb", "remove-hb.example.com")
	seedUptimeMonitor(t, db, host.ID, "mon-remove-hb")
	refuseOn(t, db, "DELETE", "uptime_heartbeats")

	err := service.RemoveCreated(written)

	assert.ErrorContains(t, err, "delete heartbeats of proxy host")
	_, snapErr := service.Snapshot(host.ID)
	assert.NoError(t, snapErr, "the host is still stored after a failed removal")
}

func TestClearMissingProxyHostReferences_LookupFailureIsReported(t *testing.T) {
	db := setupProxyHostTestDB(t)
	require.NoError(t, db.Exec("DROP TABLE ssl_certificates").Error)
	certificate := uint(5)
	host := models.ProxyHost{CertificateID: &certificate}

	// The certificate table is absent from this database, so the parent lookup cannot run.
	err := clearMissingProxyHostReferences(db, &host)

	assert.ErrorContains(t, err, "ssl_certificates")
}

func TestProxyHostReinsertDeleted_ClearFailureIsReported(t *testing.T) {
	db := setupProxyHostTestDB(t)
	require.NoError(t, db.Exec("DROP TABLE ssl_certificates").Error)
	service := NewProxyHostService(db)
	host, _ := seedRestoreHost(t, service, "reinsert-clear", "reinsert-clear.example.com")
	snap, err := service.Snapshot(host.ID)
	require.NoError(t, err)
	require.NoError(t, db.Delete(&models.ProxyHost{}, host.ID).Error)
	certificate := uint(5)
	snap.Host.CertificateID = &certificate

	err = service.ReinsertDeleted(snap)

	assert.ErrorContains(t, err, "clear references of proxy host")
}

func TestRedirectionHost_ClearMissingReferencesLookupFailureIsReported(t *testing.T) {
	db := setupRedirectionHostTestDB(t)
	require.NoError(t, db.Exec("DROP TABLE ssl_certificates").Error)
	certificate := uint(5)
	host := models.RedirectionHost{CertificateID: &certificate}

	err := clearMissingRedirectionReferences(db, &host)

	assert.ErrorContains(t, err, "ssl_certificates")
}

func TestRedirectionHostReinsertDeleted_ClearFailureIsReported(t *testing.T) {
	db := setupRedirectionHostTestDB(t)
	require.NoError(t, db.Exec("DROP TABLE ssl_certificates").Error)
	certificate := uint(5)
	snap := models.RedirectionHost{ID: 9, UUID: "redir-reinsert-clear", DomainNames: "redir-clear.example.com", CertificateID: &certificate}

	err := NewRedirectionHostService(db).ReinsertDeleted(snap)

	assert.ErrorContains(t, err, "clear references of redirection host 9")
}
