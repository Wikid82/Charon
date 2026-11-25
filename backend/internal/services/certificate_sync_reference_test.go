package services

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/models"
)

// TestCertificateSync_KeepsStaleACMERowStillUsedByHost covers a stale ACME row that a host references: the row stays,
// and an unreferenced stale row is removed.
func TestCertificateSync_KeepsStaleACMERowStillUsedByHost(t *testing.T) {
	db := openReferenceEnforcedDB(t)

	used := models.SSLCertificate{UUID: uuid.NewString(), Name: "used", Domains: "used.example.com", Provider: "letsencrypt-staging"}
	unused := models.SSLCertificate{UUID: uuid.NewString(), Name: "unused", Domains: "unused.example.com", Provider: "letsencrypt-staging"}
	require.NoError(t, db.Create(&used).Error)
	require.NoError(t, db.Create(&unused).Error)
	usedID := used.ID
	require.NoError(t, db.Create(&models.ProxyHost{
		UUID: uuid.NewString(), DomainNames: "used.example.com", ForwardHost: "10.0.0.2", ForwardPort: 80,
		CertificateID: &usedID,
	}).Error)

	cs := newTestCertificateService(t.TempDir(), db)
	require.NoError(t, cs.SyncFromDisk())

	var remaining []models.SSLCertificate
	require.NoError(t, db.Find(&remaining).Error)
	require.Len(t, remaining, 1)
	assert.Equal(t, used.ID, remaining[0].ID)
}
