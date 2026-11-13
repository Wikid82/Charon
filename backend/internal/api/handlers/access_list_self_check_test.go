package handlers

import (
	"bytes"
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

	"github.com/Wikid82/charon/backend/internal/api/middleware"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/services"
	"github.com/Wikid82/charon/backend/internal/testutil"
)

const selfCheckPath = "/access-lists/self-check"

// selfCheckFixture serves the self-check handler directly, so its own admin check is what refuses non-admin callers.
type selfCheckFixture struct {
	t       *testing.T
	db      *gorm.DB
	handler *AccessListHandler
	router  *gin.Engine
}

// newSelfCheckFixture builds the router. role is the caller's role.
func newSelfCheckFixture(t *testing.T, role string) *selfCheckFixture {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.AccessList{}, &models.ProxyHost{}, &models.SecurityConfig{}))

	handler := NewAccessListHandler(db)
	router := gin.New()
	router.POST(selfCheckPath, func(c *gin.Context) {
		c.Set(middleware.RoleKey, role)
		c.Next()
	}, handler.SelfCheck)
	return &selfCheckFixture{t: t, db: db, handler: handler, router: router}
}

// post sends a draft to the self-check from remoteAddr and returns the status and decoded body.
func (f *selfCheckFixture) post(draft any, remoteAddr string) (status int, body map[string]any) {
	f.t.Helper()
	payload, err := json.Marshal(draft)
	require.NoError(f.t, err)
	req := httptest.NewRequest(http.MethodPost, selfCheckPath, bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = remoteAddr
	w := httptest.NewRecorder()
	f.router.ServeHTTP(w, req)

	require.NoError(f.t, json.Unmarshal(w.Body.Bytes(), &body), w.Body.String())
	return w.Code, body
}

// accessListCount returns how many access list rows exist, so a test can confirm a draft was not saved.
func (f *selfCheckFixture) accessListCount() int64 {
	f.t.Helper()
	var n int64
	require.NoError(f.t, f.db.Model(&models.AccessList{}).Count(&n).Error)
	return n
}

// clientAddr is the address the caller connects from in most tests.
const clientAddr = "203.0.113.9:44321"

// selfCheckDraft mirrors the request fields the self-check accepts.
type selfCheckDraft struct {
	Type             string `json:"type"`
	CountryCodes     string `json:"country_codes"`
	IPRules          string `json:"ip_rules"`
	LocalNetworkOnly bool   `json:"local_network_only"`
	Enabled          bool   `json:"enabled"`
}

// useCountryDatabase loads a country database that resolves every address to code and wires it into the handler.
func (f *selfCheckFixture) useCountryDatabase(code string) {
	f.t.Helper()
	path := filepath.Join(f.t.TempDir(), "GeoLite2-Country.mmdb")
	testutil.WriteGeoIPCountryDB(f.t, path, 1_700_000_000,
		testutil.WithGeoIPDatabaseType("GeoLite2-Country"),
		testutil.WithGeoIPRecordISOCode(code))
	svc, err := services.NewGeoIPService(path)
	require.NoError(f.t, err)
	f.t.Cleanup(func() { _ = svc.Close() })
	f.handler.SetGeoIPService(svc)
}

func TestAccessListSelfCheck_RequiresAdmin(t *testing.T) {
	f := newSelfCheckFixture(t, string(models.RoleUser))

	status, body := f.post(selfCheckDraft{Type: "blacklist", Enabled: true}, clientAddr)

	assert.Equal(t, http.StatusForbidden, status)
	assert.Equal(t, "permissions_admin_only", body["error_code"])
}

func TestAccessListSelfCheck_InvalidBody(t *testing.T) {
	f := newSelfCheckFixture(t, string(models.RoleAdmin))
	req := httptest.NewRequest(http.MethodPost, selfCheckPath, bytes.NewReader([]byte("{not json")))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = clientAddr
	w := httptest.NewRecorder()

	f.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAccessListSelfCheck_ValidationRefusedLikeCreate(t *testing.T) {
	f := newSelfCheckFixture(t, string(models.RoleAdmin))

	status, body := f.post(selfCheckDraft{Type: "geo_whitelist", Enabled: true}, clientAddr)

	assert.Equal(t, http.StatusBadRequest, status)
	assert.Equal(t, "country codes are required for geo-blocking", body["error"])
}

func TestAccessListSelfCheck_InvalidTypeRefused(t *testing.T) {
	f := newSelfCheckFixture(t, string(models.RoleAdmin))

	status, _ := f.post(selfCheckDraft{Type: "nonsense", Enabled: true}, clientAddr)

	assert.Equal(t, http.StatusBadRequest, status)
}

func TestAccessListSelfCheck_RulesRequiredCarriesErrorCode(t *testing.T) {
	f := newSelfCheckFixture(t, string(models.RoleAdmin))

	status, body := f.post(selfCheckDraft{Type: "whitelist", Enabled: true}, clientAddr)

	assert.Equal(t, http.StatusBadRequest, status)
	assert.Equal(t, "access_list_rules_required", body["error_code"])
}

func TestAccessListSelfCheck_NoDatabaseAllowsWithReason(t *testing.T) {
	f := newSelfCheckFixture(t, string(models.RoleAdmin))

	status, body := f.post(selfCheckDraft{Type: "geo_whitelist", CountryCodes: "DE", Enabled: true}, clientAddr)

	require.Equal(t, http.StatusOK, status)
	assert.Equal(t, "203.0.113.9", body["ip_address"])
	assert.Equal(t, "", body["country_code"])
	assert.Equal(t, true, body["allowed"])
	assert.Equal(t, false, body["admin_exempt"])
	assert.Equal(t, false, body["geoip_available"])
	assert.Equal(t, services.ReasonGeoIPUnavailable, body["reason"])
}

func TestAccessListSelfCheck_CountryOutsideAllowedListRefused(t *testing.T) {
	f := newSelfCheckFixture(t, string(models.RoleAdmin))
	f.useCountryDatabase("US")

	status, body := f.post(selfCheckDraft{Type: "geo_whitelist", CountryCodes: "DE", Enabled: true}, clientAddr)

	require.Equal(t, http.StatusOK, status)
	assert.Equal(t, "US", body["country_code"])
	assert.Equal(t, false, body["allowed"])
	assert.Equal(t, true, body["geoip_available"])
	assert.Equal(t, services.ReasonCountryNotAllowed, body["reason"])
}

func TestAccessListSelfCheck_CountryInAllowedListAllowed(t *testing.T) {
	f := newSelfCheckFixture(t, string(models.RoleAdmin))
	f.useCountryDatabase("US")

	status, body := f.post(selfCheckDraft{Type: "geo_whitelist", CountryCodes: "us", Enabled: true}, clientAddr)

	require.Equal(t, http.StatusOK, status)
	assert.Equal(t, true, body["allowed"])
	assert.Equal(t, services.ReasonAllowed, body["reason"])
}

func TestAccessListSelfCheck_IPAllowListRefusesOtherAddress(t *testing.T) {
	f := newSelfCheckFixture(t, string(models.RoleAdmin))

	status, body := f.post(selfCheckDraft{
		Type:    "whitelist",
		IPRules: `[{"cidr":"10.0.0.0/8"}]`,
		Enabled: true,
	}, clientAddr)

	require.Equal(t, http.StatusOK, status)
	assert.Equal(t, false, body["allowed"])
	assert.Equal(t, services.ReasonIPNotAllowed, body["reason"])
}

func TestAccessListSelfCheck_AdminWhitelistExempts(t *testing.T) {
	f := newSelfCheckFixture(t, string(models.RoleAdmin))
	require.NoError(t, f.db.Create(&models.SecurityConfig{
		Name:           models.DefaultSecurityConfigName,
		AdminWhitelist: "203.0.113.0/24",
	}).Error)

	status, body := f.post(selfCheckDraft{
		Type:    "whitelist",
		IPRules: `[{"cidr":"10.0.0.0/8"}]`,
		Enabled: true,
	}, clientAddr)

	require.Equal(t, http.StatusOK, status)
	assert.Equal(t, true, body["allowed"])
	assert.Equal(t, true, body["admin_exempt"])
	assert.Equal(t, services.ReasonAdminWhitelisted, body["reason"])
}

func TestAccessListSelfCheck_DisabledDraftAllows(t *testing.T) {
	f := newSelfCheckFixture(t, string(models.RoleAdmin))

	status, body := f.post(selfCheckDraft{Type: "blacklist", Enabled: false}, clientAddr)

	require.Equal(t, http.StatusOK, status)
	assert.Equal(t, true, body["allowed"])
	assert.Equal(t, services.ReasonListDisabled, body["reason"])
}

func TestAccessListSelfCheck_CanonicalizesMappedAddress(t *testing.T) {
	f := newSelfCheckFixture(t, string(models.RoleAdmin))

	status, body := f.post(selfCheckDraft{Type: "blacklist", Enabled: false}, "[::ffff:203.0.113.9]:44321")

	require.Equal(t, http.StatusOK, status)
	assert.Equal(t, "203.0.113.9", body["ip_address"])
}

func TestAccessListSelfCheck_UnresolvableClientAddress(t *testing.T) {
	f := newSelfCheckFixture(t, string(models.RoleAdmin))

	status, _ := f.post(selfCheckDraft{Type: "blacklist", Enabled: true}, "not-an-address")

	assert.Equal(t, http.StatusInternalServerError, status)
}

func TestAccessListSelfCheck_EvaluateErrorIsGeneric(t *testing.T) {
	f := newSelfCheckFixture(t, string(models.RoleAdmin))
	require.NoError(t, f.db.Create(&models.SecurityConfig{
		Name:           models.DefaultSecurityConfigName,
		AdminWhitelist: "not-a-range",
	}).Error)

	status, body := f.post(selfCheckDraft{Type: "blacklist", Enabled: true}, clientAddr)

	assert.Equal(t, http.StatusInternalServerError, status)
	assert.Equal(t, "internal server error", body["error"])
}

func TestAccessListSelfCheck_NothingPersisted(t *testing.T) {
	f := newSelfCheckFixture(t, string(models.RoleAdmin))

	status, _ := f.post(selfCheckDraft{Type: "blacklist", Enabled: true}, clientAddr)

	require.Equal(t, http.StatusOK, status)
	assert.Equal(t, int64(0), f.accessListCount())
}
