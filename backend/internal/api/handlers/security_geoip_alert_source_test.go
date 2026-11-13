package handlers

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/alerts"
	"github.com/Wikid82/charon/backend/internal/caddy"
	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/services"
)

// aclOnAlertHandler returns a handler whose static configuration turns access control on, with the given GeoIP service.
func aclOnAlertHandler(t *testing.T, db *gorm.DB, manager CaddyConfigManager, svc *services.GeoIPService) *SecurityHandler {
	t.Helper()
	h := NewSecurityHandler(config.SecurityConfig{ACLMode: "enabled"}, db, manager)
	h.SetGeoIPService(svc)
	t.Cleanup(h.Close)
	return h
}

// unloadedAlertService returns a GeoIP service with no database loaded.
func unloadedAlertService(t *testing.T) *services.GeoIPService {
	t.Helper()
	return services.NewUnloadedGeoIPService(filepath.Join(t.TempDir(), "GeoLite2-Country.mmdb"))
}

// loadedAlertService returns a GeoIP service with a usable database loaded from disk.
func loadedAlertService(t *testing.T) *services.GeoIPService {
	t.Helper()
	path := filepath.Join(t.TempDir(), "GeoLite2-Country.mmdb")
	require.NoError(t, os.WriteFile(path, minimalCountryMMDB(), 0o600))
	svc, err := services.NewGeoIPService(path)
	require.NoError(t, err)
	t.Cleanup(func() { _ = svc.Close() })
	return svc
}

// sourceAlert evaluates the GeoIP alert source once.
func sourceAlert(t *testing.T, h *SecurityHandler) (alerts.Alert, bool) {
	t.Helper()
	alert, ok, err := h.GeoIPAlertSource()(context.Background())
	require.NoError(t, err, "the source never fails")
	return alert, ok
}

func TestGeoIPAlertSource_InactiveWithoutUnenforcedLists(t *testing.T) {
	db := openEnforcementStatusDB(t)
	h := aclOnAlertHandler(t, db, &plainManager{}, unloadedAlertService(t))

	_, ok := sourceAlert(t, h)
	assert.False(t, ok, "no country list in use, so no alert")
}

func TestGeoIPAlertSource_PendingApplyWithoutDatabaseNamesList(t *testing.T) {
	db := openEnforcementStatusDB(t)
	list := createCountryList(t, db, 1, "Countries", "geo_whitelist", true)
	attachHost(t, db, "site", list, true)
	h := aclOnAlertHandler(t, db, &plainManager{}, unloadedAlertService(t))

	alert, ok := sourceAlert(t, h)
	require.True(t, ok)
	assert.Equal(t, alerts.KeyGeoIPUnavailable, alert.Key)
	assert.Equal(t, models.NotificationTypeError, alert.Severity)
	assert.Equal(t, []string{"Countries"}, alert.AffectedLists)
	assert.Equal(t, 1, alert.AffectedListsTotal)
	require.NotNil(t, alert.DatabaseAvailable)
	assert.False(t, *alert.DatabaseAvailable)
}

// The database can be usable while the last applied config still does not enforce the list. The alert stays, and it
// says the database is available so the bell can ask for a reload.
func TestGeoIPAlertSource_DatabaseAvailableWithStaleAppliedReportStaysActive(t *testing.T) {
	db := openEnforcementStatusDB(t)
	manager := &appliedManager{applied: true, report: caddy.EnforcementReport{ACLOn: true, CountryListsWithoutDatabase: []string{"Countries"}}}
	h := aclOnAlertHandler(t, db, manager, loadedAlertService(t))

	alert, ok := sourceAlert(t, h)
	require.True(t, ok)
	assert.Equal(t, []string{"Countries"}, alert.AffectedLists)
	require.NotNil(t, alert.DatabaseAvailable)
	assert.True(t, *alert.DatabaseAvailable)
}

func TestGeoIPAlertSource_AppliedReportWithoutListsIsInactive(t *testing.T) {
	db := openEnforcementStatusDB(t)
	manager := &appliedManager{applied: true, report: caddy.EnforcementReport{ACLOn: true}}
	h := aclOnAlertHandler(t, db, manager, unloadedAlertService(t))

	_, ok := sourceAlert(t, h)
	assert.False(t, ok)
}

func TestGeoIPAlertSource_AppliedReportWithAccessControlOffIsInactive(t *testing.T) {
	db := openEnforcementStatusDB(t)
	manager := &appliedManager{applied: true, report: caddy.EnforcementReport{ACLOn: false, CountryListsWithoutDatabase: []string{"Countries"}}}
	h := aclOnAlertHandler(t, db, manager, unloadedAlertService(t))

	_, ok := sourceAlert(t, h)
	assert.False(t, ok)
}

// The alert carries at most alerts.MaxAffectedLists names, and the total counts them all.
func TestGeoIPAlertSource_CapsAffectedListsWithTotal(t *testing.T) {
	db := openEnforcementStatusDB(t)
	names := []string{"A", "B", "C", "D", "E", "F", "G"}
	manager := &appliedManager{applied: true, report: caddy.EnforcementReport{ACLOn: true, CountryListsWithoutDatabase: names}}
	h := aclOnAlertHandler(t, db, manager, unloadedAlertService(t))

	alert, ok := sourceAlert(t, h)
	require.True(t, ok)
	assert.Equal(t, names[:alerts.MaxAffectedLists], alert.AffectedLists)
	assert.Equal(t, len(names), alert.AffectedListsTotal)
}

// The source plugs into the alert service the bell reads, and the alert comes out with its contract fields.
func TestGeoIPAlertSource_ReachesAlertServiceActive(t *testing.T) {
	db := openEnforcementStatusDB(t)
	manager := &appliedManager{applied: true, report: caddy.EnforcementReport{ACLOn: true, CountryListsWithoutDatabase: []string{"Countries"}}}
	h := aclOnAlertHandler(t, db, manager, unloadedAlertService(t))

	active := alerts.NewService(h.GeoIPAlertSource()).Active(context.Background())
	require.Len(t, active, 1)
	assert.Equal(t, alerts.KeyGeoIPUnavailable, active[0].Key)
}
