package database

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/logger"
)

// TestRepairReferences_RedirectionHostLosesDNSProviderAndChallenge covers R11: a redirection host whose DNS provider
// no longer exists keeps its row, the provider is cleared and the DNS challenge is turned off with it. A host without a
// provider is left alone.
func TestRepairReferences_RedirectionHostLosesDNSProviderAndChallenge(t *testing.T) {
	db := openReferenceSchemaDB(t)
	require.NoError(t, db.Exec("PRAGMA foreign_keys = OFF").Error)
	require.NoError(t, db.Exec("INSERT INTO redirection_hosts (id, dns_provider_id, use_dns_challenge) VALUES (1, 555, 1), (2, NULL, 1)").Error)
	require.NoError(t, db.Exec("PRAGMA foreign_keys = ON").Error)

	results, err := RepairReferences(db)
	require.NoError(t, err)
	require.NotEmpty(t, results)

	var broken struct {
		DNSProviderID   *int64
		UseDNSChallenge int64
	}
	require.NoError(t, db.Raw("SELECT dns_provider_id, use_dns_challenge FROM redirection_hosts WHERE id = 1").Scan(&broken).Error)
	assert.Nil(t, broken.DNSProviderID, "the missing provider is cleared")
	assert.Zero(t, broken.UseDNSChallenge, "the DNS challenge is turned off with it")

	var untouched struct {
		DNSProviderID   *int64
		UseDNSChallenge int64
	}
	require.NoError(t, db.Raw("SELECT dns_provider_id, use_dns_challenge FROM redirection_hosts WHERE id = 2").Scan(&untouched).Error)
	assert.Nil(t, untouched.DNSProviderID)
	assert.Equal(t, int64(1), untouched.UseDNSChallenge, "a host without a missing provider keeps its challenge setting")
}

// TestMigrateWithIntegrity_RepairLogNamesRuleAndIDsNotRowContents covers the repair log: each changed rule is logged
// with its table, column, action, count and ids, and the DNS challenge count for R7/R11. No row content is written.
func TestMigrateWithIntegrity_RepairLogNamesRuleAndIDsNotRowContents(t *testing.T) {
	db := openReferenceSchemaDB(t)
	require.NoError(t, db.Exec("ALTER TABLE proxy_hosts ADD COLUMN domain_names TEXT").Error)
	require.NoError(t, db.Exec("PRAGMA foreign_keys = OFF").Error)
	require.NoError(t, db.Exec("INSERT INTO proxy_hosts (id, domain_names, certificate_id) VALUES (7, 'secret-host.example.com', 77)").Error)
	require.NoError(t, db.Exec("INSERT INTO redirection_hosts (id, dns_provider_id, use_dns_challenge) VALUES (9, 555, 1)").Error)
	require.NoError(t, db.Exec("PRAGMA foreign_keys = ON").Error)

	var buf bytes.Buffer
	logger.Init(true, &buf)
	t.Cleanup(func() { logger.Init(false, nil) })

	require.NoError(t, MigrateWithIntegrity(db, func(*gorm.DB) error { return nil }))

	logs := buf.String()
	assert.Contains(t, logs, "Repaired database references")
	assert.Contains(t, logs, "table=proxy_hosts")
	assert.Contains(t, logs, "column=certificate_id")
	assert.Contains(t, logs, "table=redirection_hosts")
	assert.Contains(t, logs, "column=dns_provider_id")
	assert.Contains(t, logs, "count=1")
	assert.Contains(t, logs, "ids=7")
	assert.Contains(t, logs, "ids=9")
	assert.Contains(t, logs, "dns_challenge_disabled=1", "the R11 repair reports the DNS challenge it turned off")
	assert.NotContains(t, logs, "secret-host.example.com", "row contents must never reach the log")
}

// TestMigrateWithIntegrity_PauseFailureStopsBeforeMigrating covers the fail-closed start-up path: when the statement that
// pauses reference enforcement fails, the error is returned and the migration does not run.
func TestMigrateWithIntegrity_PauseFailureStopsBeforeMigrating(t *testing.T) {
	db := openReferenceSchemaDB(t)
	simulated := errors.New("simulated pause failure")
	const pauseStatement = "PRAGMA foreign_keys = OFF"
	callbackName := "test:fail-pause-statement"
	require.NoError(t, db.Callback().Raw().Before("gorm:raw").Register(callbackName, func(tx *gorm.DB) {
		if strings.Contains(tx.Statement.SQL.String(), pauseStatement) {
			_ = tx.AddError(simulated)
		}
	}))
	t.Cleanup(func() { _ = db.Callback().Raw().Remove(callbackName) })
	migrated := false

	err := MigrateWithIntegrity(db, func(*gorm.DB) error {
		migrated = true
		return nil
	})

	require.ErrorIs(t, err, simulated)
	assert.Contains(t, err.Error(), "pause reference enforcement")
	assert.False(t, migrated, "the migration must not run when enforcement cannot be paused")
}

// TestMigrateWithIntegrity_ClosedDatabaseStopsBeforeMigrating covers the same fail-closed rule when the connection
// itself is unusable: nothing is migrated and the failure is returned.
func TestMigrateWithIntegrity_ClosedDatabaseStopsBeforeMigrating(t *testing.T) {
	db := openReferenceSchemaDB(t)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())
	migrated := false

	err = MigrateWithIntegrity(db, func(*gorm.DB) error {
		migrated = true
		return nil
	})

	require.Error(t, err)
	assert.False(t, migrated)
}
