package services

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/caddy"
	"github.com/Wikid82/charon/backend/internal/caddy/caddymatchtest"
	"github.com/Wikid82/charon/backend/internal/models"
	"gorm.io/gorm"
)

const (
	testAdminRange = "198.51.100.0/24"
	testAdminAddr  = "198.51.100.7"
	testVisitor    = "203.0.113.9"
	testPrivate    = "192.168.1.5"
)

// fakeCountryLookup answers Evaluate's country lookups without a database.
type fakeCountryLookup struct {
	available bool
	country   string
	err       error
}

func (f *fakeCountryLookup) Available() bool { return f.available }

func (f *fakeCountryLookup) LookupCountry(string) (string, error) { return f.country, f.err }

// evaluateCase is one Evaluate table row. lookup nil means no GeoIP service is set.
type evaluateCase struct {
	name        string
	acl         models.AccessList
	lookup      *fakeCountryLookup
	admin       []string
	addr        string
	allowed     bool
	reason      string
	wantCountry string
}

func TestEvaluate(t *testing.T) {
	dbUsable := func(country string, err error) *fakeCountryLookup {
		return &fakeCountryLookup{available: true, country: country, err: err}
	}
	dbMissing := &fakeCountryLookup{}
	lookupFailed := errors.New("reader failed")

	tests := []evaluateCase{
		{
			name:    "disabled list allows without a database",
			acl:     models.AccessList{Type: "geo_whitelist", CountryCodes: "DE"},
			addr:    testVisitor,
			allowed: true,
			reason:  ReasonListDisabled,
		},
		{
			name:    "admin address passes an IP allow-list it is not listed on",
			acl:     models.AccessList{Type: "whitelist", IPRules: `[{"cidr":"192.0.2.10/32"}]`, Enabled: true},
			admin:   []string{testAdminRange},
			addr:    testAdminAddr,
			allowed: true,
			reason:  ReasonAdminWhitelisted,
		},
		{
			name:    "admin address passes an IP block-list it is listed on",
			acl:     models.AccessList{Type: "blacklist", IPRules: `[{"cidr":"198.51.100.7/32"}]`, Enabled: true},
			admin:   []string{testAdminRange},
			addr:    testAdminAddr,
			allowed: true,
			reason:  ReasonAdminWhitelisted,
		},
		{
			name:    "admin address passes a country list while the database is missing",
			acl:     models.AccessList{Type: "geo_whitelist", CountryCodes: "DE", Enabled: true},
			lookup:  dbMissing,
			admin:   []string{testAdminRange},
			addr:    testAdminAddr,
			allowed: true,
			reason:  ReasonAdminWhitelisted,
		},
		{
			name:    "admin address passes a local network list on a public address",
			acl:     models.AccessList{Type: "whitelist", LocalNetworkOnly: true, Enabled: true},
			admin:   []string{testAdminRange},
			addr:    testAdminAddr,
			allowed: true,
			reason:  ReasonAdminWhitelisted,
		},
		{
			name:   "country allow-list refuses everyone while the database is missing",
			acl:    models.AccessList{Type: "geo_whitelist", CountryCodes: "DE", Enabled: true},
			lookup: dbMissing,
			addr:   testVisitor,
			reason: ReasonGeoIPUnavailable,
		},
		{
			name:   "country block-list refuses everyone while the database is missing",
			acl:    models.AccessList{Type: "geo_blacklist", CountryCodes: "RU", Enabled: true},
			lookup: dbMissing,
			addr:   testVisitor,
			reason: ReasonGeoIPUnavailable,
		},
		{
			name:        "country allow-list admits a listed country",
			acl:         models.AccessList{Type: "geo_whitelist", CountryCodes: "DE", Enabled: true},
			lookup:      dbUsable("DE", nil),
			addr:        testVisitor,
			allowed:     true,
			reason:      ReasonAllowed,
			wantCountry: "DE",
		},
		{
			name:        "country allow-list refuses an unlisted country",
			acl:         models.AccessList{Type: "geo_whitelist", CountryCodes: "DE", Enabled: true},
			lookup:      dbUsable("US", nil),
			addr:        testVisitor,
			reason:      ReasonCountryNotAllowed,
			wantCountry: "US",
		},
		{
			name:   "country allow-list refuses when no country is found",
			acl:    models.AccessList{Type: "geo_whitelist", CountryCodes: "DE", Enabled: true},
			lookup: dbUsable("", ErrCountryNotFound),
			addr:   testVisitor,
			reason: ReasonNoCountry,
		},
		{
			name:   "country allow-list refuses on a lookup error",
			acl:    models.AccessList{Type: "geo_whitelist", CountryCodes: "DE", Enabled: true},
			lookup: dbUsable("", lookupFailed),
			addr:   testVisitor,
			reason: ReasonNoCountry,
		},
		{
			name:        "country block-list refuses a listed country",
			acl:         models.AccessList{Type: "geo_blacklist", CountryCodes: "RU", Enabled: true},
			lookup:      dbUsable("RU", nil),
			addr:        testVisitor,
			reason:      ReasonCountryBlocked,
			wantCountry: "RU",
		},
		{
			name:        "country block-list admits an unlisted country",
			acl:         models.AccessList{Type: "geo_blacklist", CountryCodes: "RU", Enabled: true},
			lookup:      dbUsable("US", nil),
			addr:        testVisitor,
			allowed:     true,
			reason:      ReasonAllowed,
			wantCountry: "US",
		},
		{
			name:    "country block-list admits when no country is found",
			acl:     models.AccessList{Type: "geo_blacklist", CountryCodes: "RU", Enabled: true},
			lookup:  dbUsable("", ErrCountryNotFound),
			addr:    testVisitor,
			allowed: true,
			reason:  ReasonNoCountry,
		},
		{
			name:    "country block-list admits on a lookup error",
			acl:     models.AccessList{Type: "geo_blacklist", CountryCodes: "RU", Enabled: true},
			lookup:  dbUsable("", lookupFailed),
			addr:    testVisitor,
			allowed: true,
			reason:  ReasonNoCountry,
		},
		{
			name:        "country list ignores local network only and decides by country",
			acl:         models.AccessList{Type: "geo_whitelist", CountryCodes: "DE", LocalNetworkOnly: true, Enabled: true},
			lookup:      dbUsable("DE", nil),
			addr:        testVisitor,
			allowed:     true,
			reason:      ReasonAllowed,
			wantCountry: "DE",
		},
		{
			name:        "country list with local network only refuses an unlisted public address by country",
			acl:         models.AccessList{Type: "geo_whitelist", CountryCodes: "DE", LocalNetworkOnly: true, Enabled: true},
			lookup:      dbUsable("US", nil),
			addr:        testVisitor,
			reason:      ReasonCountryNotAllowed,
			wantCountry: "US",
		},
		{
			name:    "local network only admits a private address",
			acl:     models.AccessList{Type: "whitelist", LocalNetworkOnly: true, Enabled: true},
			addr:    testPrivate,
			allowed: true,
			reason:  ReasonAllowed,
		},
		{
			name:   "local network only refuses a public address",
			acl:    models.AccessList{Type: "whitelist", LocalNetworkOnly: true, Enabled: true},
			addr:   testVisitor,
			reason: ReasonNotLocalNetwork,
		},
		{
			name:    "IP allow-list admits a listed address",
			acl:     models.AccessList{Type: "whitelist", IPRules: `[{"cidr":"203.0.113.0/24"}]`, Enabled: true},
			addr:    testVisitor,
			allowed: true,
			reason:  ReasonAllowed,
		},
		{
			name:   "IP allow-list refuses an unlisted address",
			acl:    models.AccessList{Type: "whitelist", IPRules: `[{"cidr":"192.0.2.0/24"}]`, Enabled: true},
			addr:   testVisitor,
			reason: ReasonIPNotAllowed,
		},
		{
			name:   "IP allow-list with unreadable rules refuses everyone",
			acl:    models.AccessList{Type: "whitelist", IPRules: `not json`, Enabled: true},
			addr:   testVisitor,
			reason: ReasonIPNotAllowed,
		},
		{
			name:   "IP allow-list with no rules refuses everyone",
			acl:    models.AccessList{Type: "whitelist", IPRules: `[]`, Enabled: true},
			addr:   testVisitor,
			reason: ReasonIPNotAllowed,
		},
		{
			name:   "IP block-list refuses a listed single address",
			acl:    models.AccessList{Type: "blacklist", IPRules: `[{"cidr":"203.0.113.9"}]`, Enabled: true},
			addr:   testVisitor,
			reason: ReasonIPBlocked,
		},
		{
			name:    "IP block-list admits an unlisted address",
			acl:     models.AccessList{Type: "blacklist", IPRules: `[{"cidr":"192.0.2.0/24"}]`, Enabled: true},
			addr:    testVisitor,
			allowed: true,
			reason:  ReasonAllowed,
		},
		{
			name:    "IP block-list with unreadable rules blocks nobody",
			acl:     models.AccessList{Type: "blacklist", IPRules: `not json`, Enabled: true},
			addr:    testVisitor,
			allowed: true,
			reason:  ReasonAllowed,
		},
		{
			name:    "IP block-list with no rules blocks nobody",
			acl:     models.AccessList{Type: "blacklist", IPRules: ``, Enabled: true},
			addr:    testVisitor,
			allowed: true,
			reason:  ReasonAllowed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := NewAccessListService(nil)
			if tt.lookup != nil {
				svc.lookup = tt.lookup
			}
			d, err := svc.Evaluate(&tt.acl, netip.MustParseAddr(tt.addr), tt.admin)
			require.NoError(t, err)
			assert.Equal(t, tt.allowed, d.Allowed, "allowed")
			assert.Equal(t, tt.reason, d.Reason, "reason")
			assert.Equal(t, tt.wantCountry, d.CountryCode, "country code")
			assert.Equal(t, tt.lookup != nil && tt.lookup.available, d.GeoIPAvailable, "geoip available")
			assert.Equal(t, tt.reason == ReasonAdminWhitelisted, d.AdminExempt, "admin exempt")
		})
	}
}

func TestEvaluateRejectsMalformedInput(t *testing.T) {
	t.Run("nil access list", func(t *testing.T) {
		_, err := NewAccessListService(nil).Evaluate(nil, netip.MustParseAddr(testVisitor), nil)
		assert.Error(t, err)
	})

	t.Run("malformed admin whitelist entry", func(t *testing.T) {
		acl := &models.AccessList{Type: "whitelist", IPRules: `[{"cidr":"192.0.2.0/24"}]`, Enabled: true}
		_, err := NewAccessListService(nil).Evaluate(acl, netip.MustParseAddr(testVisitor), []string{"not-a-range"})
		assert.Error(t, err)
	})

	t.Run("malformed rule CIDR", func(t *testing.T) {
		acl := &models.AccessList{Type: "blacklist", IPRules: `[{"cidr":"999.0.0.0/8"}]`, Enabled: true}
		_, err := NewAccessListService(nil).Evaluate(acl, netip.MustParseAddr(testVisitor), nil)
		assert.Error(t, err)
	})
}

// setupSecurityConfigDB returns a test database that also holds the security configuration table.
func setupSecurityConfigDB(t *testing.T) *gorm.DB {
	t.Helper()
	db := setupTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.SecurityConfig{}))
	return db
}

func TestAdminWhitelistRanges(t *testing.T) {
	t.Run("no security config row means no ranges", func(t *testing.T) {
		assert.Empty(t, NewAccessListService(setupSecurityConfigDB(t)).AdminWhitelistRanges())
	})

	t.Run("stored whitelist is split and trimmed", func(t *testing.T) {
		db := setupSecurityConfigDB(t)
		require.NoError(t, db.Create(&models.SecurityConfig{
			Name:           models.DefaultSecurityConfigName,
			AdminWhitelist: " 198.51.100.0/24 , 192.0.2.1,, ",
		}).Error)

		assert.Equal(t, []string{"198.51.100.0/24", "192.0.2.1"}, NewAccessListService(db).AdminWhitelistRanges())
	})

	t.Run("unreadable row means no ranges, as the proxy configuration", func(t *testing.T) {
		db := setupSecurityConfigDB(t)
		require.NoError(t, db.Migrator().DropTable(&models.SecurityConfig{}))

		assert.Empty(t, NewAccessListService(db).AdminWhitelistRanges())
	})
}

// parityList is one access list shape in the parity matrix. listed is the country its country rule names.
type parityList struct {
	name   string
	acl    models.AccessList
	listed string
}

// parityOutcome is one lookup result: the country the proxy's placeholder receives and the error the lookup returns.
type parityOutcome struct {
	name    string
	country string
	err     error
}

// TestEvaluateMatchesProxyConfig checks that Evaluate agrees with the configuration the proxy is generated from, for
// every list type, admin whitelist on and off, database usable and missing, and each lookup outcome.
func TestEvaluateMatchesProxyConfig(t *testing.T) {
	const domain = "parity.example.com"
	dbPath := filepath.Join(t.TempDir(), "GeoLite2-Country.mmdb")

	lists := []parityList{
		{name: "IP allow-list", acl: models.AccessList{Type: "whitelist", IPRules: `[{"cidr":"192.0.2.10/32"}]`}, listed: "DE"},
		{name: "IP block-list", acl: models.AccessList{Type: "blacklist", IPRules: `[{"cidr":"203.0.113.9/32"},{"cidr":"198.51.100.7/32"}]`}, listed: "DE"},
		{name: "local network only", acl: models.AccessList{Type: "whitelist", LocalNetworkOnly: true}, listed: "DE"},
		{name: "country allow-list", acl: models.AccessList{Type: "geo_whitelist", CountryCodes: "DE"}, listed: "DE"},
		{name: "country block-list", acl: models.AccessList{Type: "geo_blacklist", CountryCodes: "RU"}, listed: "RU"},
	}

	for _, list := range lists {
		outcomes := []parityOutcome{
			{name: "found in list", country: list.listed},
			{name: "found outside list", country: "US"},
			{name: "not found", err: ErrCountryNotFound},
			{name: "lookup error", err: errors.New("reader failed")},
		}
		for _, admin := range []string{"", testAdminRange} {
			for _, available := range []bool{true, false} {
				for _, outcome := range outcomes {
					for _, addr := range []string{testAdminAddr, testVisitor, testPrivate} {
						name := fmt.Sprintf("%s/admin=%q/db=%t/%s/%s", list.name, admin, available, outcome.name, addr)
						t.Run(name, func(t *testing.T) {
							acl := list.acl
							acl.ID = 7
							acl.Name = "parity"
							acl.Enabled = true

							host := models.ProxyHost{
								UUID:         "parity",
								DomainNames:  domain,
								Enabled:      true,
								ForwardHost:  "app",
								ForwardPort:  8080,
								AccessListID: &acl.ID,
								AccessList:   &acl,
							}
							cfg, err := caddy.GenerateConfig([]models.ProxyHost{host}, t.TempDir(), "", "", "", false, false, false, false, true, admin, nil, nil, nil, nil, nil, caddy.WithGeoIPDatabase(dbPath, available))
							require.NoError(t, err)
							cfgJSON, err := json.Marshal(cfg)
							require.NoError(t, err)
							refused := caddymatchtest.HostRefuses(t, cfgJSON, domain, caddymatchtest.Probe{
								Addr:    netip.MustParseAddr(addr),
								Country: outcome.country,
							})

							var adminRanges []string
							if admin != "" {
								adminRanges = []string{admin}
							}
							svc := NewAccessListService(nil)
							svc.lookup = &fakeCountryLookup{available: available, country: outcome.country, err: outcome.err}
							d, err := svc.Evaluate(&acl, netip.MustParseAddr(addr), adminRanges)
							require.NoError(t, err)
							assert.Equal(t, !refused, d.Allowed, "Evaluate reason %s; proxy refused %t", d.Reason, refused)
						})
					}
				}
			}
		}
	}
}
