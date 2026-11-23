package handlers

import (
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/models"
)

// recordingMonitors stands in for the uptime service. RemoveForHost runs the delete callback, as the real one does
// under its lock, and SyncAndCheckForHost reports the host it was asked to re-sync.
type recordingMonitors struct {
	synced chan uint
}

func newRecordingMonitors() *recordingMonitors {
	return &recordingMonitors{synced: make(chan uint, 8)}
}

func (m *recordingMonitors) SyncMonitorForHost(uint) error { return nil }

func (m *recordingMonitors) SyncAndCheckForHost(hostID uint) { m.synced <- hostID }

func (m *recordingMonitors) RemoveForHost(_ uint, deleteParent func() error) error {
	return deleteParent()
}

func TestProxyHostDeleteRefusedResyncsTheMonitor(t *testing.T) {
	db := openHostRestoreDB(t)
	host := seedHost(t, db, "refused-monitor")
	f := newHostRestoreFixture(t, db, string(models.RoleAdmin), &refusingApplier{err: errRefused})
	monitors := newRecordingMonitors()
	f.handler.uptime = monitors

	w := f.do(t, http.MethodDelete, "/api/v1/proxy-hosts/"+host.UUID, nil)

	require.Equal(t, http.StatusInternalServerError, w.Code, w.Body.String())
	select {
	case id := <-monitors.synced:
		assert.Equal(t, host.ID, id, "the restored host gets a fresh monitor")
	case <-time.After(2 * time.Second):
		t.Fatal("the restored host was not re-synced with the uptime service")
	}
}

func TestProxyHostDeleteStorageErrorAnswersTheFixedText(t *testing.T) {
	db := openHostRestoreDB(t)
	host := seedHost(t, db, "delete-storage")
	require.NoError(t, db.Exec(`CREATE TRIGGER refuse_host_delete BEFORE DELETE ON proxy_hosts
		BEGIN SELECT RAISE(ABORT, 'disk on fire'); END`).Error)
	f := newHostRestoreFixture(t, db, string(models.RoleAdmin), &refusingApplier{})

	w := f.do(t, http.MethodDelete, "/api/v1/proxy-hosts/"+host.UUID, nil)

	require.Equal(t, http.StatusInternalServerError, w.Code, w.Body.String())
	body := decodeBody(t, w)
	assert.Equal(t, "Could not delete the proxy host.", body["error"])
	assert.NotContains(t, w.Body.String(), "disk on fire", "the storage detail stays out of the answer")
}

func TestRedirectionHostListStorageErrorAnswers500(t *testing.T) {
	db := openHostRestoreDB(t)
	require.NoError(t, db.Migrator().DropTable(&models.RedirectionHost{}))
	f := newHostRestoreFixture(t, db, string(models.RoleAdmin), &refusingApplier{})

	w := f.do(t, http.MethodGet, "/api/v1/redirection-hosts", nil)

	require.Equal(t, http.StatusInternalServerError, w.Code, w.Body.String())
	assert.Equal(t, "Could not load the redirection hosts.", decodeBody(t, w)["error"])
}

func TestRedirectionHostCreateRefusedLeavesNoRow(t *testing.T) {
	db := openHostRestoreDB(t)
	f := newHostRestoreFixture(t, db, string(models.RoleAdmin), &refusingApplier{err: errRefused})

	w := f.do(t, http.MethodPost, "/api/v1/redirection-hosts", map[string]any{
		"domain_names": "refused-redirect.restore.example.com",
		"target_url":   "https://target.restore.example.com",
		"status_code":  301,
	})

	require.Equal(t, http.StatusInternalServerError, w.Code, w.Body.String())
	assert.Equal(t, "proxy_apply_failed", decodeBody(t, w)["error_code"])
	var count int64
	require.NoError(t, db.Model(&models.RedirectionHost{}).Where("domain_names = ?", "refused-redirect.restore.example.com").Count(&count).Error)
	assert.Zero(t, count, "the refused host is removed again")
}

func TestProxyHostBulkDuplicateUUIDsAreChangedOnce(t *testing.T) {
	db := openHostRestoreDB(t)
	first := seedHost(t, db, "bulk-dup-1")
	second := seedHost(t, db, "bulk-dup-2")
	f := newHostRestoreFixture(t, db, string(models.RoleAdmin), &refusingApplier{})

	w := f.do(t, http.MethodPut, "/api/v1/proxy-hosts/bulk-update-acl", map[string]any{
		"host_uuids":     []string{first.UUID, first.UUID, second.UUID},
		"access_list_id": 7,
	})

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, float64(2), decodeBody(t, w)["updated"], "a repeated uuid is changed once")
}

func TestProxyHostBulkDuplicateUUIDsRestoreEveryHost(t *testing.T) {
	db := openHostRestoreDB(t)
	first := seedHost(t, db, "bulk-dup-restore-1")
	second := seedHost(t, db, "bulk-dup-restore-2")
	f := newHostRestoreFixture(t, db, string(models.RoleAdmin), &refusingApplier{err: errRefused})

	w := f.do(t, http.MethodPut, "/api/v1/proxy-hosts/bulk-update-acl", map[string]any{
		"host_uuids":     []string{first.UUID, first.UUID, second.UUID},
		"access_list_id": 7,
	})

	require.Equal(t, http.StatusInternalServerError, w.Code, w.Body.String())
	assert.Nil(t, storedHost(t, db, first.ID).AccessListID, "the repeated host is restored, not reported as superseded")
	assert.Nil(t, storedHost(t, db, second.ID).AccessListID)
}

func TestUniqueHostUUIDsKeepsTheFirstOccurrence(t *testing.T) {
	assert.Equal(t, []string{"a", "b", "c"}, uniqueHostUUIDs([]string{"a", "b", "a", "c", "b"}))
	assert.Empty(t, uniqueHostUUIDs(nil))
}

// TestProxyHostBulkWritersRaceFree runs two bulk writers over the same hosts while every apply is refused. Each answer
// is a refusal or a success, and the race detector sees the restores and the snapshots of both writers.
func TestProxyHostBulkWritersRaceFree(t *testing.T) {
	db := openHostRestoreDB(t)
	first := seedHost(t, db, "bulk-race-1")
	second := seedHost(t, db, "bulk-race-2")
	f := newHostRestoreFixture(t, db, string(models.RoleAdmin), &refusingApplier{err: errRefused})

	var wg sync.WaitGroup
	codes := make(chan int, 4)
	for _, accessList := range []int{7, 8} {
		wg.Add(1)
		go func(accessList int) {
			defer wg.Done()
			w := f.do(t, http.MethodPut, "/api/v1/proxy-hosts/bulk-update-acl", map[string]any{
				"host_uuids":     []string{first.UUID, second.UUID},
				"access_list_id": accessList,
			})
			codes <- w.Code
		}(accessList)
	}
	wg.Wait()
	close(codes)

	for code := range codes {
		assert.Contains(t, []int{http.StatusOK, http.StatusInternalServerError}, code, "every bulk write is answered")
	}
	// Which refused value a racing restore leaves behind depends on the interleaving, so only the set of possible
	// outcomes is asserted: the original value or one of the two refused values.
	allowed := map[any]bool{nil: true, uint(7): true, uint(8): true}
	for _, id := range []uint{first.ID, second.ID} {
		stored := storedHost(t, db, id).AccessListID
		var value any
		if stored != nil {
			value = *stored
		}
		assert.True(t, allowed[value], "host %d holds an unexpected access list %v", id, value)
	}
}
