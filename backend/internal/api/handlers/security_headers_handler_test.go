package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupSecurityHeadersTestRouter(t *testing.T) (*gin.Engine, *gorm.DB) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	assert.NoError(t, err)

	err = db.AutoMigrate(&models.SecurityHeaderProfile{}, &models.ProxyHost{})
	assert.NoError(t, err)

	gin.SetMode(gin.TestMode)
	router := gin.New()

	handler := NewSecurityHeadersHandler(db, nil)
	handler.RegisterRoutes(router.Group("/"))

	return router, db
}

func TestListProfiles(t *testing.T) {
	router, db := setupSecurityHeadersTestRouter(t)

	// Create test profiles
	profile1 := models.SecurityHeaderProfile{
		UUID: uuid.New().String(),
		Name: "Profile 1",
	}
	db.Create(&profile1)

	profile2 := models.SecurityHeaderProfile{
		UUID:     uuid.New().String(),
		Name:     "Profile 2",
		IsPreset: true,
	}
	db.Create(&profile2)

	req := httptest.NewRequest(http.MethodGet, "/security/headers/profiles", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string][]models.SecurityHeaderProfile
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Len(t, response["profiles"], 2)
}

func TestGetProfile_ByID(t *testing.T) {
	router, db := setupSecurityHeadersTestRouter(t)

	profile := models.SecurityHeaderProfile{
		UUID: uuid.New().String(),
		Name: "Test Profile",
	}
	db.Create(&profile)

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/security/headers/profiles/%d", profile.ID), nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]models.SecurityHeaderProfile
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "Test Profile", response["profile"].Name)
}

func TestGetProfile_ByUUID(t *testing.T) {
	router, db := setupSecurityHeadersTestRouter(t)

	testUUID := uuid.New().String()
	profile := models.SecurityHeaderProfile{
		UUID: testUUID,
		Name: "Test Profile",
	}
	db.Create(&profile)

	req := httptest.NewRequest(http.MethodGet, "/security/headers/profiles/"+testUUID, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]models.SecurityHeaderProfile
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "Test Profile", response["profile"].Name)
	assert.Equal(t, testUUID, response["profile"].UUID)
}

func TestGetProfile_NotFound(t *testing.T) {
	router, _ := setupSecurityHeadersTestRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/security/headers/profiles/99999", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestCreateProfile(t *testing.T) {
	router, _ := setupSecurityHeadersTestRouter(t)

	payload := map[string]interface{}{
		"name":                   "New Profile",
		"hsts_enabled":           true,
		"hsts_max_age":           31536000,
		"x_frame_options":        "DENY",
		"x_content_type_options": true,
	}

	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/security/headers/profiles", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var response map[string]models.SecurityHeaderProfile
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "New Profile", response["profile"].Name)
	assert.NotEmpty(t, response["profile"].UUID)
	assert.NotZero(t, response["profile"].SecurityScore)
}

func TestCreateProfile_MissingName(t *testing.T) {
	router, _ := setupSecurityHeadersTestRouter(t)

	payload := map[string]interface{}{
		"hsts_enabled": true,
	}

	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/security/headers/profiles", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestUpdateProfile(t *testing.T) {
	router, db := setupSecurityHeadersTestRouter(t)

	profile := models.SecurityHeaderProfile{
		UUID: uuid.New().String(),
		Name: "Original Name",
	}
	db.Create(&profile)

	updates := map[string]interface{}{
		"name":           "Updated Name",
		"hsts_enabled":   false,
		"csp_enabled":    true,
		"csp_directives": `{"default-src":["'self'"]}`,
	}

	body, _ := json.Marshal(updates)
	req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/security/headers/profiles/%d", profile.ID), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]models.SecurityHeaderProfile
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "Updated Name", response["profile"].Name)
	assert.False(t, response["profile"].HSTSEnabled)
	assert.True(t, response["profile"].CSPEnabled)
}

func TestUpdateProfile_CannotModifyPreset(t *testing.T) {
	router, db := setupSecurityHeadersTestRouter(t)

	preset := models.SecurityHeaderProfile{
		UUID:     uuid.New().String(),
		Name:     "Preset",
		IsPreset: true,
	}
	db.Create(&preset)

	updates := map[string]interface{}{
		"name": "Modified Preset",
	}

	body, _ := json.Marshal(updates)
	req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/security/headers/profiles/%d", preset.ID), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestDeleteProfile(t *testing.T) {
	router, db := setupSecurityHeadersTestRouter(t)

	profile := models.SecurityHeaderProfile{
		UUID: uuid.New().String(),
		Name: "To Delete",
	}
	db.Create(&profile)

	req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/security/headers/profiles/%d", profile.ID), nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// Verify deleted
	var count int64
	db.Model(&models.SecurityHeaderProfile{}).Where("id = ?", profile.ID).Count(&count)
	assert.Equal(t, int64(0), count)
}

func TestDeleteProfile_CannotDeletePreset(t *testing.T) {
	router, db := setupSecurityHeadersTestRouter(t)

	preset := models.SecurityHeaderProfile{
		UUID:     uuid.New().String(),
		Name:     "Preset",
		IsPreset: true,
	}
	db.Create(&preset)

	req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/security/headers/profiles/%d", preset.ID), nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestDeleteProfile_InUse(t *testing.T) {
	router, db := setupSecurityHeadersTestRouter(t)

	profile := models.SecurityHeaderProfile{
		UUID: uuid.New().String(),
		Name: "In Use",
	}
	db.Create(&profile)

	// Create proxy host using this profile
	host := models.ProxyHost{
		UUID:                    uuid.New().String(),
		DomainNames:             "example.com",
		ForwardHost:             "localhost",
		ForwardPort:             8080,
		SecurityHeaderProfileID: &profile.ID,
	}
	db.Create(&host)

	req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/security/headers/profiles/%d", profile.ID), nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusConflict, w.Code)
}

func TestGetPresets(t *testing.T) {
	router, _ := setupSecurityHeadersTestRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/security/headers/presets", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string][]models.SecurityHeaderProfile
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Len(t, response["presets"], 4)

	// Verify preset types
	presetTypes := make(map[string]bool)
	for _, preset := range response["presets"] {
		presetTypes[preset.PresetType] = true
	}
	assert.True(t, presetTypes["basic"])
	assert.True(t, presetTypes["api-friendly"])
	assert.True(t, presetTypes["strict"])
	assert.True(t, presetTypes["paranoid"])
}

func TestApplyPreset(t *testing.T) {
	router, _ := setupSecurityHeadersTestRouter(t)

	payload := map[string]interface{}{
		"preset_type": "basic",
		"name":        "My Basic Profile",
	}

	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/security/headers/presets/apply", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var response map[string]models.SecurityHeaderProfile
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "My Basic Profile", response["profile"].Name)
	assert.False(t, response["profile"].IsPreset) // Should not be a preset
	assert.Empty(t, response["profile"].PresetType)
	assert.NotEmpty(t, response["profile"].UUID)
}

func TestApplyPreset_InvalidType(t *testing.T) {
	router, _ := setupSecurityHeadersTestRouter(t)

	payload := map[string]interface{}{
		"preset_type": "nonexistent",
		"name":        "Test",
	}

	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/security/headers/presets/apply", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCalculateScore(t *testing.T) {
	router, _ := setupSecurityHeadersTestRouter(t)

	payload := map[string]interface{}{
		"hsts_enabled":                 true,
		"hsts_max_age":                 31536000,
		"hsts_include_subdomains":      true,
		"hsts_preload":                 true,
		"csp_enabled":                  true,
		"csp_directives":               `{"default-src":["'self'"]}`,
		"x_frame_options":              "DENY",
		"x_content_type_options":       true,
		"referrer_policy":              "no-referrer",
		"permissions_policy":           `[{"feature":"camera","allowlist":[]}]`,
		"cross_origin_opener_policy":   "same-origin",
		"cross_origin_resource_policy": "same-origin",
		"cross_origin_embedder_policy": "require-corp",
	}

	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/security/headers/score", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, float64(100), response["score"])
	assert.Equal(t, float64(100), response["max_score"])
	assert.NotNil(t, response["breakdown"])
}

func TestValidateCSP_Valid(t *testing.T) {
	router, _ := setupSecurityHeadersTestRouter(t)

	payload := map[string]interface{}{
		"csp": `{"default-src":["'self'"],"script-src":["'self'"]}`,
	}

	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/security/headers/csp/validate", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.True(t, response["valid"].(bool))
}

func TestValidateCSP_Invalid(t *testing.T) {
	router, _ := setupSecurityHeadersTestRouter(t)

	payload := map[string]interface{}{
		"csp": `not valid json`,
	}

	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/security/headers/csp/validate", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.False(t, response["valid"].(bool))
	assert.NotEmpty(t, response["errors"])
}

func TestValidateCSP_UnsafeDirectives(t *testing.T) {
	router, _ := setupSecurityHeadersTestRouter(t)

	payload := map[string]interface{}{
		"csp": `{"default-src":["'self'"],"script-src":["'self'","'unsafe-inline'","'unsafe-eval'"]}`,
	}

	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/security/headers/csp/validate", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.False(t, response["valid"].(bool))
	errors := response["errors"].([]interface{})
	assert.NotEmpty(t, errors)
}

func TestBuildCSP(t *testing.T) {
	router, _ := setupSecurityHeadersTestRouter(t)

	payload := map[string]interface{}{
		"directives": []map[string]interface{}{
			{
				"directive": "default-src",
				"values":    []string{"'self'"},
			},
			{
				"directive": "script-src",
				"values":    []string{"'self'", "https:"},
			},
		},
	}

	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/security/headers/csp/build", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.NotEmpty(t, response["csp"])

	// Verify it's valid JSON
	var cspMap map[string][]string
	err = json.Unmarshal([]byte(response["csp"]), &cspMap)
	assert.NoError(t, err)
	assert.Equal(t, []string{"'self'"}, cspMap["default-src"])
	assert.Equal(t, []string{"'self'", "https:"}, cspMap["script-src"])
}
