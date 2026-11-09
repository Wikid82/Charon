package services

import (
	"testing"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/stretchr/testify/require"
)

func TestAccessListService_CountryCodesSavedUpperCase(t *testing.T) {
	db := setupTestDB(t)
	service := NewAccessListService(db)

	for _, aclType := range []string{"geo_whitelist", "geo_blacklist"} {
		t.Run(aclType, func(t *testing.T) {
			acl := &models.AccessList{
				Name:         "Mixed case " + aclType,
				Type:         aclType,
				CountryCodes: " us, ca ,de ",
				Enabled:      true,
			}
			require.NoError(t, service.Create(acl))

			var stored models.AccessList
			require.NoError(t, db.First(&stored, acl.ID).Error)
			require.Equal(t, "US,CA,DE", stored.CountryCodes)
		})
	}

	t.Run("update normalizes country codes", func(t *testing.T) {
		acl := &models.AccessList{Name: "Update case", Type: "geo_blacklist", CountryCodes: "FR", Enabled: true}
		require.NoError(t, service.Create(acl))

		updates := &models.AccessList{Name: "Update case", Type: "geo_blacklist", CountryCodes: "es,it", Enabled: true}
		require.NoError(t, service.Update(acl.ID, updates))

		var stored models.AccessList
		require.NoError(t, db.First(&stored, acl.ID).Error)
		require.Equal(t, "ES,IT", stored.CountryCodes)
	})
}
