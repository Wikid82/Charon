package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/caddy"
	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/services"
)

// appliedManager is a config manager whose last applied report is set by the test.
type appliedManager struct {
	CaddyConfigManager
	report  caddy.EnforcementReport
	applied bool
}

func (m *appliedManager) LastAppliedReport() (caddy.EnforcementReport, bool) {
	return m.report, m.applied
}

// plainManager is a config manager with no last applied report.
type plainManager struct {
	CaddyConfigManager
}

// openEnforcementStatusDB opens a file-backed SQLite database with the tables the country list lookup reads.
func openEnforcementStatusDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "enforcement-status.db")), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	require.NoError(t, db.AutoMigrate(&models.Setting{}, &models.SecurityConfig{}, &models.AccessList{}, &models.ProxyHost{}))
	return db
}

// newEnforcementHandler returns a security handler with an unloaded GeoIP service, so no database is available.
func newEnforcementHandler(t *testing.T, db *gorm.DB, manager CaddyConfigManager) *SecurityHandler {
	t.Helper()
	h := NewSecurityHandler(config.SecurityConfig{}, db, manager)
	h.SetGeoIPService(services.NewUnloadedGeoIPService(filepath.Join(t.TempDir(), "GeoLite2-Country.mmdb")))
	t.Cleanup(h.Close)
	return h
}

// createCountryList stores an access list of the given type and enabled state.
func createCountryList(t *testing.T, db *gorm.DB, id uint, name, listType string, enabled bool) models.AccessList {
	t.Helper()
	list := models.AccessList{ID: id, UUID: name, Name: name, Type: listType, CountryCodes: "US", Enabled: enabled}
	require.NoError(t, db.Create(&list).Error)
	return list
}

// attachHost stores a proxy host behind the list. An explicit disabled state is written after the create, because a
// zero Enabled value would take the column default.
func attachHost(t *testing.T, db *gorm.DB, uuid string, list models.AccessList, enabled bool) {
	t.Helper()
	listID := list.ID
	host := models.ProxyHost{UUID: uuid, DomainNames: uuid + ".example.com", Enabled: true, ForwardHost: "app", ForwardPort: 8080, AccessListID: &listID}
	require.NoError(t, db.Create(&host).Error)
	if !enabled {
		require.NoError(t, db.Model(&host).Update("enabled", false).Error)
	}
}

func TestCountryListsNotEnforced_LastAppliedReportWins(t *testing.T) {
	db := openEnforcementStatusDB(t)
	manager := &appliedManager{applied: true, report: caddy.EnforcementReport{ACLOn: true, CountryListsWithoutDatabase: []string{"Countries"}}}
	h := newEnforcementHandler(t, db, manager)

	// The live state is ignored: ACL is off now, and no list is in the database, yet the report still names one.
	assert.Equal(t, []string{"Countries"}, h.countryListsNotEnforced(false))
}

func TestCountryListsNotEnforced_LastAppliedWithACLOffIsEmpty(t *testing.T) {
	db := openEnforcementStatusDB(t)
	manager := &appliedManager{applied: true, report: caddy.EnforcementReport{ACLOn: false, CountryListsWithoutDatabase: []string{"Countries"}}}
	h := newEnforcementHandler(t, db, manager)

	assert.Empty(t, h.countryListsNotEnforced(true))
}

func TestCountryListsNotEnforced_LastAppliedWithNoListsIsEmpty(t *testing.T) {
	db := openEnforcementStatusDB(t)
	manager := &appliedManager{applied: true, report: caddy.EnforcementReport{ACLOn: true}}
	h := newEnforcementHandler(t, db, manager)

	assert.Empty(t, h.countryListsNotEnforced(true))
}

func TestCountryListsNotEnforced_FallbackNamesAttachedEnabledList(t *testing.T) {
	db := openEnforcementStatusDB(t)
	list := createCountryList(t, db, 1, "No RU/CN", "geo_blacklist", true)
	attachHost(t, db, "site", list, true)
	h := newEnforcementHandler(t, db, &plainManager{})

	assert.Equal(t, []string{"No RU/CN"}, h.countryListsNotEnforced(true))
}

func TestCountryListsNotEnforced_FallbackNamesAreSortedAndUnique(t *testing.T) {
	db := openEnforcementStatusDB(t)
	countries := createCountryList(t, db, 1, "Countries", "geo_whitelist", true)
	alpha := createCountryList(t, db, 2, "Alpha", "geo_blacklist", true)
	attachHost(t, db, "one", countries, true)
	attachHost(t, db, "two", countries, true)
	attachHost(t, db, "three", alpha, true)
	h := newEnforcementHandler(t, db, nil)

	assert.Equal(t, []string{"Alpha", "Countries"}, h.countryListsNotEnforced(true))
}

func TestCountryListsNotEnforced_FallbackIgnoresIneligibleLists(t *testing.T) {
	db := openEnforcementStatusDB(t)
	disabledHost := createCountryList(t, db, 1, "Host off", "geo_whitelist", true)
	disabledList := createCountryList(t, db, 2, "List off", "geo_whitelist", false)
	ipList := createCountryList(t, db, 3, "IP only", "whitelist", true)
	attachHost(t, db, "off", disabledHost, false)
	attachHost(t, db, "list-off", disabledList, true)
	attachHost(t, db, "ip", ipList, true)
	h := newEnforcementHandler(t, db, nil)

	assert.Empty(t, h.countryListsNotEnforced(true))
}

func TestCountryListsNotEnforced_FallbackNeedsAccessControlOn(t *testing.T) {
	db := openEnforcementStatusDB(t)
	list := createCountryList(t, db, 1, "Countries", "geo_whitelist", true)
	attachHost(t, db, "site", list, true)
	h := newEnforcementHandler(t, db, nil)

	assert.Empty(t, h.countryListsNotEnforced(false))
}

func TestCountryListsNotEnforced_FallbackWithoutDatabaseHandleIsEmpty(t *testing.T) {
	h := newEnforcementHandler(t, nil, nil)

	assert.Empty(t, h.countryListsNotEnforced(true))
}

func TestCountryListsNotEnforced_FallbackQueryErrorIsEmpty(t *testing.T) {
	db := openEnforcementStatusDB(t)
	require.NoError(t, db.Migrator().DropTable(&models.ProxyHost{}))
	h := newEnforcementHandler(t, db, nil)

	assert.Empty(t, h.countryListsNotEnforced(true))
}

func TestAddACLGeoIPReason_SetsReasonFromLastAppliedReport(t *testing.T) {
	db := openEnforcementStatusDB(t)
	manager := &appliedManager{applied: true, report: caddy.EnforcementReport{ACLOn: true, CountryListsWithoutDatabase: []string{"Countries"}}}
	h := newEnforcementHandler(t, db, manager)

	aclStatus := gin.H{"enabled": true}
	h.addACLGeoIPReason(aclStatus, true)

	assert.Equal(t, caddy.ReasonGeoIPUnavailable, aclStatus["reason"])
}

func TestAddACLGeoIPReason_LeavesStatusWithoutUnenforcedLists(t *testing.T) {
	db := openEnforcementStatusDB(t)
	h := newEnforcementHandler(t, db, &plainManager{})

	aclStatus := gin.H{"enabled": true}
	h.addACLGeoIPReason(aclStatus, true)

	_, has := aclStatus["reason"]
	assert.False(t, has)
}

// TestEnforcementStatus_ReasonInResponse pins that the status response carries the reason computed from the report.
func TestEnforcementStatus_ReasonInResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := openEnforcementStatusDB(t)
	list := createCountryList(t, db, 1, "Countries", "geo_whitelist", true)
	attachHost(t, db, "site", list, true)
	manager := &appliedManager{applied: true, report: caddy.EnforcementReport{ACLOn: true, CountryListsWithoutDatabase: []string{"Countries"}}}
	h := NewSecurityHandler(config.SecurityConfig{CerberusEnabled: true, ACLMode: "enabled"}, db, manager)
	h.SetGeoIPService(services.NewUnloadedGeoIPService(filepath.Join(t.TempDir(), "GeoLite2-Country.mmdb")))
	t.Cleanup(h.Close)

	router := gin.New()
	router.GET("/api/v1/security/status", h.GetStatus)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/security/status", http.NoBody))

	require.Equal(t, http.StatusOK, w.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	reason, has := aclReasonFromStatus(t, body)
	assert.True(t, has)
	assert.Equal(t, caddy.ReasonGeoIPUnavailable, reason)
}
