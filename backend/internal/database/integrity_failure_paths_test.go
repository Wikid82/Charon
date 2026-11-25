package database

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// failStatements makes every statement of the given kind fail with boom when match returns true. kind is one of
// raw, row, query, update or delete, the gorm callback processors that run them. The hook is removed at cleanup.
func failStatements(t *testing.T, db *gorm.DB, kind string, match func(sql, table string) bool, boom error) {
	t.Helper()
	name := "test:fail-" + kind + "-" + t.Name()
	hook := func(tx *gorm.DB) {
		if match(tx.Statement.SQL.String(), tx.Statement.Table) {
			_ = tx.AddError(boom)
		}
	}
	var err error
	switch kind {
	case "raw":
		err = db.Callback().Raw().Before("gorm:raw").Register(name, hook)
	case "row":
		err = db.Callback().Row().Before("gorm:row").Register(name, hook)
	case "query":
		err = db.Callback().Query().Before("gorm:query").Register(name, hook)
	case "update":
		err = db.Callback().Update().Before("gorm:update").Register(name, hook)
	case "delete":
		err = db.Callback().Delete().Before("gorm:delete").Register(name, hook)
	default:
		t.Fatalf("unknown statement kind %q", kind)
	}
	require.NoError(t, err)
	t.Cleanup(func() {
		switch kind {
		case "raw":
			_ = db.Callback().Raw().Remove(name)
		case "row":
			_ = db.Callback().Row().Remove(name)
		case "query":
			_ = db.Callback().Query().Remove(name)
		case "update":
			_ = db.Callback().Update().Remove(name)
		case "delete":
			_ = db.Callback().Delete().Remove(name)
		}
	})
}

func TestRepairAction_UnknownValueNamesItself(t *testing.T) {
	assert.Equal(t, "unknown", RepairAction(99).String())
	assert.Equal(t, "clear", RepairClear.String())
	assert.Equal(t, "delete_row", RepairDeleteRow.String())
}

func TestSetForeignKeys_ReadBackFailureIsReported(t *testing.T) {
	db := openReferenceSchemaDB(t)
	boom := errors.New("simulated read failure")
	failStatements(t, db, "row", func(sql, _ string) bool {
		return strings.Contains(sql, "PRAGMA foreign_keys") && !strings.Contains(sql, "=")
	}, boom)

	err := db.Connection(func(conn *gorm.DB) error { return setForeignKeys(conn, true) })

	require.ErrorIs(t, err, boom)
	assert.Contains(t, err.Error(), "read reference enforcement")
}

func TestSetForeignKeys_SetFailureIsReported(t *testing.T) {
	db := openReferenceSchemaDB(t)
	boom := errors.New("simulated set failure")
	failStatements(t, db, "raw", func(sql, _ string) bool {
		return strings.Contains(sql, "PRAGMA foreign_keys = OFF")
	}, boom)

	err := db.Connection(PauseForeignKeys)

	require.ErrorIs(t, err, boom)
	assert.Contains(t, err.Error(), "set reference enforcement OFF")
}

func TestMigrateWithIntegrity_RepairErrorIsLoggedAndMigrationSucceeds(t *testing.T) {
	db := openReferenceSchemaDB(t)
	require.NoError(t, db.Exec("PRAGMA foreign_keys = OFF").Error)
	require.NoError(t, db.Exec("INSERT INTO proxy_hosts (id, certificate_id) VALUES (1, 77)").Error)
	require.NoError(t, db.Exec("PRAGMA foreign_keys = ON").Error)
	failStatements(t, db, "update", func(_, table string) bool { return table == TableProxyHosts },
		errors.New("simulated repair failure"))

	err := MigrateWithIntegrity(db, func(*gorm.DB) error { return nil })

	require.NoError(t, err, "repair problems are logged and do not fail the migration")
	assert.Equal(t, 1, foreignKeysOn(t, db))
}

func TestReportReferenceCheck_CheckFailureDoesNotPanic(t *testing.T) {
	db := openReferenceSchemaDB(t)
	failStatements(t, db, "row", func(sql, _ string) bool {
		return strings.Contains(sql, "foreign_key_check")
	}, errors.New("simulated check failure"))

	assert.NotPanics(t, func() { ReportReferenceCheck(db, nil) })
}

func TestCheckReferences_QueryFailureReturnsError(t *testing.T) {
	db := openReferenceSchemaDB(t)
	boom := errors.New("simulated check failure")
	failStatements(t, db, "row", func(sql, _ string) bool {
		return strings.Contains(sql, "foreign_key_check")
	}, boom)

	_, err := CheckReferences(db)

	require.ErrorIs(t, err, boom)
	assert.Contains(t, err.Error(), "run foreign_key_check")
}

func TestRepairRule_MissingChildTableIsSkipped(t *testing.T) {
	db := openReferenceSchemaDB(t)
	require.NoError(t, db.Exec("DROP TABLE dns_provider_credentials").Error)

	results, err := RepairReferences(db)

	require.NoError(t, err)
	for _, r := range results {
		assert.NotEqual(t, "dns_provider_credentials", r.Rule.Child)
	}
}

func TestRepairReferences_SampleOfIDsIsCapped(t *testing.T) {
	db := openReferenceSchemaDB(t)
	require.NoError(t, db.Exec("PRAGMA foreign_keys = OFF").Error)
	for i := 1; i <= maxLoggedIDs+2; i++ {
		require.NoError(t, db.Exec("INSERT INTO locations (id, proxy_host_id) VALUES (?, 99)", i).Error)
	}
	require.NoError(t, db.Exec("PRAGMA foreign_keys = ON").Error)

	results, err := RepairReferences(db)

	require.NoError(t, err)
	var found bool
	for _, r := range results {
		if r.Rule.Child == TableLocations {
			found = true
			assert.Equal(t, maxLoggedIDs+2, r.Count)
			assert.Len(t, r.SampleIDs, maxLoggedIDs)
		}
	}
	assert.True(t, found, "the locations rule should report its rows")
}

func TestRepairReferences_LookupFailureRollsBack(t *testing.T) {
	db := openReferenceSchemaDB(t)
	require.NoError(t, db.Exec("PRAGMA foreign_keys = OFF").Error)
	require.NoError(t, db.Exec("INSERT INTO locations (id, proxy_host_id) VALUES (1, 99)").Error)
	require.NoError(t, db.Exec("PRAGMA foreign_keys = ON").Error)
	boom := errors.New("simulated lookup failure")
	failStatements(t, db, "query", func(_, table string) bool { return table == TableLocations }, boom)

	_, err := RepairReferences(db)

	require.ErrorIs(t, err, boom)
	assert.Contains(t, err.Error(), "find broken")
}

func TestRepairReferences_ClearFailureRollsBack(t *testing.T) {
	db := openReferenceSchemaDB(t)
	require.NoError(t, db.Exec("PRAGMA foreign_keys = OFF").Error)
	require.NoError(t, db.Exec("INSERT INTO proxy_hosts (id, certificate_id) VALUES (1, 77)").Error)
	require.NoError(t, db.Exec("PRAGMA foreign_keys = ON").Error)
	boom := errors.New("simulated clear failure")
	failStatements(t, db, "update", func(_, table string) bool { return table == TableProxyHosts }, boom)

	_, err := RepairReferences(db)

	require.ErrorIs(t, err, boom)
	assert.Contains(t, err.Error(), "clear proxy_hosts.certificate_id")
}

func TestRepairReferences_DeleteFailuresAreReported(t *testing.T) {
	seedBrokenMonitor := func(t *testing.T) *gorm.DB {
		db := openReferenceSchemaDB(t)
		require.NoError(t, db.Exec("PRAGMA foreign_keys = OFF").Error)
		require.NoError(t, db.Exec("INSERT INTO uptime_monitors (id, proxy_host_id) VALUES ('m1', 99)").Error)
		require.NoError(t, db.Exec("INSERT INTO uptime_heartbeats (id, monitor_id) VALUES (1, 'm1')").Error)
		require.NoError(t, db.Exec("PRAGMA foreign_keys = ON").Error)
		return db
	}

	t.Run("dependent heartbeat delete", func(t *testing.T) {
		db := seedBrokenMonitor(t)
		failStatements(t, db, "delete", func(_, table string) bool { return table == TableUptimeHeartbeats },
			errors.New("simulated heartbeat failure"))
		_, err := RepairReferences(db)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "delete uptime_heartbeats rows of removed uptime_monitors")
	})

	t.Run("child row delete", func(t *testing.T) {
		db := openReferenceSchemaDB(t)
		require.NoError(t, db.Exec("PRAGMA foreign_keys = OFF").Error)
		require.NoError(t, db.Exec("INSERT INTO locations (id, proxy_host_id) VALUES (1, 99)").Error)
		require.NoError(t, db.Exec("PRAGMA foreign_keys = ON").Error)
		failStatements(t, db, "delete", func(_, table string) bool { return table == TableLocations },
			errors.New("simulated row failure"))
		_, err := RepairReferences(db)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "delete locations rows")
	})
}

func TestRepairReferences_UnknownActionIsRefused(t *testing.T) {
	db := openReferenceSchemaDB(t)
	require.NoError(t, db.Exec("PRAGMA foreign_keys = OFF").Error)
	require.NoError(t, db.Exec("INSERT INTO locations (id, proxy_host_id) VALUES (1, 99)").Error)
	require.NoError(t, db.Exec("PRAGMA foreign_keys = ON").Error)

	orig := ReferenceRules
	ReferenceRules = []ReferenceRule{{Child: TableLocations, Column: "proxy_host_id", Parent: TableProxyHosts, ParentColumn: "id", Action: RepairAction(99)}}
	t.Cleanup(func() { ReferenceRules = orig })

	_, err := RepairReferences(db)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown repair action")
}

func TestClearMissingReferences_TypedNilPointerIsUnset(t *testing.T) {
	db := openReferenceParents(t)
	var missing *uint

	cleared, err := ClearMissingReferences(db, TableProxyHosts, map[string]any{"certificate_id": missing})

	require.NoError(t, err)
	assert.Empty(t, cleared)
}

func TestMigrateWithIntegrity_ResumeFailureIsLoggedNotReturned(t *testing.T) {
	db := openReferenceSchemaDB(t)

	err := MigrateWithIntegrity(db, func(conn *gorm.DB) error {
		// An open transaction keeps enforcement from turning back on, which is the failure being covered.
		return conn.Exec("BEGIN").Error
	})

	require.NoError(t, err)
	assert.Equal(t, 1, foreignKeysOn(t, db), "a replacement connection enforces references")
}

func TestReportReferenceCheck_CleanDatabaseLogsDebugSummary(t *testing.T) {
	db := openReferenceSchemaDB(t)
	assert.NotPanics(t, func() { ReportReferenceCheck(db, nil) })
}
