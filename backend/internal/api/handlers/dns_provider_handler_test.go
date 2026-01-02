package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/services"
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

func (m *MockDNSProviderService) GetDecryptedCredentials(ctx context.Context, id uint) (map[string]string, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[string]string), args.Error(1)
}

func setupDNSProviderTestRouter() (*gin.Engine, *MockDNSProviderService) {
	gin.SetMode(gin.TestMode)
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

		assert.Equal(t, uint(1), response.ID)
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
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/dns-providers/invalid", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
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

		assert.Equal(t, uint(1), response.ID)
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
	router, mockService := setupDNSProviderTestRouter()

	t.Run("success", func(t *testing.T) {
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

		mockService.On("Update", mock.Anything, uint(1), reqBody).Return(updatedProvider, nil)

		body, _ := json.Marshal(reqBody)
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("PUT", "/api/v1/dns-providers/1", bytes.NewBuffer(body))
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

		name := "Test"
		reqBody := services.UpdateDNSProviderRequest{Name: &name}

		mockService.On("Update", mock.Anything, uint(999), reqBody).Return(nil, services.ErrDNSProviderNotFound)

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
	router, mockService := setupDNSProviderTestRouter()

	t.Run("success", func(t *testing.T) {
		mockService.On("Delete", mock.Anything, uint(1)).Return(nil)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("DELETE", "/api/v1/dns-providers/1", nil)
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

		mockService.On("Delete", mock.Anything, uint(999)).Return(services.ErrDNSProviderNotFound)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("DELETE", "/dns-providers/999", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
		mockService.AssertExpectations(t)
	})
}

func TestDNSProviderHandler_Test(t *testing.T) {
	router, mockService := setupDNSProviderTestRouter()

	t.Run("success", func(t *testing.T) {
		testResult := &services.TestResult{
			Success:           true,
			Message:           "Credentials validated successfully",
			PropagationTimeMs: 1234,
		}

		mockService.On("Test", mock.Anything, uint(1)).Return(testResult, nil)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/api/v1/dns-providers/1/test", nil)
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

		mockService.On("Test", mock.Anything, uint(999)).Return(nil, services.ErrDNSProviderNotFound)

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

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/dns-providers/types", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	types := response["types"].([]interface{})
	assert.NotEmpty(t, types)

	// Verify structure of first type
	cloudflare := types[0].(map[string]interface{})
	assert.Equal(t, "cloudflare", cloudflare["type"])
	assert.Equal(t, "Cloudflare", cloudflare["name"])
	assert.NotEmpty(t, cloudflare["fields"])
	assert.NotEmpty(t, cloudflare["documentation_url"])

	// Verify all expected provider types are present
	providerTypes := make(map[string]bool)
	for _, t := range types {
		typeMap := t.(map[string]interface{})
		providerTypes[typeMap["type"].(string)] = true
	}

	expectedTypes := []string{
		"cloudflare", "route53", "digitalocean", "googleclouddns",
		"namecheap", "godaddy", "azure", "hetzner", "vultr", "dnsimple",
	}

	for _, expected := range expectedTypes {
		assert.True(t, providerTypes[expected], "Missing provider type: "+expected)
	}
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
	router, _ := setupDNSProviderTestRouter()

	reqBody := map[string]string{"name": "Test"}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/api/v1/dns-providers/invalid", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestDNSProviderHandler_DeleteInvalidID(t *testing.T) {
	router, _ := setupDNSProviderTestRouter()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/api/v1/dns-providers/invalid", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestDNSProviderHandler_TestInvalidID(t *testing.T) {
	router, _ := setupDNSProviderTestRouter()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/dns-providers/invalid/test", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
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

	name := "Test"
	reqBody := services.UpdateDNSProviderRequest{Name: &name}

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
