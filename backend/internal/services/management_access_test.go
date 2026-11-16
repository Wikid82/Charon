package services

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/security"
)

func managementTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.TempDir()+"/management.db"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.Setting{}, &models.AccessList{}, &models.ProxyHost{}, &models.SecurityConfig{}))
	return db
}

func seedList(t *testing.T, db *gorm.DB, uuid, name, typ string, enabled, local bool) *models.AccessList {
	t.Helper()
	acl := &models.AccessList{UUID: uuid, Name: name, Type: typ, Enabled: enabled, LocalNetworkOnly: local}
	require.NoError(t, db.Create(acl).Error)
	return acl
}

func TestParseManagementListUUIDs(t *testing.T) {
	got, err := ParseManagementListUUIDs("")
	require.NoError(t, err)
	assert.Equal(t, []string{}, got)

	got, err = ParseManagementListUUIDs(` ["b", " a ", "", "b"] `)
	require.NoError(t, err)
	assert.Equal(t, []string{"b", "a"}, got, "blank and duplicate entries are dropped, order kept")

	_, err = ParseManagementListUUIDs("{")
	require.Error(t, err)
}

func TestManagementListUUIDsAndWrite(t *testing.T) {
	db := managementTestDB(t)
	got, err := ManagementListUUIDs(db)
	require.NoError(t, err)
	assert.Equal(t, []string{}, got, "an absent row reads as no list chosen")

	require.NoError(t, db.Transaction(func(tx *gorm.DB) error { return WriteManagementLists(tx, []string{"zz", "aa"}) }))
	got, err = ManagementListUUIDs(db)
	require.NoError(t, err)
	assert.Equal(t, []string{"aa", "zz"}, got)

	require.NoError(t, db.Transaction(func(tx *gorm.DB) error { return WriteManagementLists(tx, nil) }))
	got, err = ManagementListUUIDs(db)
	require.NoError(t, err)
	assert.Equal(t, []string{}, got)
}

func TestManagementListUUIDs_ReadError(t *testing.T) {
	db := managementTestDB(t)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())
	_, err = ManagementListUUIDs(db)
	require.Error(t, err)
}

func TestACLEnabled(t *testing.T) {
	setting := func(value string, ok bool) func(string) (string, bool) {
		return func(string) (string, bool) { return value, ok }
	}
	assert.True(t, ACLEnabled(setting("true", true), config.SecurityConfig{ACLMode: "disabled"}), "a stored switch decides")
	assert.False(t, ACLEnabled(setting("false", true), config.SecurityConfig{ACLMode: "enabled"}), "a stored off beats the static mode")
	assert.True(t, ACLEnabled(setting("", false), config.SecurityConfig{ACLMode: "enabled"}), "no stored switch uses the static mode")
	assert.False(t, ACLEnabled(setting("", false), config.SecurityConfig{ACLMode: "disabled"}))
}

func TestSettingGetter(t *testing.T) {
	db := managementTestDB(t)
	get := SettingGetter(db)
	_, ok := get("security.acl.enabled")
	assert.False(t, ok)
	require.NoError(t, db.Create(&models.Setting{Key: "security.acl.enabled", Value: "true"}).Error)
	value, ok := get("security.acl.enabled")
	assert.True(t, ok)
	assert.Equal(t, "true", value)
}

func TestSeedManagementListsOnce_Matrix(t *testing.T) {
	db := managementTestDB(t)
	seedList(t, db, "white", "white", "whitelist", true, false)
	seedList(t, db, "black", "black", "blacklist", true, false)
	seedList(t, db, "local-white", "local", "whitelist", true, true)
	seedList(t, db, "geo-local", "geo local", "geo_blacklist", true, true)
	seedList(t, db, "geo-plain", "geo plain", "geo_whitelist", true, false)
	seedList(t, db, "off", "off", "blacklist", false, false)

	require.NoError(t, SeedManagementListsOnce(db, true))
	got, err := ManagementListUUIDs(db)
	require.NoError(t, err)
	assert.Equal(t, []string{"black", "local-white", "white"}, got,
		"whitelists, blacklists and local-network lists are carried over; geo lists and disabled lists are not")
}

func TestSeedManagementListsOnce_AclOffStoresEmptySet(t *testing.T) {
	db := managementTestDB(t)
	seedList(t, db, "white", "white", "whitelist", true, false)

	require.NoError(t, SeedManagementListsOnce(db, false))
	got, err := ManagementListUUIDs(db)
	require.NoError(t, err)
	assert.Equal(t, []string{}, got)
}

func TestSeedManagementListsOnce_RunsOnce(t *testing.T) {
	db := managementTestDB(t)
	seedList(t, db, "white", "white", "whitelist", true, false)
	require.NoError(t, SeedManagementListsOnce(db, true))

	// A later change by the operator is kept: the seed never runs again once the row exists.
	require.NoError(t, db.Transaction(func(tx *gorm.DB) error { return WriteManagementLists(tx, nil) }))
	require.NoError(t, SeedManagementListsOnce(db, true))
	got, err := ManagementListUUIDs(db)
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestSeedManagementListsOnce_FreshInstallStoresEmptySet(t *testing.T) {
	db := managementTestDB(t)
	require.NoError(t, SeedManagementListsOnce(db, true))
	got, err := ManagementListUUIDs(db)
	require.NoError(t, err)
	assert.Equal(t, []string{}, got)
}

func TestSeedManagementListsOnce_ReadErrorLeavesRowAbsent(t *testing.T) {
	db := managementTestDB(t)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())
	require.Error(t, SeedManagementListsOnce(db, true))
}

func TestEvaluateManagement_EmptySetIsNotRestricted(t *testing.T) {
	svc := NewAccessListService(managementTestDB(t))
	verdict, err := svc.EvaluateManagement(nil, "", nil, nil)
	require.NoError(t, err, "no list chosen: the caller address is not even read")
	assert.True(t, verdict.Allowed)
}

func TestEvaluateManagement_UnusableClientAddress(t *testing.T) {
	db := managementTestDB(t)
	seedList(t, db, "white", "white", "whitelist", true, false)
	svc := NewAccessListService(db)
	for _, addr := range []string{"", "garbage", "0.0.0.0", "::", "fe80::1%eth0"} {
		_, err := svc.EvaluateManagement([]string{"white"}, addr, nil, nil)
		assert.ErrorIs(t, err, ErrUnusableClientAddress, "address %q", addr)
	}
}

func TestEvaluateManagement_DecisionsAndSkips(t *testing.T) {
	db := managementTestDB(t)
	allow := seedList(t, db, "allow-8", "allow-8", "whitelist", true, false)
	require.NoError(t, db.Model(allow).Update("ip_rules", `[{"cidr":"8.8.8.8/32"}]`).Error)
	seedList(t, db, "block-8", "block-8", "blacklist", true, false)
	svc := NewAccessListService(db)

	verdict, err := svc.EvaluateManagement([]string{"allow-8"}, "8.8.8.8", nil, nil)
	require.NoError(t, err)
	assert.True(t, verdict.Allowed, "the whitelisted caller is admitted")

	verdict, err = svc.EvaluateManagement([]string{"allow-8"}, "203.0.113.5", nil, nil)
	require.NoError(t, err)
	assert.False(t, verdict.Allowed)
	require.NotNil(t, verdict.Refused)
	assert.Equal(t, "allow-8", verdict.Refused.Name)
	assert.Equal(t, ReasonIPNotAllowed, verdict.Decision.Reason)

	verdict, err = svc.EvaluateManagement([]string{"block-8", "gone"}, "203.0.113.5", nil, nil)
	require.NoError(t, err)
	assert.True(t, verdict.Allowed, "a missing list is skipped")
	assert.Equal(t, []string{"gone"}, verdict.Skipped)
}

func TestEvaluateManagement_OverrideReplacesListForReview(t *testing.T) {
	db := managementTestDB(t)
	stored := seedList(t, db, "allow", "allow", "whitelist", true, false)
	require.NoError(t, db.Model(stored).Update("ip_rules", `[{"cidr":"198.51.100.0/24"}]`).Error)
	svc := NewAccessListService(db)

	verdict, err := svc.EvaluateManagement([]string{"allow"}, "198.51.100.7", nil, nil)
	require.NoError(t, err)
	assert.True(t, verdict.Allowed, "the stored list admits the caller")

	proposed := *stored
	proposed.IPRules = `[{"cidr":"203.0.113.0/24"}]`
	verdict, err = svc.EvaluateManagement([]string{"allow"}, "198.51.100.7", nil, &proposed)
	require.NoError(t, err)
	assert.False(t, verdict.Allowed, "the proposed list is judged, not the stored one")
}

func TestEvaluateManagement_AllowTypeErrorRefuses(t *testing.T) {
	db := managementTestDB(t)
	seedList(t, db, "bad-allow", "bad-allow", "whitelist", true, false)
	require.NoError(t, db.Model(&models.AccessList{}).Where("uuid = ?", "bad-allow").Update("ip_rules", `[{"cidr":"bogus"}]`).Error)
	svc := NewAccessListService(db)

	verdict, err := svc.EvaluateManagement([]string{"bad-allow"}, "8.8.8.8", nil, nil)
	require.Error(t, err)
	var listErr *ManagementListError
	require.True(t, errors.As(err, &listErr))
	assert.Equal(t, "bad-allow", listErr.List)
	assert.False(t, verdict.Allowed)
	require.NotNil(t, verdict.Refused)
}

func TestEvaluateManagement_BlockTypeErrorIsSkipped(t *testing.T) {
	db := managementTestDB(t)
	seedList(t, db, "bad-block", "bad-block", "blacklist", true, false)
	require.NoError(t, db.Model(&models.AccessList{}).Where("uuid = ?", "bad-block").Update("ip_rules", `[{"cidr":"bogus"}]`).Error)
	svc := NewAccessListService(db)

	verdict, err := svc.EvaluateManagement([]string{"bad-block"}, "8.8.8.8", nil, nil)
	require.NoError(t, err)
	assert.True(t, verdict.Allowed)
	assert.Equal(t, []string{"bad-block"}, verdict.Skipped)
}

func TestEvaluateManagement_AdminExemptionByAddress(t *testing.T) {
	db := managementTestDB(t)
	seedList(t, db, "allow-8", "allow-8", "whitelist", true, false)
	svc := NewAccessListService(db)

	verdict, err := svc.EvaluateManagement([]string{"allow-8"}, "203.0.113.5", []string{"203.0.113.5/32"}, nil)
	require.NoError(t, err)
	assert.True(t, verdict.Allowed)
}

func TestAccessListService_InUseCountsManagementSet(t *testing.T) {
	db := managementTestDB(t)
	acl := seedList(t, db, "managed", "managed", "whitelist", true, false)
	svc := NewAccessListService(db)

	inUse, err := svc.InUse(acl.ID)
	require.NoError(t, err)
	assert.False(t, inUse)

	require.NoError(t, db.Transaction(func(tx *gorm.DB) error { return WriteManagementLists(tx, []string{"managed"}) }))
	inUse, err = svc.InUse(acl.ID)
	require.NoError(t, err)
	assert.True(t, inUse)

	managed, err := svc.ManagementUses(acl.ID)
	require.NoError(t, err)
	assert.True(t, managed)

	err = svc.Delete(acl.ID)
	assert.ErrorIs(t, err, ErrAccessListManagementInUse, "a list that protects Charon's own interface is not deleted")
}

func TestAccessListService_ManagementUsesMissingList(t *testing.T) {
	svc := NewAccessListService(managementTestDB(t))
	managed, err := svc.ManagementUses(999)
	require.NoError(t, err)
	assert.False(t, managed)
}

func TestAccessListService_ProposedUpdateDoesNotSave(t *testing.T) {
	db := managementTestDB(t)
	acl := seedList(t, db, "p", "p", "whitelist", true, false)
	svc := NewAccessListService(db)

	proposed, err := svc.ProposedUpdate(acl.ID, &models.AccessList{Name: "p", Type: "blacklist", Enabled: false, IPRules: `[{"cidr":"10.0.0.0/8"}]`})
	require.NoError(t, err)
	assert.Equal(t, "blacklist", proposed.Type)
	assert.False(t, proposed.Enabled)

	stored, err := svc.GetByID(acl.ID)
	require.NoError(t, err)
	assert.Equal(t, "whitelist", stored.Type, "the stored list is unchanged")

	_, err = svc.ProposedUpdate(9999, &models.AccessList{})
	assert.ErrorIs(t, err, ErrAccessListNotFound)
}

func TestSecurityConstantsAreUsed(t *testing.T) {
	assert.Equal(t, "security.acl.management_lists", security.ManagementListsSettingKey)
}
