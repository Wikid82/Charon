package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	"github.com/Wikid82/CaddyProxyManagerPlus/backend/internal/services"
)

func TestUpdateHandler_Check(t *testing.T) {
	// Mock GitHub API
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/releases/latest" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"tag_name":"v1.0.0","html_url":"https://github.com/example/repo/releases/tag/v1.0.0"}`))
	}))
	defer server.Close()

	// Setup Service
	svc := services.NewUpdateService()
	svc.SetAPIURL(server.URL + "/releases/latest")

	// Setup Handler
	h := NewUpdateHandler(svc)

	// Setup Router
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/api/v1/update", h.Check)

	// Test Request
	req := httptest.NewRequest(http.MethodGet, "/api/v1/update", nil)
	resp := httptest.NewRecorder()
	r.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)

	var info services.UpdateInfo
	err := json.Unmarshal(resp.Body.Bytes(), &info)
	assert.NoError(t, err)
	assert.True(t, info.Available) // Assuming current version is not v1.0.0
	assert.Equal(t, "v1.0.0", info.LatestVersion)

	// Test Failure
	serverError := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer serverError.Close()

	svcError := services.NewUpdateService()
	svcError.SetAPIURL(serverError.URL)
	hError := NewUpdateHandler(svcError)

	rError := gin.New()
	rError.GET("/api/v1/update", hError.Check)

	reqError := httptest.NewRequest(http.MethodGet, "/api/v1/update", nil)
	respError := httptest.NewRecorder()
	rError.ServeHTTP(respError, reqError)

	assert.Equal(t, http.StatusOK, respError.Code)
	var infoError services.UpdateInfo
	err = json.Unmarshal(respError.Body.Bytes(), &infoError)
	assert.NoError(t, err)
	assert.False(t, infoError.Available)

	// Test Client Error (Invalid URL)
	svcClientError := services.NewUpdateService()
	svcClientError.SetAPIURL("http://invalid-url-that-does-not-exist")
	hClientError := NewUpdateHandler(svcClientError)

	rClientError := gin.New()
	rClientError.GET("/api/v1/update", hClientError.Check)

	reqClientError := httptest.NewRequest(http.MethodGet, "/api/v1/update", nil)
	respClientError := httptest.NewRecorder()
	rClientError.ServeHTTP(respClientError, reqClientError)

	// CheckForUpdates returns error on client failure
	// Handler returns 500 on error
	assert.Equal(t, http.StatusInternalServerError, respClientError.Code)
}
