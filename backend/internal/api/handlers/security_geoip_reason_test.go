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

	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/services"
)

// geoipReasonStatusBody requests /security/status and returns the decoded body.
func geoipReasonStatusBody(t *testing.T, db *gorm.DB, cfg config.SecurityConfig) map[string]any {
	t.Helper()
	gin.SetMode(gin.TestMode)
	h := NewSecurityHandler(cfg, db, nil)
	t.Cleanup(h.Close)

	router := gin.New()
	router.GET("/api/v1/security/status", h.GetStatus)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/security/status", http.NoBody))
	require.Equal(t, http.StatusOK, w.Code)

	var body map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	return body
}

// openGeoIPReasonTestDB opens a file-backed SQLite database with the tables the status reads.
func openGeoIPReasonTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "geoip-reason.db")), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	require.NoError(t, db.AutoMigrate(&models.Setting{}, &models.SecurityConfig{}, &models.AccessList{}))
	return db
}

func aclReasonFromStatus(t *testing.T, body map[string]any) (string, bool) {
	t.Helper()
	acl, ok := body["acl"].(map[string]any)
	require.True(t, ok, "status has an acl object")
	reason, has := acl["reason"]
	if !has {
		return "", false
	}
	str, ok := reason.(string)
	require.True(t, ok, "reason is a string")
	return str, true
}

func TestSecurityStatus_GeoIPStatusReportsUnavailableWithoutService(t *testing.T) {
	h := NewSecurityHandler(config.SecurityConfig{}, nil, nil)
	router := gin.New()
	router.GET("/security/geoip/status", h.GetGeoIPStatus)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/security/geoip/status", http.NoBody))
	require.Equal(t, http.StatusOK, w.Code)

	var body map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, false, body["available"])
	assert.Equal(t, "geoip_unavailable", body["reason"])
}

func TestSecurityStatus_GeoIPStatusReportsUnavailableWhenServiceNotLoaded(t *testing.T) {
	h := NewSecurityHandler(config.SecurityConfig{}, nil, nil)
	h.SetGeoIPService(&services.GeoIPService{})
	router := gin.New()
	router.GET("/security/geoip/status", h.GetGeoIPStatus)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/security/geoip/status", http.NoBody))

	var body map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, false, body["available"])
	assert.Equal(t, "geoip_unavailable", body["reason"])
}

func TestSecurityStatus_ACLReasonWhenCountryListWithoutDatabase(t *testing.T) {
	db := openGeoIPReasonTestDB(t)
	require.NoError(t, db.Create(&models.AccessList{UUID: "geo-1", Name: "geo", Type: "geo_whitelist", CountryCodes: "US", Enabled: true}).Error)

	body := geoipReasonStatusBody(t, db, config.SecurityConfig{CerberusEnabled: true, ACLMode: "enabled"})
	reason, has := aclReasonFromStatus(t, body)
	assert.True(t, has)
	assert.Equal(t, "geoip_unavailable", reason)
}

func TestSecurityStatus_ACLReasonAbsentWithoutCountryList(t *testing.T) {
	db := openGeoIPReasonTestDB(t)
	require.NoError(t, db.Create(&models.AccessList{UUID: "ip-1", Name: "ips", Type: "whitelist", IPRules: "[]", Enabled: true}).Error)

	body := geoipReasonStatusBody(t, db, config.SecurityConfig{CerberusEnabled: true, ACLMode: "enabled"})
	_, has := aclReasonFromStatus(t, body)
	assert.False(t, has)
}

func TestSecurityStatus_ACLReasonAbsentWhenCountryListDisabled(t *testing.T) {
	db := openGeoIPReasonTestDB(t)
	require.NoError(t, db.Create(&models.AccessList{UUID: "geo-2", Name: "geo", Type: "geo_blacklist", CountryCodes: "CN", Enabled: false}).Error)

	body := geoipReasonStatusBody(t, db, config.SecurityConfig{CerberusEnabled: true, ACLMode: "enabled"})
	_, has := aclReasonFromStatus(t, body)
	assert.False(t, has)
}

func TestSecurityStatus_ACLReasonAbsentWhenACLDisabled(t *testing.T) {
	db := openGeoIPReasonTestDB(t)
	require.NoError(t, db.Create(&models.AccessList{UUID: "geo-3", Name: "geo", Type: "geo_whitelist", CountryCodes: "US", Enabled: true}).Error)

	body := geoipReasonStatusBody(t, db, config.SecurityConfig{CerberusEnabled: true, ACLMode: "disabled"})
	_, has := aclReasonFromStatus(t, body)
	assert.False(t, has)
}
