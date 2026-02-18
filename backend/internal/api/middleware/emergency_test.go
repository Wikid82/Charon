package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestEmergencyBypass_NoToken(t *testing.T) {
	// Test that requests without emergency token proceed normally
	gin.SetMode(gin.TestMode)

	t.Setenv("CHARON_EMERGENCY_TOKEN", "test-token-that-meets-minimum-length-requirement-32-chars")

	router := gin.New()
	managementCIDRs := []string{"127.0.0.0/8"}
	router.Use(EmergencyBypass(managementCIDRs, nil))

	router.GET("/test", func(c *gin.Context) {
		_, exists := c.Get("emergency_bypass")
		assert.False(t, exists, "Emergency bypass flag should not be set")
		c.JSON(http.StatusOK, gin.H{"message": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestEmergencyBypass_InvalidClientIP(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Setenv("CHARON_EMERGENCY_TOKEN", "test-token-that-meets-minimum-length-requirement-32-chars")

	router := gin.New()
	managementCIDRs := []string{"127.0.0.0/8"}
	router.Use(EmergencyBypass(managementCIDRs, nil))

	router.GET("/test", func(c *gin.Context) {
		_, exists := c.Get("emergency_bypass")
		assert.False(t, exists, "Emergency bypass flag should not be set for invalid client IP")
		c.JSON(http.StatusOK, gin.H{"message": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set(EmergencyTokenHeader, "test-token-that-meets-minimum-length-requirement-32-chars")
	req.RemoteAddr = "invalid-remote-addr"
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestEmergencyBypass_ValidToken(t *testing.T) {
	// Test that valid token from allowed IP sets bypass flag
	gin.SetMode(gin.TestMode)

	t.Setenv("CHARON_EMERGENCY_TOKEN", "test-token-that-meets-minimum-length-requirement-32-chars")

	router := gin.New()
	managementCIDRs := []string{"127.0.0.0/8"}
	router.Use(EmergencyBypass(managementCIDRs, nil))

	router.GET("/test", func(c *gin.Context) {
		bypass, exists := c.Get("emergency_bypass")
		assert.True(t, exists, "Emergency bypass flag should be set")
		assert.True(t, bypass.(bool), "Emergency bypass flag should be true")
		c.JSON(http.StatusOK, gin.H{"message": "bypass active"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set(EmergencyTokenHeader, "test-token-that-meets-minimum-length-requirement-32-chars")
	req.RemoteAddr = "127.0.0.1:12345"
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// Verify token was stripped from request
	assert.Empty(t, req.Header.Get(EmergencyTokenHeader), "Token should be stripped")
}

func TestEmergencyBypass_ValidToken_IPv6Localhost(t *testing.T) {
	// Test that valid token from IPv6 localhost is treated as localhost
	gin.SetMode(gin.TestMode)

	t.Setenv("CHARON_EMERGENCY_TOKEN", "test-token-that-meets-minimum-length-requirement-32-chars")

	router := gin.New()
	_ = router.SetTrustedProxies(nil)
	managementCIDRs := []string{"127.0.0.0/8"}
	router.Use(EmergencyBypass(managementCIDRs, nil))

	router.GET("/test", func(c *gin.Context) {
		bypass, exists := c.Get("emergency_bypass")
		assert.True(t, exists, "Emergency bypass flag should be set")
		assert.True(t, bypass.(bool), "Emergency bypass flag should be true")
		c.JSON(http.StatusOK, gin.H{"message": "bypass active"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set(EmergencyTokenHeader, "test-token-that-meets-minimum-length-requirement-32-chars")
	req.RemoteAddr = "[::1]:12345"
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestEmergencyBypass_InvalidToken(t *testing.T) {
	// Test that invalid token does not set bypass flag
	gin.SetMode(gin.TestMode)

	t.Setenv("CHARON_EMERGENCY_TOKEN", "test-token-that-meets-minimum-length-requirement-32-chars")

	router := gin.New()
	managementCIDRs := []string{"127.0.0.0/8"}
	router.Use(EmergencyBypass(managementCIDRs, nil))

	router.GET("/test", func(c *gin.Context) {
		_, exists := c.Get("emergency_bypass")
		assert.False(t, exists, "Emergency bypass flag should not be set")
		c.JSON(http.StatusOK, gin.H{"message": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set(EmergencyTokenHeader, "wrong-token")
	req.RemoteAddr = "127.0.0.1:12345"
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestEmergencyBypass_UnauthorizedIP(t *testing.T) {
	// Test that valid token from disallowed IP does not set bypass flag
	gin.SetMode(gin.TestMode)

	t.Setenv("CHARON_EMERGENCY_TOKEN", "test-token-that-meets-minimum-length-requirement-32-chars")

	router := gin.New()
	managementCIDRs := []string{"127.0.0.0/8"}
	router.Use(EmergencyBypass(managementCIDRs, nil))

	router.GET("/test", func(c *gin.Context) {
		_, exists := c.Get("emergency_bypass")
		assert.False(t, exists, "Emergency bypass flag should not be set")
		c.JSON(http.StatusOK, gin.H{"message": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set(EmergencyTokenHeader, "test-token-that-meets-minimum-length-requirement-32-chars")
	req.RemoteAddr = "203.0.113.1:12345" // Public IP (not in management network)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestEmergencyBypass_TokenStripped(t *testing.T) {
	// Test that emergency token header is removed after validation
	gin.SetMode(gin.TestMode)

	t.Setenv("CHARON_EMERGENCY_TOKEN", "test-token-that-meets-minimum-length-requirement-32-chars")

	router := gin.New()
	managementCIDRs := []string{"127.0.0.0/8"}
	router.Use(EmergencyBypass(managementCIDRs, nil))

	var tokenInHandler string
	router.GET("/test", func(c *gin.Context) {
		tokenInHandler = c.GetHeader(EmergencyTokenHeader)
		c.JSON(http.StatusOK, gin.H{"message": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set(EmergencyTokenHeader, "test-token-that-meets-minimum-length-requirement-32-chars")
	req.RemoteAddr = "127.0.0.1:12345"
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Empty(t, tokenInHandler, "Token should not be visible in downstream handlers")
}

func TestEmergencyBypass_MinimumLength(t *testing.T) {
	// Test that tokens < 32 chars are rejected
	gin.SetMode(gin.TestMode)

	t.Setenv("CHARON_EMERGENCY_TOKEN", "short-token")

	router := gin.New()
	managementCIDRs := []string{"127.0.0.0/8"}
	router.Use(EmergencyBypass(managementCIDRs, nil))

	router.GET("/test", func(c *gin.Context) {
		_, exists := c.Get("emergency_bypass")
		assert.False(t, exists, "Emergency bypass flag should not be set with short token")
		c.JSON(http.StatusOK, gin.H{"message": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set(EmergencyTokenHeader, "short-token")
	req.RemoteAddr = "127.0.0.1:12345"
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestEmergencyBypass_NoTokenConfigured(t *testing.T) {
	// Test that middleware is no-op when token not configured
	gin.SetMode(gin.TestMode)

	// Don't set CHARON_EMERGENCY_TOKEN
	t.Setenv("CHARON_EMERGENCY_TOKEN", "")

	router := gin.New()
	managementCIDRs := []string{"127.0.0.0/8"}
	router.Use(EmergencyBypass(managementCIDRs, nil))

	router.GET("/test", func(c *gin.Context) {
		_, exists := c.Get("emergency_bypass")
		assert.False(t, exists, "Emergency bypass flag should not be set")
		c.JSON(http.StatusOK, gin.H{"message": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set(EmergencyTokenHeader, "any-token")
	req.RemoteAddr = "127.0.0.1:12345"
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestEmergencyBypass_DefaultCIDRs(t *testing.T) {
	// Test that RFC1918 networks are used by default
	gin.SetMode(gin.TestMode)

	t.Setenv("CHARON_EMERGENCY_TOKEN", "test-token-that-meets-minimum-length-requirement-32-chars")

	router := gin.New()
	// Pass empty CIDR list to trigger default behavior
	router.Use(EmergencyBypass([]string{}, nil))

	router.GET("/test", func(c *gin.Context) {
		bypass, exists := c.Get("emergency_bypass")
		assert.True(t, exists, "Emergency bypass flag should be set")
		assert.True(t, bypass.(bool), "Emergency bypass flag should be true")
		c.JSON(http.StatusOK, gin.H{"message": "bypass active"})
	})

	// Test with various RFC1918 addresses
	testIPs := []string{
		"10.0.0.1:12345",
		"172.16.0.1:12345",
		"192.168.1.1:12345",
		"127.0.0.1:12345",
	}

	for _, remoteAddr := range testIPs {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set(EmergencyTokenHeader, "test-token-that-meets-minimum-length-requirement-32-chars")
		req.RemoteAddr = remoteAddr
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code, "Should accept IP: %s", remoteAddr)
	}
}
