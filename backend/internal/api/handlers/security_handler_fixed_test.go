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

	tests := []struct {
		name           string
		cfg            config.SecurityConfig
		expectedStatus int
		expectedBody   map[string]any
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
			expectedBody: map[string]any{
				"cerberus": map[string]any{"enabled": false},
				"crowdsec": map[string]any{
					"mode":    "disabled",
					"api_url": "",
					"enabled": false,
				},
				"waf": map[string]any{
					"mode":    "disabled",
					"enabled": false,
				},
				"rate_limit": map[string]any{
					"mode":    "disabled",
					"enabled": false,
				},
				"acl": map[string]any{
					"mode":    "disabled",
					"enabled": false,
				},
				"config_apply": map[string]any{
					"available": false,
					"status":    "unknown",
				},
			},
		},
		{
			name: "All Enabled",
			cfg: config.SecurityConfig{
				CerberusEnabled: true, // Required for ACL to be effective
				CrowdSecMode:    "local",
				WAFMode:         "enabled",
				RateLimitMode:   "enabled",
				ACLMode:         "enabled",
			},
			expectedStatus: http.StatusOK,
			expectedBody: map[string]any{
				"cerberus": map[string]any{"enabled": true},
				"crowdsec": map[string]any{
					"mode":    "local",
					"api_url": "",
					"enabled": true,
				},
				"waf": map[string]any{
					"mode":    "enabled",
					"enabled": true,
				},
				"rate_limit": map[string]any{
					"mode":    "enabled",
					"enabled": true,
				},
				"acl": map[string]any{
					"mode":    "enabled",
					"enabled": true,
				},
				"config_apply": map[string]any{
					"available": false,
					"status":    "unknown",
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
			req, _ := http.NewRequest("GET", "/security/status", http.NoBody)
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			var response map[string]any
			err := json.Unmarshal(w.Body.Bytes(), &response)
			assert.NoError(t, err)

			expectedJSON, _ := json.Marshal(tt.expectedBody)
			var expectedNormalized map[string]any
			if err := json.Unmarshal(expectedJSON, &expectedNormalized); err != nil {
				t.Fatalf("failed to unmarshal expected JSON: %v", err)
			}

			assert.Equal(t, expectedNormalized, response)
		})
	}
}
