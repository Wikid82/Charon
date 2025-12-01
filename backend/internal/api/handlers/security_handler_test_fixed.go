package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	"github.com/Wikid82/charon/backend/internal/config"
)

func TestSecurityHandler_GetStatus_Fixed(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		cfg            config.SecurityConfig
		expectedStatus int
		expectedBody   map[string]interface{}
	}{
		{
			name: "All Disabled",
			cfg: config.SecurityConfig{
				CrowdSecMode:  "disabled",
				WAFMode:       "disabled",
				RateLimitMode: "disabled",
				ACLMode:       "disabled",
			},
			expectedStatus: http.StatusOK,
			expectedBody: map[string]interface{}{
				"cerberus": map[string]interface{}{"enabled": false},
				"crowdsec": map[string]interface{}{
					"mode":    "disabled",
					"api_url": "",
					"enabled": false,
				},
				"waf": map[string]interface{}{
					"mode":    "disabled",
					"enabled": false,
				},
				"rate_limit": map[string]interface{}{
					"mode":    "disabled",
					"enabled": false,
				},
				"acl": map[string]interface{}{
					"mode":    "disabled",
					"enabled": false,
				},
			},
		},
		{
			name: "All Enabled",
			cfg: config.SecurityConfig{
				CrowdSecMode:  "local",
				WAFMode:       "enabled",
				RateLimitMode: "enabled",
				ACLMode:       "enabled",
			},
			expectedStatus: http.StatusOK,
			expectedBody: map[string]interface{}{
				"cerberus": map[string]interface{}{"enabled": true},
				"crowdsec": map[string]interface{}{
					"mode":    "local",
					"api_url": "",
					"enabled": true,
				},
				"waf": map[string]interface{}{
					"mode":    "enabled",
					"enabled": true,
				},
				"rate_limit": map[string]interface{}{
					"mode":    "enabled",
					"enabled": true,
				},
				"acl": map[string]interface{}{
					"mode":    "enabled",
					"enabled": true,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewSecurityHandler(tt.cfg, nil, nil)
			router := gin.New()
			router.GET("/security/status", handler.GetStatus)

			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", "/security/status", nil)
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			assert.NoError(t, err)

			expectedJSON, _ := json.Marshal(tt.expectedBody)
			var expectedNormalized map[string]interface{}
			if err := json.Unmarshal(expectedJSON, &expectedNormalized); err != nil {
				t.Fatalf("failed to unmarshal expected JSON: %v", err)
			}

			assert.Equal(t, expectedNormalized, response)
		})
	}
}
