//go:build ignore

package handlers

import (
    "encoding/json"
    "net/http"
    "net/http/httptest"
    "strings"
    "testing"

    "github.com/gin-gonic/gin"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    "gorm.io/driver/sqlite"
    "gorm.io/gorm"

    "github.com/Wikid82/charon/backend/internal/config"
    "github.com/Wikid82/charon/backend/internal/models"
)

// Intentionally ignored by build to avoid duplicate test artifacts during initial scaffolding
// Use security_handler_clean_test.go for canonical tests.

func setupSecurityTestRouter(t *testing.T) (*gin.Engine, *gorm.DB) {
    db, err := gorm.Open(sqlite.Open("file::memory:?mode=memory&cache=shared"), &gorm.Config{})
    require.NoError(t, err)
    require.NoError(t, db.AutoMigrate(&models.SecurityConfig{}))

    r := gin.New()
    api := r.Group("/api/v1")
    cfg := config.SecurityConfig{}
    h := NewSecurityHandler(cfg, db)
    api.GET("/security/status", h.GetStatus)
    api.GET("/security/config", h.GetConfig)
    api.POST("/security/config", h.UpdateConfig)
    api.POST("/security/enable", h.Enable)
    api.POST("/security/disable", h.Disable)
    api.POST("/security/breakglass/generate", h.GenerateBreakGlass)
    return r, db
}

func TestSecurityHandler_ConfigUpsertAndBreakGlass(t *testing.T) {
    r, _ := setupSecurityTestRouter(t)

    body := `{"name":"default","admin_whitelist":"invalid-cidr"}`
    req := httptest.NewRequest(http.MethodPost, "/api/v1/security/config", strings.NewReader(body))
    req.Header.Set("Content-Type", "application/json")
    resp := httptest.NewRecorder()
    r.ServeHTTP(resp, req)
    assert.Equal(t, http.StatusBadRequest, resp.Code)

    body = `{"name":"default","admin_whitelist":"127.0.0.1/32"}`
    req = httptest.NewRequest(http.MethodPost, "/api/v1/security/config", strings.NewReader(body))
    req.Header.Set("Content-Type", "application/json")
    resp = httptest.NewRecorder()
    r.ServeHTTP(resp, req)
    assert.Equal(t, http.StatusOK, resp.Code)

    req = httptest.NewRequest(http.MethodPost, "/api/v1/security/breakglass/generate", nil)
    resp = httptest.NewRecorder()
    r.ServeHTTP(resp, req)
    assert.Equal(t, http.StatusOK, resp.Code)
    var tokenResp map[string]string
    require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &tokenResp))
    require.NotEmpty(t, tokenResp["token"])
}

func TestSecurityHandler_GetStatus(t *testing.T) {
    handler := NewSecurityHandler(config.SecurityConfig{CrowdSecMode: "disabled", WAFMode: "disabled", RateLimitMode: "disabled", ACLMode: "disabled"}, nil)
    router := gin.New()
    router.GET("/security/status", handler.GetStatus)

    w := httptest.NewRecorder()
    req, _ := http.NewRequest("GET", "/security/status", nil)
    router.ServeHTTP(w, req)
    assert.Equal(t, http.StatusOK, w.Code)
}
package handlers

import (
    "encoding/json"
    "net/http"
    "net/http/httptest"
    "strings"
    "testing"

    "github.com/gin-gonic/gin"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    "gorm.io/driver/sqlite"
    "gorm.io/gorm"

    "github.com/Wikid82/charon/backend/internal/config"
    "github.com/Wikid82/charon/backend/internal/models"
)

func setupSecurityTestRouter(t *testing.T) (*gin.Engine, *gorm.DB) {
    t.Helper()
    db, err := gorm.Open(sqlite.Open("file::memory:?mode=memory&cache=shared"), &gorm.Config{})
    require.NoError(t, err)
    require.NoError(t, db.AutoMigrate(&models.SecurityConfig{}))

    r := gin.New()
    api := r.Group("/api/v1")
    cfg := config.SecurityConfig{}
    h := NewSecurityHandler(cfg, db)
    api.GET("/security/status", h.GetStatus)
    api.GET("/security/config", h.GetConfig)
    api.POST("/security/config", h.UpdateConfig)
    api.POST("/security/enable", h.Enable)
    api.POST("/security/disable", h.Disable)
    api.POST("/security/breakglass/generate", h.GenerateBreakGlass)
    return r, db
}

func TestSecurityHandler_ConfigAndBreakGlassLifecycle(t *testing.T) {
    r, _ := setupSecurityTestRouter(t)

    // Invalid admin whitelist
    body := `{"name":"default","admin_whitelist":"invalid-cidr"}`
    req := httptest.NewRequest(http.MethodPost, "/api/v1/security/config", strings.NewReader(body))
    req.Header.Set("Content-Type", "application/json")
    resp := httptest.NewRecorder()
    r.ServeHTTP(resp, req)
    assert.Equal(t, http.StatusBadRequest, resp.Code)

    // Now update config with a valid admin whitelist
    body = `{"name":"default","admin_whitelist":"127.0.0.1/32"}`
    req = httptest.NewRequest(http.MethodPost, "/api/v1/security/config", strings.NewReader(body))
    req.Header.Set("Content-Type", "application/json")
    resp = httptest.NewRecorder()
    r.ServeHTTP(resp, req)
    assert.Equal(t, http.StatusOK, resp.Code)

    // Generate break-glass token
    req = httptest.NewRequest(http.MethodPost, "/api/v1/security/breakglass/generate", nil)
    resp = httptest.NewRecorder()
    r.ServeHTTP(resp, req)
    assert.Equal(t, http.StatusOK, resp.Code)
    var tokenResp map[string]string
    require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &tokenResp))
    token := tokenResp["token"]
    require.NotEmpty(t, token)

    // Enable using admin whitelist (X-Forwarded-For) - this should succeed
    req = httptest.NewRequest(http.MethodPost, "/api/v1/security/enable", strings.NewReader(`{}`))
    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("X-Forwarded-For", "127.0.0.1")
    resp = httptest.NewRecorder()
    r.ServeHTTP(resp, req)
    assert.Equal(t, http.StatusOK, resp.Code)

    // Disable using break glass token
    req = httptest.NewRequest(http.MethodPost, "/api/v1/security/disable", strings.NewReader(`{"break_glass_token":"`+token+`"}`))
    req.Header.Set("Content-Type", "application/json")
    resp = httptest.NewRecorder()
    r.ServeHTTP(resp, req)
    assert.Equal(t, http.StatusOK, resp.Code)
}

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
    "strings"
    "testing"

    "github.com/gin-gonic/gin"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    "gorm.io/driver/sqlite"
    "gorm.io/gorm"

    "github.com/Wikid82/charon/backend/internal/config"
    "github.com/Wikid82/charon/backend/internal/models"
)

func setupSecurityTestRouter(t *testing.T) (*gin.Engine, *gorm.DB) {
    t.Helper()
    db, err := gorm.Open(sqlite.Open("file::memory:?mode=memory&cache=shared"), &gorm.Config{})
    require.NoError(t, err)
    require.NoError(t, db.AutoMigrate(&models.SecurityConfig{}))

    r := gin.New()
    api := r.Group("/api/v1")
    cfg := config.SecurityConfig{}
    h := NewSecurityHandler(cfg, db)
    api.GET("/security/status", h.GetStatus)
    api.GET("/security/config", h.GetConfig)
    api.POST("/security/config", h.UpdateConfig)
    api.POST("/security/enable", h.Enable)
    api.POST("/security/disable", h.Disable)
    api.POST("/security/breakglass/generate", h.GenerateBreakGlass)
    return r, db
}

func TestSecurityHandler_ConfigAndBreakGlassLifecycle(t *testing.T) {
    r, _ := setupSecurityTestRouter(t)

    // Invalid admin whitelist
    body := `{"name":"default","admin_whitelist":"invalid-cidr"}`
    req := httptest.NewRequest(http.MethodPost, "/api/v1/security/config", strings.NewReader(body))
    req.Header.Set("Content-Type", "application/json")
    resp := httptest.NewRecorder()
    r.ServeHTTP(resp, req)
    assert.Equal(t, http.StatusBadRequest, resp.Code)

    // Now update config with a valid admin whitelist
    body = `{"name":"default","admin_whitelist":"127.0.0.1/32"}`
    req = httptest.NewRequest(http.MethodPost, "/api/v1/security/config", strings.NewReader(body))
    req.Header.Set("Content-Type", "application/json")
    resp = httptest.NewRecorder()
    r.ServeHTTP(resp, req)
    assert.Equal(t, http.StatusOK, resp.Code)

    // Generate break-glass token
    req = httptest.NewRequest(http.MethodPost, "/api/v1/security/breakglass/generate", nil)
    resp = httptest.NewRecorder()
    r.ServeHTTP(resp, req)
    assert.Equal(t, http.StatusOK, resp.Code)
    var tokenResp map[string]string
    require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &tokenResp))
    token := tokenResp["token"]
    require.NotEmpty(t, token)

    // Enable using admin whitelist (X-Forwarded-For) - this should succeed
    req = httptest.NewRequest(http.MethodPost, "/api/v1/security/enable", strings.NewReader(`{}`))
    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("X-Forwarded-For", "127.0.0.1")
    resp = httptest.NewRecorder()
    r.ServeHTTP(resp, req)
    assert.Equal(t, http.StatusOK, resp.Code)

    // Disable using break glass token
    req = httptest.NewRequest(http.MethodPost, "/api/v1/security/disable", strings.NewReader(`{"break_glass_token":"`+token+`"}`))
    req.Header.Set("Content-Type", "application/json")
    resp = httptest.NewRecorder()
    r.ServeHTTP(resp, req)
    assert.Equal(t, http.StatusOK, resp.Code)
}

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
    "strings"
    "testing"

    "github.com/gin-gonic/gin"
    "github.com/stretchr/testify/require"
    "gorm.io/driver/sqlite"
    "gorm.io/gorm"

    "github.com/Wikid82/charon/backend/internal/models"
    "github.com/Wikid82/charon/backend/internal/config"
)

func setupSecurityTestRouter(t *testing.T) (*gin.Engine, *gorm.DB) {
    t.Helper()
    db, err := gorm.Open(sqlite.Open("file::memory:?mode=memory&cache=shared"), &gorm.Config{})
    require.NoError(t, err)
    require.NoError(t, db.AutoMigrate(&models.SecurityConfig{}))

    r := gin.New()
    api := r.Group("/api/v1")
    cfg := config.SecurityConfig{}
    h := NewSecurityHandler(cfg, db)
    // The NewSecurityHandler above matches pattern; here we'll use the real handler
    // Register the routes manually
    api.GET("/security/status", h.GetStatus)
    api.GET("/security/config", h.GetConfig)
    api.POST("/security/config", h.UpdateConfig)
    api.POST("/security/enable", h.Enable)
    api.POST("/security/disable", h.Disable)
    api.POST("/security/breakglass/generate", h.GenerateBreakGlass)
    return r, db
}

func TestSecurityHandler_ConfigAndBreakGlassLifecycle(t *testing.T) {
    r, _ := setupSecurityTestRouter(t)

    // Invalid admin whitelist JSON - missing because we accept comma-separated CIDRs
    body := `{"name":"default","admin_whitelist":"invalid-cidr"}`
    req := httptest.NewRequest(http.MethodPost, "/api/v1/security/config", strings.NewReader(body))
    req.Header.Set("Content-Type", "application/json")
    resp := httptest.NewRecorder()
    r.ServeHTTP(resp, req)
    require.Equal(t, http.StatusBadRequest, resp.Code)

    // Now update config with a valid admin whitelist
    body = `{"name":"default","admin_whitelist":"127.0.0.1/32"}`
    req = httptest.NewRequest(http.MethodPost, "/api/v1/security/config", strings.NewReader(body))
    req.Header.Set("Content-Type", "application/json")
    resp = httptest.NewRecorder()
    r.ServeHTTP(resp, req)
    require.Equal(t, http.StatusOK, resp.Code)

    // Generate break-glass token
    req = httptest.NewRequest(http.MethodPost, "/api/v1/security/breakglass/generate", nil)
    resp = httptest.NewRecorder()
    r.ServeHTTP(resp, req)
    require.Equal(t, http.StatusOK, resp.Code)
    var tokenResp map[string]string
    require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &tokenResp))
    token := tokenResp["token"]
    require.NotEmpty(t, token)

    // Enable using admin whitelist (X-Forwarded-For) - this should succeed
    req = httptest.NewRequest(http.MethodPost, "/api/v1/security/enable", strings.NewReader(`{}`))
    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("X-Forwarded-For", "127.0.0.1")
    resp = httptest.NewRecorder()
    r.ServeHTTP(resp, req)
    require.Equal(t, http.StatusOK, resp.Code)

    // Disable using break glass token
    req = httptest.NewRequest(http.MethodPost, "/api/v1/security/disable", strings.NewReader(`{"break_glass_token":"`+token+`"}`))
    req.Header.Set("Content-Type", "application/json")
    resp = httptest.NewRecorder()
    r.ServeHTTP(resp, req)
    require.Equal(t, http.StatusOK, resp.Code)
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
