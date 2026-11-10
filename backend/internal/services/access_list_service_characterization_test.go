package services

import (
	"errors"
	"testing"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// insertCharacterizationACL stores an access list directly, bypassing service validation, so rows that Create would
// reject (for example an enabled allow-list with no rules, or invalid ip_rules JSON) can still be checked.
func insertCharacterizationACL(t *testing.T, db *gorm.DB, acl *models.AccessList) uint {
	t.Helper()
	acl.Name = "characterization"
	acl.UUID = uuid.New().String()
	require.NoError(t, db.Create(acl).Error)
	return acl.ID
}

// loadedGeoIPFake returns a GeoIP service whose lookups answer with isoCode, as if a database were loaded.
func loadedGeoIPFake(isoCode string) *GeoIPService {
	return &GeoIPService{dbPath: "/fixture/country.mmdb", db: &fakeGeoIPReader{isoCode: isoCode}}
}

// characterizationService builds an AccessListService that uses geo for country lookups; nil means no service is set.
func characterizationService(t *testing.T, geo *GeoIPService) *AccessListService {
	t.Helper()
	svc := NewAccessListService(setupTestDB(t))
	svc.SetGeoIPService(geo)
	return svc
}

func TestAccessListServiceTestIPCharacterization(t *testing.T) {
	t.Run("disabled list allows any type", func(t *testing.T) {
		for _, listType := range []string{"whitelist", "blacklist", "geo_whitelist", "geo_blacklist"} {
			svc := characterizationService(t, nil)
			id := insertCharacterizationACL(t, svc.db, &models.AccessList{
				Type:    listType,
				Enabled: false,
			})

			allowed, reason, err := svc.TestIP(id, "203.0.113.9")
			assert.NoError(t, err, listType)
			assert.True(t, allowed, listType)
			assert.Contains(t, reason, "Access list is disabled", listType)
		}
	})

	t.Run("invalid address returns ErrInvalidIPAddress", func(t *testing.T) {
		svc := characterizationService(t, nil)
		id := insertCharacterizationACL(t, svc.db, &models.AccessList{Type: "whitelist", Enabled: true})

		_, _, err := svc.TestIP(id, "not-an-ip")
		assert.True(t, errors.Is(err, ErrInvalidIPAddress))
	})

	t.Run("local network only admits a private address", func(t *testing.T) {
		for _, listType := range []string{"whitelist", "geo_whitelist"} {
			svc := characterizationService(t, nil)
			id := insertCharacterizationACL(t, svc.db, &models.AccessList{
				Type:             listType,
				Enabled:          true,
				LocalNetworkOnly: true,
				CountryCodes:     "DE",
			})

			allowed, reason, err := svc.TestIP(id, "192.168.1.5")
			assert.NoError(t, err, listType)
			assert.True(t, allowed, listType)
			assert.Contains(t, reason, "local network only", listType)
		}
	})

	t.Run("local network only refuses a public address before geo and IP rules", func(t *testing.T) {
		for _, listType := range []string{"blacklist", "geo_blacklist"} {
			svc := characterizationService(t, loadedGeoIPFake("US"))
			id := insertCharacterizationACL(t, svc.db, &models.AccessList{
				Type:             listType,
				Enabled:          true,
				LocalNetworkOnly: true,
				CountryCodes:     "RU",
			})

			allowed, reason, err := svc.TestIP(id, "203.0.113.9")
			assert.NoError(t, err, listType)
			assert.False(t, allowed, listType)
			assert.Contains(t, reason, "Not a private network IP", listType)
		}
	})

	t.Run("geo list with no GeoIP service allows", func(t *testing.T) {
		svc := characterizationService(t, nil)
		id := insertCharacterizationACL(t, svc.db, &models.AccessList{
			Type:         "geo_whitelist",
			Enabled:      true,
			CountryCodes: "DE",
		})

		allowed, reason, err := svc.TestIP(id, "203.0.113.9")
		assert.NoError(t, err)
		assert.True(t, allowed)
		assert.Contains(t, reason, "GeoIP database not available")
	})

	t.Run("geo list with GeoIP service not loaded allows", func(t *testing.T) {
		unloaded := &GeoIPService{dbPath: "/nonexistent/country.mmdb"}
		svc := characterizationService(t, unloaded)
		id := insertCharacterizationACL(t, svc.db, &models.AccessList{
			Type:         "geo_whitelist",
			Enabled:      true,
			CountryCodes: "DE",
		})

		allowed, reason, err := svc.TestIP(id, "203.0.113.9")
		assert.NoError(t, err)
		assert.True(t, allowed)
		assert.Contains(t, reason, "GeoIP database not available")
	})

	t.Run("geo allow-list with lookup ok", func(t *testing.T) {
		cases := []struct {
			name    string
			country string
			allowed bool
		}{
			{name: "country in list", country: "DE", allowed: true},
			{name: "country not in list", country: "US", allowed: false},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				svc := characterizationService(t, loadedGeoIPFake(tc.country))
				id := insertCharacterizationACL(t, svc.db, &models.AccessList{
					Type:         "geo_whitelist",
					Enabled:      true,
					CountryCodes: "DE",
				})

				allowed, _, err := svc.TestIP(id, "203.0.113.9")
				assert.NoError(t, err)
				assert.Equal(t, tc.allowed, allowed)
			})
		}
	})

	t.Run("geo block-list with lookup ok", func(t *testing.T) {
		cases := []struct {
			name    string
			country string
			allowed bool
		}{
			{name: "country in list", country: "RU", allowed: false},
			{name: "country not in list", country: "US", allowed: true},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				svc := characterizationService(t, loadedGeoIPFake(tc.country))
				id := insertCharacterizationACL(t, svc.db, &models.AccessList{
					Type:         "geo_blacklist",
					Enabled:      true,
					CountryCodes: "RU",
				})

				allowed, _, err := svc.TestIP(id, "203.0.113.9")
				assert.NoError(t, err)
				assert.Equal(t, tc.allowed, allowed)
			})
		}
	})

	t.Run("geo list with country not found", func(t *testing.T) {
		svc := characterizationService(t, loadedGeoIPFake(""))
		allowList := insertCharacterizationACL(t, svc.db, &models.AccessList{
			Type:         "geo_whitelist",
			Enabled:      true,
			CountryCodes: "DE",
		})
		blockList := insertCharacterizationACL(t, svc.db, &models.AccessList{
			Type:         "geo_blacklist",
			Enabled:      true,
			CountryCodes: "RU",
		})

		allowed, reason, err := svc.TestIP(allowList, "203.0.113.9")
		assert.NoError(t, err)
		assert.False(t, allowed)
		assert.Contains(t, reason, "No country found")

		allowed, reason, err = svc.TestIP(blockList, "203.0.113.9")
		assert.NoError(t, err)
		assert.True(t, allowed)
		assert.Contains(t, reason, "No country found")
	})

	t.Run("IP allow-list matches and misses", func(t *testing.T) {
		svc := characterizationService(t, nil)
		id := insertCharacterizationACL(t, svc.db, &models.AccessList{
			Type:    "whitelist",
			Enabled: true,
			IPRules: `[{"cidr":"198.51.100.0/24"}]`,
		})

		allowed, _, err := svc.TestIP(id, "198.51.100.7")
		assert.NoError(t, err)
		assert.True(t, allowed)

		allowed, reason, err := svc.TestIP(id, "203.0.113.9")
		assert.NoError(t, err)
		assert.False(t, allowed)
		assert.Contains(t, reason, "Not in whitelist")
	})

	t.Run("IP block-list matches and misses", func(t *testing.T) {
		svc := characterizationService(t, nil)
		id := insertCharacterizationACL(t, svc.db, &models.AccessList{
			Type:    "blacklist",
			Enabled: true,
			IPRules: `[{"cidr":"198.51.100.0/24"}]`,
		})

		allowed, _, err := svc.TestIP(id, "198.51.100.7")
		assert.NoError(t, err)
		assert.False(t, allowed)

		allowed, reason, err := svc.TestIP(id, "203.0.113.9")
		assert.NoError(t, err)
		assert.True(t, allowed)
		assert.Contains(t, reason, "Not in blacklist")
	})

	t.Run("invalid ip_rules JSON is ignored and the type default applies", func(t *testing.T) {
		svc := characterizationService(t, nil)
		allowList := insertCharacterizationACL(t, svc.db, &models.AccessList{
			Type:    "whitelist",
			Enabled: true,
			IPRules: "not-json",
		})
		blockList := insertCharacterizationACL(t, svc.db, &models.AccessList{
			Type:    "blacklist",
			Enabled: true,
			IPRules: "not-json",
		})

		allowed, _, err := svc.TestIP(allowList, "198.51.100.7")
		assert.NoError(t, err)
		assert.False(t, allowed)

		allowed, _, err = svc.TestIP(blockList, "198.51.100.7")
		assert.NoError(t, err)
		assert.True(t, allowed)
	})

	t.Run("empty ip_rules falls back to the type default", func(t *testing.T) {
		svc := characterizationService(t, nil)
		allowList := insertCharacterizationACL(t, svc.db, &models.AccessList{
			Type:    "whitelist",
			Enabled: true,
			IPRules: "",
		})
		blockList := insertCharacterizationACL(t, svc.db, &models.AccessList{
			Type:    "blacklist",
			Enabled: true,
			IPRules: "",
		})

		allowed, _, err := svc.TestIP(allowList, "198.51.100.7")
		assert.NoError(t, err)
		assert.False(t, allowed)

		allowed, _, err = svc.TestIP(blockList, "198.51.100.7")
		assert.NoError(t, err)
		assert.True(t, allowed)
	})
}
