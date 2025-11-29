package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupAccessListTestRouter(t *testing.T) (*gin.Engine, *gorm.DB) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	assert.NoError(t, err)

	err = db.AutoMigrate(&models.AccessList{}, &models.ProxyHost{})
	assert.NoError(t, err)

	gin.SetMode(gin.TestMode)
	router := gin.New()

	handler := NewAccessListHandler(db)
	router.POST("/access-lists", handler.Create)
	router.GET("/access-lists", handler.List)
	router.GET("/access-lists/:id", handler.Get)
	router.PUT("/access-lists/:id", handler.Update)
	router.DELETE("/access-lists/:id", handler.Delete)
	router.POST("/access-lists/:id/test", handler.TestIP)
	router.GET("/access-lists/templates", handler.GetTemplates)

	return router, db
}

func TestAccessListHandler_Create(t *testing.T) {
	router, _ := setupAccessListTestRouter(t)

	tests := []struct {
		name       string
		payload    map[string]interface{}
		wantStatus int
	}{
		{
			name: "create whitelist successfully",
			payload: map[string]interface{}{
				"name":        "Office Whitelist",
				"description": "Allow office IPs only",
				"type":        "whitelist",
				"ip_rules":    `[{"cidr":"192.168.1.0/24","description":"Office network"}]`,
				"enabled":     true,
			},
			wantStatus: http.StatusCreated,
		},
		{
			name: "create geo whitelist successfully",
			payload: map[string]interface{}{
				"name":          "US Only",
				"type":          "geo_whitelist",
				"country_codes": "US,CA",
				"enabled":       true,
			},
			wantStatus: http.StatusCreated,
		},
		{
			name: "create local network only",
			payload: map[string]interface{}{
				"name":               "Local Network",
				"type":               "whitelist",
				"local_network_only": true,
				"enabled":            true,
			},
			wantStatus: http.StatusCreated,
		},
		{
			name: "fail with invalid type",
			payload: map[string]interface{}{
				"name":    "Invalid",
				"type":    "invalid_type",
				"enabled": true,
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "fail with missing name",
			payload: map[string]interface{}{
				"type":    "whitelist",
				"enabled": true,
			},
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.payload)
			req := httptest.NewRequest(http.MethodPost, "/access-lists", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			assert.Equal(t, tt.wantStatus, w.Code)

			if w.Code == http.StatusCreated {
				var response models.AccessList
				err := json.Unmarshal(w.Body.Bytes(), &response)
				assert.NoError(t, err)
				assert.NotEmpty(t, response.UUID)
				assert.Equal(t, tt.payload["name"], response.Name)
			}
		})
	}
}

func TestAccessListHandler_List(t *testing.T) {
	router, db := setupAccessListTestRouter(t)

	// Create test data
	acls := []models.AccessList{
		{Name: "Test 1", Type: "whitelist", Enabled: true},
		{Name: "Test 2", Type: "blacklist", Enabled: false},
	}
	for i := range acls {
		acls[i].UUID = "test-uuid-" + string(rune(i))
		db.Create(&acls[i])
	}

	req := httptest.NewRequest(http.MethodGet, "/access-lists", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response []models.AccessList
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Len(t, response, 2)
}

func TestAccessListHandler_Get(t *testing.T) {
	router, db := setupAccessListTestRouter(t)

	// Create test ACL
	acl := models.AccessList{
		UUID:    "test-uuid",
		Name:    "Test ACL",
		Type:    "whitelist",
		Enabled: true,
	}
	db.Create(&acl)

	tests := []struct {
		name       string
		id         string
		wantStatus int
	}{
		{
			name:       "get existing ACL",
			id:         "1",
			wantStatus: http.StatusOK,
		},
		{
			name:       "get non-existent ACL",
			id:         "9999",
			wantStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/access-lists/"+tt.id, nil)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			assert.Equal(t, tt.wantStatus, w.Code)

			if w.Code == http.StatusOK {
				var response models.AccessList
				err := json.Unmarshal(w.Body.Bytes(), &response)
				assert.NoError(t, err)
				assert.Equal(t, acl.Name, response.Name)
			}
		})
	}
}

func TestAccessListHandler_Update(t *testing.T) {
	router, db := setupAccessListTestRouter(t)

	// Create test ACL
	acl := models.AccessList{
		UUID:    "test-uuid",
		Name:    "Original Name",
		Type:    "whitelist",
		Enabled: true,
	}
	db.Create(&acl)

	tests := []struct {
		name       string
		id         string
		payload    map[string]interface{}
		wantStatus int
	}{
		{
			name: "update successfully",
			id:   "1",
			payload: map[string]interface{}{
				"name":        "Updated Name",
				"description": "New description",
				"enabled":     false,
				"type":        "whitelist",
				"ip_rules":    `[{"cidr":"10.0.0.0/8","description":"Updated network"}]`,
			},
			wantStatus: http.StatusOK,
		},
		{
			name: "update non-existent ACL",
			id:   "9999",
			payload: map[string]interface{}{
				"name":     "Test",
				"type":     "whitelist",
				"ip_rules": `[]`,
			},
			wantStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.payload)
			req := httptest.NewRequest(http.MethodPut, "/access-lists/"+tt.id, bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Logf("Response body: %s", w.Body.String())
			}
			assert.Equal(t, tt.wantStatus, w.Code)

			if w.Code == http.StatusOK {
				var response models.AccessList
				err := json.Unmarshal(w.Body.Bytes(), &response)
				assert.NoError(t, err)
				if name, ok := tt.payload["name"].(string); ok {
					assert.Equal(t, name, response.Name)
				}
			}
		})
	}
}

func TestAccessListHandler_Delete(t *testing.T) {
	router, db := setupAccessListTestRouter(t)

	// Create test ACL
	acl := models.AccessList{
		UUID:    "test-uuid",
		Name:    "Test ACL",
		Type:    "whitelist",
		Enabled: true,
	}
	db.Create(&acl)

	// Create ACL in use
	aclInUse := models.AccessList{
		UUID:    "in-use-uuid",
		Name:    "In Use ACL",
		Type:    "whitelist",
		Enabled: true,
	}
	db.Create(&aclInUse)

	host := models.ProxyHost{
		UUID:         "host-uuid",
		Name:         "Test Host",
		DomainNames:  "test.com",
		ForwardHost:  "localhost",
		ForwardPort:  8080,
		AccessListID: &aclInUse.ID,
	}
	db.Create(&host)

	tests := []struct {
		name       string
		id         string
		wantStatus int
	}{
		{
			name:       "delete successfully",
			id:         "1",
			wantStatus: http.StatusOK,
		},
		{
			name:       "fail to delete ACL in use",
			id:         "2",
			wantStatus: http.StatusConflict,
		},
		{
			name:       "delete non-existent ACL",
			id:         "9999",
			wantStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodDelete, "/access-lists/"+tt.id, nil)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			assert.Equal(t, tt.wantStatus, w.Code)
		})
	}
}

func TestAccessListHandler_TestIP(t *testing.T) {
	router, db := setupAccessListTestRouter(t)

	// Create test ACL
	acl := models.AccessList{
		UUID:    "test-uuid",
		Name:    "Test Whitelist",
		Type:    "whitelist",
		IPRules: `[{"cidr":"192.168.1.0/24","description":"Test network"}]`,
		Enabled: true,
	}
	db.Create(&acl)

	tests := []struct {
		name       string
		id         string
		payload    map[string]string
		wantStatus int
	}{
		{
			name:       "test IP in whitelist",
			id:         "1", // Use numeric ID
			payload:    map[string]string{"ip_address": "192.168.1.100"},
			wantStatus: http.StatusOK,
		},
		{
			name:       "test IP not in whitelist",
			id:         "1",
			payload:    map[string]string{"ip_address": "10.0.0.1"},
			wantStatus: http.StatusOK,
		},
		{
			name:       "test invalid IP",
			id:         "1",
			payload:    map[string]string{"ip_address": "invalid"},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "test non-existent ACL",
			id:         "9999",
			payload:    map[string]string{"ip_address": "192.168.1.100"},
			wantStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.payload)
			req := httptest.NewRequest(http.MethodPost, "/access-lists/"+tt.id+"/test", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			assert.Equal(t, tt.wantStatus, w.Code)

			if w.Code == http.StatusOK {
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				assert.NoError(t, err)
				assert.Contains(t, response, "allowed")
				assert.Contains(t, response, "reason")
			}
		})
	}
}

func TestAccessListHandler_GetTemplates(t *testing.T) {
	router, _ := setupAccessListTestRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/access-lists/templates", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response []map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.NotEmpty(t, response)
	assert.Greater(t, len(response), 0)

	// Verify template structure
	for _, template := range response {
		assert.Contains(t, template, "name")
		assert.Contains(t, template, "description")
		assert.Contains(t, template, "type")
	}
}
