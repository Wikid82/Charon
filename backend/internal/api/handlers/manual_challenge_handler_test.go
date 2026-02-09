package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockManualChallengeService mocks the ManualChallengeServiceInterface for testing.
type MockManualChallengeService struct {
	mock.Mock
}

func (m *MockManualChallengeService) CreateChallenge(ctx context.Context, req services.CreateChallengeRequest) (*models.ManualChallenge, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.ManualChallenge), args.Error(1)
}

func (m *MockManualChallengeService) GetChallengeForUser(ctx context.Context, challengeID string, userID uint) (*models.ManualChallenge, error) {
	args := m.Called(ctx, challengeID, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.ManualChallenge), args.Error(1)
}

func (m *MockManualChallengeService) ListChallengesForProvider(ctx context.Context, providerID, userID uint) ([]models.ManualChallenge, error) {
	args := m.Called(ctx, providerID, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.ManualChallenge), args.Error(1)
}

func (m *MockManualChallengeService) VerifyChallenge(ctx context.Context, challengeID string, userID uint) (*services.VerifyResult, error) {
	args := m.Called(ctx, challengeID, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*services.VerifyResult), args.Error(1)
}

func (m *MockManualChallengeService) PollChallengeStatus(ctx context.Context, challengeID string, userID uint) (*services.ChallengeStatusResponse, error) {
	args := m.Called(ctx, challengeID, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*services.ChallengeStatusResponse), args.Error(1)
}

func (m *MockManualChallengeService) DeleteChallenge(ctx context.Context, challengeID string, userID uint) error {
	args := m.Called(ctx, challengeID, userID)
	return args.Error(0)
}

// mockDNSProviderServiceForChallenge is a minimal mock for manual challenge tests.
type mockDNSProviderServiceForChallenge struct {
	mock.Mock
}

func (m *mockDNSProviderServiceForChallenge) Get(ctx context.Context, id uint) (*models.DNSProvider, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.DNSProvider), args.Error(1)
}

func setupChallengeTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	return gin.New()
}

func setUserID(c *gin.Context, userID uint) {
	c.Set("user_id", userID)
}

func TestNewManualChallengeHandler(t *testing.T) {
	mockService := new(MockManualChallengeService)
	mockProviderService := new(mockDNSProviderServiceForChallenge)

	handler := NewManualChallengeHandler(mockService, mockProviderService)

	require.NotNil(t, handler)
	assert.Equal(t, mockService, handler.challengeService)
	assert.Equal(t, mockProviderService, handler.providerService)
}

func TestManualChallengeHandler_GetChallenge(t *testing.T) {
	mockService := new(MockManualChallengeService)
	mockProviderService := new(mockDNSProviderServiceForChallenge)
	handler := NewManualChallengeHandler(mockService, mockProviderService)

	router := setupChallengeTestRouter()
	router.GET("/dns-providers/:id/manual-challenge/:challengeId", func(c *gin.Context) {
		setUserID(c, 1)
		handler.GetChallenge(c)
	})

	now := time.Now()
	challenge := &models.ManualChallenge{
		ID:         "test-challenge-id",
		ProviderID: 1,
		UserID:     1,
		FQDN:       "_acme-challenge.example.com",
		Value:      "txtvalue",
		Status:     models.ChallengeStatusPending,
		CreatedAt:  now,
		ExpiresAt:  now.Add(10 * time.Minute),
	}

	provider := &models.DNSProvider{
		ID:           1,
		ProviderType: "manual",
	}

	mockProviderService.On("Get", mock.Anything, uint(1)).Return(provider, nil)
	mockService.On("GetChallengeForUser", mock.Anything, "test-challenge-id", uint(1)).Return(challenge, nil)

	req, _ := http.NewRequest("GET", "/dns-providers/1/manual-challenge/test-challenge-id", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp ManualChallengeResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "test-challenge-id", resp.ID)
	assert.Equal(t, uint(1), resp.ProviderID)
}

func TestManualChallengeHandler_GetChallenge_NotFound(t *testing.T) {
	mockService := new(MockManualChallengeService)
	mockProviderService := new(mockDNSProviderServiceForChallenge)
	handler := NewManualChallengeHandler(mockService, mockProviderService)

	router := setupChallengeTestRouter()
	router.GET("/dns-providers/:id/manual-challenge/:challengeId", func(c *gin.Context) {
		setUserID(c, 1)
		handler.GetChallenge(c)
	})

	provider := &models.DNSProvider{
		ID:           1,
		ProviderType: "manual",
	}

	mockProviderService.On("Get", mock.Anything, uint(1)).Return(provider, nil)
	mockService.On("GetChallengeForUser", mock.Anything, "nonexistent", uint(1)).Return(nil, services.ErrChallengeNotFound)

	req, _ := http.NewRequest("GET", "/dns-providers/1/manual-challenge/nonexistent", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestManualChallengeHandler_GetChallenge_InvalidProviderType(t *testing.T) {
	mockService := new(MockManualChallengeService)
	mockProviderService := new(mockDNSProviderServiceForChallenge)
	handler := NewManualChallengeHandler(mockService, mockProviderService)

	router := setupChallengeTestRouter()
	router.GET("/dns-providers/:id/manual-challenge/:challengeId", func(c *gin.Context) {
		setUserID(c, 1)
		handler.GetChallenge(c)
	})

	provider := &models.DNSProvider{
		ID:           1,
		ProviderType: "cloudflare", // Not manual
	}

	mockProviderService.On("Get", mock.Anything, uint(1)).Return(provider, nil)

	req, _ := http.NewRequest("GET", "/dns-providers/1/manual-challenge/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestManualChallengeHandler_CreateChallenge(t *testing.T) {
	mockService := new(MockManualChallengeService)
	mockProviderService := new(mockDNSProviderServiceForChallenge)
	handler := NewManualChallengeHandler(mockService, mockProviderService)

	router := setupChallengeTestRouter()
	router.POST("/dns-providers/:id/manual-challenges", func(c *gin.Context) {
		setUserID(c, 1)
		handler.CreateChallenge(c)
	})

	now := time.Now()
	challenge := &models.ManualChallenge{
		ID:         "new-challenge-id",
		ProviderID: 1,
		UserID:     1,
		FQDN:       "_acme-challenge.example.com",
		Value:      "txtvalue",
		Status:     models.ChallengeStatusPending,
		CreatedAt:  now,
		ExpiresAt:  now.Add(10 * time.Minute),
	}

	provider := &models.DNSProvider{
		ID:           1,
		ProviderType: "manual",
	}

	mockProviderService.On("Get", mock.Anything, uint(1)).Return(provider, nil)
	mockService.On("CreateChallenge", mock.Anything, mock.AnythingOfType("services.CreateChallengeRequest")).Return(challenge, nil)

	body := CreateChallengeRequest{
		FQDN:  "_acme-challenge.example.com",
		Value: "txtvalue",
	}
	bodyBytes, _ := json.Marshal(body)

	req, _ := http.NewRequest("POST", "/dns-providers/1/manual-challenges", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestManualChallengeHandler_CreateChallenge_InvalidRequest(t *testing.T) {
	mockService := new(MockManualChallengeService)
	mockProviderService := new(mockDNSProviderServiceForChallenge)
	handler := NewManualChallengeHandler(mockService, mockProviderService)

	router := setupChallengeTestRouter()
	router.POST("/dns-providers/:id/manual-challenges", func(c *gin.Context) {
		setUserID(c, 1)
		handler.CreateChallenge(c)
	})

	provider := &models.DNSProvider{
		ID:           1,
		ProviderType: "manual",
	}

	mockProviderService.On("Get", mock.Anything, uint(1)).Return(provider, nil)

	// Missing required field
	body := `{"fqdn": ""}`

	req, _ := http.NewRequest("POST", "/dns-providers/1/manual-challenges", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestManualChallengeHandler_VerifyChallenge(t *testing.T) {
	mockService := new(MockManualChallengeService)
	mockProviderService := new(mockDNSProviderServiceForChallenge)
	handler := NewManualChallengeHandler(mockService, mockProviderService)

	router := setupChallengeTestRouter()
	router.POST("/dns-providers/:id/manual-challenge/:challengeId/verify", func(c *gin.Context) {
		setUserID(c, 1)
		handler.VerifyChallenge(c)
	})

	now := time.Now()
	challenge := &models.ManualChallenge{
		ID:         "test-challenge",
		ProviderID: 1,
		UserID:     1,
		FQDN:       "_acme-challenge.example.com",
		Value:      "txtvalue",
		Status:     models.ChallengeStatusPending,
		CreatedAt:  now,
		ExpiresAt:  now.Add(10 * time.Minute),
	}

	provider := &models.DNSProvider{
		ID:           1,
		ProviderType: "manual",
	}

	result := &services.VerifyResult{
		Success:  true,
		DNSFound: true,
		Message:  "DNS TXT record verified successfully",
		Status:   "verified",
	}

	mockProviderService.On("Get", mock.Anything, uint(1)).Return(provider, nil)
	mockService.On("GetChallengeForUser", mock.Anything, "test-challenge", uint(1)).Return(challenge, nil)
	mockService.On("VerifyChallenge", mock.Anything, "test-challenge", uint(1)).Return(result, nil)

	req, _ := http.NewRequest("POST", "/dns-providers/1/manual-challenge/test-challenge/verify", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestManualChallengeHandler_PollChallenge(t *testing.T) {
	mockService := new(MockManualChallengeService)
	mockProviderService := new(mockDNSProviderServiceForChallenge)
	handler := NewManualChallengeHandler(mockService, mockProviderService)

	router := setupChallengeTestRouter()
	router.GET("/dns-providers/:id/manual-challenge/:challengeId/poll", func(c *gin.Context) {
		setUserID(c, 1)
		handler.PollChallenge(c)
	})

	provider := &models.DNSProvider{
		ID:           1,
		ProviderType: "manual",
	}

	now := time.Now()
	status := &services.ChallengeStatusResponse{
		ID:                   "test-challenge",
		Status:               "pending",
		DNSPropagated:        false,
		TimeRemainingSeconds: 300,
		LastCheckAt:          &now,
	}

	mockProviderService.On("Get", mock.Anything, uint(1)).Return(provider, nil)
	mockService.On("PollChallengeStatus", mock.Anything, "test-challenge", uint(1)).Return(status, nil)

	req, _ := http.NewRequest("GET", "/dns-providers/1/manual-challenge/test-challenge/poll", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestManualChallengeHandler_ListChallenges(t *testing.T) {
	mockService := new(MockManualChallengeService)
	mockProviderService := new(mockDNSProviderServiceForChallenge)
	handler := NewManualChallengeHandler(mockService, mockProviderService)

	router := setupChallengeTestRouter()
	router.GET("/dns-providers/:id/manual-challenges", func(c *gin.Context) {
		setUserID(c, 1)
		handler.ListChallenges(c)
	})

	provider := &models.DNSProvider{
		ID:           1,
		ProviderType: "manual",
	}

	now := time.Now()
	challenges := []models.ManualChallenge{
		{
			ID:         "challenge-1",
			ProviderID: 1,
			UserID:     1,
			FQDN:       "_acme-challenge.example1.com",
			Value:      "value1",
			Status:     models.ChallengeStatusPending,
			CreatedAt:  now,
			ExpiresAt:  now.Add(10 * time.Minute),
		},
		{
			ID:         "challenge-2",
			ProviderID: 1,
			UserID:     1,
			FQDN:       "_acme-challenge.example2.com",
			Value:      "value2",
			Status:     models.ChallengeStatusVerified,
			CreatedAt:  now,
			ExpiresAt:  now.Add(10 * time.Minute),
		},
	}

	mockProviderService.On("Get", mock.Anything, uint(1)).Return(provider, nil)
	mockService.On("ListChallengesForProvider", mock.Anything, uint(1), uint(1)).Return(challenges, nil)

	req, _ := http.NewRequest("GET", "/dns-providers/1/manual-challenges", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, float64(2), resp["total"])
}

func TestManualChallengeHandler_DeleteChallenge(t *testing.T) {
	mockService := new(MockManualChallengeService)
	mockProviderService := new(mockDNSProviderServiceForChallenge)
	handler := NewManualChallengeHandler(mockService, mockProviderService)

	router := setupChallengeTestRouter()
	router.DELETE("/dns-providers/:id/manual-challenge/:challengeId", func(c *gin.Context) {
		setUserID(c, 1)
		handler.DeleteChallenge(c)
	})

	provider := &models.DNSProvider{
		ID:           1,
		ProviderType: "manual",
	}

	mockProviderService.On("Get", mock.Anything, uint(1)).Return(provider, nil)
	mockService.On("DeleteChallenge", mock.Anything, "test-challenge", uint(1)).Return(nil)

	req, _ := http.NewRequest("DELETE", "/dns-providers/1/manual-challenge/test-challenge", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestManualChallengeHandler_InvalidProviderID(t *testing.T) {
	mockService := new(MockManualChallengeService)
	mockProviderService := new(mockDNSProviderServiceForChallenge)
	handler := NewManualChallengeHandler(mockService, mockProviderService)

	router := setupChallengeTestRouter()
	router.GET("/dns-providers/:id/manual-challenge/:challengeId", func(c *gin.Context) {
		setUserID(c, 1)
		handler.GetChallenge(c)
	})

	req, _ := http.NewRequest("GET", "/dns-providers/invalid/manual-challenge/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestManualChallengeHandler_ProviderNotFound(t *testing.T) {
	mockService := new(MockManualChallengeService)
	mockProviderService := new(mockDNSProviderServiceForChallenge)
	handler := NewManualChallengeHandler(mockService, mockProviderService)

	router := setupChallengeTestRouter()
	router.GET("/dns-providers/:id/manual-challenge/:challengeId", func(c *gin.Context) {
		setUserID(c, 1)
		handler.GetChallenge(c)
	})

	mockProviderService.On("Get", mock.Anything, uint(999)).Return(nil, services.ErrDNSProviderNotFound)

	req, _ := http.NewRequest("GET", "/dns-providers/999/manual-challenge/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestManualChallengeHandler_RegisterRoutes(t *testing.T) {
	mockService := new(MockManualChallengeService)
	mockProviderService := new(mockDNSProviderServiceForChallenge)
	handler := NewManualChallengeHandler(mockService, mockProviderService)

	router := setupChallengeTestRouter()
	handler.RegisterRoutes(router.Group("/"))

	// Verify routes are registered by checking that they respond (even with errors)
	routes := router.Routes()
	paths := make(map[string]bool)
	for _, route := range routes {
		paths[route.Path] = true
	}

	assert.True(t, paths["/dns-providers/:id/manual-challenges"])
	assert.True(t, paths["/dns-providers/:id/manual-challenge/:challengeId"])
	assert.True(t, paths["/dns-providers/:id/manual-challenge/:challengeId/verify"])
	assert.True(t, paths["/dns-providers/:id/manual-challenge/:challengeId/poll"])
}

func TestGetUserIDFromContext(t *testing.T) {
	tests := []struct {
		name     string
		value    interface{}
		expected uint
	}{
		{"uint value", uint(42), 42},
		{"int value", int(42), 42},
		{"int64 value", int64(42), 42},
		{"uint64 value", uint64(42), 42},
		{"missing value", nil, 0},
		{"invalid type", "42", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			if tt.value != nil {
				c.Set("user_id", tt.value)
			}

			result := getUserIDFromContext(c)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestChallengeToResponse(t *testing.T) {
	now := time.Now()
	lastCheck := now.Add(-1 * time.Minute)

	challenge := &models.ManualChallenge{
		ID:            "test-id",
		ProviderID:    1,
		UserID:        1,
		FQDN:          "_acme-challenge.example.com",
		Value:         "txtvalue",
		Status:        models.ChallengeStatusPending,
		DNSPropagated: false,
		CreatedAt:     now,
		ExpiresAt:     now.Add(10 * time.Minute),
		LastCheckAt:   &lastCheck,
		ErrorMessage:  "test error",
	}

	resp := challengeToResponse(challenge)

	assert.Equal(t, "test-id", resp.ID)
	assert.Equal(t, uint(1), resp.ProviderID)
	assert.Equal(t, "_acme-challenge.example.com", resp.FQDN)
	assert.Equal(t, "txtvalue", resp.Value)
	assert.Equal(t, "pending", resp.Status)
	assert.False(t, resp.DNSPropagated)
	assert.NotEmpty(t, resp.CreatedAt)
	assert.NotEmpty(t, resp.ExpiresAt)
	assert.NotEmpty(t, resp.LastCheckAt)
	assert.Equal(t, "test error", resp.ErrorMessage)
}

func TestNewErrorResponse(t *testing.T) {
	details := map[string]interface{}{"key": "value"}
	resp := newErrorResponse("TEST_CODE", "Test message", details)

	assert.False(t, resp.Success)
	assert.Equal(t, "TEST_CODE", resp.Error.Code)
	assert.Equal(t, "Test message", resp.Error.Message)
	assert.Equal(t, details, resp.Error.Details)
}

// Additional tests for comprehensive coverage

func TestManualChallengeHandler_GetChallenge_EmptyChallengeID(t *testing.T) {
	mockService := new(MockManualChallengeService)
	mockProviderService := new(mockDNSProviderServiceForChallenge)
	handler := NewManualChallengeHandler(mockService, mockProviderService)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: "1"}, {Key: "challengeId", Value: ""}}
	c.Request = httptest.NewRequest("GET", "/dns-providers/1/manual-challenge/", nil)
	setUserID(c, 1)

	provider := &models.DNSProvider{ID: 1, ProviderType: "manual"}
	mockProviderService.On("Get", mock.Anything, uint(1)).Return(provider, nil)

	handler.GetChallenge(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "INVALID_CHALLENGE_ID")
}

func TestManualChallengeHandler_GetChallenge_ProviderInternalError(t *testing.T) {
	mockService := new(MockManualChallengeService)
	mockProviderService := new(mockDNSProviderServiceForChallenge)
	handler := NewManualChallengeHandler(mockService, mockProviderService)

	router := setupChallengeTestRouter()
	router.GET("/dns-providers/:id/manual-challenge/:challengeId", func(c *gin.Context) {
		setUserID(c, 1)
		handler.GetChallenge(c)
	})

	// Return an internal error (not ErrDNSProviderNotFound)
	mockProviderService.On("Get", mock.Anything, uint(1)).Return(nil, errors.New("database error"))

	req, _ := http.NewRequest("GET", "/dns-providers/1/manual-challenge/test-id", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "INTERNAL_ERROR")
}

func TestManualChallengeHandler_GetChallenge_Unauthorized(t *testing.T) {
	mockService := new(MockManualChallengeService)
	mockProviderService := new(mockDNSProviderServiceForChallenge)
	handler := NewManualChallengeHandler(mockService, mockProviderService)

	router := setupChallengeTestRouter()
	router.GET("/dns-providers/:id/manual-challenge/:challengeId", func(c *gin.Context) {
		setUserID(c, 1)
		handler.GetChallenge(c)
	})

	provider := &models.DNSProvider{ID: 1, ProviderType: "manual"}
	mockProviderService.On("Get", mock.Anything, uint(1)).Return(provider, nil)
	mockService.On("GetChallengeForUser", mock.Anything, "test-id", uint(1)).Return(nil, services.ErrUnauthorized)

	req, _ := http.NewRequest("GET", "/dns-providers/1/manual-challenge/test-id", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Contains(t, w.Body.String(), "UNAUTHORIZED")
}

func TestManualChallengeHandler_GetChallenge_InternalError(t *testing.T) {
	mockService := new(MockManualChallengeService)
	mockProviderService := new(mockDNSProviderServiceForChallenge)
	handler := NewManualChallengeHandler(mockService, mockProviderService)

	router := setupChallengeTestRouter()
	router.GET("/dns-providers/:id/manual-challenge/:challengeId", func(c *gin.Context) {
		setUserID(c, 1)
		handler.GetChallenge(c)
	})

	provider := &models.DNSProvider{ID: 1, ProviderType: "manual"}
	mockProviderService.On("Get", mock.Anything, uint(1)).Return(provider, nil)
	mockService.On("GetChallengeForUser", mock.Anything, "test-id", uint(1)).Return(nil, errors.New("db error"))

	req, _ := http.NewRequest("GET", "/dns-providers/1/manual-challenge/test-id", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "INTERNAL_ERROR")
}

func TestManualChallengeHandler_GetChallenge_ProviderMismatch(t *testing.T) {
	mockService := new(MockManualChallengeService)
	mockProviderService := new(mockDNSProviderServiceForChallenge)
	handler := NewManualChallengeHandler(mockService, mockProviderService)

	router := setupChallengeTestRouter()
	router.GET("/dns-providers/:id/manual-challenge/:challengeId", func(c *gin.Context) {
		setUserID(c, 1)
		handler.GetChallenge(c)
	})

	provider := &models.DNSProvider{ID: 1, ProviderType: "manual"}
	// Challenge belongs to a different provider
	challenge := &models.ManualChallenge{
		ID:         "test-id",
		ProviderID: 999, // Different provider
		UserID:     1,
		CreatedAt:  time.Now(),
		ExpiresAt:  time.Now().Add(10 * time.Minute),
	}

	mockProviderService.On("Get", mock.Anything, uint(1)).Return(provider, nil)
	mockService.On("GetChallengeForUser", mock.Anything, "test-id", uint(1)).Return(challenge, nil)

	req, _ := http.NewRequest("GET", "/dns-providers/1/manual-challenge/test-id", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Contains(t, w.Body.String(), "CHALLENGE_NOT_FOUND")
}

func TestManualChallengeHandler_VerifyChallenge_InvalidProviderID(t *testing.T) {
	mockService := new(MockManualChallengeService)
	mockProviderService := new(mockDNSProviderServiceForChallenge)
	handler := NewManualChallengeHandler(mockService, mockProviderService)

	router := setupChallengeTestRouter()
	router.POST("/dns-providers/:id/manual-challenge/:challengeId/verify", func(c *gin.Context) {
		setUserID(c, 1)
		handler.VerifyChallenge(c)
	})

	req, _ := http.NewRequest("POST", "/dns-providers/invalid/manual-challenge/test/verify", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "INVALID_PROVIDER_ID")
}

func TestManualChallengeHandler_VerifyChallenge_EmptyChallengeID(t *testing.T) {
	mockService := new(MockManualChallengeService)
	mockProviderService := new(mockDNSProviderServiceForChallenge)
	handler := NewManualChallengeHandler(mockService, mockProviderService)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: "1"}, {Key: "challengeId", Value: ""}}
	c.Request = httptest.NewRequest("POST", "/dns-providers/1/manual-challenge//verify", nil)
	setUserID(c, 1)

	handler.VerifyChallenge(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "INVALID_CHALLENGE_ID")
}

func TestManualChallengeHandler_VerifyChallenge_ProviderNotFound(t *testing.T) {
	mockService := new(MockManualChallengeService)
	mockProviderService := new(mockDNSProviderServiceForChallenge)
	handler := NewManualChallengeHandler(mockService, mockProviderService)

	router := setupChallengeTestRouter()
	router.POST("/dns-providers/:id/manual-challenge/:challengeId/verify", func(c *gin.Context) {
		setUserID(c, 1)
		handler.VerifyChallenge(c)
	})

	mockProviderService.On("Get", mock.Anything, uint(1)).Return(nil, services.ErrDNSProviderNotFound)

	req, _ := http.NewRequest("POST", "/dns-providers/1/manual-challenge/test/verify", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Contains(t, w.Body.String(), "PROVIDER_NOT_FOUND")
}

func TestManualChallengeHandler_VerifyChallenge_ProviderInternalError(t *testing.T) {
	mockService := new(MockManualChallengeService)
	mockProviderService := new(mockDNSProviderServiceForChallenge)
	handler := NewManualChallengeHandler(mockService, mockProviderService)

	router := setupChallengeTestRouter()
	router.POST("/dns-providers/:id/manual-challenge/:challengeId/verify", func(c *gin.Context) {
		setUserID(c, 1)
		handler.VerifyChallenge(c)
	})

	mockProviderService.On("Get", mock.Anything, uint(1)).Return(nil, errors.New("db error"))

	req, _ := http.NewRequest("POST", "/dns-providers/1/manual-challenge/test/verify", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "INTERNAL_ERROR")
}

func TestManualChallengeHandler_VerifyChallenge_InvalidProviderType(t *testing.T) {
	mockService := new(MockManualChallengeService)
	mockProviderService := new(mockDNSProviderServiceForChallenge)
	handler := NewManualChallengeHandler(mockService, mockProviderService)

	router := setupChallengeTestRouter()
	router.POST("/dns-providers/:id/manual-challenge/:challengeId/verify", func(c *gin.Context) {
		setUserID(c, 1)
		handler.VerifyChallenge(c)
	})

	provider := &models.DNSProvider{ID: 1, ProviderType: "cloudflare"}
	mockProviderService.On("Get", mock.Anything, uint(1)).Return(provider, nil)

	req, _ := http.NewRequest("POST", "/dns-providers/1/manual-challenge/test/verify", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "INVALID_PROVIDER_TYPE")
}

func TestManualChallengeHandler_VerifyChallenge_ChallengeNotFound(t *testing.T) {
	mockService := new(MockManualChallengeService)
	mockProviderService := new(mockDNSProviderServiceForChallenge)
	handler := NewManualChallengeHandler(mockService, mockProviderService)

	router := setupChallengeTestRouter()
	router.POST("/dns-providers/:id/manual-challenge/:challengeId/verify", func(c *gin.Context) {
		setUserID(c, 1)
		handler.VerifyChallenge(c)
	})

	provider := &models.DNSProvider{ID: 1, ProviderType: "manual"}
	mockProviderService.On("Get", mock.Anything, uint(1)).Return(provider, nil)
	mockService.On("GetChallengeForUser", mock.Anything, "test", uint(1)).Return(nil, services.ErrChallengeNotFound)

	req, _ := http.NewRequest("POST", "/dns-providers/1/manual-challenge/test/verify", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Contains(t, w.Body.String(), "CHALLENGE_NOT_FOUND")
}

func TestManualChallengeHandler_VerifyChallenge_Unauthorized(t *testing.T) {
	mockService := new(MockManualChallengeService)
	mockProviderService := new(mockDNSProviderServiceForChallenge)
	handler := NewManualChallengeHandler(mockService, mockProviderService)

	router := setupChallengeTestRouter()
	router.POST("/dns-providers/:id/manual-challenge/:challengeId/verify", func(c *gin.Context) {
		setUserID(c, 1)
		handler.VerifyChallenge(c)
	})

	provider := &models.DNSProvider{ID: 1, ProviderType: "manual"}
	mockProviderService.On("Get", mock.Anything, uint(1)).Return(provider, nil)
	mockService.On("GetChallengeForUser", mock.Anything, "test", uint(1)).Return(nil, services.ErrUnauthorized)

	req, _ := http.NewRequest("POST", "/dns-providers/1/manual-challenge/test/verify", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Contains(t, w.Body.String(), "UNAUTHORIZED")
}

func TestManualChallengeHandler_VerifyChallenge_GetChallengeInternalError(t *testing.T) {
	mockService := new(MockManualChallengeService)
	mockProviderService := new(mockDNSProviderServiceForChallenge)
	handler := NewManualChallengeHandler(mockService, mockProviderService)

	router := setupChallengeTestRouter()
	router.POST("/dns-providers/:id/manual-challenge/:challengeId/verify", func(c *gin.Context) {
		setUserID(c, 1)
		handler.VerifyChallenge(c)
	})

	provider := &models.DNSProvider{ID: 1, ProviderType: "manual"}
	mockProviderService.On("Get", mock.Anything, uint(1)).Return(provider, nil)
	mockService.On("GetChallengeForUser", mock.Anything, "test", uint(1)).Return(nil, errors.New("db error"))

	req, _ := http.NewRequest("POST", "/dns-providers/1/manual-challenge/test/verify", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "INTERNAL_ERROR")
}

func TestManualChallengeHandler_VerifyChallenge_ProviderMismatch(t *testing.T) {
	mockService := new(MockManualChallengeService)
	mockProviderService := new(mockDNSProviderServiceForChallenge)
	handler := NewManualChallengeHandler(mockService, mockProviderService)

	router := setupChallengeTestRouter()
	router.POST("/dns-providers/:id/manual-challenge/:challengeId/verify", func(c *gin.Context) {
		setUserID(c, 1)
		handler.VerifyChallenge(c)
	})

	provider := &models.DNSProvider{ID: 1, ProviderType: "manual"}
	challenge := &models.ManualChallenge{
		ID:         "test",
		ProviderID: 999, // Different provider
		UserID:     1,
		CreatedAt:  time.Now(),
		ExpiresAt:  time.Now().Add(10 * time.Minute),
	}

	mockProviderService.On("Get", mock.Anything, uint(1)).Return(provider, nil)
	mockService.On("GetChallengeForUser", mock.Anything, "test", uint(1)).Return(challenge, nil)

	req, _ := http.NewRequest("POST", "/dns-providers/1/manual-challenge/test/verify", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Contains(t, w.Body.String(), "CHALLENGE_NOT_FOUND")
}

func TestManualChallengeHandler_VerifyChallenge_ChallengeExpired(t *testing.T) {
	mockService := new(MockManualChallengeService)
	mockProviderService := new(mockDNSProviderServiceForChallenge)
	handler := NewManualChallengeHandler(mockService, mockProviderService)

	router := setupChallengeTestRouter()
	router.POST("/dns-providers/:id/manual-challenge/:challengeId/verify", func(c *gin.Context) {
		setUserID(c, 1)
		handler.VerifyChallenge(c)
	})

	provider := &models.DNSProvider{ID: 1, ProviderType: "manual"}
	challenge := &models.ManualChallenge{
		ID:         "test",
		ProviderID: 1,
		UserID:     1,
		CreatedAt:  time.Now().Add(-20 * time.Minute),
		ExpiresAt:  time.Now().Add(-10 * time.Minute), // Expired
	}

	mockProviderService.On("Get", mock.Anything, uint(1)).Return(provider, nil)
	mockService.On("GetChallengeForUser", mock.Anything, "test", uint(1)).Return(challenge, nil)
	mockService.On("VerifyChallenge", mock.Anything, "test", uint(1)).Return(nil, services.ErrChallengeExpired)

	req, _ := http.NewRequest("POST", "/dns-providers/1/manual-challenge/test/verify", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusGone, w.Code)
	assert.Contains(t, w.Body.String(), "CHALLENGE_EXPIRED")
}

func TestManualChallengeHandler_VerifyChallenge_VerifyInternalError(t *testing.T) {
	mockService := new(MockManualChallengeService)
	mockProviderService := new(mockDNSProviderServiceForChallenge)
	handler := NewManualChallengeHandler(mockService, mockProviderService)

	router := setupChallengeTestRouter()
	router.POST("/dns-providers/:id/manual-challenge/:challengeId/verify", func(c *gin.Context) {
		setUserID(c, 1)
		handler.VerifyChallenge(c)
	})

	provider := &models.DNSProvider{ID: 1, ProviderType: "manual"}
	challenge := &models.ManualChallenge{
		ID:         "test",
		ProviderID: 1,
		UserID:     1,
		CreatedAt:  time.Now(),
		ExpiresAt:  time.Now().Add(10 * time.Minute),
	}

	mockProviderService.On("Get", mock.Anything, uint(1)).Return(provider, nil)
	mockService.On("GetChallengeForUser", mock.Anything, "test", uint(1)).Return(challenge, nil)
	mockService.On("VerifyChallenge", mock.Anything, "test", uint(1)).Return(nil, errors.New("dns lookup failed"))

	req, _ := http.NewRequest("POST", "/dns-providers/1/manual-challenge/test/verify", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "INTERNAL_ERROR")
}

func TestManualChallengeHandler_PollChallenge_InvalidProviderID(t *testing.T) {
	mockService := new(MockManualChallengeService)
	mockProviderService := new(mockDNSProviderServiceForChallenge)
	handler := NewManualChallengeHandler(mockService, mockProviderService)

	router := setupChallengeTestRouter()
	router.GET("/dns-providers/:id/manual-challenge/:challengeId/poll", func(c *gin.Context) {
		setUserID(c, 1)
		handler.PollChallenge(c)
	})

	req, _ := http.NewRequest("GET", "/dns-providers/invalid/manual-challenge/test/poll", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "INVALID_PROVIDER_ID")
}

func TestManualChallengeHandler_PollChallenge_EmptyChallengeID(t *testing.T) {
	mockService := new(MockManualChallengeService)
	mockProviderService := new(mockDNSProviderServiceForChallenge)
	handler := NewManualChallengeHandler(mockService, mockProviderService)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: "1"}, {Key: "challengeId", Value: ""}}
	c.Request = httptest.NewRequest("GET", "/dns-providers/1/manual-challenge//poll", nil)
	setUserID(c, 1)

	handler.PollChallenge(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "INVALID_CHALLENGE_ID")
}

func TestManualChallengeHandler_PollChallenge_ProviderNotFound(t *testing.T) {
	mockService := new(MockManualChallengeService)
	mockProviderService := new(mockDNSProviderServiceForChallenge)
	handler := NewManualChallengeHandler(mockService, mockProviderService)

	router := setupChallengeTestRouter()
	router.GET("/dns-providers/:id/manual-challenge/:challengeId/poll", func(c *gin.Context) {
		setUserID(c, 1)
		handler.PollChallenge(c)
	})

	mockProviderService.On("Get", mock.Anything, uint(1)).Return(nil, services.ErrDNSProviderNotFound)

	req, _ := http.NewRequest("GET", "/dns-providers/1/manual-challenge/test/poll", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Contains(t, w.Body.String(), "PROVIDER_NOT_FOUND")
}

func TestManualChallengeHandler_PollChallenge_ProviderInternalError(t *testing.T) {
	mockService := new(MockManualChallengeService)
	mockProviderService := new(mockDNSProviderServiceForChallenge)
	handler := NewManualChallengeHandler(mockService, mockProviderService)

	router := setupChallengeTestRouter()
	router.GET("/dns-providers/:id/manual-challenge/:challengeId/poll", func(c *gin.Context) {
		setUserID(c, 1)
		handler.PollChallenge(c)
	})

	mockProviderService.On("Get", mock.Anything, uint(1)).Return(nil, errors.New("db error"))

	req, _ := http.NewRequest("GET", "/dns-providers/1/manual-challenge/test/poll", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "INTERNAL_ERROR")
}

func TestManualChallengeHandler_PollChallenge_InvalidProviderType(t *testing.T) {
	mockService := new(MockManualChallengeService)
	mockProviderService := new(mockDNSProviderServiceForChallenge)
	handler := NewManualChallengeHandler(mockService, mockProviderService)

	router := setupChallengeTestRouter()
	router.GET("/dns-providers/:id/manual-challenge/:challengeId/poll", func(c *gin.Context) {
		setUserID(c, 1)
		handler.PollChallenge(c)
	})

	provider := &models.DNSProvider{ID: 1, ProviderType: "cloudflare"}
	mockProviderService.On("Get", mock.Anything, uint(1)).Return(provider, nil)

	req, _ := http.NewRequest("GET", "/dns-providers/1/manual-challenge/test/poll", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "INVALID_PROVIDER_TYPE")
}

func TestManualChallengeHandler_PollChallenge_ChallengeNotFound(t *testing.T) {
	mockService := new(MockManualChallengeService)
	mockProviderService := new(mockDNSProviderServiceForChallenge)
	handler := NewManualChallengeHandler(mockService, mockProviderService)

	router := setupChallengeTestRouter()
	router.GET("/dns-providers/:id/manual-challenge/:challengeId/poll", func(c *gin.Context) {
		setUserID(c, 1)
		handler.PollChallenge(c)
	})

	provider := &models.DNSProvider{ID: 1, ProviderType: "manual"}
	mockProviderService.On("Get", mock.Anything, uint(1)).Return(provider, nil)
	mockService.On("PollChallengeStatus", mock.Anything, "test", uint(1)).Return(nil, services.ErrChallengeNotFound)

	req, _ := http.NewRequest("GET", "/dns-providers/1/manual-challenge/test/poll", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Contains(t, w.Body.String(), "CHALLENGE_NOT_FOUND")
}

func TestManualChallengeHandler_PollChallenge_Unauthorized(t *testing.T) {
	mockService := new(MockManualChallengeService)
	mockProviderService := new(mockDNSProviderServiceForChallenge)
	handler := NewManualChallengeHandler(mockService, mockProviderService)

	router := setupChallengeTestRouter()
	router.GET("/dns-providers/:id/manual-challenge/:challengeId/poll", func(c *gin.Context) {
		setUserID(c, 1)
		handler.PollChallenge(c)
	})

	provider := &models.DNSProvider{ID: 1, ProviderType: "manual"}
	mockProviderService.On("Get", mock.Anything, uint(1)).Return(provider, nil)
	mockService.On("PollChallengeStatus", mock.Anything, "test", uint(1)).Return(nil, services.ErrUnauthorized)

	req, _ := http.NewRequest("GET", "/dns-providers/1/manual-challenge/test/poll", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Contains(t, w.Body.String(), "UNAUTHORIZED")
}

func TestManualChallengeHandler_PollChallenge_InternalError(t *testing.T) {
	mockService := new(MockManualChallengeService)
	mockProviderService := new(mockDNSProviderServiceForChallenge)
	handler := NewManualChallengeHandler(mockService, mockProviderService)

	router := setupChallengeTestRouter()
	router.GET("/dns-providers/:id/manual-challenge/:challengeId/poll", func(c *gin.Context) {
		setUserID(c, 1)
		handler.PollChallenge(c)
	})

	provider := &models.DNSProvider{ID: 1, ProviderType: "manual"}
	mockProviderService.On("Get", mock.Anything, uint(1)).Return(provider, nil)
	mockService.On("PollChallengeStatus", mock.Anything, "test", uint(1)).Return(nil, errors.New("db error"))

	req, _ := http.NewRequest("GET", "/dns-providers/1/manual-challenge/test/poll", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "INTERNAL_ERROR")
}

func TestManualChallengeHandler_ListChallenges_InvalidProviderID(t *testing.T) {
	mockService := new(MockManualChallengeService)
	mockProviderService := new(mockDNSProviderServiceForChallenge)
	handler := NewManualChallengeHandler(mockService, mockProviderService)

	router := setupChallengeTestRouter()
	router.GET("/dns-providers/:id/manual-challenges", func(c *gin.Context) {
		setUserID(c, 1)
		handler.ListChallenges(c)
	})

	req, _ := http.NewRequest("GET", "/dns-providers/invalid/manual-challenges", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "INVALID_PROVIDER_ID")
}

func TestManualChallengeHandler_ListChallenges_ProviderNotFound(t *testing.T) {
	mockService := new(MockManualChallengeService)
	mockProviderService := new(mockDNSProviderServiceForChallenge)
	handler := NewManualChallengeHandler(mockService, mockProviderService)

	router := setupChallengeTestRouter()
	router.GET("/dns-providers/:id/manual-challenges", func(c *gin.Context) {
		setUserID(c, 1)
		handler.ListChallenges(c)
	})

	mockProviderService.On("Get", mock.Anything, uint(1)).Return(nil, services.ErrDNSProviderNotFound)

	req, _ := http.NewRequest("GET", "/dns-providers/1/manual-challenges", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Contains(t, w.Body.String(), "PROVIDER_NOT_FOUND")
}

func TestManualChallengeHandler_ListChallenges_ProviderInternalError(t *testing.T) {
	mockService := new(MockManualChallengeService)
	mockProviderService := new(mockDNSProviderServiceForChallenge)
	handler := NewManualChallengeHandler(mockService, mockProviderService)

	router := setupChallengeTestRouter()
	router.GET("/dns-providers/:id/manual-challenges", func(c *gin.Context) {
		setUserID(c, 1)
		handler.ListChallenges(c)
	})

	mockProviderService.On("Get", mock.Anything, uint(1)).Return(nil, errors.New("db error"))

	req, _ := http.NewRequest("GET", "/dns-providers/1/manual-challenges", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "INTERNAL_ERROR")
}

func TestManualChallengeHandler_ListChallenges_InvalidProviderType(t *testing.T) {
	mockService := new(MockManualChallengeService)
	mockProviderService := new(mockDNSProviderServiceForChallenge)
	handler := NewManualChallengeHandler(mockService, mockProviderService)

	router := setupChallengeTestRouter()
	router.GET("/dns-providers/:id/manual-challenges", func(c *gin.Context) {
		setUserID(c, 1)
		handler.ListChallenges(c)
	})

	provider := &models.DNSProvider{ID: 1, ProviderType: "cloudflare"}
	mockProviderService.On("Get", mock.Anything, uint(1)).Return(provider, nil)

	req, _ := http.NewRequest("GET", "/dns-providers/1/manual-challenges", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "INVALID_PROVIDER_TYPE")
}

func TestManualChallengeHandler_ListChallenges_InternalError(t *testing.T) {
	mockService := new(MockManualChallengeService)
	mockProviderService := new(mockDNSProviderServiceForChallenge)
	handler := NewManualChallengeHandler(mockService, mockProviderService)

	router := setupChallengeTestRouter()
	router.GET("/dns-providers/:id/manual-challenges", func(c *gin.Context) {
		setUserID(c, 1)
		handler.ListChallenges(c)
	})

	provider := &models.DNSProvider{ID: 1, ProviderType: "manual"}
	mockProviderService.On("Get", mock.Anything, uint(1)).Return(provider, nil)
	mockService.On("ListChallengesForProvider", mock.Anything, uint(1), uint(1)).Return(nil, errors.New("db error"))

	req, _ := http.NewRequest("GET", "/dns-providers/1/manual-challenges", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "INTERNAL_ERROR")
}

func TestManualChallengeHandler_DeleteChallenge_InvalidProviderID(t *testing.T) {
	mockService := new(MockManualChallengeService)
	mockProviderService := new(mockDNSProviderServiceForChallenge)
	handler := NewManualChallengeHandler(mockService, mockProviderService)

	router := setupChallengeTestRouter()
	router.DELETE("/dns-providers/:id/manual-challenge/:challengeId", func(c *gin.Context) {
		setUserID(c, 1)
		handler.DeleteChallenge(c)
	})

	req, _ := http.NewRequest("DELETE", "/dns-providers/invalid/manual-challenge/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "INVALID_PROVIDER_ID")
}

func TestManualChallengeHandler_DeleteChallenge_EmptyChallengeID(t *testing.T) {
	mockService := new(MockManualChallengeService)
	mockProviderService := new(mockDNSProviderServiceForChallenge)
	handler := NewManualChallengeHandler(mockService, mockProviderService)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: "1"}, {Key: "challengeId", Value: ""}}
	c.Request = httptest.NewRequest("DELETE", "/dns-providers/1/manual-challenge/", nil)
	setUserID(c, 1)

	handler.DeleteChallenge(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "INVALID_CHALLENGE_ID")
}

func TestManualChallengeHandler_DeleteChallenge_ProviderNotFound(t *testing.T) {
	mockService := new(MockManualChallengeService)
	mockProviderService := new(mockDNSProviderServiceForChallenge)
	handler := NewManualChallengeHandler(mockService, mockProviderService)

	router := setupChallengeTestRouter()
	router.DELETE("/dns-providers/:id/manual-challenge/:challengeId", func(c *gin.Context) {
		setUserID(c, 1)
		handler.DeleteChallenge(c)
	})

	mockProviderService.On("Get", mock.Anything, uint(1)).Return(nil, services.ErrDNSProviderNotFound)

	req, _ := http.NewRequest("DELETE", "/dns-providers/1/manual-challenge/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Contains(t, w.Body.String(), "PROVIDER_NOT_FOUND")
}

func TestManualChallengeHandler_DeleteChallenge_ProviderInternalError(t *testing.T) {
	mockService := new(MockManualChallengeService)
	mockProviderService := new(mockDNSProviderServiceForChallenge)
	handler := NewManualChallengeHandler(mockService, mockProviderService)

	router := setupChallengeTestRouter()
	router.DELETE("/dns-providers/:id/manual-challenge/:challengeId", func(c *gin.Context) {
		setUserID(c, 1)
		handler.DeleteChallenge(c)
	})

	mockProviderService.On("Get", mock.Anything, uint(1)).Return(nil, errors.New("db error"))

	req, _ := http.NewRequest("DELETE", "/dns-providers/1/manual-challenge/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "INTERNAL_ERROR")
}

func TestManualChallengeHandler_DeleteChallenge_InvalidProviderType(t *testing.T) {
	mockService := new(MockManualChallengeService)
	mockProviderService := new(mockDNSProviderServiceForChallenge)
	handler := NewManualChallengeHandler(mockService, mockProviderService)

	router := setupChallengeTestRouter()
	router.DELETE("/dns-providers/:id/manual-challenge/:challengeId", func(c *gin.Context) {
		setUserID(c, 1)
		handler.DeleteChallenge(c)
	})

	provider := &models.DNSProvider{ID: 1, ProviderType: "cloudflare"}
	mockProviderService.On("Get", mock.Anything, uint(1)).Return(provider, nil)

	req, _ := http.NewRequest("DELETE", "/dns-providers/1/manual-challenge/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "INVALID_PROVIDER_TYPE")
}

func TestManualChallengeHandler_DeleteChallenge_ChallengeNotFound(t *testing.T) {
	mockService := new(MockManualChallengeService)
	mockProviderService := new(mockDNSProviderServiceForChallenge)
	handler := NewManualChallengeHandler(mockService, mockProviderService)

	router := setupChallengeTestRouter()
	router.DELETE("/dns-providers/:id/manual-challenge/:challengeId", func(c *gin.Context) {
		setUserID(c, 1)
		handler.DeleteChallenge(c)
	})

	provider := &models.DNSProvider{ID: 1, ProviderType: "manual"}
	mockProviderService.On("Get", mock.Anything, uint(1)).Return(provider, nil)
	mockService.On("DeleteChallenge", mock.Anything, "test", uint(1)).Return(services.ErrChallengeNotFound)

	req, _ := http.NewRequest("DELETE", "/dns-providers/1/manual-challenge/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Contains(t, w.Body.String(), "CHALLENGE_NOT_FOUND")
}

func TestManualChallengeHandler_DeleteChallenge_Unauthorized(t *testing.T) {
	mockService := new(MockManualChallengeService)
	mockProviderService := new(mockDNSProviderServiceForChallenge)
	handler := NewManualChallengeHandler(mockService, mockProviderService)

	router := setupChallengeTestRouter()
	router.DELETE("/dns-providers/:id/manual-challenge/:challengeId", func(c *gin.Context) {
		setUserID(c, 1)
		handler.DeleteChallenge(c)
	})

	provider := &models.DNSProvider{ID: 1, ProviderType: "manual"}
	mockProviderService.On("Get", mock.Anything, uint(1)).Return(provider, nil)
	mockService.On("DeleteChallenge", mock.Anything, "test", uint(1)).Return(services.ErrUnauthorized)

	req, _ := http.NewRequest("DELETE", "/dns-providers/1/manual-challenge/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Contains(t, w.Body.String(), "UNAUTHORIZED")
}

func TestManualChallengeHandler_DeleteChallenge_InternalError(t *testing.T) {
	mockService := new(MockManualChallengeService)
	mockProviderService := new(mockDNSProviderServiceForChallenge)
	handler := NewManualChallengeHandler(mockService, mockProviderService)

	router := setupChallengeTestRouter()
	router.DELETE("/dns-providers/:id/manual-challenge/:challengeId", func(c *gin.Context) {
		setUserID(c, 1)
		handler.DeleteChallenge(c)
	})

	provider := &models.DNSProvider{ID: 1, ProviderType: "manual"}
	mockProviderService.On("Get", mock.Anything, uint(1)).Return(provider, nil)
	mockService.On("DeleteChallenge", mock.Anything, "test", uint(1)).Return(errors.New("db error"))

	req, _ := http.NewRequest("DELETE", "/dns-providers/1/manual-challenge/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "INTERNAL_ERROR")
}

func TestManualChallengeHandler_CreateChallenge_InvalidProviderID(t *testing.T) {
	mockService := new(MockManualChallengeService)
	mockProviderService := new(mockDNSProviderServiceForChallenge)
	handler := NewManualChallengeHandler(mockService, mockProviderService)

	router := setupChallengeTestRouter()
	router.POST("/dns-providers/:id/manual-challenges", func(c *gin.Context) {
		setUserID(c, 1)
		handler.CreateChallenge(c)
	})

	req, _ := http.NewRequest("POST", "/dns-providers/invalid/manual-challenges", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "INVALID_PROVIDER_ID")
}

func TestManualChallengeHandler_CreateChallenge_ProviderNotFound(t *testing.T) {
	mockService := new(MockManualChallengeService)
	mockProviderService := new(mockDNSProviderServiceForChallenge)
	handler := NewManualChallengeHandler(mockService, mockProviderService)

	router := setupChallengeTestRouter()
	router.POST("/dns-providers/:id/manual-challenges", func(c *gin.Context) {
		setUserID(c, 1)
		handler.CreateChallenge(c)
	})

	mockProviderService.On("Get", mock.Anything, uint(1)).Return(nil, services.ErrDNSProviderNotFound)

	body := CreateChallengeRequest{FQDN: "_acme-challenge.example.com", Value: "test"}
	bodyBytes, _ := json.Marshal(body)

	req, _ := http.NewRequest("POST", "/dns-providers/1/manual-challenges", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Contains(t, w.Body.String(), "PROVIDER_NOT_FOUND")
}

func TestManualChallengeHandler_CreateChallenge_ProviderInternalError(t *testing.T) {
	mockService := new(MockManualChallengeService)
	mockProviderService := new(mockDNSProviderServiceForChallenge)
	handler := NewManualChallengeHandler(mockService, mockProviderService)

	router := setupChallengeTestRouter()
	router.POST("/dns-providers/:id/manual-challenges", func(c *gin.Context) {
		setUserID(c, 1)
		handler.CreateChallenge(c)
	})

	mockProviderService.On("Get", mock.Anything, uint(1)).Return(nil, errors.New("db error"))

	body := CreateChallengeRequest{FQDN: "_acme-challenge.example.com", Value: "test"}
	bodyBytes, _ := json.Marshal(body)

	req, _ := http.NewRequest("POST", "/dns-providers/1/manual-challenges", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "INTERNAL_ERROR")
}

func TestManualChallengeHandler_CreateChallenge_InvalidProviderType(t *testing.T) {
	mockService := new(MockManualChallengeService)
	mockProviderService := new(mockDNSProviderServiceForChallenge)
	handler := NewManualChallengeHandler(mockService, mockProviderService)

	router := setupChallengeTestRouter()
	router.POST("/dns-providers/:id/manual-challenges", func(c *gin.Context) {
		setUserID(c, 1)
		handler.CreateChallenge(c)
	})

	provider := &models.DNSProvider{ID: 1, ProviderType: "cloudflare"}
	mockProviderService.On("Get", mock.Anything, uint(1)).Return(provider, nil)

	body := CreateChallengeRequest{FQDN: "_acme-challenge.example.com", Value: "test"}
	bodyBytes, _ := json.Marshal(body)

	req, _ := http.NewRequest("POST", "/dns-providers/1/manual-challenges", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "INVALID_PROVIDER_TYPE")
}

func TestManualChallengeHandler_CreateChallenge_ChallengeInProgress(t *testing.T) {
	mockService := new(MockManualChallengeService)
	mockProviderService := new(mockDNSProviderServiceForChallenge)
	handler := NewManualChallengeHandler(mockService, mockProviderService)

	router := setupChallengeTestRouter()
	router.POST("/dns-providers/:id/manual-challenges", func(c *gin.Context) {
		setUserID(c, 1)
		handler.CreateChallenge(c)
	})

	provider := &models.DNSProvider{ID: 1, ProviderType: "manual"}
	mockProviderService.On("Get", mock.Anything, uint(1)).Return(provider, nil)
	mockService.On("CreateChallenge", mock.Anything, mock.Anything).Return(nil, services.ErrChallengeInProgress)

	body := CreateChallengeRequest{FQDN: "_acme-challenge.example.com", Value: "test"}
	bodyBytes, _ := json.Marshal(body)

	req, _ := http.NewRequest("POST", "/dns-providers/1/manual-challenges", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusConflict, w.Code)
	assert.Contains(t, w.Body.String(), "CHALLENGE_IN_PROGRESS")
}

func TestManualChallengeHandler_CreateChallenge_InternalError(t *testing.T) {
	mockService := new(MockManualChallengeService)
	mockProviderService := new(mockDNSProviderServiceForChallenge)
	handler := NewManualChallengeHandler(mockService, mockProviderService)

	router := setupChallengeTestRouter()
	router.POST("/dns-providers/:id/manual-challenges", func(c *gin.Context) {
		setUserID(c, 1)
		handler.CreateChallenge(c)
	})

	provider := &models.DNSProvider{ID: 1, ProviderType: "manual"}
	mockProviderService.On("Get", mock.Anything, uint(1)).Return(provider, nil)
	mockService.On("CreateChallenge", mock.Anything, mock.Anything).Return(nil, errors.New("db error"))

	body := CreateChallengeRequest{FQDN: "_acme-challenge.example.com", Value: "test"}
	bodyBytes, _ := json.Marshal(body)

	req, _ := http.NewRequest("POST", "/dns-providers/1/manual-challenges", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "INTERNAL_ERROR")
}

func TestChallengeToResponse_WithoutLastCheckAt(t *testing.T) {
	now := time.Now()

	challenge := &models.ManualChallenge{
		ID:            "test-id",
		ProviderID:    1,
		UserID:        1,
		FQDN:          "_acme-challenge.example.com",
		Value:         "txtvalue",
		Status:        models.ChallengeStatusVerified,
		DNSPropagated: true,
		CreatedAt:     now,
		ExpiresAt:     now.Add(10 * time.Minute),
		LastCheckAt:   nil, // No last check
		ErrorMessage:  "",
	}

	resp := challengeToResponse(challenge)

	assert.Equal(t, "test-id", resp.ID)
	assert.Equal(t, "verified", resp.Status)
	assert.True(t, resp.DNSPropagated)
	assert.Empty(t, resp.LastCheckAt)
	assert.Empty(t, resp.ErrorMessage)
}

func TestNewErrorResponse_NilDetails(t *testing.T) {
	resp := newErrorResponse("TEST_CODE", "Test message", nil)

	assert.False(t, resp.Success)
	assert.Equal(t, "TEST_CODE", resp.Error.Code)
	assert.Equal(t, "Test message", resp.Error.Message)
	assert.Nil(t, resp.Error.Details)
}
