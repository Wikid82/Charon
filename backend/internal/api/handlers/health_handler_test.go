package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestHealthHandler(t *testing.T) {
	r := gin.New()
	r.GET("/health", HealthHandler)

	req, _ := http.NewRequest("GET", "/health", http.NoBody)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, "ok", resp["status"])
	assert.NotEmpty(t, resp["version"])
}

func TestGetLocalIP(t *testing.T) {
	// This test just ensures getLocalIP doesn't panic
	// It may return empty string in test environments
	ip := getLocalIP()
	// IP can be empty or a valid IPv4 address
	t.Logf("getLocalIP returned: %q", ip)
	// No assertion needed - just exercising the code path
}
