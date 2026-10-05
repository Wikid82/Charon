package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/Wikid82/charon/backend/internal/database"
	"github.com/Wikid82/charon/backend/internal/models"
)

// setupTestDB creates a temporary test database
func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	// Create temp database file
	tmpFile := t.TempDir() + "/test.db"
	db, err := database.Connect(tmpFile)
	require.NoError(t, err, "Failed to create test database")

	sqlDB, err := db.DB()
	require.NoError(t, err, "Failed to access underlying sql.DB")
	t.Cleanup(func() { _ = sqlDB.Close() })

	// Run migrations
	err = db.AutoMigrate(
		&models.Setting{},
		&models.SecurityConfig{},
		&models.SecurityAudit{},
	)
	require.NoError(t, err, "Failed to run migrations")

	return db
}

func TestEmergencyServer_Disabled(t *testing.T) {
	db := setupTestDB(t)

	cfg := config.EmergencyConfig{
		Enabled: false,
	}

	server := NewEmergencyServer(db, cfg)
	err := server.Start()
	require.NoError(t, err, "Server should start successfully when disabled")

	// Server should not be running
	assert.Nil(t, server.server, "HTTP server should not be initialized when disabled")
}

func TestEmergencyServer_Health(t *testing.T) {
	db := setupTestDB(t)

	// Set emergency token required for enabled server
	require.NoError(t, os.Setenv("CHARON_EMERGENCY_TOKEN", "test-token-for-health-check-32chars"))
	defer func() { _ = os.Unsetenv("CHARON_EMERGENCY_TOKEN") }()

	cfg := config.EmergencyConfig{
		Enabled:     true,
		BindAddress: "127.0.0.1:0", // Random port for testing
	}

	server := NewEmergencyServer(db, cfg)
	err := server.Start()
	require.NoError(t, err, "Server should start successfully")
	defer func() { _ = server.Stop(context.Background()) }()

	// Wait for server to start
	time.Sleep(100 * time.Millisecond)

	// Get actual port
	addr := server.GetAddr()
	assert.NotEmpty(t, addr, "Server address should be set")

	// Make health check request
	resp, err := http.Get(fmt.Sprintf("http://%s/health", addr))
	require.NoError(t, err, "Health check request should succeed")
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusOK, resp.StatusCode, "Health check should return 200")

	var body map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&body)
	require.NoError(t, err, "Should decode JSON response")

	assert.Equal(t, "ok", body["status"], "Status should be ok")
	assert.Equal(t, "emergency", body["server"], "Server should be emergency")
	assert.NotEmpty(t, body["time"], "Time should be present")
}

func TestEmergencyServer_SecurityReset(t *testing.T) {
	db := setupTestDB(t)

	// Set emergency token
	emergencyToken := "test-emergency-token-for-testing-32chars"
	require.NoError(t, os.Setenv("CHARON_EMERGENCY_TOKEN", emergencyToken))
	defer func() { require.NoError(t, os.Unsetenv("CHARON_EMERGENCY_TOKEN")) }()

	cfg := config.EmergencyConfig{
		Enabled:     true,
		BindAddress: "127.0.0.1:0",
	}

	server := NewEmergencyServer(db, cfg)
	err := server.Start()
	require.NoError(t, err, "Server should start successfully")
	defer func() { _ = server.Stop(context.Background()) }()

	// Wait for server to start
	time.Sleep(100 * time.Millisecond)

	addr := server.GetAddr()

	// Create HTTP client
	client := &http.Client{}

	// Make emergency reset request
	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("http://%s/emergency/security-reset", addr), http.NoBody)
	require.NoError(t, err, "Should create request")
	req.Header.Set("X-Emergency-Token", emergencyToken)

	resp, err := client.Do(req)
	require.NoError(t, err, "Emergency reset request should succeed")
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusOK, resp.StatusCode, "Emergency reset should return 200")

	var body map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&body)
	require.NoError(t, err, "Should decode JSON response")

	assert.True(t, body["success"].(bool), "Success should be true")
	assert.NotNil(t, body["disabled_modules"], "Disabled modules should be present")
}

func TestEmergencyServer_BasicAuth(t *testing.T) {
	db := setupTestDB(t)

	// Set emergency token
	emergencyToken := "test-emergency-token-for-testing-32chars"
	require.NoError(t, os.Setenv("CHARON_EMERGENCY_TOKEN", emergencyToken))
	defer func() { require.NoError(t, os.Unsetenv("CHARON_EMERGENCY_TOKEN")) }()

	cfg := config.EmergencyConfig{
		Enabled:           true,
		BindAddress:       "127.0.0.1:0",
		BasicAuthUsername: "admin",
		BasicAuthPassword: "testpass",
	}

	server := NewEmergencyServer(db, cfg)
	err := server.Start()
	require.NoError(t, err, "Server should start successfully")
	defer func() { _ = server.Stop(context.Background()) }()

	// Wait for server to start
	time.Sleep(100 * time.Millisecond)

	addr := server.GetAddr()

	t.Run("WithoutAuth", func(t *testing.T) {
		// Try without Basic Auth - should fail
		req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("http://%s/emergency/security-reset", addr), http.NoBody)
		require.NoError(t, err, "Should create request")
		req.Header.Set("X-Emergency-Token", emergencyToken)

		client := &http.Client{}
		resp, err := client.Do(req)
		require.NoError(t, err, "Request should complete")
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "Should require authentication")
	})

	t.Run("WithInvalidAuth", func(t *testing.T) {
		// Try with wrong credentials
		req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("http://%s/emergency/security-reset", addr), http.NoBody)
		require.NoError(t, err, "Should create request")
		req.Header.Set("X-Emergency-Token", emergencyToken)
		req.SetBasicAuth("admin", "wrongpassword")

		client := &http.Client{}
		resp, err := client.Do(req)
		require.NoError(t, err, "Request should complete")
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "Should reject invalid credentials")
	})

	t.Run("WithValidAuth", func(t *testing.T) {
		// Try with correct credentials
		req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("http://%s/emergency/security-reset", addr), http.NoBody)
		require.NoError(t, err, "Should create request")
		req.Header.Set("X-Emergency-Token", emergencyToken)
		req.SetBasicAuth("admin", "testpass")

		client := &http.Client{}
		resp, err := client.Do(req)
		require.NoError(t, err, "Request should complete")
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusOK, resp.StatusCode, "Should accept valid credentials")

		var body map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&body)
		require.NoError(t, err, "Should decode JSON response")

		assert.True(t, body["success"].(bool), "Success should be true")
	})
}

func TestEmergencyServer_NoAuth_Warning(t *testing.T) {
	// This test verifies that a warning is logged when no auth is configured
	// We can't easily test log output, but we can verify the server starts
	db := setupTestDB(t)

	// Set emergency token required for enabled server
	require.NoError(t, os.Setenv("CHARON_EMERGENCY_TOKEN", "test-token-for-no-auth-warning-test"))
	defer func() { _ = os.Unsetenv("CHARON_EMERGENCY_TOKEN") }()

	cfg := config.EmergencyConfig{
		Enabled:     true,
		BindAddress: "127.0.0.1:0",
		// No auth configured
	}

	server := NewEmergencyServer(db, cfg)
	err := server.Start()
	require.NoError(t, err, "Server should start even without auth")
	defer func() { _ = server.Stop(context.Background()) }()

	// Wait for server to start
	time.Sleep(100 * time.Millisecond)

	// Verify server is accessible without auth
	addr := server.GetAddr()
	resp, err := http.Get(fmt.Sprintf("http://%s/health", addr))
	require.NoError(t, err, "Health check should work without auth")
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusOK, resp.StatusCode, "Should return 200")
}

func TestEmergencyServer_GracefulShutdown(t *testing.T) {
	db := setupTestDB(t)

	// Set emergency token required for enabled server
	require.NoError(t, os.Setenv("CHARON_EMERGENCY_TOKEN", "test-token-for-graceful-shutdown-test"))
	defer func() { _ = os.Unsetenv("CHARON_EMERGENCY_TOKEN") }()

	cfg := config.EmergencyConfig{
		Enabled:     true,
		BindAddress: "127.0.0.1:0",
	}

	server := NewEmergencyServer(db, cfg)
	err := server.Start()
	require.NoError(t, err, "Server should start successfully")

	// Wait for server to start
	time.Sleep(100 * time.Millisecond)

	// Verify server is running
	addr := server.GetAddr()
	resp, err := http.Get(fmt.Sprintf("http://%s/health", addr))
	require.NoError(t, err, "Server should be running")
	_ = resp.Body.Close()

	// Stop server with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = server.Stop(ctx)
	assert.NoError(t, err, "Server should stop gracefully")

	// Verify server is stopped (request should fail)
	resp, err = http.Get(fmt.Sprintf("http://%s/health", addr))
	if resp != nil {
		_ = resp.Body.Close()
	}
	assert.Error(t, err, "Server should be stopped")
}

func TestEmergencyServer_MultipleEndpoints(t *testing.T) {
	db := setupTestDB(t)

	// Set emergency token
	emergencyToken := "test-emergency-token-for-testing-32chars"
	require.NoError(t, os.Setenv("CHARON_EMERGENCY_TOKEN", emergencyToken))
	defer func() { require.NoError(t, os.Unsetenv("CHARON_EMERGENCY_TOKEN")) }()

	cfg := config.EmergencyConfig{
		Enabled:     true,
		BindAddress: "127.0.0.1:0",
	}

	server := NewEmergencyServer(db, cfg)
	err := server.Start()
	require.NoError(t, err, "Server should start successfully")
	defer func() { _ = server.Stop(context.Background()) }()

	// Wait for server to start
	time.Sleep(100 * time.Millisecond)

	addr := server.GetAddr()

	t.Run("HealthEndpoint", func(t *testing.T) {
		resp, err := http.Get(fmt.Sprintf("http://%s/health", addr))
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("EmergencyResetEndpoint", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("http://%s/emergency/security-reset", addr), http.NoBody)
		require.NoError(t, err)
		req.Header.Set("X-Emergency-Token", emergencyToken)

		client := &http.Client{}
		resp, err := client.Do(req)
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("NotFoundEndpoint", func(t *testing.T) {
		resp, err := http.Get(fmt.Sprintf("http://%s/nonexistent", addr))
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()
		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	})
}

// TestEmergencyServer_StartupValidation tests that server fails fast if token is empty or whitespace
func TestEmergencyServer_StartupValidation(t *testing.T) {
	db := setupTestDB(t)

	tests := []struct {
		name          string
		token         string
		expectSuccess bool
		description   string
	}{
		{
			name:          "EmptyToken",
			token:         "",
			expectSuccess: false,
			description:   "Server should fail to start with empty token",
		},
		{
			name:          "WhitespaceToken",
			token:         "   ",
			expectSuccess: false,
			description:   "Server should fail to start with whitespace-only token",
		},
		{
			name:          "ValidToken",
			token:         "test-emergency-token-for-testing-32chars",
			expectSuccess: true,
			description:   "Server should start successfully with valid token",
		},
		{
			name:          "ShortToken",
			token:         "short",
			expectSuccess: true, // Server starts but logs warning
			description:   "Server should start with short token but log warning",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set token
			if tt.token != "" {
				require.NoError(t, os.Setenv("CHARON_EMERGENCY_TOKEN", tt.token))
			} else {
				_ = os.Unsetenv("CHARON_EMERGENCY_TOKEN")
			}
			defer func() { _ = os.Unsetenv("CHARON_EMERGENCY_TOKEN") }()

			cfg := config.EmergencyConfig{
				Enabled:     true,
				BindAddress: "127.0.0.1:0",
			}

			server := NewEmergencyServer(db, cfg)
			err := server.Start()

			if tt.expectSuccess {
				assert.NoError(t, err, tt.description)
				if err == nil {
					_ = server.Stop(context.Background())
				}
			} else {
				assert.Error(t, err, tt.description)
			}
		})
	}
}

// TestEmergencyServer_TokenRedaction tests the token redaction function
func TestEmergencyServer_TokenRedaction(t *testing.T) {
	tests := []struct {
		name     string
		token    string
		expected string
	}{
		{
			name:     "EmptyToken",
			token:    "",
			expected: "[EMERGENCY_TOKEN:empty]",
		},
		{
			name:     "ShortToken",
			token:    "short",
			expected: "[EMERGENCY_TOKEN:***]",
		},
		{ //nolint:gosec // test fixture demonstrating token masking format
			name:     "ValidToken",
			token:    strings.Repeat("a", 64),
			expected: "[EMERGENCY_TOKEN:aaaa...aaaa]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := redactToken(tt.token)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestEmergencyServer_SecurityReset_HonorsManagementCIDRs(t *testing.T) {
	db := setupTestDB(t)

	emergencyToken := "test-emergency-token-for-testing-32chars"
	t.Setenv("CHARON_EMERGENCY_TOKEN", emergencyToken)

	// The loopback test client is outside the configured network.
	server := NewEmergencyServer(db, config.EmergencyConfig{Enabled: true, BindAddress: "127.0.0.1:0"}).
		WithManagementCIDRs([]string{"203.0.113.0/24"})
	require.NoError(t, server.Start())
	defer func() { _ = server.Stop(context.Background()) }()
	time.Sleep(100 * time.Millisecond)

	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("http://%s/emergency/security-reset", server.GetAddr()), http.NoBody)
	require.NoError(t, err)
	req.Header.Set("X-Emergency-Token", emergencyToken)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
}

func TestEmergencyServer_IgnoresRequestOriginHeaders(t *testing.T) {
	emergencyToken := "test-emergency-token-for-testing-32chars"
	hopHeaders := map[string]string{
		"X-Charon-Self-Hop":        "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		"X-Charon-Self-Hop-Client": "203.0.113.9",
		"X-Forwarded-For":          "203.0.113.9",
	}

	run := func(t *testing.T, cidrs []string) int {
		t.Helper()
		db := setupTestDB(t)
		t.Setenv("CHARON_EMERGENCY_TOKEN", emergencyToken)

		server := NewEmergencyServer(db, config.EmergencyConfig{Enabled: true, BindAddress: "127.0.0.1:0"}).WithManagementCIDRs(cidrs)
		require.NoError(t, server.Start())
		t.Cleanup(func() { _ = server.Stop(context.Background()) })
		time.Sleep(100 * time.Millisecond)

		req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("http://%s/emergency/security-reset", server.GetAddr()), http.NoBody)
		require.NoError(t, err)
		req.Header.Set("X-Emergency-Token", emergencyToken)
		for k, v := range hopHeaders {
			req.Header.Set(k, v)
		}
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()
		return resp.StatusCode
	}

	// The address used for decisions is the connection's own address: a claimed
	// client address inside the allowed network does not help a loopback peer...
	assert.Equal(t, http.StatusForbidden, run(t, []string{"203.0.113.0/24"}))
	// ...and a claimed outside address does not hurt one that is allowed.
	assert.Equal(t, http.StatusOK, run(t, []string{"127.0.0.0/8"}))
}
