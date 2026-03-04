package cerberus_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/cerberus"
	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/stretchr/testify/require"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func setupTestDB(t *testing.T) *gorm.DB {
	// Use a unique in-memory database per test run to avoid shared state.
	dsn := fmt.Sprintf("file:cerberus_test_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	// migrate only the Setting model used by Cerberus
	require.NoError(t, db.AutoMigrate(&models.Setting{}))
	return db
}

func setupFullTestDB(t *testing.T) *gorm.DB {
	dsn := fmt.Sprintf("file:cerberus_full_test_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&models.Setting{},
		&models.AccessList{},
	))
	return db
}

func TestCerberus_IsEnabled_ConfigTrue(t *testing.T) {
	db := setupTestDB(t)
	cfg := config.SecurityConfig{CerberusEnabled: true}
	cerb := cerberus.New(cfg, db)
	require.True(t, cerb.IsEnabled())
}

func TestCerberus_IsEnabled_DBSetting(t *testing.T) {
	db := setupTestDB(t)
	// We're storing 'feature.cerberus.enabled' key
	db.Create(&models.Setting{Key: "feature.cerberus.enabled", Value: "true"})
	cfg := config.SecurityConfig{CerberusEnabled: false}
	cerb := cerberus.New(cfg, db)
	require.True(t, cerb.IsEnabled())
}

func TestCerberus_IsEnabled_Disabled(t *testing.T) {
	db := setupTestDB(t)
	// Per Optional Features spec: when no DB setting exists and no config modes are enabled,
	// Cerberus defaults to true (enabled). To test disabled state, we must set DB flag to false.
	db.Create(&models.Setting{Key: "feature.cerberus.enabled", Value: "false"})
	cfg := config.SecurityConfig{CerberusEnabled: false}
	cerb := cerberus.New(cfg, db)
	t.Logf("cfg: %+v", cfg)
	t.Logf("IsEnabled() -> %v", cerb.IsEnabled())
	require.False(t, cerb.IsEnabled())
}

func TestCerberus_IsEnabled_CrowdSecLocal(t *testing.T) {
	db := setupTestDB(t)
	cfg := config.SecurityConfig{CrowdSecMode: "local"}
	cerb := cerberus.New(cfg, db)
	require.True(t, cerb.IsEnabled())
}

func TestCerberus_IsEnabled_WAFEnabled(t *testing.T) {
	db := setupTestDB(t)
	cfg := config.SecurityConfig{WAFMode: "enabled"}
	cerb := cerberus.New(cfg, db)
	require.True(t, cerb.IsEnabled())
}

func TestCerberus_IsEnabled_RateLimitEnabled(t *testing.T) {
	db := setupTestDB(t)
	cfg := config.SecurityConfig{RateLimitMode: "enabled"}
	cerb := cerberus.New(cfg, db)
	require.True(t, cerb.IsEnabled())
}

func TestCerberus_IsEnabled_ACLEnabled(t *testing.T) {
	db := setupTestDB(t)
	cfg := config.SecurityConfig{ACLMode: "enabled"}
	cerb := cerberus.New(cfg, db)
	require.True(t, cerb.IsEnabled())
}

func TestCerberus_IsEnabled_LegacySetting(t *testing.T) {
	db := setupTestDB(t)
	// Test legacy setting key for backward compatibility
	db.Create(&models.Setting{Key: "security.cerberus.enabled", Value: "true"})
	cfg := config.SecurityConfig{CerberusEnabled: false}
	cerb := cerberus.New(cfg, db)
	require.True(t, cerb.IsEnabled())
}

// ============================================
// Middleware Tests
// ============================================

func TestCerberus_Middleware_Disabled(t *testing.T) {
	db := setupTestDB(t)
	// Explicitly disable cerberus via DB setting
	db.Create(&models.Setting{Key: "feature.cerberus.enabled", Value: "false"})
	cfg := config.SecurityConfig{CerberusEnabled: false}
	cerb := cerberus.New(cfg, db)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("role", "admin")
		c.Set("userID", uint(1))
		c.Next()
	})
	router.Use(cerb.Middleware())
	router.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "OK")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", http.NoBody)
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "OK", w.Body.String())
}

func TestCerberus_Middleware_WAFEnabled(t *testing.T) {
	db := setupFullTestDB(t)
	cfg := config.SecurityConfig{
		CerberusEnabled: true,
		WAFMode:         "enabled",
	}
	cerb := cerberus.New(cfg, db)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("role", "admin")
		c.Set("userID", uint(1))
		c.Next()
	})
	router.Use(cerb.Middleware())
	router.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "OK")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", http.NoBody)
	router.ServeHTTP(w, req)

	// WAF doesn't block in middleware (handled by Caddy), just tracks metrics
	require.Equal(t, http.StatusOK, w.Code)
}

func TestCerberus_Middleware_ACLEnabled_NoAccessLists(t *testing.T) {
	db := setupFullTestDB(t)
	cfg := config.SecurityConfig{
		CerberusEnabled: true,
		ACLMode:         "enabled",
	}
	cerb := cerberus.New(cfg, db)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("role", "admin")
		c.Set("userID", uint(1))
		c.Next()
	})
	router.Use(cerb.Middleware())
	router.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "OK")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", http.NoBody)
	router.ServeHTTP(w, req)

	// No access lists, so request should pass
	require.Equal(t, http.StatusOK, w.Code)
}

func TestCerberus_Middleware_ACLEnabled_DisabledList(t *testing.T) {
	db := setupFullTestDB(t)
	// Create a disabled ACL
	db.Create(&models.AccessList{
		UUID:    "test-acl",
		Name:    "Test ACL",
		Type:    "blacklist",
		IPRules: `[{"cidr":"0.0.0.0/0","description":"Block all"}]`,
		Enabled: false, // Disabled
	})

	cfg := config.SecurityConfig{
		CerberusEnabled: true,
		ACLMode:         "enabled",
	}
	cerb := cerberus.New(cfg, db)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("role", "admin")
		c.Set("userID", uint(1))
		c.Next()
	})
	router.Use(cerb.Middleware())
	router.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "OK")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", http.NoBody)
	router.ServeHTTP(w, req)

	// ACL is disabled, so request should pass
	require.Equal(t, http.StatusOK, w.Code)
}

func TestCerberus_Middleware_ACLEnabled_Blocked(t *testing.T) {
	db := setupFullTestDB(t)
	// Create an enabled blacklist that blocks 192.168.x.x
	db.Create(&models.AccessList{
		UUID:    "test-blacklist",
		Name:    "Block Private",
		Type:    "blacklist",
		IPRules: `[{"cidr":"192.168.0.0/16","description":"Block 192.168.x.x"}]`,
		Enabled: true,
	})

	cfg := config.SecurityConfig{
		CerberusEnabled: true,
		ACLMode:         "enabled",
	}
	cerb := cerberus.New(cfg, db)

	router := gin.New()
	router.Use(cerb.Middleware())
	router.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "OK")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", http.NoBody)
	req.RemoteAddr = "192.168.1.100:12345" // IP in blacklist range
	router.ServeHTTP(w, req)

	// Request should be blocked
	require.Equal(t, http.StatusForbidden, w.Code)
}

func TestCerberus_Middleware_CrowdSecLocal(t *testing.T) {
	db := setupFullTestDB(t)
	cfg := config.SecurityConfig{
		CerberusEnabled: true,
		CrowdSecMode:    "local",
	}
	cerb := cerberus.New(cfg, db)

	router := gin.New()
	router.Use(cerb.Middleware())
	router.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "OK")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", http.NoBody)
	router.ServeHTTP(w, req)

	// CrowdSec doesn't block in middleware (handled by Caddy), just tracks metrics
	require.Equal(t, http.StatusOK, w.Code)
}

// ============================================
// Cache Tests
// ============================================

func TestCerberus_InvalidateCache(t *testing.T) {
	db := setupTestDB(t)
	db.Create(&models.Setting{Key: "security.waf.enabled", Value: "true"})
	db.Create(&models.Setting{Key: "security.acl.enabled", Value: "false"})

	cfg := config.SecurityConfig{CerberusEnabled: true}
	cerb := cerberus.New(cfg, db)

	// Prime the cache by calling getSetting
	router := gin.New()
	router.Use(cerb.Middleware())
	router.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "OK")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", http.NoBody)
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	// Now invalidate the cache
	cerb.InvalidateCache()

	// Update setting in DB
	db.Model(&models.Setting{}).Where("key = ?", "security.waf.enabled").Update("value", "false")

	// Make another request - should pick up new setting
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/test", http.NoBody)
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
}
