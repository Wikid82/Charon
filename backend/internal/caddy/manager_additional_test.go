package caddy

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestManager_ListSnapshots_ReadDirError(t *testing.T) {
	// Use a path that does not exist
	tmp := t.TempDir()
	// create manager with a non-existent subdir
	manager := NewManager(nil, nil, filepath.Join(tmp, "nope"), "", false)
	_, err := manager.listSnapshots()
	assert.Error(t, err)
}

func TestManager_RotateSnapshots_NoOp(t *testing.T) {
	tmp := t.TempDir()
	manager := NewManager(nil, nil, tmp, "", false)
	// No snapshots exist; should be no error
	err := manager.rotateSnapshots(10)
	assert.NoError(t, err)
}

func TestManager_Rollback_NoSnapshots(t *testing.T) {
	tmp := t.TempDir()
	manager := NewManager(nil, nil, tmp, "", false)
	err := manager.rollback(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no snapshots available")
}

func TestManager_Rollback_UnmarshalError(t *testing.T) {
	tmp := t.TempDir()
	// Write a non-JSON file with .json extension
	p := filepath.Join(tmp, "config-123.json")
	os.WriteFile(p, []byte("not json"), 0644)
	manager := NewManager(nil, nil, tmp, "", false)
	// Reader error should happen before client.Load
	err := manager.rollback(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unmarshal snapshot")
}

func TestManager_Rollback_LoadSnapshotFail(t *testing.T) {
	// Create a valid JSON file and set client to return error for /load
	tmp := t.TempDir()
	p := filepath.Join(tmp, "config-123.json")
	os.WriteFile(p, []byte(`{"apps":{"http":{}}}`), 0644)

	// Mock client that returns error on Load
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/load" && r.Method == http.MethodPost {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	badClient := NewClient(server.URL)
	manager := NewManager(badClient, nil, tmp, "", false)
	err := manager.rollback(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "load snapshot")
}

func TestManager_SaveSnapshot_WriteError(t *testing.T) {
	// Create a file at path to use as configDir, so writes fail
	tmp := t.TempDir()
	notDir := filepath.Join(tmp, "file-not-dir")
	os.WriteFile(notDir, []byte("data"), 0644)
	manager := NewManager(nil, nil, notDir, "", false)
	_, err := manager.saveSnapshot(&Config{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "write snapshot")
}

func TestBackupCaddyfile_MkdirAllFailure(t *testing.T) {
	tmp := t.TempDir()
	originalFile := filepath.Join(tmp, "Caddyfile")
	os.WriteFile(originalFile, []byte("original"), 0644)
	// Create a file where the backup dir should be to cause MkdirAll to fail
	badDir := filepath.Join(tmp, "notadir")
	os.WriteFile(badDir, []byte("data"), 0644)

	_, err := BackupCaddyfile(originalFile, badDir)
	assert.Error(t, err)
}

// Note: Deletion failure for rotateSnapshots is difficult to reliably simulate across environments
// (tests run as root in CI and local dev containers). If needed, add platform-specific tests.

func TestManager_SaveSnapshot_Success(t *testing.T) {
	tmp := t.TempDir()
	manager := NewManager(nil, nil, tmp, "", false)
	path, err := manager.saveSnapshot(&Config{})
	assert.NoError(t, err)
	assert.FileExists(t, path)
}

func TestManager_ApplyConfig_WithSettings(t *testing.T) {
	// Mock Caddy Admin API
	caddyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/load" && r.Method == http.MethodPost {
			w.WriteHeader(http.StatusOK)
			return
		}
		if r.URL.Path == "/config/" && r.Method == http.MethodGet {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("{\"apps\":{\"http\":{}}}"))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer caddyServer.Close()

	// Setup DB
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	assert.NoError(t, err)
	assert.NoError(t, db.AutoMigrate(&models.ProxyHost{}, &models.Location{}, &models.Setting{}, &models.CaddyConfig{}, &models.SSLCertificate{}))

	// Create settings for acme email and ssl provider
	db.Create(&models.Setting{Key: "caddy.acme_email", Value: "admin@example.com"})
	db.Create(&models.Setting{Key: "caddy.ssl_provider", Value: "zerossl"})

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

// Skipping rotate snapshot-on-apply warning test — rotation errors are non-fatal and environment
// dependent. We cover rotateSnapshots failure separately below.

func TestManager_RotateSnapshots_ListDirError(t *testing.T) {
	manager := NewManager(nil, nil, filepath.Join(t.TempDir(), "nope"), "", false)
	err := manager.rotateSnapshots(10)
	assert.Error(t, err)
}

func TestManager_RotateSnapshots_DeletesOld(t *testing.T) {
	tmp := t.TempDir()
	// create 5 snapshot files with different timestamps
	for i := 1; i <= 5; i++ {
		name := fmt.Sprintf("config-%d.json", i)
		p := filepath.Join(tmp, name)
		os.WriteFile(p, []byte("{}"), 0644)
		// tweak mod time
		os.Chtimes(p, time.Now().Add(time.Duration(i)*time.Second), time.Now().Add(time.Duration(i)*time.Second))
	}

	manager := NewManager(nil, nil, tmp, "", false)
	// Keep last 2 snapshots
	err := manager.rotateSnapshots(2)
	assert.NoError(t, err)

	// Ensure only 2 files remain
	files, _ := os.ReadDir(tmp)
	var cnt int
	for _, f := range files {
		if filepath.Ext(f.Name()) == ".json" {
			cnt++
		}
	}
	assert.Equal(t, 2, cnt)
}

func TestManager_ApplyConfig_RotateSnapshotsWarning(t *testing.T) {
	// Setup DB and Caddy server that accepts load
	caddyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/load" && r.Method == http.MethodPost {
			w.WriteHeader(http.StatusOK)
			return
		}
		if r.URL.Path == "/config/" && r.Method == http.MethodGet {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("{" + "\"apps\":{\"http\":{}}}"))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer caddyServer.Close()

	// Setup DB
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	assert.NoError(t, err)
	assert.NoError(t, db.AutoMigrate(&models.ProxyHost{}, &models.Location{}, &models.Setting{}, &models.CaddyConfig{}, &models.SSLCertificate{}))

	// Create a host so GenerateConfig produces a config
	host := models.ProxyHost{DomainNames: "rot.example.com", ForwardHost: "127.0.0.1", ForwardPort: 8080, Enabled: true}
	db.Create(&host)

	// Create manager with a configDir that is not readable (non-existent subdir)
	tmp := t.TempDir()
	// Create snapshot files: make the oldest a non-empty directory to force delete error;
	// generate 11 snapshots so rotateSnapshots(10) will attempt to delete 1
	d1 := filepath.Join(tmp, "config-1.json")
	os.MkdirAll(d1, 0755)
	os.WriteFile(filepath.Join(d1, "inner"), []byte("x"), 0644) // non-empty
	for i := 2; i <= 11; i++ {
		os.WriteFile(filepath.Join(tmp, fmt.Sprintf("config-%d.json", i)), []byte("{}"), 0644)
	}
	// Set modification times to ensure config-1.json is oldest
	for i := 1; i <= 11; i++ {
		p := filepath.Join(tmp, fmt.Sprintf("config-%d.json", i))
		if i == 1 {
			p = d1
		}
		tmo := time.Now().Add(time.Duration(-i) * time.Minute)
		os.Chtimes(p, tmo, tmo)
	}

	client := NewClient(caddyServer.URL)
	manager := NewManager(client, db, tmp, "", false)

	// ApplyConfig should succeed even if rotateSnapshots later returns an error
	err = manager.ApplyConfig(context.Background())
	assert.NoError(t, err)
}

func TestManager_ApplyConfig_LoadFailsAndRollbackFails(t *testing.T) {
	// Mock Caddy admin API which returns error for /load so ApplyConfig fails
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/load" && r.Method == http.MethodPost {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if r.URL.Path == "/config/" && r.Method == http.MethodGet {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("{" + "\"apps\":{\"http\":{}}}"))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	// Setup DB
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	assert.NoError(t, err)
	assert.NoError(t, db.AutoMigrate(&models.ProxyHost{}, &models.Location{}, &models.Setting{}, &models.CaddyConfig{}, &models.SSLCertificate{}))

	// Create a host so GenerateConfig produces a config
	host := models.ProxyHost{DomainNames: "fail.example.com", ForwardHost: "127.0.0.1", ForwardPort: 8080, Enabled: true}
	db.Create(&host)

	tmp := t.TempDir()
	client := NewClient(server.URL)
	manager := NewManager(client, db, tmp, "", false)

	err = manager.ApplyConfig(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "apply failed")
}

func TestManager_ApplyConfig_SaveSnapshotFails(t *testing.T) {
	// Setup DB and Caddy server that accepts load
	caddyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/load" && r.Method == http.MethodPost {
			w.WriteHeader(http.StatusOK)
			return
		}
		if r.URL.Path == "/config/" && r.Method == http.MethodGet {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("{" + "\"apps\":{\"http\":{}}}"))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer caddyServer.Close()

	// Setup DB
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name()+"savefail")
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	assert.NoError(t, err)
	assert.NoError(t, db.AutoMigrate(&models.ProxyHost{}, &models.Location{}, &models.Setting{}, &models.CaddyConfig{}, &models.SSLCertificate{}))

	// Create a host so GenerateConfig produces a config
	host := models.ProxyHost{DomainNames: "savefail.example.com", ForwardHost: "127.0.0.1", ForwardPort: 8080, Enabled: true}
	db.Create(&host)

	// Create a file where configDir should be to cause saveSnapshot to fail
	tmp := t.TempDir()
	filePath := filepath.Join(tmp, "file-not-dir")
	os.WriteFile(filePath, []byte("data"), 0644)

	client := NewClient(caddyServer.URL)
	manager := NewManager(client, db, filePath, "", false)

	err = manager.ApplyConfig(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "save snapshot")
}

func TestManager_ApplyConfig_LoadFailsThenRollbackSucceeds(t *testing.T) {
	// Create a server that fails the first /load but succeeds on the second /load
	var callCount int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/load" && r.Method == http.MethodPost {
			callCount++
			if callCount == 1 {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusOK)
			return
		}
		if r.URL.Path == "/config/" && r.Method == http.MethodGet {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("{" + "\"apps\":{\"http\":{}}}"))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name()+"rollbackok")
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	assert.NoError(t, err)
	assert.NoError(t, db.AutoMigrate(&models.ProxyHost{}, &models.Location{}, &models.Setting{}, &models.CaddyConfig{}, &models.SSLCertificate{}))

	// Create a host
	host := models.ProxyHost{DomainNames: "rb.example.com", ForwardHost: "127.0.0.1", ForwardPort: 8080, Enabled: true}
	db.Create(&host)

	tmp := t.TempDir()
	client := NewClient(server.URL)
	manager := NewManager(client, db, tmp, "", false)

	err = manager.ApplyConfig(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "apply failed")
}

func TestManager_SaveSnapshot_MarshalError(t *testing.T) {
	tmp := t.TempDir()
	manager := NewManager(nil, nil, tmp, "", false)
	// Stub jsonMarshallFunc to return error
	orig := jsonMarshalFunc
	jsonMarshalFunc = func(v interface{}, prefix, indent string) ([]byte, error) {
		return nil, fmt.Errorf("marshal fail")
	}
	defer func() { jsonMarshalFunc = orig }()

	_, err := manager.saveSnapshot(&Config{})
	assert.Error(t, err)
}

func TestManager_RotateSnapshots_DeleteError(t *testing.T) {
	tmp := t.TempDir()
	// Create three files to remove one
	for i := 1; i <= 3; i++ {
		p := filepath.Join(tmp, fmt.Sprintf("config-%d.json", i))
		os.WriteFile(p, []byte("{}"), 0644)
		os.Chtimes(p, time.Now().Add(time.Duration(i)*time.Second), time.Now().Add(time.Duration(i)*time.Second))
	}

	manager := NewManager(nil, nil, tmp, "", false)
	// Stub removeFileFunc to return error for specific path
	origRemove := removeFileFunc
	removeFileFunc = func(p string) error {
		if filepath.Base(p) == "config-1.json" {
			return fmt.Errorf("cannot delete")
		}
		return origRemove(p)
	}
	defer func() { removeFileFunc = origRemove }()

	err := manager.rotateSnapshots(2)
	assert.Error(t, err)
}

func TestManager_ApplyConfig_GenerateConfigFails(t *testing.T) {
	tmp := t.TempDir()
	// Setup DB - minimal
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name()+"genfail")
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	assert.NoError(t, err)
	assert.NoError(t, db.AutoMigrate(&models.ProxyHost{}, &models.Location{}, &models.Setting{}, &models.CaddyConfig{}, &models.SSLCertificate{}))

	// Create a host so ApplyConfig tries to generate config
	host := models.ProxyHost{DomainNames: "x.example.com", ForwardHost: "127.0.0.1", ForwardPort: 8080, Enabled: true}
	db.Create(&host)

	// stub generateConfigFunc to always return error
	orig := generateConfigFunc
	generateConfigFunc = func(hosts []models.ProxyHost, storageDir string, acmeEmail string, frontendDir string, sslProvider string, acmeStaging bool) (*Config, error) {
		return nil, fmt.Errorf("generate fail")
	}
	defer func() { generateConfigFunc = orig }()

	manager := NewManager(nil, db, tmp, "", false)
	err = manager.ApplyConfig(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "generate config")
}

func TestManager_ApplyConfig_ValidateFails(t *testing.T) {
	tmp := t.TempDir()
	// Setup DB - minimal
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name()+"valfail")
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	assert.NoError(t, err)
	assert.NoError(t, db.AutoMigrate(&models.ProxyHost{}, &models.Location{}, &models.Setting{}, &models.CaddyConfig{}, &models.SSLCertificate{}))

	// Create a host so ApplyConfig tries to generate config
	host := models.ProxyHost{DomainNames: "y.example.com", ForwardHost: "127.0.0.1", ForwardPort: 8080, Enabled: true}
	db.Create(&host)

	// Stub validate function to return error
	orig := validateConfigFunc
	validateConfigFunc = func(cfg *Config) error { return fmt.Errorf("validation failed stub") }
	defer func() { validateConfigFunc = orig }()

	// Use a working client so generation succeeds
	// Mock Caddy admin API that accepts loads
	caddyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/load" && r.Method == http.MethodPost {
			w.WriteHeader(http.StatusOK)
			return
		}
		if r.URL.Path == "/config/" && r.Method == http.MethodGet {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("{" + "\"apps\":{\"http\":{}}}"))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer caddyServer.Close()

	client := NewClient(caddyServer.URL)
	manager := NewManager(client, db, tmp, "", false)

	err = manager.ApplyConfig(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "validation failed")
}

func TestManager_Rollback_ReadFileError(t *testing.T) {
	tmp := t.TempDir()
	manager := NewManager(nil, nil, tmp, "", false)
	// Create snapshot entries via write
	p := filepath.Join(tmp, "config-123.json")
	os.WriteFile(p, []byte(`{"apps":{"http":{}}}`), 0644)
	// Stub readFileFunc to return error
	origRead := readFileFunc
	readFileFunc = func(p string) ([]byte, error) { return nil, fmt.Errorf("read error") }
	defer func() { readFileFunc = origRead }()

	err := manager.rollback(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "read snapshot")
}

func TestManager_ApplyConfig_RotateSnapshotsWarning_Stderr(t *testing.T) {
	// Setup minimal DB and client that accepts load
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name()+"rotwarn")
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	assert.NoError(t, err)
	assert.NoError(t, db.AutoMigrate(&models.ProxyHost{}, &models.Location{}, &models.Setting{}, &models.CaddyConfig{}, &models.SSLCertificate{}))
	host := models.ProxyHost{DomainNames: "rotwarn.example.com", ForwardHost: "127.0.0.1", ForwardPort: 8080, Enabled: true}
	db.Create(&host)

	// Setup Caddy server
	caddyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/load" && r.Method == http.MethodPost {
			w.WriteHeader(http.StatusOK)
			return
		}
		if r.URL.Path == "/config/" && r.Method == http.MethodGet {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("{" + "\"apps\":{\"http\":{}}}"))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer caddyServer.Close()

	// stub readDirFunc to return error to cause rotateSnapshots to fail
	origReadDir := readDirFunc
	readDirFunc = func(path string) ([]os.DirEntry, error) { return nil, fmt.Errorf("dir read fail") }
	defer func() { readDirFunc = origReadDir }()

	client := NewClient(caddyServer.URL)
	manager := NewManager(client, db, t.TempDir(), "", false)
	err = manager.ApplyConfig(context.Background())
	// Should succeed despite rotation warning (non-fatal)
	assert.NoError(t, err)
}
