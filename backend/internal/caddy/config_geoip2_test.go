package caddy

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/geoip"
	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/internal/models"
)

const (
	testGeoIPDBPath     = "/data/geoip/GeoLite2-Country.mmdb"
	geoDeniedBody       = "Access denied: Geographic restriction"
	geoIP2HandlerMarker = `"handler":"geoip2"`
	geoCountryMatcher   = "{geoip2.country_code}"
	// geoNotEnforcedWarning is the start of the one Warn line per apply that names the unenforced country lists.
	geoNotEnforcedWarning = "Country access list is not enforced because the GeoIP database is unavailable"
)

// generateCountryListConfig builds one enabled host behind a country list of the given type and returns the config
// and the warnings logged while generating it.
func generateCountryListConfig(t *testing.T, listType, storageDir string, opts ...GenerateConfigOption) (cfg *Config, warnings string) {
	t.Helper()
	var buf strings.Builder
	logger.Init(true, &buf)

	acl := models.AccessList{ID: 70, Name: "Countries", Enabled: true, Type: listType, CountryCodes: "US,CA"}
	host := models.ProxyHost{UUID: "geo-host", DomainNames: "geo.example.com", Enabled: true, ForwardHost: "app", ForwardPort: 8080, AccessListID: &acl.ID, AccessList: &acl}

	var err error
	cfg, err = GenerateConfig([]models.ProxyHost{host}, storageDir, "", "", "", false, false, false, false, true, "", nil, nil, nil, nil, nil, opts...)
	require.NoError(t, err)
	return cfg, buf.String()
}

func routesJSON(t *testing.T, cfg *Config) string {
	t.Helper()
	b, err := json.Marshal(cfg.Apps.HTTP.Servers["charon_server"].Routes)
	require.NoError(t, err)
	return string(b)
}

func TestGenerateConfig_CountryListWithDatabase_AddsGeoIP2AppAndStrictHandler(t *testing.T) {
	for _, listType := range []string{"geo_whitelist", "geo_blacklist"} {
		t.Run(listType, func(t *testing.T) {
			storageDir := t.TempDir()
			cfg, warnings := generateCountryListConfig(t, listType, storageDir, WithGeoIPDatabase(testGeoIPDBPath, true))

			require.NotNil(t, cfg.Apps.GeoIP2, "the geoip2 app is added when a country list is in use")
			require.Equal(t, "/data/geoip", cfg.Apps.GeoIP2.DatabaseDirectory)
			require.Equal(t, []string{"GeoLite2-Country"}, cfg.Apps.GeoIP2.EditionIDs)
			require.Equal(t, filepath.Join(storageDir, geoIP2LockFileName), cfg.Apps.GeoIP2.LockFile)

			appJSON, err := json.Marshal(cfg.Apps.GeoIP2)
			require.NoError(t, err)
			require.NotContains(t, string(appJSON), "accountId", "the module never downloads")
			require.NotContains(t, string(appJSON), "licenseKey", "the module never downloads")

			routes := routesJSON(t, cfg)
			handlerAt := strings.Index(routes, geoIP2HandlerMarker)
			countryAt := strings.Index(routes, geoCountryMatcher)
			require.Positive(t, handlerAt, "the geoip2 handler is in the pipeline")
			require.Greater(t, countryAt, handlerAt, "the geoip2 handler sits before the country subroute")
			require.Contains(t, routes, `"enable":"strict"`)
			require.NotContains(t, warnings, geoNotEnforcedWarning)
		})
	}
}

func TestGenerateConfig_CountryListWithDatabase_HandlerSitsImmediatelyBeforeSubroute(t *testing.T) {
	cfg, _ := generateCountryListConfig(t, "geo_blacklist", t.TempDir(), WithGeoIPDatabase(testGeoIPDBPath, true))

	handlers := cfg.Apps.HTTP.Servers["charon_server"].Routes[0].Handle
	require.GreaterOrEqual(t, len(handlers), 2)
	var names []string
	for _, h := range handlers {
		name, _ := h["handler"].(string)
		names = append(names, name)
	}
	idx := -1
	for i, h := range handlers {
		if h["handler"] == "geoip2" {
			idx = i
		}
	}
	require.NotEqual(t, -1, idx, "handler names: %v", names)
	require.Equal(t, "subroute", handlers[idx+1]["handler"], "the country subroute follows the geoip2 handler")
	require.Equal(t, "strict", handlers[idx]["enable"])
}

func TestGenerateConfig_NoCountryListNoGeoIP2App(t *testing.T) {
	acl := models.AccessList{ID: 71, Name: "IPs", Enabled: true, Type: "whitelist", IPRules: `[{"cidr":"10.1.1.0/24"}]`}
	host := models.ProxyHost{UUID: "ip-host", DomainNames: "ip.example.com", Enabled: true, ForwardHost: "app", ForwardPort: 8080, AccessListID: &acl.ID, AccessList: &acl}

	cfg, err := GenerateConfig([]models.ProxyHost{host}, t.TempDir(), "", "", "", false, false, false, false, true, "", nil, nil, nil, nil, nil, WithGeoIPDatabase(testGeoIPDBPath, true))
	require.NoError(t, err)

	require.Nil(t, cfg.Apps.GeoIP2)
	require.NotContains(t, routesJSON(t, cfg), geoIP2HandlerMarker)

	raw, err := json.Marshal(cfg)
	require.NoError(t, err)
	require.NotContains(t, string(raw), `"geoip2":`)
}

func TestGenerateConfig_CountryListWithoutACLEnabledNoGeoIP2App(t *testing.T) {
	acl := models.AccessList{ID: 72, Name: "Countries", Enabled: true, Type: "geo_blacklist", CountryCodes: "US"}
	host := models.ProxyHost{UUID: "off-host", DomainNames: "off.example.com", Enabled: true, ForwardHost: "app", ForwardPort: 8080, AccessListID: &acl.ID, AccessList: &acl}

	cfg, err := GenerateConfig([]models.ProxyHost{host}, t.TempDir(), "", "", "", false, false, false, false, false, "", nil, nil, nil, nil, nil, WithGeoIPDatabase(testGeoIPDBPath, true))
	require.NoError(t, err)
	require.Nil(t, cfg.Apps.GeoIP2)
}

func TestGenerateConfig_CountryListDatabaseUnavailable_LeavesHostNotEnforced(t *testing.T) {
	missingFile := filepath.Join(t.TempDir(), "GeoLite2-Country.mmdb")
	cases := map[string]struct {
		opts []GenerateConfigOption
	}{
		"database not loaded": {opts: []GenerateConfigOption{WithGeoIPDatabase(testGeoIPDBPath, false)}},
		"non-mmdb path":       {opts: []GenerateConfigOption{WithGeoIPDatabase("/data/geoip/GeoLite2-Country.dat", true)}},
		"no file configured":  {opts: []GenerateConfigOption{WithGeoIPDatabase("", false)}},
		"missing file on disk": {
			opts: []GenerateConfigOption{WithGeoIPDatabase(missingFile, geoip.DatabaseAvailable(missingFile, true))},
		},
		"no database option at all": {opts: nil},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			for _, listType := range []string{"geo_whitelist", "geo_blacklist"} {
				cfg, warnings := generateCountryListConfig(t, listType, t.TempDir(), tc.opts...)

				require.Nil(t, cfg.Apps.GeoIP2, "no geoip2 app without a usable database")
				routes := routesJSON(t, cfg)
				require.NotContains(t, routes, geoDeniedBody, "the host is not refused by the list")
				require.NotContains(t, routes, geoCountryMatcher, "the country subroute is not emitted")
				require.NotContains(t, routes, geoIP2HandlerMarker)
				require.Contains(t, routes, `"reverse_proxy"`, "the host keeps its proxy handler")
				require.Equal(t, 1, strings.Count(warnings, geoNotEnforcedWarning), "one Warn per apply")
				require.Contains(t, warnings, "Countries", "the Warn names the list")
			}
		})
	}
}

func TestGenerateConfig_CountryListDatabaseUnavailable_WarnsOncePerApply(t *testing.T) {
	var buf strings.Builder
	logger.Init(true, &buf)

	acl := models.AccessList{ID: 73, Name: "Shared", Enabled: true, Type: "geo_whitelist", CountryCodes: "US"}
	hostA := models.ProxyHost{UUID: "geo-a", DomainNames: "a-geo.example.com", Enabled: true, ForwardHost: "app", ForwardPort: 8080, AccessListID: &acl.ID, AccessList: &acl}
	hostB := models.ProxyHost{UUID: "geo-b", DomainNames: "b-geo.example.com", Enabled: true, ForwardHost: "app", ForwardPort: 8080, AccessListID: &acl.ID, AccessList: &acl}

	cfg, err := GenerateConfig([]models.ProxyHost{hostA, hostB}, t.TempDir(), "", "", "", false, false, false, false, true, "", nil, nil, nil, nil, nil)
	require.NoError(t, err)

	require.Zero(t, strings.Count(routesJSON(t, cfg), geoDeniedBody), "no host gets a country deny handler")
	require.Equal(t, 1, strings.Count(buf.String(), geoNotEnforcedWarning), "one Warn per apply")
}

func TestGeoIP2AppFor_SplitsDirectoryAndEdition(t *testing.T) {
	storageDir := "/app/data/caddy/data"
	app := geoIP2AppFor("/srv/maps/Custom-City.mmdb", storageDir)
	require.Equal(t, "/srv/maps", app.DatabaseDirectory)
	require.Equal(t, []string{"Custom-City"}, app.EditionIDs)
	require.Equal(t, filepath.Join(storageDir, geoIP2LockFileName), app.LockFile)
}

func TestGenerateConfig_GeoIPOptionDefaultsToUnavailable(t *testing.T) {
	resolved := resolveGenerateConfigOptions([]GenerateConfigOption{WithGeoIPDatabase(testGeoIPDBPath, true), nil})
	require.Equal(t, testGeoIPDBPath, resolved.geoIPPath)
	require.True(t, resolved.geoIPAvailable)

	empty := resolveGenerateConfigOptions(nil)
	require.Empty(t, empty.geoIPPath)
	require.False(t, empty.geoIPAvailable)
}
