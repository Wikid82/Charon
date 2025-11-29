//go:build ignore
// +build ignore

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

// The original file had duplicated content and misplaced build tags.
// Keep a single, well-structured test to verify both enabled/disabled security states.
func TestSecurityHandler_GetStatus(t *testing.T) {
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
            handler := NewSecurityHandler(tt.cfg, nil)
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
            json.Unmarshal(expectedJSON, &expectedNormalized)

            assert.Equal(t, expectedNormalized, response)
        })
    }
}
//go:build ignore
// +build ignore

//go:build ignore
// +build ignore

package handlers

/*
    File intentionally ignored/build-tagged - see security_handler_clean_test.go for tests.
*/

// EOF

func TestSecurityHandler_GetStatus(t *testing.T) {
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
            handler := NewSecurityHandler(tt.cfg, nil)
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
            json.Unmarshal(expectedJSON, &expectedNormalized)

            assert.Equal(t, expectedNormalized, response)
        })
    }
}
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

func TestSecurityHandler_GetStatus(t *testing.T) {
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
            handler := NewSecurityHandler(tt.cfg, nil)
            router := gin.New()
            router.GET("/security/status", handler.GetStatus)

            w := httptest.NewRecorder()
            req, _ := http.NewRequest("GET", "/security/status", nil)
            router.ServeHTTP(w, req)

            assert.Equal(t, tt.expectedStatus, w.Code)

            var response map[string]interface{}
            err := json.Unmarshal(w.Body.Bytes(), &response)
            assert.NoError(t, err)

            // Helper to convert map[string]interface{} to JSON and back to normalize types
            // (e.g. int vs float64)
            expectedJSON, _ := json.Marshal(tt.expectedBody)
            var expectedNormalized map[string]interface{}
            json.Unmarshal(expectedJSON, &expectedNormalized)

            assert.Equal(t, expectedNormalized, response)
        })
    }
}
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

func TestSecurityHandler_GetStatus(t *testing.T) {
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
            handler := NewSecurityHandler(tt.cfg, nil)
            router := gin.New()
            router.GET("/security/status", handler.GetStatus)

            w := httptest.NewRecorder()
            req, _ := http.NewRequest("GET", "/security/status", nil)
            router.ServeHTTP(w, req)

            assert.Equal(t, tt.expectedStatus, w.Code)

            var response map[string]interface{}
            err := json.Unmarshal(w.Body.Bytes(), &response)
            assert.NoError(t, err)

            // Helper to convert map[string]interface{} to JSON and back to normalize types
            // (e.g. int vs float64)
            expectedJSON, _ := json.Marshal(tt.expectedBody)
            var expectedNormalized map[string]interface{}
            json.Unmarshal(expectedJSON, &expectedNormalized)

            assert.Equal(t, expectedNormalized, response)
        })
    }
}
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

func TestSecurityHandler_GetStatus(t *testing.T) {
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
            handler := NewSecurityHandler(tt.cfg, nil)
            router := gin.New()
            router.GET("/security/status", handler.GetStatus)

            w := httptest.NewRecorder()
            req, _ := http.NewRequest("GET", "/security/status", nil)
            router.ServeHTTP(w, req)

            assert.Equal(t, tt.expectedStatus, w.Code)

            var response map[string]interface{}
            err := json.Unmarshal(w.Body.Bytes(), &response)
            assert.NoError(t, err)

            // Helper to convert map[string]interface{} to JSON and back to normalize types
            // (e.g. int vs float64)
            expectedJSON, _ := json.Marshal(tt.expectedBody)
            var expectedNormalized map[string]interface{}
            json.Unmarshal(expectedJSON, &expectedNormalized)

            assert.Equal(t, expectedNormalized, response)
        })
    }
}
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

func TestSecurityHandler_GetStatus(t *testing.T) {
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
            handler := NewSecurityHandler(tt.cfg, nil)
            router := gin.New()
            router.GET("/security/status", handler.GetStatus)

            w := httptest.NewRecorder()
            req, _ := http.NewRequest("GET", "/security/status", nil)
            router.ServeHTTP(w, req)

            assert.Equal(t, tt.expectedStatus, w.Code)

            var response map[string]interface{}
            err := json.Unmarshal(w.Body.Bytes(), &response)
            assert.NoError(t, err)

            // Helper to convert map[string]interface{} to JSON and back to normalize types
            // (e.g. int vs float64)
            expectedJSON, _ := json.Marshal(tt.expectedBody)
            var expectedNormalized map[string]interface{}
            json.Unmarshal(expectedJSON, &expectedNormalized)

            assert.Equal(t, expectedNormalized, response)
        })
    }
}
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

func TestSecurityHandler_GetStatus(t *testing.T) {
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
            handler := NewSecurityHandler(tt.cfg, nil)
            router := gin.New()
            router.GET("/security/status", handler.GetStatus)

            w := httptest.NewRecorder()
            req, _ := http.NewRequest("GET", "/security/status", nil)
            router.ServeHTTP(w, req)

            assert.Equal(t, tt.expectedStatus, w.Code)

            var response map[string]interface{}
            err := json.Unmarshal(w.Body.Bytes(), &response)
            assert.NoError(t, err)

            // Helper to convert map[string]interface{} to JSON and back to normalize types
            // (e.g. int vs float64)
            expectedJSON, _ := json.Marshal(tt.expectedBody)
            var expectedNormalized map[string]interface{}
            json.Unmarshal(expectedJSON, &expectedNormalized)

            assert.Equal(t, expectedNormalized, response)
        })
    }
}
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

func TestSecurityHandler_GetStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		cfg            config.SecurityConfig
		expectedStatus int
		expectedBody   map[string]interface{}
	}{
		{
				expectedBody: map[string]interface{}{
					"cerberus": map[string]interface{}{"enabled": false},
			cfg: config.SecurityConfig{
				CrowdSecMode:  "disabled",
				WAFMode:       "disabled",
				RateLimitMode: "disabled",
				ACLMode:       "disabled",
			},
			expectedStatus: http.StatusOK,
			expectedBody: map[string]interface{}{
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
			handler := NewSecurityHandler(tt.cfg, nil)
					"enabled": true,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewSecurityHandler(tt.cfg)
			router := gin.New()
			router.GET("/security/status", handler.GetStatus)

			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", "/security/status", nil)
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			assert.NoError(t, err)

			// Helper to convert map[string]interface{} to JSON and back to normalize types
			// (e.g. int vs float64)
			expectedJSON, _ := json.Marshal(tt.expectedBody)
			var expectedNormalized map[string]interface{}
			json.Unmarshal(expectedJSON, &expectedNormalized)

			assert.Equal(t, expectedNormalized, response)
		})
	}
}
