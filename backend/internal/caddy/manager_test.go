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
	require.NoError(t, db.AutoMigrate(&models.ProxyHost{}, &models.Setting{}, &models.CaddyConfig{}))

	// Setup Manager
	tmpDir := t.TempDir()
	client := NewClient(caddyServer.URL)
	manager := NewManager(client, db, tmpDir)

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
	require.NoError(t, db.AutoMigrate(&models.ProxyHost{}, &models.Setting{}, &models.CaddyConfig{}))

	// Setup Manager
	tmpDir := t.TempDir()
	client := NewClient(caddyServer.URL)
	manager := NewManager(client, db, tmpDir)

	// Create a host
	host := models.ProxyHost{
		DomainNames: "example.com",
		ForwardHost: "127.0.0.1",
		ForwardPort: 8080,
	}
	db.Create(&host)

	// Apply Config - Should fail and trigger rollback
	// Since we mock failure, rollback (which tries to apply the same config) will also fail.
	err = manager.ApplyConfig(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "apply failed")
	assert.Contains(t, err.Error(), "rollback also failed")

	// Check if failure was recorded in DB
	// Since rollback failed, recordConfigChange is NOT called.
	var configLog models.CaddyConfig
	err = db.First(&configLog).Error
	assert.Error(t, err) // Should be record not found
	assert.Equal(t, gorm.ErrRecordNotFound, err)
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
	require.NoError(t, db.AutoMigrate(&models.ProxyHost{}, &models.Setting{}, &models.CaddyConfig{}))

	client := NewClient(caddyServer.URL)
	manager := NewManager(client, db, tmpDir)

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
