package services

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/models"
)

// refuseOn installs a SQLite trigger that aborts every statement of kind ("INSERT", "UPDATE" or "DELETE") on table,
// so a restore step that writes there fails the way a storage error would.
func refuseOn(t *testing.T, db *gorm.DB, kind, table string) {
	t.Helper()
	name := "refuse_" + kind + "_" + table
	require.NoError(t, db.Exec("CREATE TRIGGER "+name+" BEFORE "+kind+" ON "+table+
		" BEGIN SELECT RAISE(ABORT, 'refused by test'); END;").Error)
}

// setupAllHostTablesTestDB migrates proxy hosts, their locations and redirection hosts in one private database.
func setupAllHostTablesTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.ProxyHost{}, &models.Location{}, &models.RedirectionHost{}))
	return db
}

func TestProxyHostSnapshotFailsWhenLocationsCannotBeRead(t *testing.T) {
	t.Parallel()
	db := setupProxyHostTestDB(t)
	service := NewProxyHostService(db)
	host, written := seedRestoreHost(t, service, "snap-noloc", "snap-noloc.example.com")
	require.NoError(t, db.Migrator().DropTable(&models.Location{}))

	_, err := service.Snapshot(host.ID)
	assert.ErrorContains(t, err, "read locations")

	err = service.RemoveCreated(written)
	assert.ErrorContains(t, err, "read locations")
	assert.NotErrorIs(t, err, ErrRestoreSuperseded)
}

func TestProxyHostRestoreUpdatedOfAMissingRowFails(t *testing.T) {
	t.Parallel()
	db := setupProxyHostTestDB(t)
	service := NewProxyHostService(db)
	host, written := seedRestoreHost(t, service, "restore-gone", "restore-gone.example.com")
	require.NoError(t, db.Delete(&models.ProxyHost{}, host.ID).Error)

	err := service.RestoreUpdated(written, written)

	assert.ErrorContains(t, err, "read proxy host")
}

func TestProxyHostRestoreUpdatedReportsAStorageErrorOnTheRow(t *testing.T) {
	t.Parallel()
	db := setupProxyHostTestDB(t)
	service := NewProxyHostService(db)
	_, written := seedRestoreHost(t, service, "restore-rowfail", "restore-rowfail.example.com")
	refuseOn(t, db, "UPDATE", "proxy_hosts")

	err := service.RestoreUpdated(written, written)

	assert.ErrorContains(t, err, "restore proxy host")
}

func TestProxyHostRestoreUpdatedReportsAStorageErrorOnTheLocations(t *testing.T) {
	t.Parallel()
	db := setupProxyHostTestDB(t)
	service := NewProxyHostService(db)
	_, written := seedRestoreHost(t, service, "restore-locfail", "restore-locfail.example.com")
	refuseOn(t, db, "INSERT", "locations")

	err := service.RestoreUpdated(written, written)

	assert.ErrorContains(t, err, "insert location")
	// The whole restore runs in one transaction, so the locations are still there.
	after, snapErr := service.Snapshot(written.Host.ID)
	require.NoError(t, snapErr)
	assert.Equal(t, []string{"/a", "/b"}, locationPaths(after.Locations))
}

func TestProxyHostRemoveCreatedReportsStorageErrors(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, table, want string
	}{
		{name: "locations", table: "locations", want: "remove locations"},
		{name: "row", table: "proxy_hosts", want: "remove proxy host"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			db := setupProxyHostTestDB(t)
			service := NewProxyHostService(db)
			host, written := seedRestoreHost(t, service, "remove-"+tc.name, "remove-"+tc.name+".example.com")
			refuseOn(t, db, "DELETE", tc.table)

			err := service.RemoveCreated(written)

			assert.ErrorContains(t, err, tc.want)
			_, snapErr := service.Snapshot(host.ID)
			assert.NoError(t, snapErr, "the host is still stored after a failed removal")
		})
	}
}

func TestProxyHostReinsertDeletedSkipsWhenARedirectionHostClaimsTheDomain(t *testing.T) {
	t.Parallel()
	db := setupAllHostTablesTestDB(t)
	service := NewProxyHostService(db)
	host := models.ProxyHost{UUID: "reinsert-redir", DomainNames: "reinsert-redir.example.com", ForwardHost: "10.0.0.1", ForwardPort: 80}
	require.NoError(t, service.CreateAs(&host, true))
	before, err := service.Snapshot(host.ID)
	require.NoError(t, err)
	require.NoError(t, service.Delete(host.ID))
	claim := models.RedirectionHost{UUID: "redir-claim", DomainNames: "reinsert-redir.example.com", TargetURL: "https://a.example.com", StatusCode: 301}
	require.NoError(t, db.Create(&claim).Error)

	err = service.ReinsertDeleted(before)

	require.ErrorIs(t, err, ErrRestoreSuperseded)
	_, err = service.GetByID(host.ID)
	assert.Error(t, err, "the deleted host stays deleted")
}

func TestProxyHostReinsertDeletedReportsStorageErrors(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, kind, table, want string
	}{
		{name: "clear-locations", kind: "DELETE", table: "locations", want: "clear locations"},
		{name: "insert-row", kind: "INSERT", table: "proxy_hosts", want: "re-insert proxy host"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			db := setupProxyHostTestDB(t)
			service := NewProxyHostService(db)
			host, _ := seedRestoreHost(t, service, "reinsert-"+tc.name, "reinsert-"+tc.name+".example.com")
			before, err := service.Snapshot(host.ID)
			require.NoError(t, err)
			require.NoError(t, service.Delete(host.ID))
			refuseOn(t, db, tc.kind, tc.table)

			err = service.ReinsertDeleted(before)

			assert.ErrorContains(t, err, tc.want)
			assert.NotErrorIs(t, err, ErrRestoreSuperseded)
		})
	}
}

func TestProxyHostReinsertDeletedReportsAFailedLookup(t *testing.T) {
	t.Parallel()
	db := setupProxyHostTestDB(t)
	service := NewProxyHostService(db)
	host, _ := seedRestoreHost(t, service, "reinsert-lookup", "reinsert-lookup.example.com")
	before, err := service.Snapshot(host.ID)
	require.NoError(t, err)
	require.NoError(t, service.Delete(host.ID))
	require.NoError(t, db.Migrator().DropColumn(&models.ProxyHost{}, "uuid"))

	err = service.ReinsertDeleted(before)

	assert.ErrorContains(t, err, "check proxy host")
}

func TestProxyHostRestoreColumnRestoresEveryUnchangedRow(t *testing.T) {
	t.Parallel()
	db := setupProxyHostTestDB(t)
	service := NewProxyHostService(db)
	host, _ := seedRestoreHost(t, service, "column-all", "column-all.example.com")
	current, err := service.GetByID(host.ID)
	require.NoError(t, err)
	group := uint(7)
	current.ProxyGroupID = &group
	_, written, err := service.UpdateAsSnapshot(current, true)
	require.NoError(t, err)

	err = service.RestoreColumn("proxy_group_id", []ColumnChange{{ID: host.ID, Old: nil, WrittenAt: written.Host.UpdatedAt}})

	require.NoError(t, err)
	restored, err := service.GetByID(host.ID)
	require.NoError(t, err)
	assert.Nil(t, restored.ProxyGroupID)
}

func TestProxyHostRestoreColumnCountsADeletedRowAsKept(t *testing.T) {
	t.Parallel()
	db := setupProxyHostTestDB(t)
	service := NewProxyHostService(db)
	host, written := seedRestoreHost(t, service, "column-gone", "column-gone.example.com")
	require.NoError(t, db.Delete(&models.ProxyHost{}, host.ID).Error)

	err := service.RestoreColumn("access_list_id", []ColumnChange{{ID: host.ID, WrittenAt: written.Host.UpdatedAt}})

	require.ErrorIs(t, err, ErrRestoreSuperseded)
}

func TestProxyHostRestoreColumnReportsStorageErrors(t *testing.T) {
	t.Parallel()
	t.Run("read", func(t *testing.T) {
		t.Parallel()
		db := setupProxyHostTestDB(t)
		service := NewProxyHostService(db)
		host, written := seedRestoreHost(t, service, "column-read", "column-read.example.com")
		// A blob is not a time: reading the row fails while the table and its write lock still work.
		require.NoError(t, db.Exec("UPDATE proxy_hosts SET updated_at = X'FF00'").Error)

		err := service.RestoreColumn("access_list_id", []ColumnChange{{ID: host.ID, WrittenAt: written.Host.UpdatedAt}})

		assert.ErrorContains(t, err, "read proxy host")
	})
	t.Run("write", func(t *testing.T) {
		t.Parallel()
		db := setupProxyHostTestDB(t)
		service := NewProxyHostService(db)
		host, written := seedRestoreHost(t, service, "column-write", "column-write.example.com")
		refuseOn(t, db, "UPDATE", "proxy_hosts")

		err := service.RestoreColumn("access_list_id", []ColumnChange{{ID: host.ID, WrittenAt: written.Host.UpdatedAt}})

		assert.ErrorContains(t, err, "restore access_list_id")
	})
}

func seedRedirectionRestoreHost(t *testing.T, service *RedirectionHostService, uuid string) models.RedirectionHost {
	t.Helper()
	host := models.RedirectionHost{UUID: uuid, DomainNames: uuid + ".example.com", TargetURL: "https://a.example.com", StatusCode: 301}
	written, err := service.CreateAsSnapshot(&host, true)
	require.NoError(t, err)
	return written
}

func TestRedirectionHostCreateAsSnapshotRefusesATakenDomain(t *testing.T) {
	t.Parallel()
	service := NewRedirectionHostService(setupDomainUniquenessTestDB(t))
	seedRedirectionRestoreHost(t, service, "redir-taken")

	again := models.RedirectionHost{UUID: "redir-taken-2", DomainNames: "redir-taken.example.com", TargetURL: "https://b.example.com", StatusCode: 301}
	_, err := service.CreateAsSnapshot(&again, true)

	assert.Error(t, err)
}

func TestRedirectionHostCreateAsSnapshotReportsAStorageError(t *testing.T) {
	t.Parallel()
	db := setupDomainUniquenessTestDB(t)
	service := NewRedirectionHostService(db)
	refuseOn(t, db, "INSERT", "redirection_hosts")

	host := models.RedirectionHost{UUID: "redir-insfail", DomainNames: "redir-insfail.example.com", TargetURL: "https://a.example.com", StatusCode: 301}
	_, err := service.CreateAsSnapshot(&host, true)

	assert.ErrorContains(t, err, "refused by test")
}

func TestRedirectionHostUpdateAsSnapshotReportsAFailedRead(t *testing.T) {
	t.Parallel()
	db := setupDomainUniquenessTestDB(t)
	service := NewRedirectionHostService(db)
	written := seedRedirectionRestoreHost(t, service, "redir-readfail")
	// Text in an integer column cannot be scanned: reading the stored row fails.
	require.NoError(t, db.Exec("UPDATE redirection_hosts SET status_code = 'not-a-number'").Error)

	changed := written
	changed.TargetURL = "https://b.example.com"
	_, _, err := service.UpdateAsSnapshot(&changed, true)

	assert.ErrorContains(t, err, "read redirection host")
}

func TestRedirectionHostUpdateAsSnapshotOfAMissingRowWritesNothing(t *testing.T) {
	t.Parallel()
	service := NewRedirectionHostService(setupDomainUniquenessTestDB(t))
	missing := models.RedirectionHost{ID: 4242, UUID: "redir-missing", DomainNames: "redir-missing.example.com", TargetURL: "https://a.example.com", StatusCode: 301}

	before, written, err := service.UpdateAsSnapshot(&missing, true)

	require.NoError(t, err)
	assert.Zero(t, before.ID)
	assert.Zero(t, written.ID)
}

func TestRedirectionHostUpdateAsSnapshotReportsAStorageError(t *testing.T) {
	t.Parallel()
	db := setupDomainUniquenessTestDB(t)
	service := NewRedirectionHostService(db)
	written := seedRedirectionRestoreHost(t, service, "redir-updfail")
	refuseOn(t, db, "UPDATE", "redirection_hosts")

	changed := written
	changed.TargetURL = "https://b.example.com"
	_, _, err := service.UpdateAsSnapshot(&changed, true)

	assert.ErrorContains(t, err, "refused by test")
}

func TestRedirectionHostRestoreUpdatedOfAMissingRowFails(t *testing.T) {
	t.Parallel()
	db := setupDomainUniquenessTestDB(t)
	service := NewRedirectionHostService(db)
	written := seedRedirectionRestoreHost(t, service, "redir-restore-gone")
	require.NoError(t, db.Delete(&models.RedirectionHost{}, written.ID).Error)

	err := service.RestoreUpdated(written, written)

	assert.ErrorContains(t, err, "read redirection host")
}

func TestRedirectionHostRemoveCreatedOfAMissingRowIsSuperseded(t *testing.T) {
	t.Parallel()
	db := setupDomainUniquenessTestDB(t)
	service := NewRedirectionHostService(db)
	written := seedRedirectionRestoreHost(t, service, "redir-remove-gone")
	require.NoError(t, db.Delete(&models.RedirectionHost{}, written.ID).Error)

	err := service.RemoveCreated(written)

	require.ErrorIs(t, err, ErrRestoreSuperseded)
}

func TestRedirectionHostRemoveCreatedReportsAFailedLookup(t *testing.T) {
	t.Parallel()
	db := setupDomainUniquenessTestDB(t)
	service := NewRedirectionHostService(db)
	written := seedRedirectionRestoreHost(t, service, "redir-remove-lookup")
	// Text in an integer column cannot be scanned: reading the row fails while the table still works.
	require.NoError(t, db.Exec("UPDATE redirection_hosts SET status_code = 'not-a-number'").Error)

	err := service.RemoveCreated(written)

	assert.ErrorContains(t, err, "read redirection host")
	assert.NotErrorIs(t, err, ErrRestoreSuperseded)
}

func TestRedirectionHostReinsertDeletedSkipsWhenADomainIsClaimed(t *testing.T) {
	t.Parallel()
	t.Run("by another redirection host", func(t *testing.T) {
		t.Parallel()
		db := setupDomainUniquenessTestDB(t)
		service := NewRedirectionHostService(db)
		written := seedRedirectionRestoreHost(t, service, "redir-claim-same")
		require.NoError(t, db.Delete(&models.RedirectionHost{}, written.ID).Error)
		claim := models.RedirectionHost{UUID: "redir-claimer", DomainNames: written.DomainNames, TargetURL: "https://c.example.com", StatusCode: 301}
		require.NoError(t, db.Create(&claim).Error)

		require.ErrorIs(t, service.ReinsertDeleted(written), ErrRestoreSuperseded)
	})
	t.Run("by a proxy host", func(t *testing.T) {
		t.Parallel()
		db := setupDomainUniquenessTestDB(t)
		service := NewRedirectionHostService(db)
		written := seedRedirectionRestoreHost(t, service, "redir-claim-cross")
		require.NoError(t, db.Delete(&models.RedirectionHost{}, written.ID).Error)
		claim := models.ProxyHost{UUID: "proxy-claimer", DomainNames: written.DomainNames, ForwardHost: "10.0.0.1", ForwardPort: 80}
		require.NoError(t, db.Create(&claim).Error)

		require.ErrorIs(t, service.ReinsertDeleted(written), ErrRestoreSuperseded)
	})
}

func TestRedirectionHostReinsertDeletedReportsStorageErrors(t *testing.T) {
	t.Parallel()
	t.Run("lookup", func(t *testing.T) {
		t.Parallel()
		db := setupDomainUniquenessTestDB(t)
		service := NewRedirectionHostService(db)
		written := seedRedirectionRestoreHost(t, service, "redir-reinsert-lookup")
		require.NoError(t, db.Delete(&models.RedirectionHost{}, written.ID).Error)
		require.NoError(t, db.Migrator().DropColumn(&models.RedirectionHost{}, "uuid"))

		assert.ErrorContains(t, service.ReinsertDeleted(written), "check redirection host")
	})
	t.Run("insert", func(t *testing.T) {
		t.Parallel()
		db := setupDomainUniquenessTestDB(t)
		service := NewRedirectionHostService(db)
		written := seedRedirectionRestoreHost(t, service, "redir-reinsert-insert")
		require.NoError(t, db.Delete(&models.RedirectionHost{}, written.ID).Error)
		refuseOn(t, db, "INSERT", "redirection_hosts")

		err := service.ReinsertDeleted(written)

		assert.ErrorContains(t, err, "re-insert redirection host")
		assert.NotErrorIs(t, err, ErrRestoreSuperseded)
	})
}
