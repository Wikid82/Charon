package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/Wikid82/charon/backend/internal/services"
	"github.com/gin-gonic/gin"
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

	// Create a simple table to make it a valid database
	return db.Exec("CREATE TABLE IF NOT EXISTS test (id INTEGER PRIMARY KEY, data TEXT)").Error
}

// Use a real BackupService, but point it at tmpDir for isolation

func TestBackupHandlerQuick(t *testing.T) {
	tmpDir := t.TempDir()
	// Create a valid SQLite database for backup operations
	dbPath := filepath.Join(tmpDir, "db.sqlite")
	if err := createValidSQLiteDB(t, dbPath); err != nil {
		t.Fatalf("failed to create tmp db: %v", err)
	}

	svc := &services.BackupService{DataDir: tmpDir, BackupDir: tmpDir, DatabaseName: "db.sqlite", Cron: nil}
	h := NewBackupHandler(svc)

	r := gin.New()
	r.Use(func(c *gin.Context) {
		setAdminContext(c)
		c.Next()
	})
	// register routes used
	r.GET("/backups", h.List)
	r.POST("/backups", h.Create)
	r.DELETE("/backups/:filename", h.Delete)
	r.GET("/backups/:filename", h.Download)
	r.POST("/backups/:filename/restore", h.Restore)

	// List
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/backups", http.NoBody)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	// Create (backup)
	w2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodPost, "/backups", http.NoBody)
	r.ServeHTTP(w2, req2)
	if w2.Code != http.StatusCreated {
		t.Fatalf("create expected 201 got %d", w2.Code)
	}

	var createResp struct {
		Filename string `json:"filename"`
	}
	if err := json.Unmarshal(w2.Body.Bytes(), &createResp); err != nil {
		t.Fatalf("invalid create json: %v", err)
	}

	// Delete missing
	w3 := httptest.NewRecorder()
	req3 := httptest.NewRequest(http.MethodDelete, "/backups/missing", http.NoBody)
	r.ServeHTTP(w3, req3)
	if w3.Code != http.StatusNotFound {
		t.Fatalf("delete missing expected 404 got %d", w3.Code)
	}

	// Download missing
	w4 := httptest.NewRecorder()
	req4 := httptest.NewRequest(http.MethodGet, "/backups/missing", http.NoBody)
	r.ServeHTTP(w4, req4)
	if w4.Code != http.StatusNotFound {
		t.Fatalf("download missing expected 404 got %d", w4.Code)
	}

	// Download present (use filename returned from create)
	w5 := httptest.NewRecorder()
	req5 := httptest.NewRequest(http.MethodGet, "/backups/"+createResp.Filename, http.NoBody)
	r.ServeHTTP(w5, req5)
	if w5.Code != http.StatusOK {
		t.Fatalf("download expected 200 got %d", w5.Code)
	}

	// Restore missing
	w6 := httptest.NewRecorder()
	req6 := httptest.NewRequest(http.MethodPost, "/backups/missing/restore", http.NoBody)
	r.ServeHTTP(w6, req6)
	if w6.Code != http.StatusNotFound {
		t.Fatalf("restore missing expected 404 got %d", w6.Code)
	}

	// Restore ok
	w7 := httptest.NewRecorder()
	req7 := httptest.NewRequest(http.MethodPost, "/backups/"+createResp.Filename+"/restore", http.NoBody)
	r.ServeHTTP(w7, req7)
	if w7.Code != http.StatusOK {
		t.Fatalf("restore expected 200 got %d", w7.Code)
	}
}
