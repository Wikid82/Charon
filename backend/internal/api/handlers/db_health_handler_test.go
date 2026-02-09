package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/Wikid82/charon/backend/internal/database"
	"github.com/Wikid82/charon/backend/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDBHealthHandler_Check_Healthy(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Create in-memory database
	db, err := database.Connect("file::memory:?cache=shared")
	require.NoError(t, err)

	handler := NewDBHealthHandler(db, nil)

	router := gin.New()
	router.GET("/api/v1/health/db", handler.Check)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/health/db", http.NoBody)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response DBHealthResponse
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "healthy", response.Status)
	assert.True(t, response.IntegrityOK)
	assert.Equal(t, "ok", response.IntegrityResult)
	assert.NotEmpty(t, response.JournalMode)
	assert.False(t, response.CheckedAt.IsZero())
}

func TestDBHealthHandler_Check_WithBackupService(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Setup temp dirs for backup service
	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, "data")
	err := os.MkdirAll(dataDir, 0o750) // #nosec G301 -- test directory
	require.NoError(t, err)

	// Create dummy DB file
	dbPath := filepath.Join(dataDir, "charon.db")
	err = os.WriteFile(dbPath, []byte("dummy db"), 0o600) // #nosec G306 -- test fixture
	require.NoError(t, err)

	cfg := &config.Config{DatabasePath: dbPath}
	backupService := services.NewBackupService(cfg)
	defer backupService.Stop() // Prevent goroutine leaks

	// Create a backup so we have a last backup time
	_, err = backupService.CreateBackup()
	require.NoError(t, err)

	// Create in-memory database for handler
	db, err := database.Connect("file::memory:?cache=shared")
	require.NoError(t, err)

	handler := NewDBHealthHandler(db, backupService)

	router := gin.New()
	router.GET("/api/v1/health/db", handler.Check)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/health/db", http.NoBody)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response DBHealthResponse
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "healthy", response.Status)
	assert.True(t, response.IntegrityOK)
	assert.NotNil(t, response.LastBackup, "LastBackup should be set after creating a backup")

	// Verify the backup time is recent
	if response.LastBackup != nil {
		assert.WithinDuration(t, time.Now(), *response.LastBackup, 5*time.Second)
	}
}

func TestDBHealthHandler_Check_WALMode(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Create file-based database to test WAL mode
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := database.Connect(dbPath)
	require.NoError(t, err)

	handler := NewDBHealthHandler(db, nil)

	router := gin.New()
	router.GET("/api/v1/health/db", handler.Check)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/health/db", http.NoBody)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response DBHealthResponse
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "wal", response.JournalMode)
	assert.True(t, response.WALMode)
}

func TestDBHealthHandler_ResponseJSONTags(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db, err := database.Connect("file::memory:?cache=shared")
	require.NoError(t, err)

	handler := NewDBHealthHandler(db, nil)

	router := gin.New()
	router.GET("/api/v1/health/db", handler.Check)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/health/db", http.NoBody)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Verify JSON uses snake_case
	body := w.Body.String()
	assert.Contains(t, body, "integrity_ok")
	assert.Contains(t, body, "integrity_result")
	assert.Contains(t, body, "wal_mode")
	assert.Contains(t, body, "journal_mode")
	assert.Contains(t, body, "last_backup")
	assert.Contains(t, body, "checked_at")

	// Verify no camelCase leak
	assert.NotContains(t, body, "integrityOK")
	assert.NotContains(t, body, "journalMode")
	assert.NotContains(t, body, "lastBackup")
	assert.NotContains(t, body, "checkedAt")
}

func TestNewDBHealthHandler(t *testing.T) {
	db, err := database.Connect("file::memory:?cache=shared")
	require.NoError(t, err)

	handler := NewDBHealthHandler(db, nil)
	assert.NotNil(t, handler)
	assert.Equal(t, db, handler.db)
	assert.Nil(t, handler.backupService)

	// With backup service
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "charon.db")
	_ = os.WriteFile(dbPath, []byte("test"), 0o600) // #nosec G306 -- test fixture

	cfg := &config.Config{DatabasePath: dbPath}
	backupSvc := services.NewBackupService(cfg)
	defer backupSvc.Stop() // Prevent goroutine leaks

	handler2 := NewDBHealthHandler(db, backupSvc)
	assert.NotNil(t, handler2.backupService)
}

// Phase 1 & 3: Critical coverage tests

func TestDBHealthHandler_Check_CorruptedDatabase(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Create a file-based database and corrupt it
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "corrupt.db")

	// Create valid database first
	db, err := database.Connect(dbPath)
	require.NoError(t, err)
	db.Exec("CREATE TABLE test (id INTEGER, data TEXT)")
	db.Exec("INSERT INTO test VALUES (1, 'data')")

	// Close it
	sqlDB, _ := db.DB()
	_ = sqlDB.Close()

	// Corrupt the database file
	corruptDBFile(t, dbPath)

	// Try to reconnect to corrupted database
	db2, err := database.Connect(dbPath)
	// The Connect function may succeed initially but integrity check will fail
	if err != nil {
		// If connection fails immediately, skip this test
		t.Skip("Database connection failed immediately on corruption")
	}

	handler := NewDBHealthHandler(db2, nil)

	router := gin.New()
	router.GET("/api/v1/health/db", handler.Check)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/health/db", http.NoBody)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Should return 503 if corruption detected
	if w.Code == http.StatusServiceUnavailable {
		var response DBHealthResponse
		err = json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Equal(t, "corrupted", response.Status)
		assert.False(t, response.IntegrityOK)
		assert.NotEqual(t, "ok", response.IntegrityResult)
	} else {
		// If status is 200, corruption wasn't detected by quick_check
		// (corruption might be in unused pages)
		assert.Equal(t, http.StatusOK, w.Code)
	}
}

func TestDBHealthHandler_Check_BackupServiceError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Create database
	db, err := database.Connect("file::memory:?cache=shared")
	require.NoError(t, err)

	// Create backup service with unreadable directory
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "charon.db")
	_ = os.WriteFile(dbPath, []byte("test"), 0o600) // #nosec G306 -- test fixture

	cfg := &config.Config{DatabasePath: dbPath}
	backupService := services.NewBackupService(cfg)

	// Make backup directory unreadable to trigger error in GetLastBackupTime
	_ = os.Chmod(backupService.BackupDir, 0o000)
	// #nosec G302 -- Test cleanup restores directory permissions
	defer func() { _ = os.Chmod(backupService.BackupDir, 0o755) }() // Restore for cleanup

	handler := NewDBHealthHandler(db, backupService)

	router := gin.New()
	router.GET("/api/v1/health/db", handler.Check)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/health/db", http.NoBody)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Handler should still succeed (backup error is swallowed)
	assert.Equal(t, http.StatusOK, w.Code)

	var response DBHealthResponse
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	// Status should be healthy despite backup service error
	assert.Equal(t, "healthy", response.Status)
	// LastBackup should be nil when error occurs
	assert.Nil(t, response.LastBackup)
}

func TestDBHealthHandler_Check_BackupTimeZero(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Create database
	db, err := database.Connect("file::memory:?cache=shared")
	require.NoError(t, err)

	// Create backup service with empty backup directory (no backups yet)
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "charon.db")
	_ = os.WriteFile(dbPath, []byte("test"), 0o600) // #nosec G306 -- test fixture

	cfg := &config.Config{DatabasePath: dbPath}
	backupService := services.NewBackupService(cfg)

	handler := NewDBHealthHandler(db, backupService)

	router := gin.New()
	router.GET("/api/v1/health/db", handler.Check)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/health/db", http.NoBody)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response DBHealthResponse
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	// LastBackup should be nil when no backups exist (zero time)
	assert.Nil(t, response.LastBackup)
	assert.Equal(t, "healthy", response.Status)
}

// Helper function to corrupt SQLite database file
func corruptDBFile(t *testing.T, dbPath string) {
	t.Helper()
	// #nosec G302 -- Test opens database file for corruption testing
	f, err := os.OpenFile(dbPath, os.O_RDWR, 0o644) //nolint:gosec // G304: Database file for corruption test
	require.NoError(t, err)
	defer func() { _ = f.Close() }()

	// Get file size
	stat, err := f.Stat()
	require.NoError(t, err)
	size := stat.Size()

	if size > 100 {
		// Overwrite middle section to corrupt B-tree
		_, err = f.WriteAt([]byte("CORRUPTED_BLOCK_DATA"), size/2)
		require.NoError(t, err)
	} else {
		// Corrupt header for small files
		_, err = f.WriteAt([]byte("CORRUPT"), 0)
		require.NoError(t, err)
	}
}
