package handlers

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// createValidSQLiteDB creates a minimal valid SQLite database for backup testing
func createValidSQLiteDB(t *testing.T, dbPath string) error {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		return err
	}
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	defer func() { _ = sqlDB.Close() }()

	// Create a simple table to make it a valid database, plus the "users"
	// and "proxy_hosts" tables RestoreBackupSafe's V6 sanity check (spec
	// §3.5) requires of anything it restores.
	if err := db.Exec("CREATE TABLE IF NOT EXISTS test (id INTEGER PRIMARY KEY, data TEXT)").Error; err != nil {
		return err
	}
	if err := db.Exec("CREATE TABLE IF NOT EXISTS users (id INTEGER PRIMARY KEY, email TEXT)").Error; err != nil {
		return err
	}
	return db.Exec("CREATE TABLE IF NOT EXISTS proxy_hosts (id INTEGER PRIMARY KEY, domain_names TEXT)").Error
}

// Use a real BackupService, but point it at tmpDir for isolation

func TestBackupHandlerQuick(t *testing.T) {
	tmpDir := t.TempDir()
	// Create a valid SQLite database for backup operations
	dbPath := filepath.Join(tmpDir, "db.sqlite")
	if err := createValidSQLiteDB(t, dbPath); err != nil {
		t.Fatalf("failed to create tmp db: %v", err)
	}

	// Async job tracking (spec §3.1/§3.3.1) requires a real db —
	// StartCreateBackupJob/StartRestoreJob error out synchronously
	// otherwise. Reuse the same file createValidSQLiteDB just wrote to,
	// mirroring setupBackupTest's pattern elsewhere in this package.
	gdb, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, gdb.AutoMigrate(&models.BackupJob{}))
	t.Cleanup(func() {
		if sqlDB, dbErr := gdb.DB(); dbErr == nil {
			_ = sqlDB.Close()
		}
	})

	svc := services.NewBackupService(&config.Config{DatabasePath: dbPath}, gdb, nil)
	t.Cleanup(svc.Stop)
	h := NewBackupHandler(svc)

	r := gin.New()
	r.Use(func(c *gin.Context) {
		setAdminContext(c)
		c.Next()
	})
	// register routes used — under /api/v1, matching createBackupViaRouter/
	// restoreBackupViaRouter's hardcoded paths (backup_handler_test.go).
	r.GET("/api/v1/backups", h.List)
	r.POST("/api/v1/backups", h.Create)
	r.GET("/api/v1/backups/jobs/:job_id", h.GetJob)
	r.DELETE("/api/v1/backups/:filename", h.Delete)
	r.GET("/api/v1/backups/:filename", h.Download)
	r.POST("/api/v1/backups/:filename/restore", h.Restore)

	// List
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/backups", http.NoBody)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	// Create (backup) — async job contract, spec §3.2.1: 202 + job_id, then
	// poll until the job completes.
	filename := createBackupViaRouter(t, r, svc)

	// Delete missing
	w3 := httptest.NewRecorder()
	req3 := httptest.NewRequest(http.MethodDelete, "/api/v1/backups/missing", http.NoBody)
	r.ServeHTTP(w3, req3)
	if w3.Code != http.StatusNotFound {
		t.Fatalf("delete missing expected 404 got %d", w3.Code)
	}

	// Download missing
	w4 := httptest.NewRecorder()
	req4 := httptest.NewRequest(http.MethodGet, "/api/v1/backups/missing", http.NoBody)
	r.ServeHTTP(w4, req4)
	if w4.Code != http.StatusNotFound {
		t.Fatalf("download missing expected 404 got %d", w4.Code)
	}

	// Download present (use filename returned from create)
	w5 := httptest.NewRecorder()
	req5 := httptest.NewRequest(http.MethodGet, "/api/v1/backups/"+filename, http.NoBody)
	r.ServeHTTP(w5, req5)
	if w5.Code != http.StatusOK {
		t.Fatalf("download expected 200 got %d", w5.Code)
	}

	// Restore missing — stays a synchronous 404 (spec §3.3.1's
	// ErrBackupNotFound fix).
	w6 := httptest.NewRecorder()
	req6 := httptest.NewRequest(http.MethodPost, "/api/v1/backups/missing/restore", http.NoBody)
	r.ServeHTTP(w6, req6)
	if w6.Code != http.StatusNotFound {
		t.Fatalf("restore missing expected 404 got %d", w6.Code)
	}

	// Restore ok — async job contract, spec §3.2.2.
	restoreResult := restoreBackupViaRouter(t, r, svc, filename)
	if _, ok := restoreResult["message"]; !ok {
		t.Fatalf("expected restore result to contain a message, got %#v", restoreResult)
	}
}
