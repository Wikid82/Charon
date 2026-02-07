package cerberus

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/time/rate"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/Wikid82/charon/backend/internal/models"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func setupRateLimitTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:rate_limit_test_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.Setting{}))
	return db
}

func TestRateLimitMiddleware(t *testing.T) {
	t.Run("Blocks excessive requests", func(t *testing.T) {
		// Limit to 5 requests per second, with burst of 5
		mw := NewRateLimitMiddleware(5, 1, 5)

		r := gin.New()
		r.Use(mw)
		r.GET("/", func(c *gin.Context) {
			c.Status(http.StatusOK)
		})

		// Make 5 allowed requests
		for i := 0; i < 5; i++ {
			req, _ := http.NewRequest("GET", "/", nil)
			req.RemoteAddr = "192.168.1.1:1234"
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			assert.Equal(t, http.StatusOK, w.Code)
		}

		// Make 6th request (should fail)
		req, _ := http.NewRequest("GET", "/", nil)
		req.RemoteAddr = "192.168.1.1:1234"
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusTooManyRequests, w.Code)
	})

	t.Run("Different IPs have separate limits", func(t *testing.T) {
		mw := NewRateLimitMiddleware(1, 1, 1)

		r := gin.New()
		r.Use(mw)
		r.GET("/", func(c *gin.Context) {
			c.Status(http.StatusOK)
		})

		// 1st User
		req1, _ := http.NewRequest("GET", "/", nil)
		req1.RemoteAddr = "10.0.0.1:1234"
		w1 := httptest.NewRecorder()
		r.ServeHTTP(w1, req1)
		assert.Equal(t, http.StatusOK, w1.Code)

		// 2nd User (should pass)
		req2, _ := http.NewRequest("GET", "/", nil)
		req2.RemoteAddr = "10.0.0.2:1234"
		w2 := httptest.NewRecorder()
		r.ServeHTTP(w2, req2)
		assert.Equal(t, http.StatusOK, w2.Code)
	})

	t.Run("Replenishes tokens over time", func(t *testing.T) {
		// 1 request per second (burst 1)
		mw := NewRateLimitMiddleware(1, 1, 1)
		// Manually override the burst/limit for predictable testing isn't easy with wrapper
		// So we rely on the implementation using x/time/rate
		// Test:
		// 1. Consume 1
		// 2. Consume 2 (Fail)
		// 3. Wait until refill
		// 4. Consume 3 (Pass)

		r := gin.New()
		r.Use(mw)
		r.GET("/", func(c *gin.Context) {
			c.Status(http.StatusOK)
		})

		req, _ := http.NewRequest("GET", "/", nil)
		req.RemoteAddr = "1.2.3.4:1234"

		// 1. Consume
		w1 := httptest.NewRecorder()
		r.ServeHTTP(w1, req)
		assert.Equal(t, http.StatusOK, w1.Code)

		// 2. Consume Fail
		w2 := httptest.NewRecorder()
		r.ServeHTTP(w2, req)
		assert.Equal(t, http.StatusTooManyRequests, w2.Code)

		// 3. Wait until refill
		require.Eventually(t, func() bool {
			w3 := httptest.NewRecorder()
			r.ServeHTTP(w3, req)
			return w3.Code == http.StatusOK
		}, 1500*time.Millisecond, 25*time.Millisecond)
	})
}

func TestRateLimitManager_ReconfiguresLimiter(t *testing.T) {
	mgr := &rateLimitManager{
		limiters: make(map[string]*rate.Limiter),
		lastSeen: make(map[string]time.Time),
	}

	limiter := mgr.getLimiter("10.0.0.1", rate.Limit(1), 1)
	assert.Equal(t, rate.Limit(1), limiter.Limit())
	assert.Equal(t, 1, limiter.Burst())

	limiter = mgr.getLimiter("10.0.0.1", rate.Limit(2), 2)
	assert.Equal(t, rate.Limit(2), limiter.Limit())
	assert.Equal(t, 2, limiter.Burst())
}

func TestRateLimitManager_CleanupRemovesStaleEntries(t *testing.T) {
	mgr := &rateLimitManager{
		limiters: map[string]*rate.Limiter{
			"10.0.0.1": rate.NewLimiter(rate.Limit(1), 1),
		},
		lastSeen: map[string]time.Time{
			"10.0.0.1": time.Now().Add(-11 * time.Minute),
		},
	}

	mgr.cleanup()
	assert.Empty(t, mgr.limiters)
	assert.Empty(t, mgr.lastSeen)
}

func TestRateLimitMiddleware_EmergencyBypass(t *testing.T) {
	mw := NewRateLimitMiddleware(1, 1, 1)

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("emergency_bypass", true)
		c.Next()
	})
	r.Use(mw)
	r.GET("/", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	for i := 0; i < 2; i++ {
		req, _ := http.NewRequest("GET", "/", nil)
		req.RemoteAddr = "10.0.0.1:1234"
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	}
}

func TestCerberusRateLimitMiddleware_DisabledAllowsTraffic(t *testing.T) {
	cerb := New(config.SecurityConfig{RateLimitMode: "disabled"}, nil)

	r := gin.New()
	r.Use(cerb.RateLimitMiddleware())
	r.GET("/", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	for i := 0; i < 3; i++ {
		req, _ := http.NewRequest("GET", "/", nil)
		req.RemoteAddr = "10.0.0.1:1234"
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	}
}

func TestCerberusRateLimitMiddleware_EnabledByConfig(t *testing.T) {
	cfg := config.SecurityConfig{
		RateLimitMode:      "enabled",
		RateLimitRequests:  1,
		RateLimitWindowSec: 1,
		RateLimitBurst:     1,
	}
	cerb := New(cfg, nil)

	r := gin.New()
	r.Use(cerb.RateLimitMiddleware())
	r.GET("/", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req, _ := http.NewRequest("GET", "/", nil)
	req.RemoteAddr = "10.0.0.1:1234"
	for i := 0; i < 2; i++ {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if i == 0 {
			assert.Equal(t, http.StatusOK, w.Code)
		} else {
			assert.Equal(t, http.StatusTooManyRequests, w.Code)
		}
	}
}

func TestCerberusRateLimitMiddleware_EmergencyBypass(t *testing.T) {
	cfg := config.SecurityConfig{
		RateLimitMode:      "enabled",
		RateLimitRequests:  1,
		RateLimitWindowSec: 1,
		RateLimitBurst:     1,
	}
	cerb := New(cfg, nil)

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("emergency_bypass", true)
		c.Next()
	})
	r.Use(cerb.RateLimitMiddleware())
	r.GET("/", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	for i := 0; i < 2; i++ {
		req, _ := http.NewRequest("GET", "/", nil)
		req.RemoteAddr = "10.0.0.1:1234"
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	}
}

func TestCerberusRateLimitMiddleware_EnabledBySetting(t *testing.T) {
	db := setupRateLimitTestDB(t)
	require.NoError(t, db.Create(&models.Setting{Key: "security.rate_limit.enabled", Value: "true"}).Error)
	require.NoError(t, db.Create(&models.Setting{Key: "security.rate_limit.requests", Value: "1"}).Error)
	require.NoError(t, db.Create(&models.Setting{Key: "security.rate_limit.window", Value: "1"}).Error)
	require.NoError(t, db.Create(&models.Setting{Key: "security.rate_limit.burst", Value: "1"}).Error)

	cerb := New(config.SecurityConfig{RateLimitMode: "disabled"}, db)

	r := gin.New()
	r.Use(cerb.RateLimitMiddleware())
	r.GET("/", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req, _ := http.NewRequest("GET", "/", nil)
	req.RemoteAddr = "10.0.0.1:1234"

	w1 := httptest.NewRecorder()
	r.ServeHTTP(w1, req)
	assert.Equal(t, http.StatusOK, w1.Code)

	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req)
	assert.Equal(t, http.StatusTooManyRequests, w2.Code)
}

func TestCerberusRateLimitMiddleware_OverridesConfigWithSettings(t *testing.T) {
	db := setupRateLimitTestDB(t)
	require.NoError(t, db.Create(&models.Setting{Key: "security.rate_limit.enabled", Value: "true"}).Error)
	require.NoError(t, db.Create(&models.Setting{Key: "security.rate_limit.requests", Value: "1"}).Error)
	require.NoError(t, db.Create(&models.Setting{Key: "security.rate_limit.window", Value: "1"}).Error)
	require.NoError(t, db.Create(&models.Setting{Key: "security.rate_limit.burst", Value: "1"}).Error)

	cfg := config.SecurityConfig{
		RateLimitMode:      "enabled",
		RateLimitRequests:  10,
		RateLimitWindowSec: 10,
		RateLimitBurst:     10,
	}
	cerb := New(cfg, db)

	r := gin.New()
	r.Use(cerb.RateLimitMiddleware())
	r.GET("/", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req, _ := http.NewRequest("GET", "/", nil)
	req.RemoteAddr = "10.0.0.1:1234"

	w1 := httptest.NewRecorder()
	r.ServeHTTP(w1, req)
	assert.Equal(t, http.StatusOK, w1.Code)

	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req)
	assert.Equal(t, http.StatusTooManyRequests, w2.Code)
}

func TestCerberusRateLimitMiddleware_WindowFallback(t *testing.T) {
	cfg := config.SecurityConfig{
		RateLimitMode:      "enabled",
		RateLimitRequests:  1,
		RateLimitWindowSec: 0,
		RateLimitBurst:     1,
	}
	cerb := New(cfg, nil)

	r := gin.New()
	r.Use(cerb.RateLimitMiddleware())
	r.GET("/", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req, _ := http.NewRequest("GET", "/", nil)
	req.RemoteAddr = "10.0.0.1:1234"

	w1 := httptest.NewRecorder()
	r.ServeHTTP(w1, req)
	assert.Equal(t, http.StatusOK, w1.Code)

	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req)
	assert.Equal(t, http.StatusTooManyRequests, w2.Code)
}
