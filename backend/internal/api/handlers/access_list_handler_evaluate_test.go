package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/services"
)

// setupTesterDB returns a router serving the tester, and the database behind it. withSecurityConfig controls whether the
// security configuration table exists, so the admin whitelist can be read.
func setupTesterDB(t *testing.T, withSecurityConfig bool) (*gin.Engine, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	schema := []any{&models.AccessList{}, &models.ProxyHost{}}
	if withSecurityConfig {
		schema = append(schema, &models.SecurityConfig{})
	}
	require.NoError(t, db.AutoMigrate(schema...))

	router := gin.New()
	router.POST("/access-lists/:id/test", NewAccessListHandler(db).TestIP)
	return router, db
}

// postAccessListTest calls the tester for the access list with ID 1 (the only list each test creates) and returns the
// status and decoded body.
func postAccessListTest(t *testing.T, router *gin.Engine, ip string) (status int, body map[string]any) {
	t.Helper()
	payload, err := json.Marshal(map[string]string{"ip_address": ip})
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPost, "/access-lists/1/test", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	return w.Code, body
}

func TestAccessListTester_AdminWhitelistExemptsListedAddress(t *testing.T) {
	router, db := setupTesterDB(t, true)
	require.NoError(t, db.Create(&models.SecurityConfig{
		Name:           models.DefaultSecurityConfigName,
		AdminWhitelist: "198.51.100.0/24",
	}).Error)
	require.NoError(t, db.Create(&models.AccessList{
		Name:    "block",
		Type:    "blacklist",
		IPRules: `[{"cidr":"203.0.113.9"},{"cidr":"198.51.100.7"}]`,
		Enabled: true,
	}).Error)

	status, body := postAccessListTest(t, router, "203.0.113.9")
	assert.Equal(t, http.StatusOK, status)
	assert.Equal(t, false, body["allowed"])
	assert.Equal(t, "Blocked by blacklist rule", body["reason"])

	status, body = postAccessListTest(t, router, "198.51.100.7")
	assert.Equal(t, http.StatusOK, status)
	assert.Equal(t, true, body["allowed"])
	assert.Equal(t, "Address is in the admin whitelist", body["reason"])
}

// With no database the country list is not enforced, so the tester allows the address, as the proxy does.
func TestAccessListTester_CountryListAllowsWithoutDatabase(t *testing.T) {
	router, db := setupTesterDB(t, true)
	require.NoError(t, db.Create(&models.AccessList{
		Name:         "de only",
		Type:         "geo_whitelist",
		CountryCodes: "DE",
		Enabled:      true,
	}).Error)

	status, body := postAccessListTest(t, router, "203.0.113.9")
	assert.Equal(t, http.StatusOK, status)
	assert.Equal(t, true, body["allowed"])
	assert.Contains(t, body["reason"], "country lists are not enforced")
}

func TestAccessListTester_InvalidAddress(t *testing.T) {
	router, db := setupTesterDB(t, true)
	require.NoError(t, db.Create(&models.AccessList{
		Name:    "off",
		Type:    "whitelist",
		Enabled: false,
	}).Error)

	for _, ip := range []string{"not-an-ip", "fe80::1%eth0"} {
		status, body := postAccessListTest(t, router, ip)
		assert.Equal(t, http.StatusBadRequest, status, ip)
		assert.Equal(t, "invalid IP address", body["error"], ip)
	}
}

func TestAccessListTester_MalformedStoredRuleIsInternalError(t *testing.T) {
	router, db := setupTesterDB(t, true)
	require.NoError(t, db.Create(&models.AccessList{
		Name:    "bad rule",
		Type:    "blacklist",
		IPRules: `[{"cidr":"999.0.0.0/8"}]`,
		Enabled: true,
	}).Error)

	status, _ := postAccessListTest(t, router, "203.0.113.9")
	assert.Equal(t, http.StatusInternalServerError, status)
}

// With no readable security configuration the proxy generates no admin exemption, so the tester applies none either.
func TestAccessListTester_UnreadableAdminWhitelistIsNotExempt(t *testing.T) {
	router, db := setupTesterDB(t, false)
	require.NoError(t, db.Create(&models.AccessList{
		Name:    "block",
		Type:    "blacklist",
		IPRules: `[{"cidr":"203.0.113.9"}]`,
		Enabled: true,
	}).Error)

	status, body := postAccessListTest(t, router, "203.0.113.9")
	assert.Equal(t, http.StatusOK, status)
	assert.Equal(t, false, body["allowed"])
}

func TestAccessListDecisionText(t *testing.T) {
	tests := map[string]string{
		services.ReasonListDisabled:      "Access list is disabled - all traffic allowed",
		services.ReasonAdminWhitelisted:  "Address is in the admin whitelist",
		services.ReasonAllowed:           "Allowed by access list",
		services.ReasonGeoIPUnavailable:  "GeoIP database not available - country lists are not enforced",
		services.ReasonCountryNotAllowed: "Blocked by geo whitelist: country is not in the allowed countries",
		services.ReasonCountryBlocked:    "Blocked by geo blacklist: country is in the blocked countries",
		services.ReasonNoCountry:         "No country found for IP",
		services.ReasonNotLocalNetwork:   "Not a private network IP (RFC1918)",
		services.ReasonIPNotAllowed:      "Not in whitelist",
		services.ReasonIPBlocked:         "Blocked by blacklist rule",
		"unknown_reason":                 "unknown_reason",
	}
	for reason, want := range tests {
		assert.Equal(t, want, accessListDecisionText(reason), reason)
	}
}
