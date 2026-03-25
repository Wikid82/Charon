package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sort"
	"testing"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/services"
	"github.com/Wikid82/charon/backend/pkg/dnsprovider"
	_ "github.com/Wikid82/charon/backend/pkg/dnsprovider/builtin" // Auto-register DNS providers
	_ "github.com/Wikid82/charon/backend/pkg/dnsprovider/custom"  // Auto-register custom providers (manual)
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockDNSProviderService is a mock implementation of DNSProviderService for testing.
type MockDNSProviderService struct {
	mock.Mock
}

func (m *MockDNSProviderService) List(ctx context.Context) ([]models.DNSProvider, error) {
	args := m.Called(ctx)
	return args.Get(0).([]models.DNSProvider), args.Error(1)
}

func (m *MockDNSProviderService) Get(ctx context.Context, id uint) (*models.DNSProvider, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.DNSProvider), args.Error(1)
}

func (m *MockDNSProviderService) GetByUUID(ctx context.Context, uuid string) (*models.DNSProvider, error) {
	args := m.Called(ctx, uuid)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.DNSProvider), args.Error(1)
}

func (m *MockDNSProviderService) Create(ctx context.Context, req services.CreateDNSProviderRequest) (*models.DNSProvider, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.DNSProvider), args.Error(1)
}

func (m *MockDNSProviderService) Update(ctx context.Context, id uint, req services.UpdateDNSProviderRequest) (*models.DNSProvider, error) {
	args := m.Called(ctx, id, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.DNSProvider), args.Error(1)
}

func (m *MockDNSProviderService) Delete(ctx context.Context, id uint) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockDNSProviderService) Test(ctx context.Context, id uint) (*services.TestResult, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*services.TestResult), args.Error(1)
}

func (m *MockDNSProviderService) TestCredentials(ctx context.Context, req services.CreateDNSProviderRequest) (*services.TestResult, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*services.TestResult), args.Error(1)
}

func (m *MockDNSProviderService) GetSupportedProviderTypes() []string {
	args := m.Called()
	return args.Get(0).([]string)
}

func (m *MockDNSProviderService) GetProviderCredentialFields(providerType string) ([]dnsprovider.CredentialFieldSpec, error) {
	args := m.Called(providerType)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]dnsprovider.CredentialFieldSpec), args.Error(1)
}

func (m *MockDNSProviderService) GetDecryptedCredentials(ctx context.Context, id uint) (map[string]string, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[string]string), args.Error(1)
}

func setupDNSProviderTestRouter() (*gin.Engine, *MockDNSProviderService) {
	router := gin.New()
	mockService := new(MockDNSProviderService)
	handler := NewDNSProviderHandler(mockService)

	api := router.Group("/api/v1")
	{
		api.GET("/dns-providers", handler.List)
		api.GET("/dns-providers/:id", handler.Get)
		api.POST("/dns-providers", handler.Create)
		api.PUT("/dns-providers/:id", handler.Update)
		api.DELETE("/dns-providers/:id", handler.Delete)
		api.POST("/dns-providers/:id/test", handler.Test)
		api.POST("/dns-providers/test", handler.TestCredentials)
		api.GET("/dns-providers/types", handler.GetTypes)
	}

	return router, mockService
}

func TestDNSProviderHandler_List(t *testing.T) {
	router, mockService := setupDNSProviderTestRouter()

	t.Run("success", func(t *testing.T) {
		providers := []models.DNSProvider{
			{
				ID:                   1,
				UUID:                 "uuid-1",
				Name:                 "Cloudflare",
				ProviderType:         "cloudflare",
				Enabled:              true,
				IsDefault:            true,
				CredentialsEncrypted: "encrypted-data",
			},
			{
				ID:                   2,
				UUID:                 "uuid-2",
				Name:                 "Route53",
				ProviderType:         "route53",
				Enabled:              true,
				IsDefault:            false,
				CredentialsEncrypted: "encrypted-data-2",
			},
		}

		mockService.On("List", mock.Anything).Return(providers, nil)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/dns-providers", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.Equal(t, float64(2), response["total"])
		providersArray := response["providers"].([]interface{})
		assert.Len(t, providersArray, 2)

		// Verify credentials are not exposed
		provider1 := providersArray[0].(map[string]interface{})
		assert.True(t, provider1["has_credentials"].(bool))
		assert.NotContains(t, provider1, "credentials_encrypted")

		mockService.AssertExpectations(t)
	})

	t.Run("service error", func(t *testing.T) {
		mockService := new(MockDNSProviderService)
		handler := NewDNSProviderHandler(mockService)
		router := gin.New()
		router.GET("/dns-providers", handler.List)

		mockService.On("List", mock.Anything).Return([]models.DNSProvider{}, errors.New("database error"))

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/dns-providers", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
		mockService.AssertExpectations(t)
	})
}

func TestDNSProviderHandler_Get(t *testing.T) {
	router, mockService := setupDNSProviderTestRouter()

	t.Run("success", func(t *testing.T) {
		provider := &models.DNSProvider{
			ID:                   1,
			UUID:                 "uuid-1",
			Name:                 "Test Provider",
			ProviderType:         "cloudflare",
			Enabled:              true,
			CredentialsEncrypted: "encrypted-data",
		}

		mockService.On("Get", mock.Anything, uint(1)).Return(provider, nil)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/dns-providers/1", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response services.DNSProviderResponse
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.Equal(t, "uuid-1", response.UUID)
		assert.Equal(t, "Test Provider", response.Name)
		assert.True(t, response.HasCredentials)

		mockService.AssertExpectations(t)
	})

	t.Run("not found", func(t *testing.T) {
		mockService := new(MockDNSProviderService)
		handler := NewDNSProviderHandler(mockService)
		router := gin.New()
		router.GET("/dns-providers/:id", handler.Get)

		mockService.On("Get", mock.Anything, uint(999)).Return(nil, services.ErrDNSProviderNotFound)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/dns-providers/999", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
		mockService.AssertExpectations(t)
	})

	t.Run("invalid id", func(t *testing.T) {
		mockService := new(MockDNSProviderService)
		handler := NewDNSProviderHandler(mockService)
		router := gin.New()
		router.GET("/dns-providers/:id", handler.Get)

		// Non-numeric IDs are treated as UUIDs, returning not found
		mockService.On("GetByUUID", mock.Anything, "invalid").Return(nil, services.ErrDNSProviderNotFound)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/dns-providers/invalid", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
		mockService.AssertExpectations(t)
	})
}

func TestDNSProviderHandler_Create(t *testing.T) {
	router, mockService := setupDNSProviderTestRouter()

	t.Run("success", func(t *testing.T) {
		reqBody := services.CreateDNSProviderRequest{
			Name:         "Test Provider",
			ProviderType: "cloudflare",
			Credentials: map[string]string{
				"api_token": "test-token",
			},
			PropagationTimeout: 120,
			PollingInterval:    5,
			IsDefault:          true,
		}

		createdProvider := &models.DNSProvider{
			ID:                   1,
			UUID:                 "uuid-1",
			Name:                 reqBody.Name,
			ProviderType:         reqBody.ProviderType,
			Enabled:              true,
			IsDefault:            reqBody.IsDefault,
			PropagationTimeout:   reqBody.PropagationTimeout,
			PollingInterval:      reqBody.PollingInterval,
			CredentialsEncrypted: "encrypted-data",
		}

		mockService.On("Create", mock.Anything, reqBody).Return(createdProvider, nil)

		body, _ := json.Marshal(reqBody)
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/api/v1/dns-providers", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusCreated, w.Code)

		var response services.DNSProviderResponse
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.Equal(t, "uuid-1", response.UUID)
		assert.Equal(t, "Test Provider", response.Name)
		assert.True(t, response.HasCredentials)

		mockService.AssertExpectations(t)
	})

	t.Run("validation error", func(t *testing.T) {
		reqBody := map[string]interface{}{
			"name": "Missing Provider Type",
			// Missing provider_type and credentials
		}

		body, _ := json.Marshal(reqBody)
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/api/v1/dns-providers", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("invalid provider type", func(t *testing.T) {
		mockService := new(MockDNSProviderService)
		handler := NewDNSProviderHandler(mockService)
		router := gin.New()
		router.POST("/dns-providers", handler.Create)

		reqBody := services.CreateDNSProviderRequest{
			Name:         "Test",
			ProviderType: "invalid",
			Credentials:  map[string]string{"key": "value"},
		}

		mockService.On("Create", mock.Anything, reqBody).Return(nil, services.ErrInvalidProviderType)

		body, _ := json.Marshal(reqBody)
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/dns-providers", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		mockService.AssertExpectations(t)
	})

	t.Run("invalid credentials", func(t *testing.T) {
		mockService := new(MockDNSProviderService)
		handler := NewDNSProviderHandler(mockService)
		router := gin.New()
		router.POST("/dns-providers", handler.Create)

		reqBody := services.CreateDNSProviderRequest{
			Name:         "Test",
			ProviderType: "cloudflare",
			Credentials:  map[string]string{},
		}

		mockService.On("Create", mock.Anything, reqBody).Return(nil, services.ErrInvalidCredentials)

		body, _ := json.Marshal(reqBody)
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/dns-providers", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		mockService.AssertExpectations(t)
	})
}

func TestDNSProviderHandler_Update(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mockService := new(MockDNSProviderService)
		handler := NewDNSProviderHandler(mockService)
		router := gin.New()
		router.PUT("/dns-providers/:id", handler.Update)

		existingProvider := &models.DNSProvider{
			ID:                   1,
			UUID:                 "uuid-1",
			Name:                 "Old Name",
			ProviderType:         "cloudflare",
			Enabled:              true,
			CredentialsEncrypted: "encrypted-data",
		}

		newName := "Updated Name"
		reqBody := services.UpdateDNSProviderRequest{
			Name: &newName,
		}

		updatedProvider := &models.DNSProvider{
			ID:                   1,
			UUID:                 "uuid-1",
			Name:                 newName,
			ProviderType:         "cloudflare",
			Enabled:              true,
			CredentialsEncrypted: "encrypted-data",
		}

		// resolveProvider calls Get first
		mockService.On("Get", mock.Anything, uint(1)).Return(existingProvider, nil)
		mockService.On("Update", mock.Anything, uint(1), reqBody).Return(updatedProvider, nil)

		body, _ := json.Marshal(reqBody)
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("PUT", "/dns-providers/1", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response services.DNSProviderResponse
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.Equal(t, newName, response.Name)

		mockService.AssertExpectations(t)
	})

	t.Run("not found", func(t *testing.T) {
		mockService := new(MockDNSProviderService)
		handler := NewDNSProviderHandler(mockService)
		router := gin.New()
		router.PUT("/dns-providers/:id", handler.Update)

		// resolveProvider calls Get first, which returns not found
		mockService.On("Get", mock.Anything, uint(999)).Return(nil, services.ErrDNSProviderNotFound)

		name := "Test"
		reqBody := services.UpdateDNSProviderRequest{Name: &name}

		body, _ := json.Marshal(reqBody)
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("PUT", "/dns-providers/999", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
		mockService.AssertExpectations(t)
	})
}

func TestDNSProviderHandler_Delete(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mockService := new(MockDNSProviderService)
		handler := NewDNSProviderHandler(mockService)
		router := gin.New()
		router.DELETE("/dns-providers/:id", handler.Delete)

		existingProvider := &models.DNSProvider{
			ID:           1,
			UUID:         "uuid-1",
			Name:         "Test Provider",
			ProviderType: "cloudflare",
		}

		// resolveProvider calls Get first
		mockService.On("Get", mock.Anything, uint(1)).Return(existingProvider, nil)
		mockService.On("Delete", mock.Anything, uint(1)).Return(nil)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("DELETE", "/dns-providers/1", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.Contains(t, response["message"], "deleted successfully")

		mockService.AssertExpectations(t)
	})

	t.Run("not found", func(t *testing.T) {
		mockService := new(MockDNSProviderService)
		handler := NewDNSProviderHandler(mockService)
		router := gin.New()
		router.DELETE("/dns-providers/:id", handler.Delete)

		// resolveProvider calls Get first, which returns not found
		mockService.On("Get", mock.Anything, uint(999)).Return(nil, services.ErrDNSProviderNotFound)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("DELETE", "/dns-providers/999", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
		mockService.AssertExpectations(t)
	})
}

func TestDNSProviderHandler_Test(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mockService := new(MockDNSProviderService)
		handler := NewDNSProviderHandler(mockService)
		router := gin.New()
		router.POST("/dns-providers/:id/test", handler.Test)

		existingProvider := &models.DNSProvider{
			ID:           1,
			UUID:         "uuid-1",
			Name:         "Test Provider",
			ProviderType: "cloudflare",
		}

		testResult := &services.TestResult{
			Success:           true,
			Message:           "Credentials validated successfully",
			PropagationTimeMs: 1234,
		}

		// resolveProvider calls Get first
		mockService.On("Get", mock.Anything, uint(1)).Return(existingProvider, nil)
		mockService.On("Test", mock.Anything, uint(1)).Return(testResult, nil)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/dns-providers/1/test", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response services.TestResult
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.True(t, response.Success)
		assert.Equal(t, "Credentials validated successfully", response.Message)

		mockService.AssertExpectations(t)
	})

	t.Run("not found", func(t *testing.T) {
		mockService := new(MockDNSProviderService)
		handler := NewDNSProviderHandler(mockService)
		router := gin.New()
		router.POST("/dns-providers/:id/test", handler.Test)

		// resolveProvider calls Get first, which returns not found
		mockService.On("Get", mock.Anything, uint(999)).Return(nil, services.ErrDNSProviderNotFound)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/dns-providers/999/test", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
		mockService.AssertExpectations(t)
	})
}

func TestDNSProviderHandler_TestCredentials(t *testing.T) {
	router, mockService := setupDNSProviderTestRouter()

	t.Run("success", func(t *testing.T) {
		reqBody := services.CreateDNSProviderRequest{
			Name:         "Test",
			ProviderType: "cloudflare",
			Credentials:  map[string]string{"api_token": "token"},
		}

		testResult := &services.TestResult{
			Success: true,
			Message: "Credentials validated",
		}

		mockService.On("TestCredentials", mock.Anything, reqBody).Return(testResult, nil)

		body, _ := json.Marshal(reqBody)
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/api/v1/dns-providers/test", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response services.TestResult
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.True(t, response.Success)

		mockService.AssertExpectations(t)
	})

	t.Run("validation error", func(t *testing.T) {
		reqBody := map[string]interface{}{
			"name": "Test",
			// Missing provider_type and credentials
		}

		body, _ := json.Marshal(reqBody)
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/api/v1/dns-providers/test", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

func TestDNSProviderHandler_GetTypes(t *testing.T) {
	router, _ := setupDNSProviderTestRouter()

	t.Run("returns registry-driven types", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/dns-providers/types", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		types := response["types"].([]interface{})
		assert.NotEmpty(t, types)

		// Build a map for easier lookup
		typeMap := make(map[string]map[string]interface{})
		for _, providerData := range types {
			typeData := providerData.(map[string]interface{})
			typeMap[typeData["type"].(string)] = typeData
		}

		// Verify cloudflare (built-in) is present with correct metadata
		cloudflare, exists := typeMap["cloudflare"]
		assert.True(t, exists, "cloudflare provider should exist")
		if exists {
			assert.Equal(t, "Cloudflare", cloudflare["name"])
			assert.Equal(t, true, cloudflare["is_built_in"])
			assert.NotEmpty(t, cloudflare["description"])
			assert.NotEmpty(t, cloudflare["documentation_url"])
			assert.NotEmpty(t, cloudflare["fields"])

			// Verify field structure
			fields := cloudflare["fields"].([]interface{})
			assert.NotEmpty(t, fields)
			firstField := fields[0].(map[string]interface{})
			assert.NotEmpty(t, firstField["name"])
			assert.NotEmpty(t, firstField["label"])
			assert.NotEmpty(t, firstField["type"])
			_, hasRequired := firstField["required"]
			assert.True(t, hasRequired, "field should have required attribute")
		}

		// Verify manual (custom, non-built-in) is present
		manual, exists := typeMap["manual"]
		assert.True(t, exists, "manual provider should exist")
		if exists {
			assert.Equal(t, "Manual (No Automation)", manual["name"])
			assert.Equal(t, false, manual["is_built_in"])
			assert.NotEmpty(t, manual["description"])
		}

		// Verify types are sorted alphabetically
		var typeNames []string
		for _, providerData := range types {
			typeData := providerData.(map[string]interface{})
			typeNames = append(typeNames, typeData["type"].(string))
		}
		sortedNames := make([]string, len(typeNames))
		copy(sortedNames, typeNames)
		sort.Strings(sortedNames)
		assert.Equal(t, sortedNames, typeNames, "Types should be sorted alphabetically")
	})

	t.Run("includes all expected built-in providers", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/dns-providers/types", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		types := response["types"].([]interface{})

		// Build type map
		providerTypes := make(map[string]bool)
		for _, providerData := range types {
			typeData := providerData.(map[string]interface{})
			providerTypes[typeData["type"].(string)] = true
		}

		// Expected built-in types
		expectedTypes := []string{
			"cloudflare", "route53", "digitalocean", "googleclouddns",
			"namecheap", "godaddy", "azure", "hetzner", "vultr", "dnsimple",
		}

		for _, expected := range expectedTypes {
			assert.True(t, providerTypes[expected], "Missing provider type: "+expected)
		}

		// Verify manual provider is included (custom, not built-in)
		assert.True(t, providerTypes["manual"], "Missing manual provider type")
	})

	t.Run("fields include required flag", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/dns-providers/types", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		types := response["types"].([]interface{})

		// Find cloudflare to test required fields
		for _, providerData := range types {
			typeData := providerData.(map[string]interface{})
			if typeData["type"] == "cloudflare" {
				fields := typeData["fields"].([]interface{})
				// Cloudflare has at least one required field (api_token)
				foundRequired := false
				for _, f := range fields {
					field := f.(map[string]interface{})
					if field["name"] == "api_token" {
						assert.Equal(t, true, field["required"])
						foundRequired = true
					}
				}
				assert.True(t, foundRequired, "Cloudflare should have required api_token field")
			}

			// Manual provider has optional fields
			if typeData["type"] == "manual" {
				fields := typeData["fields"].([]interface{})
				for _, f := range fields {
					field := f.(map[string]interface{})
					// Manual provider's fields are optional
					assert.Equal(t, false, field["required"])
				}
			}
		}
	})

	t.Run("optional field attributes are included when present", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/dns-providers/types", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		types := response["types"].([]interface{})

		// Find a provider with hint/placeholder to verify optional fields
		for _, providerData := range types {
			typeData := providerData.(map[string]interface{})
			if typeData["type"] == "cloudflare" {
				fields := typeData["fields"].([]interface{})
				for _, f := range fields {
					field := f.(map[string]interface{})
					if field["name"] == "api_token" {
						// Cloudflare api_token should have hint and/or placeholder
						_, hasHint := field["hint"]
						_, hasPlaceholder := field["placeholder"]
						assert.True(t, hasHint || hasPlaceholder, "api_token field should have hint or placeholder")
					}
				}
			}
		}
	})
}

func TestDNSProviderHandler_CredentialsNeverExposed(t *testing.T) {
	router, mockService := setupDNSProviderTestRouter()

	provider := &models.DNSProvider{
		ID:                   1,
		UUID:                 "uuid-1",
		Name:                 "Test",
		ProviderType:         "cloudflare",
		CredentialsEncrypted: "super-secret-encrypted-data",
	}

	t.Run("Get endpoint", func(t *testing.T) {
		mockService.On("Get", mock.Anything, uint(1)).Return(provider, nil)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/dns-providers/1", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.NotContains(t, w.Body.String(), "credentials_encrypted")
		assert.NotContains(t, w.Body.String(), "super-secret-encrypted-data")
		assert.Contains(t, w.Body.String(), "has_credentials")
	})

	t.Run("List endpoint", func(t *testing.T) {
		mockService := new(MockDNSProviderService)
		handler := NewDNSProviderHandler(mockService)
		router := gin.New()
		router.GET("/dns-providers", handler.List)

		providers := []models.DNSProvider{*provider}
		mockService.On("List", mock.Anything).Return(providers, nil)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/dns-providers", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.NotContains(t, w.Body.String(), "credentials_encrypted")
		assert.NotContains(t, w.Body.String(), "super-secret-encrypted-data")
		assert.Contains(t, w.Body.String(), "has_credentials")
	})
}

func TestDNSProviderHandler_UpdateInvalidID(t *testing.T) {
	mockService := new(MockDNSProviderService)
	handler := NewDNSProviderHandler(mockService)
	router := gin.New()
	router.PUT("/dns-providers/:id", handler.Update)

	// Non-numeric IDs are treated as UUIDs
	mockService.On("GetByUUID", mock.Anything, "invalid").Return(nil, services.ErrDNSProviderNotFound)

	reqBody := map[string]string{"name": "Test"}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/dns-providers/invalid", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
	mockService.AssertExpectations(t)
}

func TestDNSProviderHandler_DeleteInvalidID(t *testing.T) {
	mockService := new(MockDNSProviderService)
	handler := NewDNSProviderHandler(mockService)
	router := gin.New()
	router.DELETE("/dns-providers/:id", handler.Delete)

	// Non-numeric IDs are treated as UUIDs
	mockService.On("GetByUUID", mock.Anything, "invalid").Return(nil, services.ErrDNSProviderNotFound)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/dns-providers/invalid", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
	mockService.AssertExpectations(t)
}

func TestDNSProviderHandler_TestInvalidID(t *testing.T) {
	mockService := new(MockDNSProviderService)
	handler := NewDNSProviderHandler(mockService)
	router := gin.New()
	router.POST("/dns-providers/:id/test", handler.Test)

	// Non-numeric IDs are treated as UUIDs
	mockService.On("GetByUUID", mock.Anything, "invalid").Return(nil, services.ErrDNSProviderNotFound)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/dns-providers/invalid/test", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
	mockService.AssertExpectations(t)
}

func TestDNSProviderHandler_CreateEncryptionFailure(t *testing.T) {
	mockService := new(MockDNSProviderService)
	handler := NewDNSProviderHandler(mockService)
	router := gin.New()
	router.POST("/dns-providers", handler.Create)

	reqBody := services.CreateDNSProviderRequest{
		Name:         "Test",
		ProviderType: "cloudflare",
		Credentials:  map[string]string{"api_token": "token"},
	}

	mockService.On("Create", mock.Anything, reqBody).Return(nil, services.ErrEncryptionFailed)

	body, _ := json.Marshal(reqBody)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/dns-providers", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	mockService.AssertExpectations(t)
}

func TestDNSProviderHandler_UpdateEncryptionFailure(t *testing.T) {
	mockService := new(MockDNSProviderService)
	handler := NewDNSProviderHandler(mockService)
	router := gin.New()
	router.PUT("/dns-providers/:id", handler.Update)

	existingProvider := &models.DNSProvider{
		ID:           1,
		UUID:         "uuid-1",
		Name:         "Test Provider",
		ProviderType: "cloudflare",
	}

	name := "Test"
	reqBody := services.UpdateDNSProviderRequest{Name: &name}

	// resolveProvider calls Get first
	mockService.On("Get", mock.Anything, uint(1)).Return(existingProvider, nil)
	mockService.On("Update", mock.Anything, uint(1), reqBody).Return(nil, services.ErrEncryptionFailed)

	body, _ := json.Marshal(reqBody)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/dns-providers/1", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	mockService.AssertExpectations(t)
}

func TestDNSProviderHandler_GetServiceError(t *testing.T) {
	mockService := new(MockDNSProviderService)
	handler := NewDNSProviderHandler(mockService)
	router := gin.New()
	router.GET("/dns-providers/:id", handler.Get)

	mockService.On("Get", mock.Anything, uint(1)).Return(nil, errors.New("database error"))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/dns-providers/1", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	mockService.AssertExpectations(t)
}

func TestDNSProviderHandler_DeleteServiceError(t *testing.T) {
	mockService := new(MockDNSProviderService)
	handler := NewDNSProviderHandler(mockService)
	router := gin.New()
	router.DELETE("/dns-providers/:id", handler.Delete)

	existingProvider := &models.DNSProvider{
		ID:           1,
		UUID:         "uuid-1",
		Name:         "Test Provider",
		ProviderType: "cloudflare",
	}

	// resolveProvider calls Get first
	mockService.On("Get", mock.Anything, uint(1)).Return(existingProvider, nil)
	mockService.On("Delete", mock.Anything, uint(1)).Return(errors.New("database error"))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/dns-providers/1", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	mockService.AssertExpectations(t)
}

func TestDNSProviderHandler_TestServiceError(t *testing.T) {
	mockService := new(MockDNSProviderService)
	handler := NewDNSProviderHandler(mockService)
	router := gin.New()
	router.POST("/dns-providers/:id/test", handler.Test)

	existingProvider := &models.DNSProvider{
		ID:           1,
		UUID:         "uuid-1",
		Name:         "Test Provider",
		ProviderType: "cloudflare",
	}

	// resolveProvider calls Get first
	mockService.On("Get", mock.Anything, uint(1)).Return(existingProvider, nil)
	mockService.On("Test", mock.Anything, uint(1)).Return(nil, errors.New("service error"))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/dns-providers/1/test", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	mockService.AssertExpectations(t)
}

func TestDNSProviderHandler_TestCredentialsServiceError(t *testing.T) {
	mockService := new(MockDNSProviderService)
	handler := NewDNSProviderHandler(mockService)
	router := gin.New()
	router.POST("/dns-providers/test", handler.TestCredentials)

	reqBody := services.CreateDNSProviderRequest{
		Name:         "Test",
		ProviderType: "cloudflare",
		Credentials:  map[string]string{"api_token": "token"},
	}

	mockService.On("TestCredentials", mock.Anything, reqBody).Return(nil, errors.New("service error"))

	body, _ := json.Marshal(reqBody)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/dns-providers/test", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	mockService.AssertExpectations(t)
}

func TestDNSProviderHandler_UpdateInvalidCredentials(t *testing.T) {
	mockService := new(MockDNSProviderService)
	handler := NewDNSProviderHandler(mockService)
	router := gin.New()
	router.PUT("/dns-providers/:id", handler.Update)

	existingProvider := &models.DNSProvider{
		ID:           1,
		UUID:         "uuid-1",
		Name:         "Test Provider",
		ProviderType: "cloudflare",
	}

	name := "Test"
	reqBody := services.UpdateDNSProviderRequest{Name: &name}

	// resolveProvider calls Get first
	mockService.On("Get", mock.Anything, uint(1)).Return(existingProvider, nil)
	mockService.On("Update", mock.Anything, uint(1), reqBody).Return(nil, services.ErrInvalidCredentials)

	body, _ := json.Marshal(reqBody)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/dns-providers/1", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "Invalid credentials")
	mockService.AssertExpectations(t)
}

func TestDNSProviderHandler_UpdateBindJSONError(t *testing.T) {
	mockService := new(MockDNSProviderService)
	handler := NewDNSProviderHandler(mockService)
	router := gin.New()
	router.PUT("/dns-providers/:id", handler.Update)

	existingProvider := &models.DNSProvider{
		ID:           1,
		UUID:         "uuid-1",
		Name:         "Test Provider",
		ProviderType: "cloudflare",
	}

	// resolveProvider calls Get first
	mockService.On("Get", mock.Anything, uint(1)).Return(existingProvider, nil)

	// Send invalid JSON
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/dns-providers/1", bytes.NewBufferString("not valid json"))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestDNSProviderHandler_UpdateGenericError(t *testing.T) {
	mockService := new(MockDNSProviderService)
	handler := NewDNSProviderHandler(mockService)
	router := gin.New()
	router.PUT("/dns-providers/:id", handler.Update)

	existingProvider := &models.DNSProvider{
		ID:           1,
		UUID:         "uuid-1",
		Name:         "Test Provider",
		ProviderType: "cloudflare",
	}

	name := "Test"
	reqBody := services.UpdateDNSProviderRequest{Name: &name}

	// resolveProvider calls Get first
	mockService.On("Get", mock.Anything, uint(1)).Return(existingProvider, nil)
	// Return a generic error that doesn't match any known error types
	mockService.On("Update", mock.Anything, uint(1), reqBody).Return(nil, errors.New("unknown database error"))

	body, _ := json.Marshal(reqBody)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/dns-providers/1", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "unknown database error")
	mockService.AssertExpectations(t)
}

func TestDNSProviderHandler_CreateGenericError(t *testing.T) {
	mockService := new(MockDNSProviderService)
	handler := NewDNSProviderHandler(mockService)
	router := gin.New()
	router.POST("/dns-providers", handler.Create)

	reqBody := services.CreateDNSProviderRequest{
		Name:         "Test",
		ProviderType: "cloudflare",
		Credentials:  map[string]string{"api_token": "token"},
	}

	// Return a generic error that doesn't match any known error types
	mockService.On("Create", mock.Anything, reqBody).Return(nil, errors.New("unknown database error"))

	body, _ := json.Marshal(reqBody)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/dns-providers", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "unknown database error")
	mockService.AssertExpectations(t)
}
