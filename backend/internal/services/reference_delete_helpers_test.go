package services

import (
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/testutil"
)

// openReferenceEnforcedDB opens an in-memory database with reference enforcement on, migrated with the tables the
// delete and restore paths touch. The parent tables are migrated first so the declared references exist.
func openReferenceEnforcedDB(t *testing.T) *gorm.DB {
	t.Helper()
	db := testutil.OpenMemoryDBWithForeignKeys(t)
	require.NoError(t, db.AutoMigrate(
		&models.SecurityAudit{},
		&models.ProxyGroup{},
		&models.AccessList{},
		&models.SecurityHeaderProfile{},
		&models.SSLCertificate{},
		&models.DNSProvider{},
		&models.DNSProviderCredential{},
		&models.ProxyHost{},
		&models.Location{},
		&models.RedirectionHost{},
		&models.Setting{},
	))
	return db
}
