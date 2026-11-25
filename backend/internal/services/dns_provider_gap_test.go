package services

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestDNSProviderInUseError_MessageNamesHostCount(t *testing.T) {
	err := &DNSProviderInUseError{HostCount: 3}

	assert.Contains(t, err.Error(), "3 host(s)")
	assert.True(t, errors.Is(err, ErrDNSProviderInUse))
}

// openBareDNSDB opens a database with only the tables and columns a test names, so each delete step can be made to fail.
func openBareDNSDB(t *testing.T, statements ...string) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	for _, stmt := range statements {
		require.NoError(t, db.Exec(stmt).Error, stmt)
	}
	return db
}

func TestDeleteDNSProviderRows_StepFailuresAreReported(t *testing.T) {
	t.Run("proxy host count", func(t *testing.T) {
		db := openBareDNSDB(t, "CREATE TABLE proxy_hosts (id INTEGER PRIMARY KEY)")
		err := deleteDNSProviderRows(db, 1)
		assert.ErrorContains(t, err, "count proxy hosts using dns provider 1")
	})

	t.Run("redirection host count", func(t *testing.T) {
		db := openBareDNSDB(t, "CREATE TABLE redirection_hosts (id INTEGER PRIMARY KEY)")
		err := deleteDNSProviderRows(db, 1)
		assert.ErrorContains(t, err, "count redirection hosts using dns provider 1")
	})

	t.Run("credential delete", func(t *testing.T) {
		db := openBareDNSDB(t, "CREATE TABLE dns_provider_credentials (id INTEGER PRIMARY KEY)")
		err := deleteDNSProviderRows(db, 1)
		assert.ErrorContains(t, err, "delete credentials of dns provider 1")
	})

	t.Run("provider delete", func(t *testing.T) {
		db := openBareDNSDB(t,
			"CREATE TABLE dns_providers (id INTEGER PRIMARY KEY)",
			"INSERT INTO dns_providers (id) VALUES (1)",
			"CREATE TRIGGER refuse_dns_delete BEFORE DELETE ON dns_providers BEGIN SELECT RAISE(ABORT, 'refused by test'); END;",
		)
		err := deleteDNSProviderRows(db, 1)
		assert.ErrorContains(t, err, "delete dns provider 1")
	})

	t.Run("unknown provider", func(t *testing.T) {
		db := openBareDNSDB(t, "CREATE TABLE dns_providers (id INTEGER PRIMARY KEY)")
		err := deleteDNSProviderRows(db, 999)
		assert.ErrorIs(t, err, ErrDNSProviderNotFound)
	})
}
