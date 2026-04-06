package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/services"
)

// setupUpdateTestRouter creates a test router with the proxy host handler registered.
// Uses a dedicated in-memory SQLite database with all required models migrated.
func setupUpdateTestRouter(t *testing.T) (*gin.Engine, *gorm.DB) {
	t.Helper()

	dsn := "file:" + t.Name() + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&models.ProxyHost{},
		&models.Location{},
		&models.SecurityHeaderProfile{},
		&models.Notification{},
		&models.NotificationProvider{},
	))

	ns := services.NewNotificationService(db, nil)
	h := NewProxyHostHandler(db, nil, ns, nil)

	r := gin.New()
	api := r.Group("/api/v1")
	h.RegisterRoutes(api)

	return r, db
}

// createTestProxyHost creates a proxy host in the database for testing.
func createTestProxyHost(t *testing.T, db *gorm.DB, name string) models.ProxyHost {
	t.Helper()
	host := models.ProxyHost{
		UUID:          uuid.NewString(),
		Name:          name,
		DomainNames:   name + ".test.com",
		ForwardScheme: "http",
		ForwardHost:   "localhost",
		ForwardPort:   8080,
		Enabled:       true,
	}
	require.NoError(t, db.Create(&host).Error)
	return host
}

// createTestSecurityHeaderProfile creates a security header profile for testing.
func createTestSecurityHeaderProfile(t *testing.T, db *gorm.DB, name string) models.SecurityHeaderProfile {
	t.Helper()
	profile := models.SecurityHeaderProfile{
		UUID:          uuid.NewString(),
		Name:          name,
		IsPreset:      false,
		SecurityScore: 85,
	}
	require.NoError(t, db.Create(&profile).Error)
	return profile
}

// createTestAccessList creates an access list for testing.
func createTestAccessList(t *testing.T, db *gorm.DB, name string) models.AccessList {
	t.Helper()
	acl := models.AccessList{
		UUID:    uuid.NewString(),
		Name:    name,
		Type:    "ip",
		Enabled: true,
	}
	require.NoError(t, db.Create(&acl).Error)
	return acl
}

func TestProxyHostUpdate_AccessListID_Transitions_NoUnrelatedMutation(t *testing.T) {
	t.Parallel()
	router, db := setupUpdateTestRouter(t)

	aclOne := createTestAccessList(t, db, "ACL One")
	aclTwo := createTestAccessList(t, db, "ACL Two")

	host := models.ProxyHost{
		UUID:          uuid.NewString(),
		Name:          "Access List Transition Host",
		DomainNames:   "acl-transition.test.com",
		ForwardScheme: "http",
		ForwardHost:   "localhost",
		ForwardPort:   8080,
		Enabled:       true,
		SSLForced:     true,
		Application:   "none",
		AccessListID:  &aclOne.ID,
	}
	require.NoError(t, db.Create(&host).Error)

	assertUnrelatedFields := func(t *testing.T, current models.ProxyHost) {
		t.Helper()
		assert.Equal(t, "Access List Transition Host", current.Name)
		assert.Equal(t, "acl-transition.test.com", current.DomainNames)
		assert.Equal(t, "localhost", current.ForwardHost)
		assert.Equal(t, 8080, current.ForwardPort)
		assert.True(t, current.SSLForced)
		assert.Equal(t, "none", current.Application)
	}

	runUpdate := func(t *testing.T, update map[string]any) {
		t.Helper()
		body, _ := json.Marshal(update)
		req := httptest.NewRequest(http.MethodPut, "/api/v1/proxy-hosts/"+host.UUID, bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp := httptest.NewRecorder()
		router.ServeHTTP(resp, req)
		require.Equal(t, http.StatusOK, resp.Code)
	}

	// value -> value
	runUpdate(t, map[string]any{"access_list_id": aclTwo.ID})
	var updated models.ProxyHost
	require.NoError(t, db.First(&updated, "uuid = ?", host.UUID).Error)
	require.NotNil(t, updated.AccessListID)
	assert.Equal(t, aclTwo.ID, *updated.AccessListID)
	assertUnrelatedFields(t, updated)

	// value -> null
	runUpdate(t, map[string]any{"access_list_id": nil})
	require.NoError(t, db.First(&updated, "uuid = ?", host.UUID).Error)
	assert.Nil(t, updated.AccessListID)
	assertUnrelatedFields(t, updated)

	// null -> value
	runUpdate(t, map[string]any{"access_list_id": aclOne.ID})
	require.NoError(t, db.First(&updated, "uuid = ?", host.UUID).Error)
	require.NotNil(t, updated.AccessListID)
	assert.Equal(t, aclOne.ID, *updated.AccessListID)
	assertUnrelatedFields(t, updated)
}

func TestProxyHostUpdate_AccessListID_UUIDNotFound_ReturnsBadRequest(t *testing.T) {
	t.Parallel()
	router, db := setupUpdateTestRouter(t)

	host := createTestProxyHost(t, db, "acl-uuid-not-found")

	updateBody := map[string]any{
		"name":           "ACL UUID Not Found",
		"domain_names":   "acl-uuid-not-found.test.com",
		"forward_scheme": "http",
		"forward_host":   "localhost",
		"forward_port":   8080,
		"access_list_id": uuid.NewString(),
	}
	body, _ := json.Marshal(updateBody)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/proxy-hosts/"+host.UUID, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	require.Equal(t, http.StatusBadRequest, resp.Code)

	var result map[string]any
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &result))
	assert.Contains(t, result["error"], "access list not found")
}

func TestProxyHostUpdate_AccessListID_ResolveQueryFailure_ReturnsBadRequest(t *testing.T) {
	t.Parallel()
	router, db := setupUpdateTestRouter(t)

	host := createTestProxyHost(t, db, "acl-resolve-query-failure")

	require.NoError(t, db.Migrator().DropTable(&models.AccessList{}))

	updateBody := map[string]any{
		"name":           "ACL Resolve Query Failure",
		"domain_names":   "acl-resolve-query-failure.test.com",
		"forward_scheme": "http",
		"forward_host":   "localhost",
		"forward_port":   8080,
		"access_list_id": uuid.NewString(),
	}
	body, _ := json.Marshal(updateBody)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/proxy-hosts/"+host.UUID, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	require.Equal(t, http.StatusBadRequest, resp.Code)

	var result map[string]any
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &result))
	assert.Contains(t, result["error"], "failed to resolve access list")
}

func TestProxyHostUpdate_SecurityHeaderProfileID_Transitions_NoUnrelatedMutation(t *testing.T) {
	t.Parallel()
	router, db := setupUpdateTestRouter(t)

	profileOne := createTestSecurityHeaderProfile(t, db, "Security Profile One")
	profileTwo := createTestSecurityHeaderProfile(t, db, "Security Profile Two")

	host := models.ProxyHost{
		UUID:                    uuid.NewString(),
		Name:                    "Security Profile Transition Host",
		DomainNames:             "security-transition.test.com",
		ForwardScheme:           "http",
		ForwardHost:             "localhost",
		ForwardPort:             9090,
		Enabled:                 true,
		SSLForced:               true,
		Application:             "none",
		SecurityHeaderProfileID: &profileOne.ID,
	}
	require.NoError(t, db.Create(&host).Error)

	assertUnrelatedFields := func(t *testing.T, current models.ProxyHost) {
		t.Helper()
		assert.Equal(t, "Security Profile Transition Host", current.Name)
		assert.Equal(t, "security-transition.test.com", current.DomainNames)
		assert.Equal(t, "localhost", current.ForwardHost)
		assert.Equal(t, 9090, current.ForwardPort)
		assert.True(t, current.SSLForced)
		assert.Equal(t, "none", current.Application)
	}

	runUpdate := func(t *testing.T, update map[string]any) {
		t.Helper()
		body, _ := json.Marshal(update)
		req := httptest.NewRequest(http.MethodPut, "/api/v1/proxy-hosts/"+host.UUID, bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp := httptest.NewRecorder()
		router.ServeHTTP(resp, req)
		require.Equal(t, http.StatusOK, resp.Code)
	}

	// value -> value
	runUpdate(t, map[string]any{"security_header_profile_id": fmt.Sprintf("%d", profileTwo.ID)})
	var updated models.ProxyHost
	require.NoError(t, db.First(&updated, "uuid = ?", host.UUID).Error)
	require.NotNil(t, updated.SecurityHeaderProfileID)
	assert.Equal(t, profileTwo.ID, *updated.SecurityHeaderProfileID)
	assertUnrelatedFields(t, updated)

	// value -> null
	runUpdate(t, map[string]any{"security_header_profile_id": ""})
	require.NoError(t, db.First(&updated, "uuid = ?", host.UUID).Error)
	assert.Nil(t, updated.SecurityHeaderProfileID)
	assertUnrelatedFields(t, updated)

	// null -> value
	runUpdate(t, map[string]any{"security_header_profile_id": fmt.Sprintf("%d", profileOne.ID)})
	require.NoError(t, db.First(&updated, "uuid = ?", host.UUID).Error)
	require.NotNil(t, updated.SecurityHeaderProfileID)
	assert.Equal(t, profileOne.ID, *updated.SecurityHeaderProfileID)
	assertUnrelatedFields(t, updated)
}

// TestProxyHostUpdate_EnableStandardHeaders_Null tests updating enable_standard_headers to null.
func TestProxyHostUpdate_EnableStandardHeaders_Null(t *testing.T) {
	t.Parallel()
	router, db := setupUpdateTestRouter(t)

	// Create host with enable_standard_headers set to true
	enabled := true
	host := models.ProxyHost{
		UUID:                  uuid.NewString(),
		Name:                  "Test Host",
		DomainNames:           "test.example.com",
		ForwardScheme:         "http",
		ForwardHost:           "localhost",
		ForwardPort:           8080,
		Enabled:               true,
		EnableStandardHeaders: &enabled,
	}
	require.NoError(t, db.Create(&host).Error)

	// Verify initial state
	require.NotNil(t, host.EnableStandardHeaders)
	require.True(t, *host.EnableStandardHeaders)

	// Update with enable_standard_headers: null
	updateBody := map[string]any{
		"name":                    "Test Host Updated",
		"domain_names":            "test.example.com",
		"forward_scheme":          "http",
		"forward_host":            "localhost",
		"forward_port":            8080,
		"enable_standard_headers": nil,
	}
	body, _ := json.Marshal(updateBody)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/proxy-hosts/"+host.UUID, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	require.Equal(t, http.StatusOK, resp.Code)

	// Verify enable_standard_headers is now nil
	var updated models.ProxyHost
	require.NoError(t, db.First(&updated, "uuid = ?", host.UUID).Error)
	assert.Nil(t, updated.EnableStandardHeaders)
}

// TestProxyHostUpdate_EnableStandardHeaders_True tests updating enable_standard_headers to true.
func TestProxyHostUpdate_EnableStandardHeaders_True(t *testing.T) {
	t.Parallel()
	router, db := setupUpdateTestRouter(t)

	// Create host with enable_standard_headers set to nil
	host := models.ProxyHost{
		UUID:                  uuid.NewString(),
		Name:                  "Test Host",
		DomainNames:           "test.example.com",
		ForwardScheme:         "http",
		ForwardHost:           "localhost",
		ForwardPort:           8080,
		Enabled:               true,
		EnableStandardHeaders: nil,
	}
	require.NoError(t, db.Create(&host).Error)

	// Update with enable_standard_headers: true
	updateBody := map[string]any{
		"name":                    "Test Host Updated",
		"domain_names":            "test.example.com",
		"forward_scheme":          "http",
		"forward_host":            "localhost",
		"forward_port":            8080,
		"enable_standard_headers": true,
	}
	body, _ := json.Marshal(updateBody)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/proxy-hosts/"+host.UUID, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	require.Equal(t, http.StatusOK, resp.Code)

	// Verify enable_standard_headers is now true
	var updated models.ProxyHost
	require.NoError(t, db.First(&updated, "uuid = ?", host.UUID).Error)
	require.NotNil(t, updated.EnableStandardHeaders)
	assert.True(t, *updated.EnableStandardHeaders)
}

// TestProxyHostUpdate_EnableStandardHeaders_False tests updating enable_standard_headers to false.
func TestProxyHostUpdate_EnableStandardHeaders_False(t *testing.T) {
	t.Parallel()
	router, db := setupUpdateTestRouter(t)

	// Create host with enable_standard_headers set to true
	enabled := true
	host := models.ProxyHost{
		UUID:                  uuid.NewString(),
		Name:                  "Test Host",
		DomainNames:           "test.example.com",
		ForwardScheme:         "http",
		ForwardHost:           "localhost",
		ForwardPort:           8080,
		Enabled:               true,
		EnableStandardHeaders: &enabled,
	}
	require.NoError(t, db.Create(&host).Error)

	// Update with enable_standard_headers: false
	updateBody := map[string]any{
		"name":                    "Test Host Updated",
		"domain_names":            "test.example.com",
		"forward_scheme":          "http",
		"forward_host":            "localhost",
		"forward_port":            8080,
		"enable_standard_headers": false,
	}
	body, _ := json.Marshal(updateBody)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/proxy-hosts/"+host.UUID, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	require.Equal(t, http.StatusOK, resp.Code)

	// Verify enable_standard_headers is now false
	var updated models.ProxyHost
	require.NoError(t, db.First(&updated, "uuid = ?", host.UUID).Error)
	require.NotNil(t, updated.EnableStandardHeaders)
	assert.False(t, *updated.EnableStandardHeaders)
}

// TestProxyHostUpdate_ForwardAuthEnabled tests updating forward_auth_enabled from false to true.
func TestProxyHostUpdate_ForwardAuthEnabled(t *testing.T) {
	t.Parallel()
	router, db := setupUpdateTestRouter(t)

	// Create host with forward_auth_enabled = false
	host := models.ProxyHost{
		UUID:               uuid.NewString(),
		Name:               "Test Host",
		DomainNames:        "test.example.com",
		ForwardScheme:      "http",
		ForwardHost:        "localhost",
		ForwardPort:        8080,
		Enabled:            true,
		ForwardAuthEnabled: false,
	}
	require.NoError(t, db.Create(&host).Error)
	require.False(t, host.ForwardAuthEnabled)

	// Update with forward_auth_enabled: true
	updateBody := map[string]any{
		"name":                 "Test Host Updated",
		"domain_names":         "test.example.com",
		"forward_scheme":       "http",
		"forward_host":         "localhost",
		"forward_port":         8080,
		"forward_auth_enabled": true,
	}
	body, _ := json.Marshal(updateBody)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/proxy-hosts/"+host.UUID, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	require.Equal(t, http.StatusOK, resp.Code)

	// Verify forward_auth_enabled is now true
	var updated models.ProxyHost
	require.NoError(t, db.First(&updated, "uuid = ?", host.UUID).Error)
	assert.True(t, updated.ForwardAuthEnabled)
}

// TestProxyHostUpdate_WAFDisabled tests updating waf_disabled from false to true.
func TestProxyHostUpdate_WAFDisabled(t *testing.T) {
	t.Parallel()
	router, db := setupUpdateTestRouter(t)

	// Create host with waf_disabled = false
	host := models.ProxyHost{
		UUID:          uuid.NewString(),
		Name:          "Test Host",
		DomainNames:   "test.example.com",
		ForwardScheme: "http",
		ForwardHost:   "localhost",
		ForwardPort:   8080,
		Enabled:       true,
		WAFDisabled:   false,
	}
	require.NoError(t, db.Create(&host).Error)
	require.False(t, host.WAFDisabled)

	// Update with waf_disabled: true
	updateBody := map[string]any{
		"name":           "Test Host Updated",
		"domain_names":   "test.example.com",
		"forward_scheme": "http",
		"forward_host":   "localhost",
		"forward_port":   8080,
		"waf_disabled":   true,
	}
	body, _ := json.Marshal(updateBody)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/proxy-hosts/"+host.UUID, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	require.Equal(t, http.StatusOK, resp.Code)

	// Verify waf_disabled is now true
	var updated models.ProxyHost
	require.NoError(t, db.First(&updated, "uuid = ?", host.UUID).Error)
	assert.True(t, updated.WAFDisabled)
}

func TestProxyHostUpdate_DNSChallengeFieldsPersist(t *testing.T) {
	t.Parallel()
	router, db := setupUpdateTestRouter(t)

	host := models.ProxyHost{
		UUID:            uuid.NewString(),
		Name:            "DNS Challenge Host",
		DomainNames:     "dns-challenge.example.com",
		ForwardScheme:   "http",
		ForwardHost:     "localhost",
		ForwardPort:     8080,
		Enabled:         true,
		UseDNSChallenge: false,
		DNSProviderID:   nil,
	}
	require.NoError(t, db.Create(&host).Error)

	updateBody := map[string]any{
		"domain_names":      "dns-challenge.example.com",
		"forward_host":      "localhost",
		"forward_port":      8080,
		"dns_provider_id":   "7",
		"use_dns_challenge": true,
	}
	body, _ := json.Marshal(updateBody)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/proxy-hosts/"+host.UUID, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	require.Equal(t, http.StatusOK, resp.Code)

	var updated models.ProxyHost
	require.NoError(t, db.First(&updated, "uuid = ?", host.UUID).Error)
	require.NotNil(t, updated.DNSProviderID)
	assert.Equal(t, uint(7), *updated.DNSProviderID)
	assert.True(t, updated.UseDNSChallenge)
}

func TestProxyHostUpdate_DNSChallengeRequiresProvider(t *testing.T) {
	t.Parallel()
	router, db := setupUpdateTestRouter(t)

	host := createTestProxyHost(t, db, "dns-validation")

	updateBody := map[string]any{
		"domain_names":      "dns-validation.test.com",
		"forward_host":      "localhost",
		"forward_port":      8080,
		"dns_provider_id":   nil,
		"use_dns_challenge": true,
	}
	body, _ := json.Marshal(updateBody)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/proxy-hosts/"+host.UUID, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	require.Equal(t, http.StatusBadRequest, resp.Code)

	var updated models.ProxyHost
	require.NoError(t, db.First(&updated, "uuid = ?", host.UUID).Error)
	assert.False(t, updated.UseDNSChallenge)
	assert.Nil(t, updated.DNSProviderID)
}

func TestProxyHostUpdate_InvalidForwardPortRejected(t *testing.T) {
	t.Parallel()
	router, db := setupUpdateTestRouter(t)

	host := createTestProxyHost(t, db, "invalid-forward-port")

	updateBody := map[string]any{
		"forward_port": 70000,
	}
	body, _ := json.Marshal(updateBody)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/proxy-hosts/"+host.UUID, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	require.Equal(t, http.StatusBadRequest, resp.Code)

	var updated models.ProxyHost
	require.NoError(t, db.First(&updated, "uuid = ?", host.UUID).Error)
	assert.Equal(t, 8080, updated.ForwardPort)
}

func TestProxyHostUpdate_InvalidCertificateIDRejected(t *testing.T) {
	t.Parallel()
	router, db := setupUpdateTestRouter(t)

	host := createTestProxyHost(t, db, "invalid-certificate-id")

	updateBody := map[string]any{
		"certificate_id": true,
	}
	body, _ := json.Marshal(updateBody)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/proxy-hosts/"+host.UUID, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	require.Equal(t, http.StatusBadRequest, resp.Code)

	var result map[string]any
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &result))
	assert.Contains(t, result["error"], "invalid certificate_id")
}

func TestProxyHostUpdate_RejectsEmptyDomainNamesAndPreservesOriginal(t *testing.T) {
	t.Parallel()
	router, db := setupUpdateTestRouter(t)

	host := models.ProxyHost{
		UUID:          uuid.NewString(),
		Name:          "Validation Test Host",
		DomainNames:   "original.example.com",
		ForwardScheme: "http",
		ForwardHost:   "localhost",
		ForwardPort:   8080,
		Enabled:       true,
	}
	require.NoError(t, db.Create(&host).Error)

	updateBody := map[string]any{
		"domain_names": "",
	}
	body, _ := json.Marshal(updateBody)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/proxy-hosts/"+host.UUID, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	require.Equal(t, http.StatusBadRequest, resp.Code)

	var updated models.ProxyHost
	require.NoError(t, db.First(&updated, "uuid = ?", host.UUID).Error)
	assert.Equal(t, "original.example.com", updated.DomainNames)
}

// TestProxyHostUpdate_SecurityHeaderProfileID_NegativeFloat tests that a negative float64
// for security_header_profile_id returns a 400 Bad Request.
func TestProxyHostUpdate_SecurityHeaderProfileID_NegativeFloat(t *testing.T) {
	t.Parallel()
	router, db := setupUpdateTestRouter(t)

	host := createTestProxyHost(t, db, "negative-float-test")

	// Update with security_header_profile_id as negative float64
	// JSON numbers default to float64 in Go
	updateBody := map[string]any{
		"name":                       "Test Host Updated",
		"domain_names":               "negative-float-test.test.com",
		"forward_scheme":             "http",
		"forward_host":               "localhost",
		"forward_port":               8080,
		"security_header_profile_id": -1.0,
	}
	body, _ := json.Marshal(updateBody)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/proxy-hosts/"+host.UUID, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	require.Equal(t, http.StatusBadRequest, resp.Code)

	var result map[string]any
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &result))
	assert.Contains(t, result["error"], "invalid security_header_profile_id")
}

// TestProxyHostUpdate_SecurityHeaderProfileID_NegativeInt tests that a negative int
// for security_header_profile_id returns a 400 Bad Request.
// Note: JSON decoding in Go typically produces float64, but we test the int branch
// by ensuring the conversion logic handles negative values correctly.
func TestProxyHostUpdate_SecurityHeaderProfileID_NegativeInt(t *testing.T) {
	t.Parallel()
	router, db := setupUpdateTestRouter(t)

	host := createTestProxyHost(t, db, "negative-int-test")

	// Update with security_header_profile_id as negative number
	// In JSON, -5 will be decoded as float64(-5), triggering the float64 path
	updateBody := map[string]any{
		"name":                       "Test Host Updated",
		"domain_names":               "negative-int-test.test.com",
		"forward_scheme":             "http",
		"forward_host":               "localhost",
		"forward_port":               8080,
		"security_header_profile_id": -5,
	}
	body, _ := json.Marshal(updateBody)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/proxy-hosts/"+host.UUID, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	require.Equal(t, http.StatusBadRequest, resp.Code)

	var result map[string]any
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &result))
	assert.Contains(t, result["error"], "invalid security_header_profile_id")
}

// TestProxyHostUpdate_SecurityHeaderProfileID_InvalidString tests that an invalid string
// for security_header_profile_id returns a 400 Bad Request.
func TestProxyHostUpdate_SecurityHeaderProfileID_InvalidString(t *testing.T) {
	t.Parallel()
	router, db := setupUpdateTestRouter(t)

	host := createTestProxyHost(t, db, "invalid-string-test")

	// Update with security_header_profile_id as invalid string
	updateBody := map[string]any{
		"name":                       "Test Host Updated",
		"domain_names":               "invalid-string-test.test.com",
		"forward_scheme":             "http",
		"forward_host":               "localhost",
		"forward_port":               8080,
		"security_header_profile_id": "not-a-number",
	}
	body, _ := json.Marshal(updateBody)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/proxy-hosts/"+host.UUID, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	require.Equal(t, http.StatusBadRequest, resp.Code)

	var result map[string]any
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &result))
	assert.Contains(t, result["error"], "security header profile not found")
}

// TestProxyHostUpdate_SecurityHeaderProfileID_PresetSlugUUID tests that a preset-style UUID
// slug (e.g. "preset-basic") resolves correctly to the numeric profile ID via a DB lookup,
// bypassing the uuid.Parse gate that would otherwise reject non-standard slug formats.
func TestProxyHostUpdate_SecurityHeaderProfileID_PresetSlugUUID(t *testing.T) {
	t.Parallel()
	router, db := setupUpdateTestRouter(t)

	// Create a profile whose UUID mimics a preset slug (non-standard UUID format)
	slugUUID := "preset-basic"
	profile := models.SecurityHeaderProfile{
		UUID:          slugUUID,
		Name:          "Basic Security",
		IsPreset:      true,
		SecurityScore: 65,
	}
	require.NoError(t, db.Create(&profile).Error)

	host := createTestProxyHost(t, db, "preset-slug-test")

	updateBody := map[string]any{
		"name":                       "Test Host Updated",
		"domain_names":               "preset-slug-test.test.com",
		"forward_scheme":             "http",
		"forward_host":               "localhost",
		"forward_port":               8080,
		"security_header_profile_id": slugUUID,
	}
	body, _ := json.Marshal(updateBody)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/proxy-hosts/"+host.UUID, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	require.Equal(t, http.StatusOK, resp.Code)

	var updated models.ProxyHost
	require.NoError(t, db.First(&updated, "uuid = ?", host.UUID).Error)
	require.NotNil(t, updated.SecurityHeaderProfileID)
	assert.Equal(t, profile.ID, *updated.SecurityHeaderProfileID)
}

// TestProxyHostUpdate_SecurityHeaderProfileID_UnsupportedType tests that an unsupported type
// (boolean) for security_header_profile_id returns a 400 Bad Request.
func TestProxyHostUpdate_SecurityHeaderProfileID_UnsupportedType(t *testing.T) {
	t.Parallel()
	router, db := setupUpdateTestRouter(t)

	host := createTestProxyHost(t, db, "unsupported-type-test")

	testCases := []struct {
		name  string
		value any
	}{
		{
			name:  "boolean_true",
			value: true,
		},
		{
			name:  "boolean_false",
			value: false,
		},
		{
			name:  "array",
			value: []int{1, 2, 3},
		},
		{
			name:  "object",
			value: map[string]any{"id": 1},
		},
	}

	for _, tc := range testCases {
		tc := tc // capture range variable
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			updateBody := map[string]any{
				"name":                       "Test Host Updated",
				"domain_names":               "unsupported-type-test.test.com",
				"forward_scheme":             "http",
				"forward_host":               "localhost",
				"forward_port":               8080,
				"security_header_profile_id": tc.value,
			}
			body, _ := json.Marshal(updateBody)

			req := httptest.NewRequest(http.MethodPut, "/api/v1/proxy-hosts/"+host.UUID, bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			resp := httptest.NewRecorder()
			router.ServeHTTP(resp, req)

			require.Equal(t, http.StatusBadRequest, resp.Code)

			var result map[string]any
			require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &result))
			assert.Contains(t, result["error"], "invalid security_header_profile_id")
		})
	}
}

// TestProxyHostUpdate_SecurityHeaderProfileID_ValidAssignment tests that a valid
// security_header_profile_id can be assigned to a proxy host.
func TestProxyHostUpdate_SecurityHeaderProfileID_ValidAssignment(t *testing.T) {
	t.Parallel()
	router, db := setupUpdateTestRouter(t)

	// Create a security header profile
	profile := createTestSecurityHeaderProfile(t, db, "Valid Profile")

	// Create host without a profile
	host := createTestProxyHost(t, db, "valid-assignment-test")
	require.Nil(t, host.SecurityHeaderProfileID)

	// Test cases for valid assignment using different type representations
	testCases := []struct {
		name  string
		value any
	}{
		{
			name:  "as_float64",
			value: float64(profile.ID),
		},
		{
			name:  "as_string",
			value: fmt.Sprintf("%d", profile.ID),
		},
		{
			name:  "as_uuid_string",
			value: profile.UUID,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			// Reset host's profile to nil before each sub-test
			db.Model(&host).Update("security_header_profile_id", nil)

			updateBody := map[string]any{
				"name":                       "Test Host Updated",
				"domain_names":               "valid-assignment-test.test.com",
				"forward_scheme":             "http",
				"forward_host":               "localhost",
				"forward_port":               8080,
				"security_header_profile_id": tc.value,
			}
			body, _ := json.Marshal(updateBody)

			req := httptest.NewRequest(http.MethodPut, "/api/v1/proxy-hosts/"+host.UUID, bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			resp := httptest.NewRecorder()
			router.ServeHTTP(resp, req)

			require.Equal(t, http.StatusOK, resp.Code)

			// Verify the profile was assigned
			var updated models.ProxyHost
			require.NoError(t, db.First(&updated, "uuid = ?", host.UUID).Error)
			require.NotNil(t, updated.SecurityHeaderProfileID)
			assert.Equal(t, profile.ID, *updated.SecurityHeaderProfileID)
		})
	}
}

// TestProxyHostUpdate_SecurityHeaderProfileID_SetToNull tests that setting
// security_header_profile_id to null removes the profile assignment.
func TestProxyHostUpdate_SecurityHeaderProfileID_SetToNull(t *testing.T) {
	t.Parallel()
	router, db := setupUpdateTestRouter(t)

	// Create a security header profile
	profile := createTestSecurityHeaderProfile(t, db, "Null Test Profile")

	// Create host with profile assigned
	host := models.ProxyHost{
		UUID:                    uuid.NewString(),
		Name:                    "Test Host",
		DomainNames:             "null-profile-test.test.com",
		ForwardScheme:           "http",
		ForwardHost:             "localhost",
		ForwardPort:             8080,
		Enabled:                 true,
		SecurityHeaderProfileID: &profile.ID,
	}
	require.NoError(t, db.Create(&host).Error)
	require.NotNil(t, host.SecurityHeaderProfileID)

	// Update with security_header_profile_id: null
	updateBody := map[string]any{
		"name":                       "Test Host Updated",
		"domain_names":               "null-profile-test.test.com",
		"forward_scheme":             "http",
		"forward_host":               "localhost",
		"forward_port":               8080,
		"security_header_profile_id": nil,
	}
	body, _ := json.Marshal(updateBody)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/proxy-hosts/"+host.UUID, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	require.Equal(t, http.StatusOK, resp.Code)

	// Verify the profile was removed
	var updated models.ProxyHost
	require.NoError(t, db.First(&updated, "uuid = ?", host.UUID).Error)
	assert.Nil(t, updated.SecurityHeaderProfileID)
}

// TestBulkUpdateSecurityHeaders_DBError_NonNotFound tests that a database error
// (other than not found) during profile lookup returns a 500 Internal Server Error.
func TestBulkUpdateSecurityHeaders_DBError_NonNotFound(t *testing.T) {
	t.Parallel()

	dsn := "file:" + t.Name() + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&models.ProxyHost{},
		&models.Location{},
		&models.SecurityHeaderProfile{},
		&models.Notification{},
		&models.NotificationProvider{},
	))

	// Create a valid security header profile
	profile := createTestSecurityHeaderProfile(t, db, "DB Error Test Profile")

	// Create a valid proxy host
	host := models.ProxyHost{
		UUID:          uuid.NewString(),
		Name:          "DB Error Test Host",
		DomainNames:   "dberror.test.com",
		ForwardScheme: "http",
		ForwardHost:   "localhost",
		ForwardPort:   8080,
		Enabled:       true,
	}
	require.NoError(t, db.Create(&host).Error)

	ns := services.NewNotificationService(db, nil)
	h := NewProxyHostHandler(db, nil, ns, nil)

	r := gin.New()
	api := r.Group("/api/v1")
	h.RegisterRoutes(api)

	// Close the underlying SQL connection to simulate a DB error
	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())

	// Attempt bulk update - should fail with internal server error due to closed DB
	reqBody := map[string]any{
		"host_uuids":                 []string{host.UUID},
		"security_header_profile_id": profile.ID,
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/proxy-hosts/bulk-update-security-headers", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	r.ServeHTTP(resp, req)

	// The handler should return 500 when DB operations fail
	require.Equal(t, http.StatusInternalServerError, resp.Code)
}

func TestParseNullableUintField(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		value      any
		wantID     *uint
		wantErr    bool
		errContain string
	}{
		{name: "nil", value: nil, wantID: nil, wantErr: false},
		{name: "float64", value: 5.0, wantID: func() *uint { v := uint(5); return &v }(), wantErr: false},
		{name: "int", value: 9, wantID: func() *uint { v := uint(9); return &v }(), wantErr: false},
		{name: "string", value: "12", wantID: func() *uint { v := uint(12); return &v }(), wantErr: false},
		{name: "blank string", value: "   ", wantID: nil, wantErr: false},
		{name: "negative float", value: -1.0, wantErr: true, errContain: "invalid test_field"},
		{name: "invalid string", value: "nope", wantErr: true, errContain: "invalid test_field"},
		{name: "unsupported", value: true, wantErr: true, errContain: "invalid test_field"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			id, _, err := parseNullableUintField(tt.value, "test_field")
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContain)
				return
			}

			require.NoError(t, err)
			if tt.wantID == nil {
				assert.Nil(t, id)
				return
			}
			require.NotNil(t, id)
			assert.Equal(t, *tt.wantID, *id)
		})
	}
}

func TestParseForwardPortField(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		value      any
		wantPort   int
		wantErr    bool
		errContain string
	}{
		{name: "float integer", value: 8080.0, wantPort: 8080, wantErr: false},
		{name: "float decimal", value: 8080.5, wantErr: true, errContain: "must be an integer"},
		{name: "int", value: 3000, wantPort: 3000, wantErr: false},
		{name: "int low", value: 0, wantErr: true, errContain: "between 1 and 65535"},
		{name: "string", value: "443", wantPort: 443, wantErr: false},
		{name: "string blank", value: " ", wantErr: true, errContain: "between 1 and 65535"},
		{name: "string invalid", value: "abc", wantErr: true, errContain: "must be an integer"},
		{name: "unsupported", value: true, wantErr: true, errContain: "unsupported type"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			port, err := parseForwardPortField(tt.value)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContain)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.wantPort, port)
		})
	}
}
