package services

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/testutil"
)

// candidateDB writes a country database that resolves every address to country iso.
func candidateDB(t *testing.T, iso string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "candidate.mmdb")
	record := map[string]any{"country": map[string]any{"iso_code": iso}}
	testutil.WriteGeoIPCountryDB(t, path, 1, testutil.WithGeoIPRecord(record), testutil.WithGeoIPDatabaseType("GeoLite2-Country"))
	return path
}

// candidateChoice stores one country list and chooses it as the management set.
func candidateChoice(t *testing.T, db *gorm.DB, listType, codes string) []string {
	t.Helper()
	acl := &models.AccessList{UUID: "geo-" + t.Name(), Name: "geo", Type: listType, CountryCodes: codes, Enabled: true}
	require.NoError(t, db.Create(acl).Error)
	return []string{acl.UUID}
}

func TestManagementCandidateCheck_RefusesCallerTheCandidateWouldRefuse(t *testing.T) {
	db := managementTestDB(t)
	const caller = "198.51.100.7"

	tests := []struct {
		name     string
		listType string
		codes    string
		iso      string
		refused  bool
	}{
		{name: "country allow-list without the caller's country", listType: "geo_whitelist", codes: "DE", iso: "US", refused: true},
		{name: "country allow-list with the caller's country", listType: "geo_whitelist", codes: "DE", iso: "DE", refused: false},
		{name: "country block-list naming the caller's country", listType: "geo_blacklist", codes: "RU", iso: "RU", refused: true},
		{name: "country block-list not naming the caller's country", listType: "geo_blacklist", codes: "RU", iso: "US", refused: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			uuids := candidateChoice(t, db, tt.listType, tt.codes)
			check := ManagementCandidateCheck(db, uuids, caller, nil, true)
			err := check(candidateDB(t, tt.iso))
			if tt.refused {
				assert.ErrorIs(t, err, ErrGeoIPWouldLockOut)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestManagementCandidateCheck_NoCountryAdmitsBlockList(t *testing.T) {
	db := managementTestDB(t)
	uuids := candidateChoice(t, db, "geo_blacklist", "RU")
	// A database that resolves no country: the block-list cannot match, so the caller is admitted.
	path := filepath.Join(t.TempDir(), "no-country.mmdb")
	testutil.WriteGeoIPCountryDB(t, path, 1, testutil.WithGeoIPRecord(map[string]any{"continent": map[string]any{"code": "EU"}}))
	assert.NoError(t, ManagementCandidateCheck(db, uuids, "198.51.100.7", nil, true)(path))
}

func TestManagementCandidateCheck_AdminExemptionKeepsCaller(t *testing.T) {
	db := managementTestDB(t)
	uuids := candidateChoice(t, db, "geo_whitelist", "DE")
	check := ManagementCandidateCheck(db, uuids, "198.51.100.7", []string{"198.51.100.7/32"}, true)
	assert.NoError(t, check(candidateDB(t, "US")))
}

func TestManagementCandidateCheck_NotEnforcedAdmitsEverything(t *testing.T) {
	db := managementTestDB(t)
	uuids := candidateChoice(t, db, "geo_whitelist", "DE")
	assert.NoError(t, ManagementCandidateCheck(db, uuids, "198.51.100.7", nil, false)("/nonexistent.mmdb"), "access control off")
	assert.NoError(t, ManagementCandidateCheck(db, nil, "198.51.100.7", nil, true)("/nonexistent.mmdb"), "no list chosen")
}

func TestManagementCandidateCheck_UnreadableCandidateIsAnError(t *testing.T) {
	db := managementTestDB(t)
	uuids := candidateChoice(t, db, "geo_whitelist", "DE")
	err := ManagementCandidateCheck(db, uuids, "198.51.100.7", nil, true)(filepath.Join(t.TempDir(), "missing.mmdb"))
	require.Error(t, err)
	assert.False(t, errors.Is(err, ErrGeoIPWouldLockOut), "an unreadable candidate is not a lock-out")
}

func TestManagementCandidateCheck_UnusableCallerAddressIsRefused(t *testing.T) {
	db := managementTestDB(t)
	uuids := candidateChoice(t, db, "geo_whitelist", "DE")
	err := ManagementCandidateCheck(db, uuids, "garbage", nil, true)(candidateDB(t, "DE"))
	assert.ErrorIs(t, err, ErrGeoIPWouldLockOut)
}

func TestWithCandidateGuard_RunsCheckFromContext(t *testing.T) {
	assert.NoError(t, runCandidateGuard(context.Background(), "/any"), "no guard in context")

	var seen string
	ctx := WithCandidateGuard(context.Background(), func(path string) error {
		seen = path
		return ErrGeoIPWouldLockOut
	})
	assert.ErrorIs(t, runCandidateGuard(ctx, "/candidate.mmdb"), ErrGeoIPWouldLockOut)
	assert.Equal(t, "/candidate.mmdb", seen)
}
