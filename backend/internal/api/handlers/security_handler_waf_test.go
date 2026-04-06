package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/Wikid82/charon/backend/internal/models"
)

// Tests for GetWAFExclusions handler
func TestSecurityHandler_GetWAFExclusions_Empty(t *testing.T) {
	db := setupTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.SecurityConfig{}))

	handler := NewSecurityHandler(config.SecurityConfig{}, db, nil)
	router := gin.New()
	router.GET("/security/waf/exclusions", handler.GetWAFExclusions)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/security/waf/exclusions", http.NoBody)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	exclusions := resp["exclusions"].([]any)
	assert.Len(t, exclusions, 0)
}

func TestSecurityHandler_GetWAFExclusions_WithExclusions(t *testing.T) {
	db := setupTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.SecurityConfig{}))

	// Create config with exclusions
	exclusionsJSON := `[{"rule_id":942100,"description":"SQL Injection rule"},{"rule_id":941100,"target":"ARGS:password"}]`
	cfg := models.SecurityConfig{Name: "default", WAFExclusions: exclusionsJSON}
	db.Create(&cfg)

	handler := NewSecurityHandler(config.SecurityConfig{}, db, nil)
	router := gin.New()
	router.GET("/security/waf/exclusions", handler.GetWAFExclusions)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/security/waf/exclusions", http.NoBody)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	exclusions := resp["exclusions"].([]any)
	assert.Len(t, exclusions, 2)

	// Verify first exclusion
	first := exclusions[0].(map[string]any)
	assert.Equal(t, float64(942100), first["rule_id"])
	assert.Equal(t, "SQL Injection rule", first["description"])
}

func TestSecurityHandler_GetWAFExclusions_InvalidJSON(t *testing.T) {
	db := setupTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.SecurityConfig{}))

	// Create config with invalid JSON
	cfg := models.SecurityConfig{Name: "default", WAFExclusions: "invalid json"}
	db.Create(&cfg)

	handler := NewSecurityHandler(config.SecurityConfig{}, db, nil)
	router := gin.New()
	router.GET("/security/waf/exclusions", handler.GetWAFExclusions)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/security/waf/exclusions", http.NoBody)
	router.ServeHTTP(w, req)

	// Should return empty array on parse failure
	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	exclusions := resp["exclusions"].([]any)
	assert.Len(t, exclusions, 0)
}

// Tests for AddWAFExclusion handler
func TestSecurityHandler_AddWAFExclusion_Success(t *testing.T) {
	db := setupTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.SecurityConfig{}, &models.SecurityAudit{}))

	handler := NewSecurityHandler(config.SecurityConfig{}, db, nil)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("role", "admin")
		c.Next()
	})
	router.POST("/security/waf/exclusions", handler.AddWAFExclusion)

	payload := map[string]any{
		"rule_id":     942100,
		"description": "SQL Injection false positive",
	}
	body, _ := json.Marshal(payload)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/security/waf/exclusions", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	exclusion := resp["exclusion"].(map[string]any)
	assert.Equal(t, float64(942100), exclusion["rule_id"])
	assert.Equal(t, "SQL Injection false positive", exclusion["description"])
}

func TestSecurityHandler_AddWAFExclusion_WithTarget(t *testing.T) {
	db := setupTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.SecurityConfig{}, &models.SecurityAudit{}))

	handler := NewSecurityHandler(config.SecurityConfig{}, db, nil)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("role", "admin")
		c.Next()
	})
	router.POST("/security/waf/exclusions", handler.AddWAFExclusion)

	payload := map[string]any{
		"rule_id":     942100,
		"target":      "ARGS:password",
		"description": "Skip password field for SQL injection",
	}
	body, _ := json.Marshal(payload)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/security/waf/exclusions", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	exclusion := resp["exclusion"].(map[string]any)
	assert.Equal(t, "ARGS:password", exclusion["target"])
}

func TestSecurityHandler_AddWAFExclusion_ToExistingConfig(t *testing.T) {
	db := setupTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.SecurityConfig{}, &models.SecurityAudit{}))

	// Create config with existing exclusion
	existingExclusions := `[{"rule_id":941100,"description":"XSS rule"}]`
	cfg := models.SecurityConfig{Name: "default", WAFExclusions: existingExclusions}
	db.Create(&cfg)

	handler := NewSecurityHandler(config.SecurityConfig{}, db, nil)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("role", "admin")
		c.Next()
	})
	router.POST("/security/waf/exclusions", handler.AddWAFExclusion)
	router.GET("/security/waf/exclusions", handler.GetWAFExclusions)

	// Add new exclusion
	payload := map[string]any{
		"rule_id":     942100,
		"description": "SQL Injection rule",
	}
	body, _ := json.Marshal(payload)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/security/waf/exclusions", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// Verify both exclusions exist
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/security/waf/exclusions", http.NoBody)
	router.ServeHTTP(w, req)

	var resp map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	exclusions := resp["exclusions"].([]any)
	assert.Len(t, exclusions, 2)
}

func TestSecurityHandler_AddWAFExclusion_Duplicate(t *testing.T) {
	db := setupTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.SecurityConfig{}, &models.SecurityAudit{}))

	// Create config with existing exclusion
	existingExclusions := `[{"rule_id":942100,"description":"SQL Injection rule"}]`
	cfg := models.SecurityConfig{Name: "default", WAFExclusions: existingExclusions}
	db.Create(&cfg)

	handler := NewSecurityHandler(config.SecurityConfig{}, db, nil)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("role", "admin")
		c.Next()
	})
	router.POST("/security/waf/exclusions", handler.AddWAFExclusion)

	// Try to add duplicate
	payload := map[string]any{
		"rule_id":     942100,
		"description": "Another description",
	}
	body, _ := json.Marshal(payload)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/security/waf/exclusions", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusConflict, w.Code)
}

func TestSecurityHandler_AddWAFExclusion_DuplicateWithDifferentTarget(t *testing.T) {
	db := setupTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.SecurityConfig{}, &models.SecurityAudit{}))

	// Create config with existing exclusion (no target)
	existingExclusions := `[{"rule_id":942100}]`
	cfg := models.SecurityConfig{Name: "default", WAFExclusions: existingExclusions}
	db.Create(&cfg)

	handler := NewSecurityHandler(config.SecurityConfig{}, db, nil)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("role", "admin")
		c.Next()
	})
	router.POST("/security/waf/exclusions", handler.AddWAFExclusion)

	// Add same rule_id with different target - should succeed
	payload := map[string]any{
		"rule_id": 942100,
		"target":  "ARGS:password",
	}
	body, _ := json.Marshal(payload)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/security/waf/exclusions", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestSecurityHandler_AddWAFExclusion_MissingRuleID(t *testing.T) {
	db := setupTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.SecurityConfig{}))

	handler := NewSecurityHandler(config.SecurityConfig{}, db, nil)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("role", "admin")
		c.Next()
	})
	router.POST("/security/waf/exclusions", handler.AddWAFExclusion)

	payload := map[string]any{
		"description": "Missing rule_id",
	}
	body, _ := json.Marshal(payload)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/security/waf/exclusions", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestSecurityHandler_AddWAFExclusion_InvalidRuleID(t *testing.T) {
	db := setupTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.SecurityConfig{}))

	handler := NewSecurityHandler(config.SecurityConfig{}, db, nil)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("role", "admin")
		c.Next()
	})
	router.POST("/security/waf/exclusions", handler.AddWAFExclusion)

	// Zero rule_id
	payload := map[string]any{
		"rule_id": 0,
	}
	body, _ := json.Marshal(payload)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/security/waf/exclusions", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestSecurityHandler_AddWAFExclusion_NegativeRuleID(t *testing.T) {
	db := setupTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.SecurityConfig{}))

	handler := NewSecurityHandler(config.SecurityConfig{}, db, nil)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("role", "admin")
		c.Next()
	})
	router.POST("/security/waf/exclusions", handler.AddWAFExclusion)

	payload := map[string]any{
		"rule_id": -1,
	}
	body, _ := json.Marshal(payload)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/security/waf/exclusions", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestSecurityHandler_AddWAFExclusion_InvalidPayload(t *testing.T) {
	db := setupTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.SecurityConfig{}))

	handler := NewSecurityHandler(config.SecurityConfig{}, db, nil)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("role", "admin")
		c.Next()
	})
	router.POST("/security/waf/exclusions", handler.AddWAFExclusion)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/security/waf/exclusions", strings.NewReader("invalid json"))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// Tests for DeleteWAFExclusion handler
func TestSecurityHandler_DeleteWAFExclusion_Success(t *testing.T) {
	db := setupTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.SecurityConfig{}, &models.SecurityAudit{}))

	// Create config with exclusions
	exclusionsJSON := `[{"rule_id":942100},{"rule_id":941100}]`
	cfg := models.SecurityConfig{Name: "default", WAFExclusions: exclusionsJSON}
	db.Create(&cfg)

	handler := NewSecurityHandler(config.SecurityConfig{}, db, nil)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("role", "admin")
		c.Next()
	})
	router.DELETE("/security/waf/exclusions/:rule_id", handler.DeleteWAFExclusion)
	router.GET("/security/waf/exclusions", handler.GetWAFExclusions)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/security/waf/exclusions/942100", http.NoBody)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	assert.True(t, resp["deleted"].(bool))

	// Verify only one exclusion remains
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/security/waf/exclusions", http.NoBody)
	router.ServeHTTP(w, req)

	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	exclusions := resp["exclusions"].([]any)
	assert.Len(t, exclusions, 1)
	first := exclusions[0].(map[string]any)
	assert.Equal(t, float64(941100), first["rule_id"])
}

func TestSecurityHandler_DeleteWAFExclusion_WithTarget(t *testing.T) {
	db := setupTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.SecurityConfig{}, &models.SecurityAudit{}))

	// Create config with targeted exclusion
	exclusionsJSON := `[{"rule_id":942100,"target":"ARGS:password"},{"rule_id":942100}]`
	cfg := models.SecurityConfig{Name: "default", WAFExclusions: exclusionsJSON}
	db.Create(&cfg)

	handler := NewSecurityHandler(config.SecurityConfig{}, db, nil)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("role", "admin")
		c.Next()
	})
	router.DELETE("/security/waf/exclusions/:rule_id", handler.DeleteWAFExclusion)
	router.GET("/security/waf/exclusions", handler.GetWAFExclusions)

	// Delete exclusion with target
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/security/waf/exclusions/942100?target=ARGS:password", http.NoBody)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// Verify only the non-targeted exclusion remains
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/security/waf/exclusions", http.NoBody)
	router.ServeHTTP(w, req)

	var resp map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	exclusions := resp["exclusions"].([]any)
	assert.Len(t, exclusions, 1)
	first := exclusions[0].(map[string]any)
	assert.Equal(t, float64(942100), first["rule_id"])
	assert.Empty(t, first["target"])
}

func TestSecurityHandler_DeleteWAFExclusion_NotFound(t *testing.T) {
	db := setupTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.SecurityConfig{}))

	// Create config with exclusions
	exclusionsJSON := `[{"rule_id":942100}]`
	cfg := models.SecurityConfig{Name: "default", WAFExclusions: exclusionsJSON}
	db.Create(&cfg)

	handler := NewSecurityHandler(config.SecurityConfig{}, db, nil)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("role", "admin")
		c.Next()
	})
	router.DELETE("/security/waf/exclusions/:rule_id", handler.DeleteWAFExclusion)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/security/waf/exclusions/999999", http.NoBody)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestSecurityHandler_DeleteWAFExclusion_NoConfig(t *testing.T) {
	db := setupTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.SecurityConfig{}))

	handler := NewSecurityHandler(config.SecurityConfig{}, db, nil)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("role", "admin")
		c.Next()
	})
	router.DELETE("/security/waf/exclusions/:rule_id", handler.DeleteWAFExclusion)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/security/waf/exclusions/942100", http.NoBody)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestSecurityHandler_DeleteWAFExclusion_InvalidRuleID(t *testing.T) {
	db := setupTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.SecurityConfig{}))

	handler := NewSecurityHandler(config.SecurityConfig{}, db, nil)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("role", "admin")
		c.Next()
	})
	router.DELETE("/security/waf/exclusions/:rule_id", handler.DeleteWAFExclusion)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/security/waf/exclusions/invalid", http.NoBody)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestSecurityHandler_DeleteWAFExclusion_ZeroRuleID(t *testing.T) {
	db := setupTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.SecurityConfig{}))

	handler := NewSecurityHandler(config.SecurityConfig{}, db, nil)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("role", "admin")
		c.Next()
	})
	router.DELETE("/security/waf/exclusions/:rule_id", handler.DeleteWAFExclusion)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/security/waf/exclusions/0", http.NoBody)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestSecurityHandler_DeleteWAFExclusion_NegativeRuleID(t *testing.T) {
	db := setupTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.SecurityConfig{}))

	handler := NewSecurityHandler(config.SecurityConfig{}, db, nil)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("role", "admin")
		c.Next()
	})
	router.DELETE("/security/waf/exclusions/:rule_id", handler.DeleteWAFExclusion)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/security/waf/exclusions/-1", http.NoBody)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// Integration test: Full WAF exclusion workflow
func TestSecurityHandler_WAFExclusion_FullWorkflow(t *testing.T) {

	// Create a temporary file-based SQLite database for complete isolation
	// This avoids all the shared memory locking issues with in-memory databases
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, fmt.Sprintf("waf_test_%d.db", time.Now().UnixNano()))
	dsn := fmt.Sprintf("%s?_journal_mode=WAL&_busy_timeout=10000", dbPath)
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err, "failed to open test database")

	// Ensure cleanup
	t.Cleanup(func() {
		sqlDB, _ := db.DB()
		if sqlDB != nil {
			_ = sqlDB.Close()
		}
		_ = os.Remove(dbPath)
		_ = os.Remove(dbPath + "-wal")
		_ = os.Remove(dbPath + "-shm")
	})

	// Migrate the required models
	require.NoError(t, db.AutoMigrate(&models.SecurityConfig{}, &models.SecurityAudit{}))

	handler := NewSecurityHandler(config.SecurityConfig{}, db, nil)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("role", "admin")
		c.Next()
	})
	router.GET("/security/waf/exclusions", handler.GetWAFExclusions)
	router.POST("/security/waf/exclusions", handler.AddWAFExclusion)
	router.DELETE("/security/waf/exclusions/:rule_id", handler.DeleteWAFExclusion)

	// Step 1: Start with empty exclusions
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/security/waf/exclusions", http.NoBody)
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code, "Step 1: GET should return 200")
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp), "Step 1: response should be valid JSON")
	exclusions, ok := resp["exclusions"].([]any)
	require.True(t, ok, "Step 1: exclusions should be an array")
	require.Len(t, exclusions, 0, "Step 1: should start with 0 exclusions")

	// Step 2: Add first exclusion (full rule removal)
	payload := map[string]any{
		"rule_id":     942100,
		"description": "SQL Injection false positive",
	}
	body, _ := json.Marshal(payload)
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("POST", "/security/waf/exclusions", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code, "Step 2: POST first exclusion should return 200, got: %s", w.Body.String())

	// Step 3: Add second exclusion (targeted)
	payload = map[string]any{
		"rule_id":     941100,
		"target":      "ARGS:content",
		"description": "XSS false positive in content field",
	}
	body, _ = json.Marshal(payload)
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("POST", "/security/waf/exclusions", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code, "Step 3: POST second exclusion should return 200, got: %s", w.Body.String())

	// Step 4: Verify both exclusions exist
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/security/waf/exclusions", http.NoBody)
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code, "Step 4: GET should return 200")
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp), "Step 4: response should be valid JSON")
	exclusions, ok = resp["exclusions"].([]any)
	require.True(t, ok, "Step 4: exclusions should be an array")
	require.Len(t, exclusions, 2, "Step 4: should have 2 exclusions after adding 2")

	// Step 5: Delete first exclusion
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("DELETE", "/security/waf/exclusions/942100", http.NoBody)
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code, "Step 5: DELETE should return 200, got: %s", w.Body.String())

	// Step 6: Verify only second exclusion remains
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/security/waf/exclusions", http.NoBody)
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code, "Step 6: GET should return 200")
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp), "Step 6: response should be valid JSON")
	exclusions, ok = resp["exclusions"].([]any)
	require.True(t, ok, "Step 6: exclusions should be an array")
	require.Len(t, exclusions, 1, "Step 6: should have 1 exclusion after deleting 1")
	first := exclusions[0].(map[string]any)
	assert.Equal(t, float64(941100), first["rule_id"])
	assert.Equal(t, "ARGS:content", first["target"])
}

// Test WAFDisabled field on ProxyHost
func TestProxyHost_WAFDisabled_DefaultFalse(t *testing.T) {
	db := setupTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.ProxyHost{}))

	host := models.ProxyHost{
		UUID:        "test-uuid",
		DomainNames: "example.com",
		ForwardHost: "backend",
		ForwardPort: 8080,
		Enabled:     true,
	}
	db.Create(&host)

	var retrieved models.ProxyHost
	db.First(&retrieved, host.ID)

	assert.False(t, retrieved.WAFDisabled, "WAFDisabled should default to false")
}

func TestProxyHost_WAFDisabled_SetTrue(t *testing.T) {
	db := setupTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.ProxyHost{}))

	host := models.ProxyHost{
		UUID:        "test-uuid",
		DomainNames: "example.com",
		ForwardHost: "backend",
		ForwardPort: 8080,
		Enabled:     true,
		WAFDisabled: true,
	}
	db.Create(&host)

	var retrieved models.ProxyHost
	db.First(&retrieved, host.ID)

	assert.True(t, retrieved.WAFDisabled, "WAFDisabled should be true when set")
}

// Test WAFParanoiaLevel field on SecurityConfig
func TestSecurityConfig_WAFParanoiaLevel_Default(t *testing.T) {
	db := setupTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.SecurityConfig{}))

	cfg := models.SecurityConfig{
		Name:    "default",
		WAFMode: "block",
	}
	db.Create(&cfg)

	var retrieved models.SecurityConfig
	db.First(&retrieved, cfg.ID)

	// GORM default is 1
	assert.Equal(t, 1, retrieved.WAFParanoiaLevel, "WAFParanoiaLevel should default to 1")
}

func TestSecurityConfig_WAFParanoiaLevel_CustomValue(t *testing.T) {
	db := setupTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.SecurityConfig{}))

	cfg := models.SecurityConfig{
		Name:             "default",
		WAFMode:          "block",
		WAFParanoiaLevel: 3,
	}
	db.Create(&cfg)

	var retrieved models.SecurityConfig
	db.First(&retrieved, cfg.ID)

	assert.Equal(t, 3, retrieved.WAFParanoiaLevel, "WAFParanoiaLevel should be 3")
}

// Test WAFExclusions field on SecurityConfig
func TestSecurityConfig_WAFExclusions_Empty(t *testing.T) {
	db := setupTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.SecurityConfig{}))

	cfg := models.SecurityConfig{
		Name:    "default",
		WAFMode: "block",
	}
	db.Create(&cfg)

	var retrieved models.SecurityConfig
	db.First(&retrieved, cfg.ID)

	assert.Empty(t, retrieved.WAFExclusions, "WAFExclusions should be empty by default")
}

func TestSecurityConfig_WAFExclusions_JSONArray(t *testing.T) {
	db := setupTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.SecurityConfig{}))

	exclusions := `[{"rule_id":942100,"target":"ARGS:password","description":"Skip password field"}]`
	cfg := models.SecurityConfig{
		Name:          "default",
		WAFMode:       "block",
		WAFExclusions: exclusions,
	}
	db.Create(&cfg)

	var retrieved models.SecurityConfig
	db.First(&retrieved, cfg.ID)

	assert.Equal(t, exclusions, retrieved.WAFExclusions)

	// Verify it can be parsed
	var parsed []map[string]any
	err := json.Unmarshal([]byte(retrieved.WAFExclusions), &parsed)
	require.NoError(t, err)
	assert.Len(t, parsed, 1)
	assert.Equal(t, float64(942100), parsed[0]["rule_id"])
}
