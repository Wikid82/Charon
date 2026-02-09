package handlers

import (
	"bytes"
	"encoding/json"
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

func setupTestRouterForSecurityHeaders(t *testing.T) (*gin.Engine, *gorm.DB) {
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

	ns := services.NewNotificationService(db)
	h := NewProxyHostHandler(db, nil, ns, nil)
	r := gin.New()
	api := r.Group("/api/v1")
	h.RegisterRoutes(api)

	return r, db
}

func TestBulkUpdateSecurityHeaders_Success(t *testing.T) {
	router, db := setupTestRouterForSecurityHeaders(t)

	// Create test security header profile
	profile := models.SecurityHeaderProfile{
		UUID:          uuid.NewString(),
		Name:          "Test Profile",
		IsPreset:      false,
		SecurityScore: 85,
	}
	require.NoError(t, db.Create(&profile).Error)

	// Create test proxy hosts
	host1 := models.ProxyHost{
		UUID:          uuid.NewString(),
		Name:          "Host 1",
		DomainNames:   "host1.test.com",
		ForwardScheme: "http",
		ForwardHost:   "localhost",
		ForwardPort:   8001,
	}
	host2 := models.ProxyHost{
		UUID:          uuid.NewString(),
		Name:          "Host 2",
		DomainNames:   "host2.test.com",
		ForwardScheme: "http",
		ForwardHost:   "localhost",
		ForwardPort:   8002,
	}
	host3 := models.ProxyHost{
		UUID:          uuid.NewString(),
		Name:          "Host 3",
		DomainNames:   "host3.test.com",
		ForwardScheme: "http",
		ForwardHost:   "localhost",
		ForwardPort:   8003,
	}
	require.NoError(t, db.Create(&host1).Error)
	require.NoError(t, db.Create(&host2).Error)
	require.NoError(t, db.Create(&host3).Error)

	// Apply profile to all hosts
	reqBody := map[string]any{
		"host_uuids":                 []string{host1.UUID, host2.UUID, host3.UUID},
		"security_header_profile_id": profile.ID,
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/proxy-hosts/bulk-update-security-headers", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	require.Equal(t, http.StatusOK, resp.Code)

	var result map[string]any
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &result))
	assert.Equal(t, float64(3), result["updated"])
	assert.Empty(t, result["errors"])

	// Verify all hosts have the profile assigned
	var updatedHost1, updatedHost2, updatedHost3 models.ProxyHost
	require.NoError(t, db.First(&updatedHost1, "uuid = ?", host1.UUID).Error)
	require.NoError(t, db.First(&updatedHost2, "uuid = ?", host2.UUID).Error)
	require.NoError(t, db.First(&updatedHost3, "uuid = ?", host3.UUID).Error)

	require.NotNil(t, updatedHost1.SecurityHeaderProfileID)
	require.NotNil(t, updatedHost2.SecurityHeaderProfileID)
	require.NotNil(t, updatedHost3.SecurityHeaderProfileID)
	assert.Equal(t, profile.ID, *updatedHost1.SecurityHeaderProfileID)
	assert.Equal(t, profile.ID, *updatedHost2.SecurityHeaderProfileID)
	assert.Equal(t, profile.ID, *updatedHost3.SecurityHeaderProfileID)
}

func TestBulkUpdateSecurityHeaders_RemoveProfile(t *testing.T) {
	router, db := setupTestRouterForSecurityHeaders(t)

	// Create test security header profile
	profile := models.SecurityHeaderProfile{
		UUID:          uuid.NewString(),
		Name:          "Test Profile",
		IsPreset:      false,
		SecurityScore: 85,
	}
	require.NoError(t, db.Create(&profile).Error)

	// Create test proxy hosts with existing profile
	host1 := models.ProxyHost{
		UUID:                    uuid.NewString(),
		Name:                    "Host 1",
		DomainNames:             "host1.test.com",
		ForwardScheme:           "http",
		ForwardHost:             "localhost",
		ForwardPort:             8001,
		SecurityHeaderProfileID: &profile.ID,
	}
	host2 := models.ProxyHost{
		UUID:                    uuid.NewString(),
		Name:                    "Host 2",
		DomainNames:             "host2.test.com",
		ForwardScheme:           "http",
		ForwardHost:             "localhost",
		ForwardPort:             8002,
		SecurityHeaderProfileID: &profile.ID,
	}
	require.NoError(t, db.Create(&host1).Error)
	require.NoError(t, db.Create(&host2).Error)

	// Remove profile from all hosts (set to null)
	reqBody := map[string]any{
		"host_uuids":                 []string{host1.UUID, host2.UUID},
		"security_header_profile_id": nil,
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/proxy-hosts/bulk-update-security-headers", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	require.Equal(t, http.StatusOK, resp.Code)

	var result map[string]any
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &result))
	assert.Equal(t, float64(2), result["updated"])

	// Verify all hosts have no profile
	var updatedHost1, updatedHost2 models.ProxyHost
	require.NoError(t, db.First(&updatedHost1, "uuid = ?", host1.UUID).Error)
	require.NoError(t, db.First(&updatedHost2, "uuid = ?", host2.UUID).Error)

	assert.Nil(t, updatedHost1.SecurityHeaderProfileID)
	assert.Nil(t, updatedHost2.SecurityHeaderProfileID)
}

func TestBulkUpdateSecurityHeaders_InvalidProfileID(t *testing.T) {
	router, db := setupTestRouterForSecurityHeaders(t)

	// Create test proxy host
	host := models.ProxyHost{
		UUID:          uuid.NewString(),
		Name:          "Host 1",
		DomainNames:   "host1.test.com",
		ForwardScheme: "http",
		ForwardHost:   "localhost",
		ForwardPort:   8001,
	}
	require.NoError(t, db.Create(&host).Error)

	// Try to apply non-existent profile
	nonExistentProfileID := uint(99999)
	reqBody := map[string]any{
		"host_uuids":                 []string{host.UUID},
		"security_header_profile_id": nonExistentProfileID,
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/proxy-hosts/bulk-update-security-headers", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	require.Equal(t, http.StatusBadRequest, resp.Code)

	var result map[string]any
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &result))
	assert.Contains(t, result["error"], "security header profile not found")
}

func TestBulkUpdateSecurityHeaders_EmptyUUIDs(t *testing.T) {
	router, _ := setupTestRouterForSecurityHeaders(t)

	// Try to update with empty host UUIDs
	reqBody := map[string]any{
		"host_uuids":                 []string{},
		"security_header_profile_id": nil,
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/proxy-hosts/bulk-update-security-headers", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	require.Equal(t, http.StatusBadRequest, resp.Code)

	var result map[string]any
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &result))
	assert.Contains(t, result["error"], "host_uuids cannot be empty")
}

func TestBulkUpdateSecurityHeaders_PartialFailure(t *testing.T) {
	router, db := setupTestRouterForSecurityHeaders(t)

	// Create test security header profile
	profile := models.SecurityHeaderProfile{
		UUID:          uuid.NewString(),
		Name:          "Test Profile",
		IsPreset:      false,
		SecurityScore: 85,
	}
	require.NoError(t, db.Create(&profile).Error)

	// Create one valid host
	host1 := models.ProxyHost{
		UUID:          uuid.NewString(),
		Name:          "Host 1",
		DomainNames:   "host1.test.com",
		ForwardScheme: "http",
		ForwardHost:   "localhost",
		ForwardPort:   8001,
	}
	require.NoError(t, db.Create(&host1).Error)

	// Include one valid and one invalid UUID
	invalidUUID := "non-existent-uuid"
	reqBody := map[string]any{
		"host_uuids":                 []string{host1.UUID, invalidUUID},
		"security_header_profile_id": profile.ID,
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/proxy-hosts/bulk-update-security-headers", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	require.Equal(t, http.StatusOK, resp.Code)

	var result map[string]any
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &result))
	assert.Equal(t, float64(1), result["updated"])

	// Check errors array
	errors, ok := result["errors"].([]any)
	require.True(t, ok)
	require.Len(t, errors, 1)

	errorMap := errors[0].(map[string]any)
	assert.Equal(t, invalidUUID, errorMap["uuid"])
	assert.Contains(t, errorMap["error"], "proxy host not found")

	// Verify the valid host was updated
	var updatedHost models.ProxyHost
	require.NoError(t, db.First(&updatedHost, "uuid = ?", host1.UUID).Error)
	require.NotNil(t, updatedHost.SecurityHeaderProfileID)
	assert.Equal(t, profile.ID, *updatedHost.SecurityHeaderProfileID)
}

func TestBulkUpdateSecurityHeaders_TransactionRollback(t *testing.T) {
	router, db := setupTestRouterForSecurityHeaders(t)

	// Try to update with all invalid UUIDs
	invalidUUID1 := "invalid-uuid-1"
	invalidUUID2 := "invalid-uuid-2"
	reqBody := map[string]any{
		"host_uuids":                 []string{invalidUUID1, invalidUUID2},
		"security_header_profile_id": nil,
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/proxy-hosts/bulk-update-security-headers", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	require.Equal(t, http.StatusBadRequest, resp.Code)

	var result map[string]any
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &result))
	assert.Contains(t, result["error"], "All updates failed")
	assert.Equal(t, float64(0), result["updated"])

	// Verify no hosts exist in the database (transaction rolled back)
	var count int64
	db.Model(&models.ProxyHost{}).Count(&count)
	assert.Equal(t, int64(0), count)
}

func TestBulkUpdateSecurityHeaders_InvalidJSON(t *testing.T) {
	router, _ := setupTestRouterForSecurityHeaders(t)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/proxy-hosts/bulk-update-security-headers", bytes.NewReader([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	require.Equal(t, http.StatusBadRequest, resp.Code)
}

func TestBulkUpdateSecurityHeaders_MixedProfileStates(t *testing.T) {
	router, db := setupTestRouterForSecurityHeaders(t)

	// Create two profiles
	profile1 := models.SecurityHeaderProfile{
		UUID:          uuid.NewString(),
		Name:          "Profile 1",
		IsPreset:      false,
		SecurityScore: 75,
	}
	profile2 := models.SecurityHeaderProfile{
		UUID:          uuid.NewString(),
		Name:          "Profile 2",
		IsPreset:      false,
		SecurityScore: 90,
	}
	require.NoError(t, db.Create(&profile1).Error)
	require.NoError(t, db.Create(&profile2).Error)

	// Create hosts with different profile states
	host1 := models.ProxyHost{
		UUID:                    uuid.NewString(),
		Name:                    "Host 1",
		DomainNames:             "host1.test.com",
		ForwardScheme:           "http",
		ForwardHost:             "localhost",
		ForwardPort:             8001,
		SecurityHeaderProfileID: &profile1.ID,
	}
	host2 := models.ProxyHost{
		UUID:                    uuid.NewString(),
		Name:                    "Host 2",
		DomainNames:             "host2.test.com",
		ForwardScheme:           "http",
		ForwardHost:             "localhost",
		ForwardPort:             8002,
		SecurityHeaderProfileID: nil, // No profile
	}
	host3 := models.ProxyHost{
		UUID:                    uuid.NewString(),
		Name:                    "Host 3",
		DomainNames:             "host3.test.com",
		ForwardScheme:           "http",
		ForwardHost:             "localhost",
		ForwardPort:             8003,
		SecurityHeaderProfileID: &profile1.ID,
	}
	require.NoError(t, db.Create(&host1).Error)
	require.NoError(t, db.Create(&host2).Error)
	require.NoError(t, db.Create(&host3).Error)

	// Apply profile2 to all hosts
	reqBody := map[string]any{
		"host_uuids":                 []string{host1.UUID, host2.UUID, host3.UUID},
		"security_header_profile_id": profile2.ID,
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/proxy-hosts/bulk-update-security-headers", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	require.Equal(t, http.StatusOK, resp.Code)

	var result map[string]any
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &result))
	assert.Equal(t, float64(3), result["updated"])

	// Verify all hosts now have profile2
	var updatedHost1, updatedHost2, updatedHost3 models.ProxyHost
	require.NoError(t, db.First(&updatedHost1, "uuid = ?", host1.UUID).Error)
	require.NoError(t, db.First(&updatedHost2, "uuid = ?", host2.UUID).Error)
	require.NoError(t, db.First(&updatedHost3, "uuid = ?", host3.UUID).Error)

	require.NotNil(t, updatedHost1.SecurityHeaderProfileID)
	require.NotNil(t, updatedHost2.SecurityHeaderProfileID)
	require.NotNil(t, updatedHost3.SecurityHeaderProfileID)
	assert.Equal(t, profile2.ID, *updatedHost1.SecurityHeaderProfileID)
	assert.Equal(t, profile2.ID, *updatedHost2.SecurityHeaderProfileID)
	assert.Equal(t, profile2.ID, *updatedHost3.SecurityHeaderProfileID)
}

func TestBulkUpdateSecurityHeaders_SingleHost(t *testing.T) {
	router, db := setupTestRouterForSecurityHeaders(t)

	// Create test security header profile
	profile := models.SecurityHeaderProfile{
		UUID:          uuid.NewString(),
		Name:          "Test Profile",
		IsPreset:      true,
		SecurityScore: 95,
	}
	require.NoError(t, db.Create(&profile).Error)

	// Create single test proxy host
	host := models.ProxyHost{
		UUID:          uuid.NewString(),
		Name:          "Single Host",
		DomainNames:   "single.test.com",
		ForwardScheme: "http",
		ForwardHost:   "localhost",
		ForwardPort:   8001,
	}
	require.NoError(t, db.Create(&host).Error)

	// Apply profile to single host
	reqBody := map[string]any{
		"host_uuids":                 []string{host.UUID},
		"security_header_profile_id": profile.ID,
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/proxy-hosts/bulk-update-security-headers", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	require.Equal(t, http.StatusOK, resp.Code)

	var result map[string]any
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &result))
	assert.Equal(t, float64(1), result["updated"])
	assert.Empty(t, result["errors"])

	// Verify host has the profile assigned
	var updatedHost models.ProxyHost
	require.NoError(t, db.First(&updatedHost, "uuid = ?", host.UUID).Error)
	require.NotNil(t, updatedHost.SecurityHeaderProfileID)
	assert.Equal(t, profile.ID, *updatedHost.SecurityHeaderProfileID)
}
