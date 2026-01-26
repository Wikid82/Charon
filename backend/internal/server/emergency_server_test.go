package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
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

	cfg := config.EmergencyConfig{
		Enabled:     true,
		BindAddress: "127.0.0.1:0", // Random port for testing
	}

	server := NewEmergencyServer(db, cfg)
	err := server.Start()
	require.NoError(t, err, "Server should start successfully")
	defer server.Stop(context.Background())

	// Wait for server to start
	time.Sleep(100 * time.Millisecond)

	// Get actual port
	addr := server.GetAddr()
	assert.NotEmpty(t, addr, "Server address should be set")

	// Make health check request
	resp, err := http.Get(fmt.Sprintf("http://%s/health", addr))
	require.NoError(t, err, "Health check request should succeed")
	defer resp.Body.Close()

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
	os.Setenv("CHARON_EMERGENCY_TOKEN", emergencyToken)
	defer os.Unsetenv("CHARON_EMERGENCY_TOKEN")

	cfg := config.EmergencyConfig{
		Enabled:     true,
		BindAddress: "127.0.0.1:0",
	}

	server := NewEmergencyServer(db, cfg)
	err := server.Start()
	require.NoError(t, err, "Server should start successfully")
	defer server.Stop(context.Background())

	// Wait for server to start
	time.Sleep(100 * time.Millisecond)

	addr := server.GetAddr()

	// Create HTTP client
	client := &http.Client{}

	// Make emergency reset request
	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("http://%s/emergency/security-reset", addr), nil)
	require.NoError(t, err, "Should create request")
	req.Header.Set("X-Emergency-Token", emergencyToken)

	resp, err := client.Do(req)
	require.NoError(t, err, "Emergency reset request should succeed")
	defer resp.Body.Close()

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
	os.Setenv("CHARON_EMERGENCY_TOKEN", emergencyToken)
	defer os.Unsetenv("CHARON_EMERGENCY_TOKEN")

	cfg := config.EmergencyConfig{
		Enabled:           true,
		BindAddress:       "127.0.0.1:0",
		BasicAuthUsername: "admin",
		BasicAuthPassword: "testpass",
	}

	server := NewEmergencyServer(db, cfg)
	err := server.Start()
	require.NoError(t, err, "Server should start successfully")
	defer server.Stop(context.Background())

	// Wait for server to start
	time.Sleep(100 * time.Millisecond)

	addr := server.GetAddr()

	t.Run("WithoutAuth", func(t *testing.T) {
		// Try without Basic Auth - should fail
		req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("http://%s/emergency/security-reset", addr), nil)
		require.NoError(t, err, "Should create request")
		req.Header.Set("X-Emergency-Token", emergencyToken)

		client := &http.Client{}
		resp, err := client.Do(req)
		require.NoError(t, err, "Request should complete")
		defer resp.Body.Close()

		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "Should require authentication")
	})

	t.Run("WithInvalidAuth", func(t *testing.T) {
		// Try with wrong credentials
		req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("http://%s/emergency/security-reset", addr), nil)
		require.NoError(t, err, "Should create request")
		req.Header.Set("X-Emergency-Token", emergencyToken)
		req.SetBasicAuth("admin", "wrongpassword")

		client := &http.Client{}
		resp, err := client.Do(req)
		require.NoError(t, err, "Request should complete")
		defer resp.Body.Close()

		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "Should reject invalid credentials")
	})

	t.Run("WithValidAuth", func(t *testing.T) {
		// Try with correct credentials
		req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("http://%s/emergency/security-reset", addr), nil)
		require.NoError(t, err, "Should create request")
		req.Header.Set("X-Emergency-Token", emergencyToken)
		req.SetBasicAuth("admin", "testpass")

		client := &http.Client{}
		resp, err := client.Do(req)
		require.NoError(t, err, "Request should complete")
		defer resp.Body.Close()

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

	cfg := config.EmergencyConfig{
		Enabled:     true,
		BindAddress: "127.0.0.1:0",
		// No auth configured
	}

	server := NewEmergencyServer(db, cfg)
	err := server.Start()
	require.NoError(t, err, "Server should start even without auth")
	defer server.Stop(context.Background())

	// Wait for server to start
	time.Sleep(100 * time.Millisecond)

	// Verify server is accessible without auth
	addr := server.GetAddr()
	resp, err := http.Get(fmt.Sprintf("http://%s/health", addr))
	require.NoError(t, err, "Health check should work without auth")
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode, "Should return 200")
}

func TestEmergencyServer_GracefulShutdown(t *testing.T) {
	db := setupTestDB(t)

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
	resp.Body.Close()

	// Stop server with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = server.Stop(ctx)
	assert.NoError(t, err, "Server should stop gracefully")

	// Verify server is stopped (request should fail)
	_, err = http.Get(fmt.Sprintf("http://%s/health", addr))
	assert.Error(t, err, "Server should be stopped")
}

func TestEmergencyServer_MultipleEndpoints(t *testing.T) {
	db := setupTestDB(t)

	// Set emergency token
	emergencyToken := "test-emergency-token-for-testing-32chars"
	os.Setenv("CHARON_EMERGENCY_TOKEN", emergencyToken)
	defer os.Unsetenv("CHARON_EMERGENCY_TOKEN")

	cfg := config.EmergencyConfig{
		Enabled:     true,
		BindAddress: "127.0.0.1:0",
	}

	server := NewEmergencyServer(db, cfg)
	err := server.Start()
	require.NoError(t, err, "Server should start successfully")
	defer server.Stop(context.Background())

	// Wait for server to start
	time.Sleep(100 * time.Millisecond)

	addr := server.GetAddr()

	t.Run("HealthEndpoint", func(t *testing.T) {
		resp, err := http.Get(fmt.Sprintf("http://%s/health", addr))
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("EmergencyResetEndpoint", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("http://%s/emergency/security-reset", addr), nil)
		require.NoError(t, err)
		req.Header.Set("X-Emergency-Token", emergencyToken)

		client := &http.Client{}
		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("NotFoundEndpoint", func(t *testing.T) {
		resp, err := http.Get(fmt.Sprintf("http://%s/nonexistent", addr))
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	})
}
