package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupAuditLogTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open test database: %v", err)
	}

	if err := db.AutoMigrate(&models.SecurityAudit{}); err != nil {
		t.Fatalf("failed to migrate test database: %v", err)
	}

	return db
}

func TestAuditLogHandler_List(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupAuditLogTestDB(t)
	securityService := services.NewSecurityService(db)
	handler := NewAuditLogHandler(securityService)

	// Create test audit logs
	now := time.Now()
	testAudits := []models.SecurityAudit{
		{
			UUID:          "audit-1",
			Actor:         "user-1",
			Action:        "dns_provider_create",
			EventCategory: "dns_provider",
			ResourceUUID:  "provider-1",
			Details:       `{"name":"Test Provider"}`,
			IPAddress:     "192.168.1.1",
			UserAgent:     "Mozilla/5.0",
			CreatedAt:     now,
		},
		{
			UUID:          "audit-2",
			Actor:         "user-2",
			Action:        "dns_provider_update",
			EventCategory: "dns_provider",
			ResourceUUID:  "provider-2",
			Details:       `{"changed_fields":{"name":true}}`,
			IPAddress:     "192.168.1.2",
			UserAgent:     "Mozilla/5.0",
			CreatedAt:     now.Add(-1 * time.Hour),
		},
	}

	for _, audit := range testAudits {
		if err := db.Create(&audit).Error; err != nil {
			t.Fatalf("failed to create test audit: %v", err)
		}
	}

	tests := []struct {
		name           string
		queryParams    string
		expectedStatus int
		expectedCount  int
	}{
		{
			name:           "List all audit logs",
			queryParams:    "",
			expectedStatus: http.StatusOK,
			expectedCount:  2,
		},
		{
			name:           "Filter by actor",
			queryParams:    "?actor=user-1",
			expectedStatus: http.StatusOK,
			expectedCount:  1,
		},
		{
			name:           "Filter by action",
			queryParams:    "?action=dns_provider_create",
			expectedStatus: http.StatusOK,
			expectedCount:  1,
		},
		{
			name:           "Filter by event_category",
			queryParams:    "?event_category=dns_provider",
			expectedStatus: http.StatusOK,
			expectedCount:  2,
		},
		{
			name:           "Pagination - page 1, limit 1",
			queryParams:    "?page=1&limit=1",
			expectedStatus: http.StatusOK,
			expectedCount:  1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request, _ = http.NewRequest(http.MethodGet, "/api/v1/audit-logs"+tt.queryParams, nil)

			handler.List(c)

			assert.Equal(t, tt.expectedStatus, w.Code)

			if w.Code == http.StatusOK {
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				assert.NoError(t, err)

				audits := response["audit_logs"].([]interface{})
				assert.Equal(t, tt.expectedCount, len(audits))
			}
		})
	}
}

func TestAuditLogHandler_Get(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupAuditLogTestDB(t)
	securityService := services.NewSecurityService(db)
	handler := NewAuditLogHandler(securityService)

	// Create test audit log
	testAudit := models.SecurityAudit{
		UUID:          "audit-test-uuid",
		Actor:         "user-1",
		Action:        "dns_provider_create",
		EventCategory: "dns_provider",
		ResourceUUID:  "provider-1",
		Details:       `{"name":"Test Provider"}`,
		IPAddress:     "192.168.1.1",
		UserAgent:     "Mozilla/5.0",
		CreatedAt:     time.Now(),
	}

	if err := db.Create(&testAudit).Error; err != nil {
		t.Fatalf("failed to create test audit: %v", err)
	}

	tests := []struct {
		name           string
		uuid           string
		expectedStatus int
	}{
		{
			name:           "Get existing audit log",
			uuid:           "audit-test-uuid",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Get non-existent audit log",
			uuid:           "non-existent-uuid",
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "Get with empty UUID",
			uuid:           "",
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Params = gin.Params{gin.Param{Key: "uuid", Value: tt.uuid}}
			c.Request, _ = http.NewRequest(http.MethodGet, "/api/v1/audit-logs/"+tt.uuid, nil)

			handler.Get(c)

			assert.Equal(t, tt.expectedStatus, w.Code)

			if w.Code == http.StatusOK {
				var response models.SecurityAudit
				err := json.Unmarshal(w.Body.Bytes(), &response)
				assert.NoError(t, err)
				assert.Equal(t, testAudit.UUID, response.UUID)
				assert.Equal(t, testAudit.Actor, response.Actor)
			}
		})
	}
}

func TestAuditLogHandler_ListByProvider(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupAuditLogTestDB(t)
	securityService := services.NewSecurityService(db)
	handler := NewAuditLogHandler(securityService)

	// Create test audit logs
	providerID := uint(123)
	now := time.Now()
	testAudits := []models.SecurityAudit{
		{
			UUID:          "audit-provider-1",
			Actor:         "user-1",
			Action:        "dns_provider_create",
			EventCategory: "dns_provider",
			ResourceID:    &providerID,
			ResourceUUID:  "provider-uuid-1",
			Details:       `{"name":"Test Provider"}`,
			CreatedAt:     now,
		},
		{
			UUID:          "audit-provider-2",
			Actor:         "user-1",
			Action:        "dns_provider_update",
			EventCategory: "dns_provider",
			ResourceID:    &providerID,
			ResourceUUID:  "provider-uuid-1",
			Details:       `{"changed_fields":{"name":true}}`,
			CreatedAt:     now.Add(-1 * time.Hour),
		},
	}

	for _, audit := range testAudits {
		if err := db.Create(&audit).Error; err != nil {
			t.Fatalf("failed to create test audit: %v", err)
		}
	}

	tests := []struct {
		name           string
		providerID     string
		expectedStatus int
		expectedCount  int
	}{
		{
			name:           "List audit logs for provider",
			providerID:     "123",
			expectedStatus: http.StatusOK,
			expectedCount:  2,
		},
		{
			name:           "List audit logs for non-existent provider",
			providerID:     "999",
			expectedStatus: http.StatusOK,
			expectedCount:  0,
		},
		{
			name:           "Invalid provider ID",
			providerID:     "invalid",
			expectedStatus: http.StatusBadRequest,
			expectedCount:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Params = gin.Params{gin.Param{Key: "id", Value: tt.providerID}}
			c.Request, _ = http.NewRequest(http.MethodGet, "/api/v1/dns-providers/"+tt.providerID+"/audit-logs", nil)

			handler.ListByProvider(c)

			assert.Equal(t, tt.expectedStatus, w.Code)

			if w.Code == http.StatusOK {
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				assert.NoError(t, err)

				audits := response["audit_logs"].([]interface{})
				assert.Equal(t, tt.expectedCount, len(audits))
			}
		})
	}
}

func TestAuditLogHandler_ListWithDateFilters(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupAuditLogTestDB(t)
	securityService := services.NewSecurityService(db)
	handler := NewAuditLogHandler(securityService)

	// Create test audit logs with different timestamps
	now := time.Now()
	yesterday := now.Add(-24 * time.Hour)
	twoDaysAgo := now.Add(-48 * time.Hour)

	testAudits := []models.SecurityAudit{
		{
			UUID:          "audit-today",
			Actor:         "user-1",
			Action:        "dns_provider_create",
			EventCategory: "dns_provider",
			CreatedAt:     now,
		},
		{
			UUID:          "audit-yesterday",
			Actor:         "user-1",
			Action:        "dns_provider_update",
			EventCategory: "dns_provider",
			CreatedAt:     yesterday,
		},
		{
			UUID:          "audit-two-days-ago",
			Actor:         "user-1",
			Action:        "dns_provider_delete",
			EventCategory: "dns_provider",
			CreatedAt:     twoDaysAgo,
		},
	}

	for _, audit := range testAudits {
		if err := db.Create(&audit).Error; err != nil {
			t.Fatalf("failed to create test audit: %v", err)
		}
	}

	tests := []struct {
		name          string
		queryParams   string
		expectedCount int
	}{
		{
			name:          "Filter by start_date",
			queryParams:   "?start_date=" + yesterday.Add(-1*time.Hour).Format(time.RFC3339),
			expectedCount: 2,
		},
		{
			name:          "Filter by end_date",
			queryParams:   "?end_date=" + yesterday.Add(1*time.Hour).Format(time.RFC3339),
			expectedCount: 2,
		},
		{
			name:          "Filter by date range",
			queryParams:   "?start_date=" + twoDaysAgo.Add(-1*time.Hour).Format(time.RFC3339) + "&end_date=" + yesterday.Add(1*time.Hour).Format(time.RFC3339),
			expectedCount: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request, _ = http.NewRequest(http.MethodGet, "/api/v1/audit-logs"+tt.queryParams, nil)

			handler.List(c)

			assert.Equal(t, http.StatusOK, w.Code)

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			assert.NoError(t, err)

			audits := response["audit_logs"].([]interface{})
			assert.Equal(t, tt.expectedCount, len(audits))
		})
	}
}
