package caddy

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/models"
)

// countryReportHost builds an enabled host behind an access list of the given name, type and enabled state.
func countryReportHost(uuid string, hostEnabled bool, acl *models.AccessList) models.ProxyHost {
	return models.ProxyHost{
		UUID:         uuid,
		DomainNames:  uuid + ".example.com",
		Enabled:      hostEnabled,
		ForwardHost:  "app",
		ForwardPort:  8080,
		AccessListID: &acl.ID,
		AccessList:   acl,
	}
}

// generateACLReport generates a config with the ACL flag and database state given and returns the report it stamped.
func generateACLReport(t *testing.T, hosts []models.ProxyHost, aclOn, databaseAvailable bool) EnforcementReport {
	t.Helper()
	var reports []EnforcementReport
	_, err := GenerateConfig(hosts, t.TempDir(), "", "", "", false, false, false, false, aclOn, "", nil, nil, nil, nil, nil,
		WithGeoIPDatabase(testGeoIPDBPath, databaseAvailable),
		WithEnforcementSink(func(r EnforcementReport) { reports = append(reports, r) }))
	require.NoError(t, err)
	require.Len(t, reports, 1, "the sink is called exactly once per generation")
	return reports[0]
}

func TestEnforcementReport_ACLOnFollowsFlag(t *testing.T) {
	assert.True(t, generateACLReport(t, nil, true, false).ACLOn)
	assert.False(t, generateACLReport(t, nil, false, false).ACLOn)
}

func TestEnforcementReport_CountryListWithoutDatabaseIsNamed(t *testing.T) {
	acl := &models.AccessList{ID: 1, Name: "Countries", Enabled: true, Type: "geo_whitelist", CountryCodes: "US"}
	hosts := []models.ProxyHost{countryReportHost("geo-a", true, acl)}

	report := generateACLReport(t, hosts, true, false)

	assert.Equal(t, []string{"Countries"}, report.CountryListsWithoutDatabase)
}

func TestEnforcementReport_CountryListNamesAreSortedAndUnique(t *testing.T) {
	countries := &models.AccessList{ID: 1, Name: "Countries", Enabled: true, Type: "geo_blacklist", CountryCodes: "RU"}
	alpha := &models.AccessList{ID: 2, Name: "Alpha", Enabled: true, Type: "geo_whitelist", CountryCodes: "DE"}
	hosts := []models.ProxyHost{
		countryReportHost("one", true, countries),
		countryReportHost("two", true, countries),
		countryReportHost("three", true, alpha),
	}

	report := generateACLReport(t, hosts, true, false)

	assert.Equal(t, []string{"Alpha", "Countries"}, report.CountryListsWithoutDatabase)
}

func TestEnforcementReport_CountryListNameSortsBeforeExisting(t *testing.T) {
	zeta := &models.AccessList{ID: 1, Name: "Zeta", Enabled: true, Type: "geo_whitelist", CountryCodes: "DE"}
	beta := &models.AccessList{ID: 2, Name: "Beta", Enabled: true, Type: "geo_whitelist", CountryCodes: "FR"}
	hosts := []models.ProxyHost{countryReportHost("z", true, zeta), countryReportHost("b", true, beta)}

	report := generateACLReport(t, hosts, true, false)

	assert.Equal(t, []string{"Beta", "Zeta"}, report.CountryListsWithoutDatabase)
}

func TestEnforcementReport_NoCountryListNamesWhenNotUnenforced(t *testing.T) {
	countries := &models.AccessList{ID: 1, Name: "Countries", Enabled: true, Type: "geo_whitelist", CountryCodes: "US"}
	ipList := &models.AccessList{ID: 2, Name: "IPs", Enabled: true, Type: "whitelist", IPRules: "[]"}
	disabledList := &models.AccessList{ID: 3, Name: "Off", Enabled: false, Type: "geo_whitelist", CountryCodes: "US"}
	hosts := []models.ProxyHost{
		countryReportHost("disabled-host", false, countries),
		countryReportHost("ip-host", true, ipList),
		countryReportHost("off-host", true, disabledList),
	}

	t.Run("database missing with only ineligible lists", func(t *testing.T) {
		assert.Empty(t, generateACLReport(t, hosts, true, false).CountryListsWithoutDatabase)
	})
	t.Run("database available", func(t *testing.T) {
		report := generateACLReport(t, []models.ProxyHost{countryReportHost("geo", true, countries)}, true, true)
		assert.Empty(t, report.CountryListsWithoutDatabase)
	})
	t.Run("access control off", func(t *testing.T) {
		report := generateACLReport(t, []models.ProxyHost{countryReportHost("geo", true, countries)}, false, false)
		assert.Empty(t, report.CountryListsWithoutDatabase)
	})
}
