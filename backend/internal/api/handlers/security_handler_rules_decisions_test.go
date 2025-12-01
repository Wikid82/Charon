package handlers

import (
    "encoding/json"
    "net/http"
    "net/http/httptest"
    "strings"
    "testing"
    "strconv"
    "time"
    "path/filepath"

    "github.com/gin-gonic/gin"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    "gorm.io/driver/sqlite"
    "gorm.io/gorm"

    "github.com/Wikid82/charon/backend/internal/config"
    "github.com/Wikid82/charon/backend/internal/models"
    "github.com/Wikid82/charon/backend/internal/caddy"
)

func setupSecurityTestRouterWithExtras(t *testing.T) (*gin.Engine, *gorm.DB) {
    t.Helper()
    // Use a file-backed sqlite DB to avoid shared memory connection issues in tests
    dsn := filepath.Join(t.TempDir(), "test.db")
    db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
    require.NoError(t, err)
    require.NoError(t, db.AutoMigrate(&models.ProxyHost{}, &models.Location{}, &models.Setting{}, &models.CaddyConfig{}, &models.SSLCertificate{}, &models.AccessList{}, &models.SecurityConfig{}, &models.SecurityDecision{}, &models.SecurityAudit{}, &models.SecurityRuleSet{}))

    r := gin.New()
    api := r.Group("/api/v1")
    cfg := config.SecurityConfig{}
    h := NewSecurityHandler(cfg, db, nil)
    api.POST("/security/decisions", h.CreateDecision)
    api.GET("/security/decisions", h.ListDecisions)
    api.POST("/security/rulesets", h.UpsertRuleSet)
    api.GET("/security/rulesets", h.ListRuleSets)
    api.DELETE("/security/rulesets/:id", h.DeleteRuleSet)
    return r, db
}

func TestSecurityHandler_CreateAndListDecisionAndRulesets(t *testing.T) {
    r, _ := setupSecurityTestRouterWithExtras(t)

    payload := `{"ip":"1.2.3.4","action":"block","host":"example.com","rule_id":"manual-1","details":"test"}`
    req := httptest.NewRequest(http.MethodPost, "/api/v1/security/decisions", strings.NewReader(payload))
    req.Header.Set("Content-Type", "application/json")
    resp := httptest.NewRecorder()
    r.ServeHTTP(resp, req)
    if resp.Code != http.StatusOK {
        t.Fatalf("Create decision expected status 200, got %d; body: %s", resp.Code, resp.Body.String())
    }

    var decisionResp map[string]interface{}
    require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &decisionResp))
    require.NotNil(t, decisionResp["decision"])

    req = httptest.NewRequest(http.MethodGet, "/api/v1/security/decisions?limit=10", nil)
    resp = httptest.NewRecorder()
    r.ServeHTTP(resp, req)
    if resp.Code != http.StatusOK {
        t.Fatalf("Upsert ruleset expected status 200, got %d; body: %s", resp.Code, resp.Body.String())
    }
    var listResp map[string][]map[string]interface{}
    require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &listResp))
    require.GreaterOrEqual(t, len(listResp["decisions"]), 1)

    // Now test ruleset upsert
    rpayload := `{"name":"owasp-crs","source_url":"https://example.com/owasp","mode":"owasp","content":"test"}`
    req = httptest.NewRequest(http.MethodPost, "/api/v1/security/rulesets", strings.NewReader(rpayload))
    req.Header.Set("Content-Type", "application/json")
    resp = httptest.NewRecorder()
    r.ServeHTTP(resp, req)
    if resp.Code != http.StatusOK {
        t.Fatalf("Upsert ruleset expected status 200, got %d; body: %s", resp.Code, resp.Body.String())
    }
    var rsResp map[string]interface{}
    require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &rsResp))
    require.NotNil(t, rsResp["ruleset"])

    req = httptest.NewRequest(http.MethodGet, "/api/v1/security/rulesets", nil)
    resp = httptest.NewRecorder()
    r.ServeHTTP(resp, req)
    if resp.Code != http.StatusOK {
        t.Fatalf("List rulesets expected status 200, got %d; body: %s", resp.Code, resp.Body.String())
    }
    var listRsResp map[string][]map[string]interface{}
    require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &listRsResp))
    require.GreaterOrEqual(t, len(listRsResp["rulesets"]), 1)

    // Delete the ruleset we just created
    idFloat, ok := listRsResp["rulesets"][0]["id"].(float64)
    require.True(t, ok)
    id := int(idFloat)
    req = httptest.NewRequest(http.MethodDelete, "/api/v1/security/rulesets/"+strconv.Itoa(id), nil)
    resp = httptest.NewRecorder()
    r.ServeHTTP(resp, req)
    assert.Equal(t, http.StatusOK, resp.Code)
    var delResp map[string]interface{}
    require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &delResp))
    require.Equal(t, true, delResp["deleted"].(bool))
}

func TestSecurityHandler_UpsertDeleteTriggersApplyConfig(t *testing.T) {
    t.Helper()
    // Setup DB
    db, err := gorm.Open(sqlite.Open("file::memory:?mode=memory&cache=shared"), &gorm.Config{})
    require.NoError(t, err)
    require.NoError(t, db.AutoMigrate(&models.SecurityConfig{}, &models.SecurityDecision{}, &models.SecurityAudit{}, &models.SecurityRuleSet{}))

    // Ensure DB has expected tables (migrations executed above)

    // Ensure proxy_hosts table exists in case AutoMigrate didn't create it
    db.Exec("CREATE TABLE IF NOT EXISTS proxy_hosts (id INTEGER PRIMARY KEY AUTOINCREMENT, domain_names TEXT, forward_host TEXT, forward_port INTEGER, enabled BOOLEAN)")
    // Create minimal settings and caddy_configs tables to satisfy Manager.ApplyConfig queries
    db.Exec("CREATE TABLE IF NOT EXISTS settings (id INTEGER PRIMARY KEY AUTOINCREMENT, key TEXT, value TEXT, type TEXT, category TEXT, updated_at datetime)")
    db.Exec("CREATE TABLE IF NOT EXISTS caddy_configs (id INTEGER PRIMARY KEY AUTOINCREMENT, config_hash TEXT, applied_at datetime, success BOOLEAN, error_msg TEXT)")
    // debug: tables exist

    // Caddy admin server to capture /load calls
    loadCh := make(chan struct{}, 2)
    caddyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if r.URL.Path == "/load" && r.Method == http.MethodPost {
            loadCh <- struct{}{}
            w.WriteHeader(http.StatusOK)
            return
        }
        w.WriteHeader(http.StatusNotFound)
    }))
    defer caddyServer.Close()

    client := caddy.NewClient(caddyServer.URL)
    tmp := t.TempDir()
    m := caddy.NewManager(client, db, tmp, "", false, config.SecurityConfig{CerberusEnabled: true, WAFMode: "block"})

    r := gin.New()
    api := r.Group("/api/v1")
    cfg := config.SecurityConfig{}
    h := NewSecurityHandler(cfg, db, m)
    api.POST("/security/rulesets", h.UpsertRuleSet)
    api.DELETE("/security/rulesets/:id", h.DeleteRuleSet)

    // Upsert ruleset should trigger manager.ApplyConfig -> POST /load
    rpayload := `{"name":"owasp-crs","source_url":"https://example.com/owasp","mode":"owasp","content":"test"}`
    req := httptest.NewRequest(http.MethodPost, "/api/v1/security/rulesets", strings.NewReader(rpayload))
    req.Header.Set("Content-Type", "application/json")
    resp := httptest.NewRecorder()
    r.ServeHTTP(resp, req)
    assert.Equal(t, http.StatusOK, resp.Code)
    select {
    case <-loadCh:
    case <-time.After(2 * time.Second):
        t.Fatal("timed out waiting for manager ApplyConfig /load post on upsert")
    }

    // Now delete the ruleset and ensure /load is triggered again
    // Read ID from DB
    var rs models.SecurityRuleSet
    assert.NoError(t, db.First(&rs).Error)
    req = httptest.NewRequest(http.MethodDelete, "/api/v1/security/rulesets/"+strconv.Itoa(int(rs.ID)), nil)
    resp = httptest.NewRecorder()
    r.ServeHTTP(resp, req)
    assert.Equal(t, http.StatusOK, resp.Code)
    select {
    case <-loadCh:
    case <-time.After(2 * time.Second):
        t.Fatal("timed out waiting for manager ApplyConfig /load post on delete")
    }
}
