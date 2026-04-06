package handlers

import (
	"bytes"
	"context"
	"encoding/json"
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

// mockDNSDetectionService is a mock implementation of DNSDetectionService
type mockDNSDetectionService struct {
	mock.Mock
}

func (m *mockDNSDetectionService) DetectProvider(domain string) (*services.DetectionResult, error) {
	args := m.Called(domain)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*services.DetectionResult), args.Error(1)
}

func (m *mockDNSDetectionService) SuggestConfiguredProvider(ctx context.Context, domain string) (*models.DNSProvider, error) {
	args := m.Called(ctx, domain)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.DNSProvider), args.Error(1)
}

func (m *mockDNSDetectionService) GetNameserverPatterns() map[string]string {
	args := m.Called()
	return args.Get(0).(map[string]string)
}

func TestNewDNSDetectionHandler(t *testing.T) {
	mockService := new(mockDNSDetectionService)
	handler := NewDNSDetectionHandler(mockService)

	assert.NotNil(t, handler)
	assert.NotNil(t, handler.service)
}

func TestDetect_Success(t *testing.T) {

	mockService := new(mockDNSDetectionService)
	handler := NewDNSDetectionHandler(mockService)

	t.Run("successful detection without configured provider", func(t *testing.T) {
		domain := "example.com"
		expectedResult := &services.DetectionResult{
			Domain:       domain,
			Detected:     true,
			ProviderType: "cloudflare",
			Nameservers:  []string{"ns1.cloudflare.com", "ns2.cloudflare.com"},
			Confidence:   "high",
		}

		mockService.On("DetectProvider", domain).Return(expectedResult, nil).Once()
		mockService.On("SuggestConfiguredProvider", mock.Anything, domain).Return(nil, nil).Once()

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		reqBody := DetectRequest{Domain: domain}
		bodyBytes, _ := json.Marshal(reqBody)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/dns-providers/detect", bytes.NewBuffer(bodyBytes))
		c.Request.Header.Set("Content-Type", "application/json")

		handler.Detect(c)

		assert.Equal(t, http.StatusOK, w.Code)

		var response services.DetectionResult
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.Equal(t, domain, response.Domain)
		assert.True(t, response.Detected)
		assert.Equal(t, "cloudflare", response.ProviderType)
		assert.Equal(t, "high", response.Confidence)
		assert.Len(t, response.Nameservers, 2)
		assert.Nil(t, response.SuggestedProvider)

		mockService.AssertExpectations(t)
	})

	t.Run("successful detection with configured provider", func(t *testing.T) {
		domain := "example.com"
		expectedResult := &services.DetectionResult{
			Domain:       domain,
			Detected:     true,
			ProviderType: "cloudflare",
			Nameservers:  []string{"ns1.cloudflare.com"},
			Confidence:   "high",
		}

		suggestedProvider := &models.DNSProvider{
			ID:           1,
			UUID:         "test-uuid",
			Name:         "Production Cloudflare",
			ProviderType: "cloudflare",
			Enabled:      true,
			IsDefault:    true,
		}

		mockService.On("DetectProvider", domain).Return(expectedResult, nil).Once()
		mockService.On("SuggestConfiguredProvider", mock.Anything, domain).Return(suggestedProvider, nil).Once()

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		reqBody := DetectRequest{Domain: domain}
		bodyBytes, _ := json.Marshal(reqBody)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/dns-providers/detect", bytes.NewBuffer(bodyBytes))
		c.Request.Header.Set("Content-Type", "application/json")

		handler.Detect(c)

		assert.Equal(t, http.StatusOK, w.Code)

		var response services.DetectionResult
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.True(t, response.Detected)
		assert.NotNil(t, response.SuggestedProvider)
		assert.Equal(t, "Production Cloudflare", response.SuggestedProvider.Name)
		assert.Equal(t, "cloudflare", response.SuggestedProvider.ProviderType)

		mockService.AssertExpectations(t)
	})

	t.Run("detection not found", func(t *testing.T) {
		domain := "unknown-provider.com"
		expectedResult := &services.DetectionResult{
			Domain:      domain,
			Detected:    false,
			Nameservers: []string{"ns1.custom.com", "ns2.custom.com"},
			Confidence:  "none",
		}

		mockService.On("DetectProvider", domain).Return(expectedResult, nil).Once()

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		reqBody := DetectRequest{Domain: domain}
		bodyBytes, _ := json.Marshal(reqBody)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/dns-providers/detect", bytes.NewBuffer(bodyBytes))
		c.Request.Header.Set("Content-Type", "application/json")

		handler.Detect(c)

		assert.Equal(t, http.StatusOK, w.Code)

		var response services.DetectionResult
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.False(t, response.Detected)
		assert.Equal(t, "none", response.Confidence)
		assert.Len(t, response.Nameservers, 2)

		mockService.AssertExpectations(t)
	})
}

func TestDetect_ValidationErrors(t *testing.T) {

	mockService := new(mockDNSDetectionService)
	handler := NewDNSDetectionHandler(mockService)

	t.Run("missing domain", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		reqBody := map[string]string{} // Empty request
		bodyBytes, _ := json.Marshal(reqBody)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/dns-providers/detect", bytes.NewBuffer(bodyBytes))
		c.Request.Header.Set("Content-Type", "application/json")

		handler.Detect(c)

		assert.Equal(t, http.StatusBadRequest, w.Code)

		var response map[string]string
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.Contains(t, response["error"], "domain is required")
	})

	t.Run("invalid JSON", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/dns-providers/detect", bytes.NewBuffer([]byte("invalid json")))
		c.Request.Header.Set("Content-Type", "application/json")

		handler.Detect(c)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

func TestDetect_ServiceError(t *testing.T) {

	mockService := new(mockDNSDetectionService)
	handler := NewDNSDetectionHandler(mockService)

	domain := "example.com"
	mockService.On("DetectProvider", domain).Return(nil, assert.AnError).Once()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	reqBody := DetectRequest{Domain: domain}
	bodyBytes, _ := json.Marshal(reqBody)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/dns-providers/detect", bytes.NewBuffer(bodyBytes))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.Detect(c)

	assert.Equal(t, http.StatusInternalServerError, w.Code)

	var response map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Contains(t, response["error"], "Failed to detect DNS provider")

	mockService.AssertExpectations(t)
}

func TestGetPatterns(t *testing.T) {

	mockService := new(mockDNSDetectionService)
	handler := NewDNSDetectionHandler(mockService)

	patterns := map[string]string{
		".ns.cloudflare.com": "cloudflare",
		".awsdns":            "route53",
		".digitalocean.com":  "digitalocean",
	}

	mockService.On("GetNameserverPatterns").Return(patterns).Once()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/dns-providers/detection-patterns", nil)

	handler.GetPatterns(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Contains(t, response, "patterns")
	assert.Contains(t, response, "total")

	patternsList := response["patterns"].([]interface{})
	assert.Len(t, patternsList, 3)

	// Verify structure
	firstPattern := patternsList[0].(map[string]interface{})
	assert.Contains(t, firstPattern, "pattern")
	assert.Contains(t, firstPattern, "provider_type")

	mockService.AssertExpectations(t)
}

func TestDetect_WildcardDomain(t *testing.T) {

	mockService := new(mockDNSDetectionService)
	handler := NewDNSDetectionHandler(mockService)

	// The service should receive the domain without wildcard prefix
	domain := "*.example.com"
	expectedResult := &services.DetectionResult{
		Domain:       domain, // Service normalizes this
		Detected:     true,
		ProviderType: "cloudflare",
		Nameservers:  []string{"ns1.cloudflare.com"},
		Confidence:   "high",
	}

	mockService.On("DetectProvider", domain).Return(expectedResult, nil).Once()
	mockService.On("SuggestConfiguredProvider", mock.Anything, domain).Return(nil, nil).Once()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	reqBody := DetectRequest{Domain: domain}
	bodyBytes, _ := json.Marshal(reqBody)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/dns-providers/detect", bytes.NewBuffer(bodyBytes))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.Detect(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response services.DetectionResult
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.True(t, response.Detected)

	mockService.AssertExpectations(t)
}

func TestDetect_LowConfidence(t *testing.T) {

	mockService := new(mockDNSDetectionService)
	handler := NewDNSDetectionHandler(mockService)

	domain := "example.com"
	expectedResult := &services.DetectionResult{
		Domain:       domain,
		Detected:     true,
		ProviderType: "cloudflare",
		Nameservers:  []string{"ns1.cloudflare.com", "ns1.other.com", "ns2.other.com"},
		Confidence:   "low",
	}

	mockService.On("DetectProvider", domain).Return(expectedResult, nil).Once()
	mockService.On("SuggestConfiguredProvider", mock.Anything, domain).Return(nil, nil).Once()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	reqBody := DetectRequest{Domain: domain}
	bodyBytes, _ := json.Marshal(reqBody)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/dns-providers/detect", bytes.NewBuffer(bodyBytes))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.Detect(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response services.DetectionResult
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.True(t, response.Detected)
	assert.Equal(t, "low", response.Confidence)
	assert.Equal(t, "cloudflare", response.ProviderType)

	mockService.AssertExpectations(t)
}

func TestDetect_DNSLookupError(t *testing.T) {

	mockService := new(mockDNSDetectionService)
	handler := NewDNSDetectionHandler(mockService)

	domain := "nonexistent-domain-12345.com"
	expectedResult := &services.DetectionResult{
		Domain:      domain,
		Detected:    false,
		Nameservers: []string{},
		Confidence:  "none",
		Error:       "DNS lookup failed: no such host",
	}

	mockService.On("DetectProvider", domain).Return(expectedResult, nil).Once()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	reqBody := DetectRequest{Domain: domain}
	bodyBytes, _ := json.Marshal(reqBody)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/dns-providers/detect", bytes.NewBuffer(bodyBytes))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.Detect(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response services.DetectionResult
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.False(t, response.Detected)
	assert.Equal(t, "none", response.Confidence)
	assert.NotEmpty(t, response.Error)
	assert.Contains(t, response.Error, "DNS lookup failed")

	mockService.AssertExpectations(t)
}

func TestDetectRequest_Binding(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		wantErr bool
	}{
		{
			name:    "valid request",
			body:    `{"domain": "example.com"}`,
			wantErr: false,
		},
		{
			name:    "missing domain",
			body:    `{}`,
			wantErr: true,
		},
		{
			name:    "empty domain",
			body:    `{"domain": ""}`,
			wantErr: true,
		},
		{
			name:    "invalid JSON",
			body:    `{"domain": }`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(tt.body))
			c.Request.Header.Set("Content-Type", "application/json")

			var req DetectRequest
			err := c.ShouldBindJSON(&req)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotEmpty(t, req.Domain)
			}
		})
	}
}
