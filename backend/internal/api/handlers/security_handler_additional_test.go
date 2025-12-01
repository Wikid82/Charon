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

    "github.com/Wikid82/charon/backend/internal/config"
    "github.com/Wikid82/charon/backend/internal/models"
)

func TestSecurityHandler_GetConfigAndUpdateConfig(t *testing.T) {
    t.Helper()
    // Setup DB and router
    db, err := gorm.Open(sqlite.Open("file::memory:?mode=memory&cache=shared"), &gorm.Config{})
    require.NoError(t, err)
    require.NoError(t, db.AutoMigrate(&models.SecurityConfig{}))

    cfg := config.SecurityConfig{}
    h := NewSecurityHandler(cfg, db)

    // Create a gin test context for GetConfig when no config exists
    w := httptest.NewRecorder()
    c, _ := gin.CreateTestContext(w)
    req := httptest.NewRequest("GET", "/security/config", nil)
    c.Request = req
    h.GetConfig(c)
    require.Equal(t, http.StatusOK, w.Code)
    var body map[string]interface{}
    require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
    // Should return config: null
    if _, ok := body["config"]; !ok {
        t.Fatalf("expected 'config' in response, got %v", body)
    }

    // Now update config
    w = httptest.NewRecorder()
    c, _ = gin.CreateTestContext(w)
    payload := `{"name":"default","admin_whitelist":"127.0.0.1/32"}`
    req = httptest.NewRequest("POST", "/security/config", strings.NewReader(payload))
    req.Header.Set("Content-Type", "application/json")
    c.Request = req
    h.UpdateConfig(c)
    require.Equal(t, http.StatusOK, w.Code)

    // Now call GetConfig again and ensure config is returned
    w = httptest.NewRecorder()
    c, _ = gin.CreateTestContext(w)
    req = httptest.NewRequest("GET", "/security/config", nil)
    c.Request = req
    h.GetConfig(c)
    require.Equal(t, http.StatusOK, w.Code)
    var body2 map[string]interface{}
    require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body2))
    cfgVal, ok := body2["config"].(map[string]interface{})
    if !ok {
        t.Fatalf("expected config object, got %v", body2["config"])
    }
    if cfgVal["admin_whitelist"] != "127.0.0.1/32" {
        t.Fatalf("unexpected admin_whitelist: %v", cfgVal["admin_whitelist"])
    }
}
