package handlers_test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/api/handlers"
	"github.com/Wikid82/charon/backend/internal/models"
)

type mockCaddyConfigManager struct {
	applyFunc func(context.Context) error
	calls     int
}

type mockCacheInvalidator struct {
	calls int
}

func (m *mockCacheInvalidator) InvalidateCache() {
	m.calls++
}

func (m *mockCaddyConfigManager) ApplyConfig(ctx context.Context) error {
	m.calls++
	if m.applyFunc != nil {
		return m.applyFunc(ctx)
	}
	return nil
}

func startTestSMTPServer(t *testing.T) (host string, port int) {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen for smtp test server: %v", err)
	}

	var wg sync.WaitGroup
	acceptDone := make(chan struct{})
	go func() {
		defer close(acceptDone)
		for {
			conn, acceptErr := ln.Accept()
			if acceptErr != nil {
				return
			}
			wg.Add(1)
			go func(c net.Conn) {
				defer wg.Done()
				defer func() { _ = c.Close() }()
				handleSMTPConnection(c)
			}(conn)
		}
	}()

	t.Cleanup(func() {
		_ = ln.Close()
		<-acceptDone
		wg.Wait()
	})

	host, portStr, err := net.SplitHostPort(ln.Addr().String())
	if err != nil {
		t.Fatalf("failed to split smtp listener addr: %v", err)
	}
	if _, err := fmt.Sscanf(portStr, "%d", &port); err != nil {
		t.Fatalf("failed to parse smtp listener port: %v", err)
	}

	return host, port
}

func handleSMTPConnection(conn net.Conn) {
	r := bufio.NewReader(conn)
	w := bufio.NewWriter(conn)

	writeLine := func(line string) {
		_, _ = w.WriteString(line + "\r\n")
		_ = w.Flush()
	}

	writeLine("220 localhost ESMTP test")

	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return
		}
		cmd := strings.TrimSpace(line)
		upper := strings.ToUpper(cmd)

		switch {
		case strings.HasPrefix(upper, "EHLO") || strings.HasPrefix(upper, "HELO"):
			writeLine("250-localhost")
			writeLine("250 OK")
		case strings.HasPrefix(upper, "MAIL FROM:"):
			writeLine("250 OK")
		case strings.HasPrefix(upper, "RCPT TO:"):
			writeLine("250 OK")
		case strings.HasPrefix(upper, "DATA"):
			writeLine("354 End data with <CR><LF>.<CR><LF>")
			for {
				dataLine, err := r.ReadString('\n')
				if err != nil {
					return
				}
				if strings.TrimRight(dataLine, "\r\n") == "." {
					break
				}
			}
			writeLine("250 OK")
		case strings.HasPrefix(upper, "RSET"):
			writeLine("250 OK")
		case strings.HasPrefix(upper, "NOOP"):
			writeLine("250 OK")
		case strings.HasPrefix(upper, "QUIT"):
			writeLine("221 Bye")
			return
		default:
			writeLine("250 OK")
		}
	}
}

func setupSettingsTestDB(t *testing.T) *gorm.DB {
	dsn := "file:" + t.Name() + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		panic("failed to connect to test database")
	}
	_ = db.AutoMigrate(&models.Setting{}, &models.SecurityConfig{})
	return db
}

func newAdminRouter() *gin.Engine {
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("role", "admin")
		c.Set("userID", uint(1))
		c.Next()
	})
	return router
}

func TestSettingsHandler_GetSettings(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupSettingsTestDB(t)

	// Seed data
	db.Create(&models.Setting{Key: "test_key", Value: "test_value", Category: "general", Type: "string"})

	handler := handlers.NewSettingsHandler(db)
	router := newAdminRouter()
	router.GET("/settings", handler.GetSettings)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/settings", http.NoBody)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "test_value", response["test_key"])
}

func TestSettingsHandler_GetSettings_MasksSensitiveValues(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupSettingsTestDB(t)

	db.Create(&models.Setting{Key: "smtp_password", Value: "super-secret-password", Category: "smtp", Type: "string"})

	handler := handlers.NewSettingsHandler(db)
	router := newAdminRouter()
	router.GET("/settings", handler.GetSettings)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/settings", http.NoBody)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]any
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "********", response["smtp_password"])
	assert.Equal(t, true, response["smtp_password.has_secret"])
	_, hasRaw := response["super-secret-password"]
	assert.False(t, hasRaw)
}

func TestSettingsHandler_GetSettings_DatabaseError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupSettingsTestDB(t)

	// Close the database to force an error
	sqlDB, _ := db.DB()
	_ = sqlDB.Close()

	handler := handlers.NewSettingsHandler(db)
	router := newAdminRouter()
	router.GET("/settings", handler.GetSettings)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/settings", http.NoBody)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Contains(t, response["error"], "Failed to fetch settings")
}

func TestSettingsHandler_UpdateSettings(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupSettingsTestDB(t)

	handler := handlers.NewSettingsHandler(db)
	router := newAdminRouter()
	router.POST("/settings", handler.UpdateSetting)

	// Test Create
	payload := map[string]string{
		"key":      "new_key",
		"value":    "new_value",
		"category": "system",
		"type":     "string",
	}
	body, _ := json.Marshal(payload)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/settings", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var setting models.Setting
	db.Where("key = ?", "new_key").First(&setting)
	assert.Equal(t, "new_value", setting.Value)

	// Test Update
	payload["value"] = "updated_value"
	body, _ = json.Marshal(payload)

	w = httptest.NewRecorder()
	req, _ = http.NewRequest("POST", "/settings", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	db.Where("key = ?", "new_key").First(&setting)
	assert.Equal(t, "updated_value", setting.Value)
}

func TestSettingsHandler_UpdateSetting_SyncsAdminWhitelist(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupSettingsTestDB(t)

	handler := handlers.NewSettingsHandler(db)
	router := newAdminRouter()
	router.POST("/settings", handler.UpdateSetting)

	payload := map[string]string{
		"key":   "security.admin_whitelist",
		"value": "192.0.2.1/32",
	}
	body, _ := json.Marshal(payload)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/settings", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var cfg models.SecurityConfig
	err := db.Where("name = ?", "default").First(&cfg).Error
	assert.NoError(t, err)
	assert.Equal(t, "192.0.2.1/32", cfg.AdminWhitelist)
}

func TestSettingsHandler_UpdateSetting_EnablesCerberusWhenACLEnabled(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupSettingsTestDB(t)

	handler := handlers.NewSettingsHandler(db)
	router := newAdminRouter()
	router.POST("/settings", handler.UpdateSetting)

	payload := map[string]string{
		"key":   "security.acl.enabled",
		"value": "true",
	}
	body, _ := json.Marshal(payload)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/settings", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var setting models.Setting
	err := db.Where("key = ?", "feature.cerberus.enabled").First(&setting).Error
	assert.NoError(t, err)
	assert.Equal(t, "true", setting.Value)

	var legacySetting models.Setting
	err = db.Where("key = ?", "security.cerberus.enabled").First(&legacySetting).Error
	assert.NoError(t, err)
	assert.Equal(t, "true", legacySetting.Value)

	var aclSetting models.Setting
	err = db.Where("key = ?", "security.acl.enabled").First(&aclSetting).Error
	assert.NoError(t, err)
	assert.Equal(t, "true", aclSetting.Value)

	var cfg models.SecurityConfig
	err = db.Where("name = ?", "default").First(&cfg).Error
	assert.NoError(t, err)
	assert.True(t, cfg.Enabled)
}

func TestSettingsHandler_UpdateSetting_SecurityKeyAppliesConfigSynchronously(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupSettingsTestDB(t)

	mgr := &mockCaddyConfigManager{}
	handler := handlers.NewSettingsHandlerWithDeps(db, mgr, nil, nil, "")
	router := newAdminRouter()
	router.POST("/settings", handler.UpdateSetting)

	payload := map[string]string{
		"key":   "security.waf.enabled",
		"value": "true",
	}
	body, _ := json.Marshal(payload)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/settings", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, 1, mgr.calls)
}

func TestSettingsHandler_UpdateSetting_SecurityKeyApplyFailureReturnsError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupSettingsTestDB(t)

	mgr := &mockCaddyConfigManager{applyFunc: func(context.Context) error {
		return fmt.Errorf("apply failed")
	}}
	handler := handlers.NewSettingsHandlerWithDeps(db, mgr, nil, nil, "")
	router := newAdminRouter()
	router.POST("/settings", handler.UpdateSetting)

	payload := map[string]string{
		"key":   "security.waf.enabled",
		"value": "true",
	}
	body, _ := json.Marshal(payload)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/settings", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Equal(t, 1, mgr.calls)
}

func TestSettingsHandler_UpdateSetting_NonAdminForbidden(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupSettingsTestDB(t)

	handler := handlers.NewSettingsHandler(db)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("role", "user")
		c.Next()
	})
	router.POST("/settings", handler.UpdateSetting)

	payload := map[string]string{"key": "security.waf.enabled", "value": "true"}
	body, _ := json.Marshal(payload)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/settings", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestSettingsHandler_UpdateSetting_InvalidAdminWhitelist(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupSettingsTestDB(t)

	handler := handlers.NewSettingsHandler(db)
	router := newAdminRouter()
	router.POST("/settings", handler.UpdateSetting)

	payload := map[string]string{
		"key":   "security.admin_whitelist",
		"value": "invalid-cidr-without-prefix",
	}
	body, _ := json.Marshal(payload)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/settings", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "Invalid admin_whitelist")
}

func TestSettingsHandler_UpdateSetting_EmptyValueAccepted(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupSettingsTestDB(t)

	handler := handlers.NewSettingsHandler(db)
	router := newAdminRouter()
	router.POST("/settings", handler.UpdateSetting)

	payload := map[string]string{
		"key":   "some.setting",
		"value": "",
	}
	body, _ := json.Marshal(payload)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/settings", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var setting models.Setting
	require.NoError(t, db.Where("key = ?", "some.setting").First(&setting).Error)
	assert.Equal(t, "some.setting", setting.Key)
	assert.Equal(t, "", setting.Value)
}

func TestSettingsHandler_UpdateSetting_MissingKeyRejected(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupSettingsTestDB(t)

	handler := handlers.NewSettingsHandler(db)
	router := newAdminRouter()
	router.POST("/settings", handler.UpdateSetting)

	payload := map[string]string{
		"value": "some-value",
	}
	body, _ := json.Marshal(payload)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/settings", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "Key")
}

func TestSettingsHandler_UpdateSetting_InvalidKeepaliveIdle(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupSettingsTestDB(t)

	handler := handlers.NewSettingsHandler(db)
	router := newAdminRouter()
	router.POST("/settings", handler.UpdateSetting)

	payload := map[string]string{
		"key":   "caddy.keepalive_idle",
		"value": "bad-duration",
	}
	body, _ := json.Marshal(payload)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/settings", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "invalid caddy.keepalive_idle")
}

func TestSettingsHandler_UpdateSetting_ValidKeepaliveCount(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupSettingsTestDB(t)

	handler := handlers.NewSettingsHandler(db)
	router := newAdminRouter()
	router.POST("/settings", handler.UpdateSetting)

	payload := map[string]string{
		"key":      "caddy.keepalive_count",
		"value":    "9",
		"category": "caddy",
		"type":     "number",
	}
	body, _ := json.Marshal(payload)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/settings", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var setting models.Setting
	err := db.Where("key = ?", "caddy.keepalive_count").First(&setting).Error
	assert.NoError(t, err)
	assert.Equal(t, "9", setting.Value)
}

func TestSettingsHandler_UpdateSetting_SecurityKeyInvalidatesCache(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupSettingsTestDB(t)

	mgr := &mockCaddyConfigManager{}
	inv := &mockCacheInvalidator{}
	handler := handlers.NewSettingsHandlerWithDeps(db, mgr, inv, nil, "")
	router := newAdminRouter()
	router.POST("/settings", handler.UpdateSetting)

	payload := map[string]string{
		"key":   "security.rate_limit.enabled",
		"value": "true",
	}
	body, _ := json.Marshal(payload)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/settings", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, 1, inv.calls)
	assert.Equal(t, 1, mgr.calls)
}

func TestSettingsHandler_PatchConfig_InvalidAdminWhitelist(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupSettingsTestDB(t)

	handler := handlers.NewSettingsHandler(db)
	router := newAdminRouter()
	router.PATCH("/config", handler.PatchConfig)

	payload := map[string]any{
		"security": map[string]any{
			"admin_whitelist": "bad-cidr",
		},
	}
	body, _ := json.Marshal(payload)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPatch, "/config", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "Invalid admin_whitelist")
}

func TestSettingsHandler_PatchConfig_InvalidKeepaliveCount(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupSettingsTestDB(t)

	handler := handlers.NewSettingsHandler(db)
	router := newAdminRouter()
	router.PATCH("/config", handler.PatchConfig)

	payload := map[string]any{
		"caddy": map[string]any{
			"keepalive_count": 0,
		},
	}
	body, _ := json.Marshal(payload)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPatch, "/config", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "invalid caddy.keepalive_count")
}

func TestSettingsHandler_PatchConfig_ValidKeepaliveSettings(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupSettingsTestDB(t)

	handler := handlers.NewSettingsHandler(db)
	router := newAdminRouter()
	router.PATCH("/config", handler.PatchConfig)

	payload := map[string]any{
		"caddy": map[string]any{
			"keepalive_idle":  "30s",
			"keepalive_count": 12,
		},
	}
	body, _ := json.Marshal(payload)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPatch, "/config", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var idle models.Setting
	err := db.Where("key = ?", "caddy.keepalive_idle").First(&idle).Error
	assert.NoError(t, err)
	assert.Equal(t, "30s", idle.Value)

	var count models.Setting
	err = db.Where("key = ?", "caddy.keepalive_count").First(&count).Error
	assert.NoError(t, err)
	assert.Equal(t, "12", count.Value)
}

func TestSettingsHandler_PatchConfig_ReloadFailureReturns500(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupSettingsTestDB(t)

	mgr := &mockCaddyConfigManager{applyFunc: func(context.Context) error {
		return fmt.Errorf("reload failed")
	}}
	inv := &mockCacheInvalidator{}
	handler := handlers.NewSettingsHandlerWithDeps(db, mgr, inv, nil, "")
	router := newAdminRouter()
	router.PATCH("/config", handler.PatchConfig)

	payload := map[string]any{
		"security": map[string]any{
			"waf": map[string]any{"enabled": true},
		},
	}
	body, _ := json.Marshal(payload)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPatch, "/config", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Equal(t, 1, inv.calls)
	assert.Equal(t, 1, mgr.calls)
	assert.Contains(t, w.Body.String(), "Failed to reload configuration")
}

func TestSettingsHandler_PatchConfig_SyncsAdminWhitelist(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupSettingsTestDB(t)

	handler := handlers.NewSettingsHandler(db)
	router := newAdminRouter()
	router.Use(func(c *gin.Context) {
		c.Set("role", "admin")
		c.Next()
	})
	router.PATCH("/config", handler.PatchConfig)

	payload := map[string]any{
		"security": map[string]any{
			"admin_whitelist": "203.0.113.0/24",
		},
	}
	body, _ := json.Marshal(payload)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PATCH", "/config", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var cfg models.SecurityConfig
	err := db.Where("name = ?", "default").First(&cfg).Error
	assert.NoError(t, err)
	assert.Equal(t, "203.0.113.0/24", cfg.AdminWhitelist)
}

func TestSettingsHandler_PatchConfig_EnablesCerberusWhenACLEnabled(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupSettingsTestDB(t)

	handler := handlers.NewSettingsHandler(db)
	router := newAdminRouter()
	router.Use(func(c *gin.Context) {
		c.Set("role", "admin")
		c.Next()
	})
	router.PATCH("/config", handler.PatchConfig)

	payload := map[string]any{
		"security": map[string]any{
			"acl": map[string]any{
				"enabled": true,
			},
		},
	}
	body, _ := json.Marshal(payload)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PATCH", "/config", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var setting models.Setting
	err := db.Where("key = ?", "feature.cerberus.enabled").First(&setting).Error
	assert.NoError(t, err)
	assert.Equal(t, "true", setting.Value)

	var cfg models.SecurityConfig
	err = db.Where("name = ?", "default").First(&cfg).Error
	assert.NoError(t, err)
	assert.True(t, cfg.Enabled)
}

func TestSettingsHandler_UpdateSetting_DatabaseError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupSettingsTestDB(t)

	handler := handlers.NewSettingsHandler(db)
	router := newAdminRouter()
	router.POST("/settings", handler.UpdateSetting)

	// Close the database to force an error
	sqlDB, _ := db.DB()
	_ = sqlDB.Close()

	payload := map[string]string{
		"key":   "test_key",
		"value": "test_value",
	}
	body, _ := json.Marshal(payload)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/settings", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Contains(t, response["error"], "Failed to save setting")
}

func TestSettingsHandler_Errors(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupSettingsTestDB(t)

	handler := handlers.NewSettingsHandler(db)
	router := newAdminRouter()
	router.POST("/settings", handler.UpdateSetting)

	// Invalid JSON
	req, _ := http.NewRequest("POST", "/settings", bytes.NewBuffer([]byte("invalid")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	// Value omitted — allowed since binding:"required" was removed; empty string is a valid value
	payload := map[string]string{
		"key": "some_key",
		// value intentionally absent; defaults to empty string
	}
	body, _ := json.Marshal(payload)
	req, _ = http.NewRequest("POST", "/settings", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// Missing key — key is still binding:"required" so this must return 400
	payloadNoKey := map[string]string{
		"value": "some_value",
	}
	bodyNoKey, _ := json.Marshal(payloadNoKey)
	req, _ = http.NewRequest("POST", "/settings", bytes.NewBuffer(bodyNoKey))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ============= SMTP Settings Tests =============

func setupSettingsHandlerWithMail(t *testing.T) (*handlers.SettingsHandler, *gorm.DB) {
	dsn := "file:" + t.Name() + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		panic("failed to connect to test database")
	}
	_ = db.AutoMigrate(&models.Setting{})
	return handlers.NewSettingsHandler(db), db
}

func TestSettingsHandler_GetSMTPConfig(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, db := setupSettingsHandlerWithMail(t)

	// Seed SMTP config
	db.Create(&models.Setting{Key: "smtp_host", Value: "smtp.example.com", Category: "smtp", Type: "string"})
	db.Create(&models.Setting{Key: "smtp_port", Value: "587", Category: "smtp", Type: "number"})
	db.Create(&models.Setting{Key: "smtp_username", Value: "user@example.com", Category: "smtp", Type: "string"})
	db.Create(&models.Setting{Key: "smtp_password", Value: "secret123", Category: "smtp", Type: "string"})
	db.Create(&models.Setting{Key: "smtp_from_address", Value: "noreply@example.com", Category: "smtp", Type: "string"})
	db.Create(&models.Setting{Key: "smtp_encryption", Value: "starttls", Category: "smtp", Type: "string"})

	router := newAdminRouter()
	router.GET("/settings/smtp", handler.GetSMTPConfig)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/settings/smtp", http.NoBody)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, "smtp.example.com", resp["host"])
	assert.Equal(t, float64(587), resp["port"])
	assert.Equal(t, "********", resp["password"]) // Password should be masked
	assert.Equal(t, true, resp["configured"])
}

func TestSettingsHandler_GetSMTPConfig_Empty(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, _ := setupSettingsHandlerWithMail(t)

	router := newAdminRouter()
	router.GET("/settings/smtp", handler.GetSMTPConfig)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/settings/smtp", http.NoBody)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, false, resp["configured"])
}

func TestSettingsHandler_GetSMTPConfig_DatabaseError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, db := setupSettingsHandlerWithMail(t)
	sqlDB, _ := db.DB()
	_ = sqlDB.Close()

	router := newAdminRouter()
	router.GET("/settings/smtp", handler.GetSMTPConfig)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/settings/smtp", http.NoBody)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestSettingsHandler_GetSMTPConfig_NonAdminForbidden(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, _ := setupSettingsHandlerWithMail(t)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("role", "user")
		c.Set("userID", uint(2))
		c.Next()
	})
	router.GET("/api/v1/settings/smtp", handler.GetSMTPConfig)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/settings/smtp", http.NoBody)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestSettingsHandler_UpdateSMTPConfig_NonAdmin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, _ := setupSettingsHandlerWithMail(t)

	router := newAdminRouter()
	router.Use(func(c *gin.Context) {
		c.Set("role", "user")
		c.Next()
	})
	router.PUT("/settings/smtp", handler.UpdateSMTPConfig)

	body := map[string]any{
		"host":         "smtp.example.com",
		"port":         587,
		"from_address": "test@example.com",
		"encryption":   "starttls",
	}
	jsonBody, _ := json.Marshal(body)
	req, _ := http.NewRequest("PUT", "/settings/smtp", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestSettingsHandler_UpdateSMTPConfig_InvalidJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, _ := setupSettingsHandlerWithMail(t)

	router := newAdminRouter()
	router.Use(func(c *gin.Context) {
		c.Set("role", "admin")
		c.Next()
	})
	router.PUT("/settings/smtp", handler.UpdateSMTPConfig)

	req, _ := http.NewRequest("PUT", "/settings/smtp", bytes.NewBufferString("invalid"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestSettingsHandler_UpdateSMTPConfig_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, _ := setupSettingsHandlerWithMail(t)

	router := newAdminRouter()
	router.Use(func(c *gin.Context) {
		c.Set("role", "admin")
		c.Next()
	})
	router.PUT("/settings/smtp", handler.UpdateSMTPConfig)

	body := map[string]any{
		"host":         "smtp.example.com",
		"port":         587,
		"username":     "user@example.com",
		"password":     "password123",
		"from_address": "noreply@example.com",
		"encryption":   "starttls",
	}
	jsonBody, _ := json.Marshal(body)
	req, _ := http.NewRequest("PUT", "/settings/smtp", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestSettingsHandler_UpdateSMTPConfig_KeepExistingPassword(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, db := setupSettingsHandlerWithMail(t)

	// Seed existing password
	db.Create(&models.Setting{Key: "smtp_password", Value: "existingpassword", Category: "smtp", Type: "string"})
	db.Create(&models.Setting{Key: "smtp_host", Value: "old.example.com", Category: "smtp", Type: "string"})
	db.Create(&models.Setting{Key: "smtp_port", Value: "25", Category: "smtp", Type: "number"})
	db.Create(&models.Setting{Key: "smtp_from_address", Value: "old@example.com", Category: "smtp", Type: "string"})
	db.Create(&models.Setting{Key: "smtp_encryption", Value: "none", Category: "smtp", Type: "string"})

	router := newAdminRouter()
	router.Use(func(c *gin.Context) {
		c.Set("role", "admin")
		c.Next()
	})
	router.PUT("/settings/smtp", handler.UpdateSMTPConfig)

	// Send masked password (simulating frontend sending back masked value)
	body := map[string]any{
		"host":         "smtp.example.com",
		"port":         587,
		"password":     "********", // Masked
		"from_address": "noreply@example.com",
		"encryption":   "starttls",
	}
	jsonBody, _ := json.Marshal(body)
	req, _ := http.NewRequest("PUT", "/settings/smtp", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// Verify password was preserved
	var setting models.Setting
	db.Where("key = ?", "smtp_password").First(&setting)
	assert.Equal(t, "existingpassword", setting.Value)
}

func TestSettingsHandler_TestSMTPConfig_NonAdmin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, _ := setupSettingsHandlerWithMail(t)

	router := newAdminRouter()
	router.Use(func(c *gin.Context) {
		c.Set("role", "user")
		c.Next()
	})
	router.POST("/settings/smtp/test", handler.TestSMTPConfig)

	req, _ := http.NewRequest("POST", "/settings/smtp/test", http.NoBody)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestSettingsHandler_TestSMTPConfig_NotConfigured(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, _ := setupSettingsHandlerWithMail(t)

	router := newAdminRouter()
	router.Use(func(c *gin.Context) {
		c.Set("role", "admin")
		c.Next()
	})
	router.POST("/settings/smtp/test", handler.TestSMTPConfig)

	req, _ := http.NewRequest("POST", "/settings/smtp/test", http.NoBody)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	var resp map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, false, resp["success"])
}

func TestSettingsHandler_TestSMTPConfig_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, db := setupSettingsHandlerWithMail(t)

	host, port := startTestSMTPServer(t)

	// Seed SMTP config for local test server.
	db.Create(&models.Setting{Key: "smtp_host", Value: host, Category: "smtp", Type: "string"})
	db.Create(&models.Setting{Key: "smtp_port", Value: fmt.Sprintf("%d", port), Category: "smtp", Type: "number"})
	db.Create(&models.Setting{Key: "smtp_encryption", Value: "none", Category: "smtp", Type: "string"})

	router := newAdminRouter()
	router.Use(func(c *gin.Context) {
		c.Set("role", "admin")
		c.Next()
	})
	router.POST("/settings/smtp/test", handler.TestSMTPConfig)

	req, _ := http.NewRequest("POST", "/settings/smtp/test", http.NoBody)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, true, resp["success"])
}

func TestSettingsHandler_SendTestEmail_NonAdmin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, _ := setupSettingsHandlerWithMail(t)

	router := newAdminRouter()
	router.Use(func(c *gin.Context) {
		c.Set("role", "user")
		c.Next()
	})
	router.POST("/settings/smtp/send-test", handler.SendTestEmail)

	body := map[string]string{"to": "test@example.com"}
	jsonBody, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", "/settings/smtp/send-test", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestSettingsHandler_SendTestEmail_InvalidJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, _ := setupSettingsHandlerWithMail(t)

	router := newAdminRouter()
	router.Use(func(c *gin.Context) {
		c.Set("role", "admin")
		c.Next()
	})
	router.POST("/settings/smtp/send-test", handler.SendTestEmail)

	req, _ := http.NewRequest("POST", "/settings/smtp/send-test", bytes.NewBufferString("invalid"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestSettingsHandler_SendTestEmail_NotConfigured(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, _ := setupSettingsHandlerWithMail(t)

	router := newAdminRouter()
	router.Use(func(c *gin.Context) {
		c.Set("role", "admin")
		c.Next()
	})
	router.POST("/settings/smtp/send-test", handler.SendTestEmail)

	body := map[string]string{"to": "test@example.com"}
	jsonBody, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", "/settings/smtp/send-test", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	var resp map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, false, resp["success"])
}

func TestSettingsHandler_SendTestEmail_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, db := setupSettingsHandlerWithMail(t)

	host, port := startTestSMTPServer(t)

	// Seed SMTP config for local test server.
	db.Create(&models.Setting{Key: "smtp_host", Value: host, Category: "smtp", Type: "string"})
	db.Create(&models.Setting{Key: "smtp_port", Value: fmt.Sprintf("%d", port), Category: "smtp", Type: "number"})
	db.Create(&models.Setting{Key: "smtp_from_address", Value: "noreply@example.com", Category: "smtp", Type: "string"})
	db.Create(&models.Setting{Key: "smtp_encryption", Value: "none", Category: "smtp", Type: "string"})

	router := newAdminRouter()
	router.Use(func(c *gin.Context) {
		c.Set("role", "admin")
		c.Next()
	})
	router.POST("/settings/smtp/send-test", handler.SendTestEmail)

	body := map[string]string{"to": "test@example.com"}
	jsonBody, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", "/settings/smtp/send-test", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, true, resp["success"])
}

func TestMaskPassword(t *testing.T) {
	// Empty password
	assert.Equal(t, "", handlers.MaskPasswordForTest(""))

	// Non-empty password
	assert.Equal(t, "********", handlers.MaskPasswordForTest("secret"))
}

// ============= URL Testing Tests =============

func TestSettingsHandler_ValidatePublicURL_NonAdmin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, _ := setupSettingsHandlerWithMail(t)

	router := newAdminRouter()
	router.Use(func(c *gin.Context) {
		c.Set("role", "user")
		c.Next()
	})
	router.POST("/settings/validate-url", handler.ValidatePublicURL)

	body := map[string]string{"url": "https://example.com"}
	jsonBody, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", "/settings/validate-url", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestSettingsHandler_ValidatePublicURL_InvalidFormat(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, _ := setupSettingsHandlerWithMail(t)

	router := newAdminRouter()
	router.Use(func(c *gin.Context) {
		c.Set("role", "admin")
		c.Next()
	})
	router.POST("/settings/validate-url", handler.ValidatePublicURL)

	testCases := []struct {
		name string
		url  string
	}{
		{"Missing scheme", "example.com"},
		{"Invalid scheme", "ftp://example.com"},
		{"URL with path", "https://example.com/path"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			body := map[string]string{"url": tc.url}
			jsonBody, _ := json.Marshal(body)
			req, _ := http.NewRequest("POST", "/settings/validate-url", bytes.NewBuffer(jsonBody))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusBadRequest, w.Code)
			var resp map[string]any
			_ = json.Unmarshal(w.Body.Bytes(), &resp)
			assert.Equal(t, false, resp["valid"])
		})
	}
}

func TestSettingsHandler_ValidatePublicURL_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, _ := setupSettingsHandlerWithMail(t)

	router := newAdminRouter()
	router.Use(func(c *gin.Context) {
		c.Set("role", "admin")
		c.Next()
	})
	router.POST("/settings/validate-url", handler.ValidatePublicURL)

	testCases := []struct {
		name     string
		url      string
		expected string
	}{
		{"HTTPS URL", "https://example.com", "https://example.com"},
		{"HTTP URL", "http://example.com", "http://example.com"},
		{"URL with port", "https://example.com:8080", "https://example.com:8080"},
		{"URL with trailing slash", "https://example.com/", "https://example.com"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			body := map[string]string{"url": tc.url}
			jsonBody, _ := json.Marshal(body)
			req, _ := http.NewRequest("POST", "/settings/validate-url", bytes.NewBuffer(jsonBody))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)
			var resp map[string]any
			_ = json.Unmarshal(w.Body.Bytes(), &resp)
			assert.Equal(t, true, resp["valid"])
			assert.Equal(t, tc.expected, resp["normalized"])
		})
	}
}

func TestSettingsHandler_TestPublicURL_NonAdmin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, _ := setupSettingsHandlerWithMail(t)

	router := newAdminRouter()
	router.Use(func(c *gin.Context) {
		c.Set("role", "user")
		c.Next()
	})
	router.POST("/settings/test-url", handler.TestPublicURL)

	body := map[string]string{"url": "https://example.com"}
	jsonBody, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", "/settings/test-url", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestSettingsHandler_TestPublicURL_NoRole(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, _ := setupSettingsHandlerWithMail(t)

	router := gin.New()
	// No role set in context
	router.POST("/settings/test-url", handler.TestPublicURL)

	body := map[string]string{"url": "https://example.com"}
	jsonBody, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", "/settings/test-url", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestSettingsHandler_TestPublicURL_InvalidJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, _ := setupSettingsHandlerWithMail(t)

	router := newAdminRouter()
	router.Use(func(c *gin.Context) {
		c.Set("role", "admin")
		c.Next()
	})
	router.POST("/settings/test-url", handler.TestPublicURL)

	req, _ := http.NewRequest("POST", "/settings/test-url", bytes.NewBufferString("invalid json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestSettingsHandler_TestPublicURL_InvalidURL(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, _ := setupSettingsHandlerWithMail(t)

	router := newAdminRouter()
	router.Use(func(c *gin.Context) {
		c.Set("role", "admin")
		c.Next()
	})
	router.POST("/settings/test-url", handler.TestPublicURL)

	body := map[string]string{"url": "not-a-valid-url"}
	jsonBody, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", "/settings/test-url", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	var resp map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	// BadRequest responses only have 'error' field, not 'reachable'
	assert.Contains(t, resp["error"].(string), "parse")
}

func TestSettingsHandler_TestPublicURL_PrivateIPBlocked(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, _ := setupSettingsHandlerWithMail(t)

	router := newAdminRouter()
	router.Use(func(c *gin.Context) {
		c.Set("role", "admin")
		c.Next()
	})
	router.POST("/settings/test-url", handler.TestPublicURL)

	// Test various private IPs that should be blocked
	testCases := []struct {
		name string
		url  string
	}{
		{"localhost", "http://localhost"},
		{"127.0.0.1", "http://127.0.0.1"},
		{"Private 10.x", "http://10.0.0.1"},
		{"Private 192.168.x", "http://192.168.1.1"},
		{"AWS metadata", "http://169.254.169.254"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			body := map[string]string{"url": tc.url}
			jsonBody, _ := json.Marshal(body)
			req, _ := http.NewRequest("POST", "/settings/test-url", bytes.NewBuffer(jsonBody))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code) // Returns 200 but with reachable=false
			var resp map[string]any
			_ = json.Unmarshal(w.Body.Bytes(), &resp)
			assert.Equal(t, false, resp["reachable"])
			// Verify error message contains relevant security text
			errorMsg := resp["error"].(string)
			assert.True(t,
				contains(errorMsg, "private ip") || contains(errorMsg, "metadata") || contains(errorMsg, "blocked"),
				"Expected security error message, got: %s", errorMsg)
		})
	}
}

// Helper function for case-insensitive contains
func contains(s, substr string) bool {
	return bytes.Contains([]byte(s), []byte(substr))
}

func TestSettingsHandler_TestPublicURL_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, _ := setupSettingsHandlerWithMail(t)

	// NOTE: Using a real public URL instead of httptest.NewServer() because
	// SSRF protection (correctly) blocks localhost/127.0.0.1.
	// Using example.com which is guaranteed to be reachable and is designed for testing
	// Alternative: Refactor handler to accept injectable URL validator (future improvement).
	publicTestURL := "https://example.com"

	router := newAdminRouter()
	router.Use(func(c *gin.Context) {
		c.Set("role", "admin")
		c.Next()
	})
	router.POST("/settings/test-url", handler.TestPublicURL)

	body := map[string]string{"url": publicTestURL}
	jsonBody, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", "/settings/test-url", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &resp)

	// The test verifies the handler works with a real public URL
	assert.Equal(t, true, resp["reachable"], "example.com should be reachable")
	assert.NotNil(t, resp["latency"])
	// Note: message field is no longer included in response
}

func TestSettingsHandler_TestPublicURL_DNSFailure(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, _ := setupSettingsHandlerWithMail(t)

	router := newAdminRouter()
	router.Use(func(c *gin.Context) {
		c.Set("role", "admin")
		c.Next()
	})
	router.POST("/settings/test-url", handler.TestPublicURL)

	body := map[string]string{"url": "http://nonexistent-domain-12345.invalid"}
	jsonBody, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", "/settings/test-url", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code) // Returns 200 but with reachable=false
	var resp map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, false, resp["reachable"])
	// DNS errors contain "dns" or "resolution" keywords (case-insensitive)
	errorMsg := resp["error"].(string)
	assert.True(t,
		contains(errorMsg, "dns") || contains(errorMsg, "resolution"),
		"Expected DNS error message, got: %s", errorMsg)
}

func TestSettingsHandler_TestPublicURL_ConnectivityError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, _ := setupSettingsHandlerWithMail(t)

	router := newAdminRouter()
	router.Use(func(c *gin.Context) {
		c.Set("role", "admin")
		c.Next()
	})
	router.POST("/settings/test-url", handler.TestPublicURL)

	// 192.0.2.0/24 is reserved for documentation/testing and is not considered private by
	// network.IsPrivateIP(). Using a closed port should trigger a deterministic connect error
	// after passing SSRF validation.
	body := map[string]string{"url": "http://192.0.2.1:1"}
	jsonBody, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", "/settings/test-url", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, false, resp["reachable"])
	_, ok := resp["error"].(string)
	assert.True(t, ok)
}

// ============= SSRF Protection Tests =============

func TestSettingsHandler_TestPublicURL_SSRFProtection(t *testing.T) {
	tests := []struct {
		name              string
		url               string
		expectedStatus    int
		expectedReachable bool
		errorContains     string
	}{
		{
			name:              "blocks RFC 1918 - 10.x",
			url:               "http://10.0.0.1",
			expectedStatus:    http.StatusOK,
			expectedReachable: false,
			errorContains:     "private",
		},
		{
			name:              "blocks RFC 1918 - 192.168.x",
			url:               "http://192.168.1.1",
			expectedStatus:    http.StatusOK,
			expectedReachable: false,
			errorContains:     "private",
		},
		{
			name:              "blocks RFC 1918 - 172.16.x",
			url:               "http://172.16.0.1",
			expectedStatus:    http.StatusOK,
			expectedReachable: false,
			errorContains:     "private",
		},
		{
			name:              "blocks localhost",
			url:               "http://localhost",
			expectedStatus:    http.StatusOK,
			expectedReachable: false,
			errorContains:     "private",
		},
		{
			name:              "blocks 127.0.0.1",
			url:               "http://127.0.0.1",
			expectedStatus:    http.StatusOK,
			expectedReachable: false,
			errorContains:     "private",
		},
		{
			name:              "blocks cloud metadata",
			url:               "http://169.254.169.254",
			expectedStatus:    http.StatusOK,
			expectedReachable: false,
			errorContains:     "cloud metadata",
		},
		{
			name:              "blocks link-local",
			url:               "http://169.254.1.1",
			expectedStatus:    http.StatusOK,
			expectedReachable: false,
			errorContains:     "private",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			handler, _ := setupSettingsHandlerWithMail(t)

			router := newAdminRouter()
			router.Use(func(c *gin.Context) {
				c.Set("role", "admin")
				c.Next()
			})
			router.POST("/settings/test-url", handler.TestPublicURL)

			body := map[string]string{"url": tt.url}
			jsonBody, _ := json.Marshal(body)
			req, _ := http.NewRequest("POST", "/settings/test-url", bytes.NewBuffer(jsonBody))
			req.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			var resp map[string]any
			err := json.Unmarshal(w.Body.Bytes(), &resp)
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedReachable, resp["reachable"])

			if tt.errorContains != "" {
				errorMsg, ok := resp["error"].(string)
				assert.True(t, ok, "error field should be a string")
				assert.Contains(t, errorMsg, tt.errorContains)
			}
		})
	}
}

func TestSettingsHandler_TestPublicURL_EmbeddedCredentials(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, _ := setupSettingsHandlerWithMail(t)

	router := newAdminRouter()
	router.Use(func(c *gin.Context) {
		c.Set("role", "admin")
		c.Next()
	})
	router.POST("/settings/test-url", handler.TestPublicURL)

	// Test URL with embedded credentials (parser differential attack)
	body := map[string]string{"url": "http://evil.com@127.0.0.1/"}
	jsonBody, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", "/settings/test-url", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	assert.False(t, resp["reachable"].(bool))
	assert.Contains(t, resp["error"].(string), "credentials")
}

func TestSettingsHandler_TestPublicURL_EmptyURL(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, _ := setupSettingsHandlerWithMail(t)

	router := newAdminRouter()
	router.Use(func(c *gin.Context) {
		c.Set("role", "admin")
		c.Next()
	})
	router.POST("/settings/test-url", handler.TestPublicURL)

	tests := []struct {
		name    string
		payload string
	}{
		{"empty string", `{"url": ""}`},
		{"missing field", `{}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, _ := http.NewRequest("POST", "/settings/test-url", bytes.NewBufferString(tt.payload))
			req.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusBadRequest, w.Code)
		})
	}
}

func TestSettingsHandler_TestPublicURL_InvalidScheme(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, _ := setupSettingsHandlerWithMail(t)

	router := newAdminRouter()
	router.Use(func(c *gin.Context) {
		c.Set("role", "admin")
		c.Next()
	})
	router.POST("/settings/test-url", handler.TestPublicURL)

	tests := []struct {
		name string
		url  string
	}{
		{"ftp scheme", "ftp://example.com"},
		{"file scheme", "file:///etc/passwd"},
		{"javascript scheme", "javascript:alert(1)"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := map[string]string{"url": tt.url}
			jsonBody, _ := json.Marshal(body)
			req, _ := http.NewRequest("POST", "/settings/test-url", bytes.NewBuffer(jsonBody))
			req.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusBadRequest, w.Code)
			var resp map[string]any
			err := json.Unmarshal(w.Body.Bytes(), &resp)
			require.NoError(t, err, "Failed to unmarshal response")
			// BadRequest responses only have 'error' field, not 'reachable'
			assert.Contains(t, resp["error"].(string), "parse")
		})
	}
}

func TestSettingsHandler_ValidatePublicURL_InvalidJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, _ := setupSettingsHandlerWithMail(t)

	router := newAdminRouter()
	router.Use(func(c *gin.Context) {
		c.Set("role", "admin")
		c.Next()
	})
	router.POST("/settings/validate-url", handler.ValidatePublicURL)

	req, _ := http.NewRequest("POST", "/settings/validate-url", bytes.NewBufferString("not-json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestSettingsHandler_ValidatePublicURL_URLWithWarning(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, _ := setupSettingsHandlerWithMail(t)

	router := newAdminRouter()
	router.Use(func(c *gin.Context) {
		c.Set("role", "admin")
		c.Next()
	})
	router.POST("/settings/validate-url", handler.ValidatePublicURL)

	// URL with HTTP scheme may generate a warning
	body := map[string]string{"url": "http://example.com"}
	jsonBody, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", "/settings/validate-url", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err, "Failed to unmarshal response")
	assert.Equal(t, true, resp["valid"])
	// May have a warning about HTTP vs HTTPS
}

func TestSettingsHandler_UpdateSMTPConfig_DatabaseError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, db := setupSettingsHandlerWithMail(t)

	// Close the database to force an error
	sqlDB, _ := db.DB()
	_ = sqlDB.Close()

	router := newAdminRouter()
	router.Use(func(c *gin.Context) {
		c.Set("role", "admin")
		c.Next()
	})
	router.PUT("/settings/smtp", handler.UpdateSMTPConfig)

	// Include password (not masked) to skip GetSMTPConfig path which would also fail
	body := map[string]any{
		"host":         "smtp.example.com",
		"port":         587,
		"from_address": "test@example.com",
		"encryption":   "starttls",
		"password":     "test-password", // Provide password to skip GetSMTPConfig call
	}
	jsonBody, _ := json.Marshal(body)
	req, _ := http.NewRequest("PUT", "/settings/smtp", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "Failed to save")
}

func TestSettingsHandler_TestPublicURL_IPv6LocalhostBlocked(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, _ := setupSettingsHandlerWithMail(t)

	router := newAdminRouter()
	router.Use(func(c *gin.Context) {
		c.Set("role", "admin")
		c.Next()
	})
	router.POST("/settings/test-url", handler.TestPublicURL)

	// Test IPv6 loopback address
	body := map[string]string{"url": "http://[::1]"}
	jsonBody, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", "/settings/test-url", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err, "Failed to unmarshal response")
	assert.False(t, resp["reachable"].(bool))
	// IPv6 loopback should be blocked
}

// TestUpdateSetting_EmptyValueIsAccepted guards the PR-1 fix: Value must NOT carry
// binding:"required". Gin treats "" as missing for string fields and returns 400 if
// the tag is present. Re-adding the tag would silently regress the CrowdSec enable
// flow (which sends value="" to clear the setting).
func TestUpdateSetting_EmptyValueIsAccepted(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupSettingsTestDB(t)

	handler := handlers.NewSettingsHandler(db)
	router := newAdminRouter()
	router.POST("/settings", handler.UpdateSetting)

	body := `{"key":"security.crowdsec.enabled","value":""}`
	req, _ := http.NewRequest(http.MethodPost, "/settings", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code, "empty Value must not trigger a 400 validation error")

	var s models.Setting
	require.NoError(t, db.Where("key = ?", "security.crowdsec.enabled").First(&s).Error)
	assert.Equal(t, "", s.Value)
}

// TestUpdateSetting_MissingKeyRejected ensures binding:"required" was only removed
// from Value and not accidentally also from Key. A request with no "key" field must
// still return 400.
func TestUpdateSetting_MissingKeyRejected(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupSettingsTestDB(t)

	handler := handlers.NewSettingsHandler(db)
	router := newAdminRouter()
	router.POST("/settings", handler.UpdateSetting)

	body := `{"value":"true"}`
	req, _ := http.NewRequest(http.MethodPost, "/settings", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}
