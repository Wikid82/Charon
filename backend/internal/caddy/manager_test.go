package caddy

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Wikid82/CaddyProxyManagerPlus/backend/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestManager_ApplyConfig(t *testing.T) {
	// Mock Caddy Admin API
	caddyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/load" && r.Method == "POST" {
			// Verify payload
			var config Config
			err := json.NewDecoder(r.Body).Decode(&config)
			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer caddyServer.Close()

	// Setup DB
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.ProxyHost{}, &models.Location{}, &models.Setting{}, &models.CaddyConfig{}, &models.SSLCertificate{}, &models.ForwardAuthConfig{}))

	// Setup Manager
	tmpDir := t.TempDir()
	client := NewClient(caddyServer.URL)
	manager := NewManager(client, db, tmpDir, "", false)

	// Create a host
	host := models.ProxyHost{
		DomainNames: "example.com",
		ForwardHost: "127.0.0.1",
		ForwardPort: 8080,
	}
	db.Create(&host)

	// Apply Config
	err = manager.ApplyConfig(context.Background())
	assert.NoError(t, err)

	// Verify config was saved to DB
	var caddyConfig models.CaddyConfig
	err = db.First(&caddyConfig).Error
	assert.NoError(t, err)
	assert.True(t, caddyConfig.Success)
}

func TestManager_ApplyConfig_Failure(t *testing.T) {
	// Mock Caddy Admin API to fail
	caddyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer caddyServer.Close()

	// Setup DB
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.ProxyHost{}, &models.Location{}, &models.Setting{}, &models.CaddyConfig{}, &models.SSLCertificate{}, &models.ForwardAuthConfig{}))

	// Setup Manager
	tmpDir := t.TempDir()
	client := NewClient(caddyServer.URL)
	manager := NewManager(client, db, tmpDir, "", false)

	// Create a host
	host := models.ProxyHost{
		DomainNames: "example.com",
		ForwardHost: "127.0.0.1",
		ForwardPort: 8080,
	}
	require.NoError(t, db.Create(&host).Error)

	// Apply Config - should fail
	err = manager.ApplyConfig(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "apply failed")

	// Verify failure was recorded
	var caddyConfig models.CaddyConfig
	err = db.First(&caddyConfig).Error
	assert.NoError(t, err)
	assert.False(t, caddyConfig.Success)
	assert.NotEmpty(t, caddyConfig.ErrorMsg)
}

func TestManager_Ping(t *testing.T) {
	// Mock Caddy Admin API
	caddyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/config/" && r.Method == "GET" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer caddyServer.Close()

	client := NewClient(caddyServer.URL)
	manager := NewManager(client, nil, "", "", false)

	err := manager.Ping(context.Background())
	assert.NoError(t, err)
}

func TestManager_GetCurrentConfig(t *testing.T) {
	// Mock Caddy Admin API
	caddyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/config/" && r.Method == "GET" {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"apps": {"http": {}}}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer caddyServer.Close()

	client := NewClient(caddyServer.URL)
	manager := NewManager(client, nil, "", "", false)

	config, err := manager.GetCurrentConfig(context.Background())
	assert.NoError(t, err)
	assert.NotNil(t, config)
	assert.NotNil(t, config.Apps)
	assert.NotNil(t, config.Apps.HTTP)
}

func TestManager_RotateSnapshots(t *testing.T) {
	// Setup Manager
	tmpDir := t.TempDir()

	// Mock Caddy Admin API (Success)
	caddyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer caddyServer.Close()

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.ProxyHost{}, &models.Location{}, &models.Setting{}, &models.CaddyConfig{}, &models.SSLCertificate{}, &models.ForwardAuthConfig{}))

	client := NewClient(caddyServer.URL)
	manager := NewManager(client, db, tmpDir, "", false)

	// Create 15 dummy config files
	for i := 0; i < 15; i++ {
		// Use past timestamps
		ts := time.Now().Add(-time.Duration(i+1) * time.Minute).Unix()
		fname := fmt.Sprintf("config-%d.json", ts)
		f, _ := os.Create(filepath.Join(tmpDir, fname))
		f.Close()
	}

	// Call ApplyConfig once
	err = manager.ApplyConfig(context.Background())
	assert.NoError(t, err)

	// Check number of files
	files, _ := os.ReadDir(tmpDir)

	// Count files matching config-*.json
	count := 0
	for _, f := range files {
		if filepath.Ext(f.Name()) == ".json" {
			count++
		}
	}
	// Should be 10 (kept)
	assert.Equal(t, 10, count)
}

func TestManager_Rollback_Success(t *testing.T) {
	// Mock Caddy Admin API
	// First call succeeds (initial setup), second call fails (bad config), third call succeeds (rollback)
	callCount := 0
	caddyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if r.URL.Path == "/load" && r.Method == "POST" {
			if callCount == 2 {
				w.WriteHeader(http.StatusInternalServerError) // Fail the second apply
				return
			}
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer caddyServer.Close()

	// Setup DB
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.ProxyHost{}, &models.Location{}, &models.Setting{}, &models.CaddyConfig{}, &models.SSLCertificate{}, &models.ForwardAuthConfig{}))

	// Setup Manager
	tmpDir := t.TempDir()
	client := NewClient(caddyServer.URL)
	manager := NewManager(client, db, tmpDir, "", false)

	// 1. Apply valid config (creates snapshot)
	host1 := models.ProxyHost{
		UUID:        "uuid-1",
		DomainNames: "example.com",
		ForwardHost: "127.0.0.1",
		ForwardPort: 8080,
	}
	db.Create(&host1)
	err = manager.ApplyConfig(context.Background())
	assert.NoError(t, err)

	// Verify snapshot exists
	snapshots, _ := manager.listSnapshots()
	assert.Len(t, snapshots, 1)

	// Sleep to ensure different timestamp for next snapshot
	time.Sleep(1100 * time.Millisecond)

	// 2. Apply another config (will fail at Caddy level)
	host2 := models.ProxyHost{
		UUID:        "uuid-2",
		DomainNames: "fail.com",
		ForwardHost: "127.0.0.1",
		ForwardPort: 8081,
	}
	db.Create(&host2)

	// This should fail, trigger rollback, and succeed in rolling back
	err = manager.ApplyConfig(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "apply failed (rolled back)")

	// Verify we still have 1 snapshot (the failed one was removed)
	snapshots, _ = manager.listSnapshots()
	assert.Len(t, snapshots, 1)
}

func TestManager_ApplyConfig_DBError(t *testing.T) {
	// Setup DB
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.ProxyHost{}, &models.Location{}, &models.Setting{}, &models.CaddyConfig{}, &models.SSLCertificate{}, &models.ForwardAuthConfig{}))

	// Setup Manager
	tmpDir := t.TempDir()
	client := NewClient("http://localhost")
	manager := NewManager(client, db, tmpDir, "", false)

	// Close DB to force error
	sqlDB, _ := db.DB()
	sqlDB.Close()

	err = manager.ApplyConfig(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "fetch proxy hosts")
}

func TestManager_ApplyConfig_ValidationError(t *testing.T) {
	// Setup DB
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.ProxyHost{}, &models.Location{}, &models.Setting{}, &models.CaddyConfig{}, &models.SSLCertificate{}, &models.ForwardAuthConfig{}))

	// Setup Manager with a file as configDir to force saveSnapshot error
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, "config-file")
	os.WriteFile(configDir, []byte("not a dir"), 0644)

	client := NewClient("http://localhost")
	manager := NewManager(client, db, configDir, "", false)

	host := models.ProxyHost{
		DomainNames: "example.com",
		ForwardHost: "127.0.0.1",
		ForwardPort: 8080,
	}
	db.Create(&host)

	err = manager.ApplyConfig(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "save snapshot")
}

func TestManager_Rollback_Failure(t *testing.T) {
	// Mock Caddy Admin API - Always Fail
	caddyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer caddyServer.Close()

	// Setup DB
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.ProxyHost{}, &models.Location{}, &models.Setting{}, &models.CaddyConfig{}, &models.SSLCertificate{}, &models.ForwardAuthConfig{}))

	// Setup Manager
	tmpDir := t.TempDir()
	client := NewClient(caddyServer.URL)
	manager := NewManager(client, db, tmpDir, "", false)

	// Create a dummy snapshot manually so rollback has something to try
	os.WriteFile(filepath.Join(tmpDir, "config-123.json"), []byte("{}"), 0644)

	// Apply Config - will fail, try rollback, rollback will fail
	err = manager.ApplyConfig(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "rollback also failed")
}
