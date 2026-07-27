package handlers

import (
	"net/http"
	"time"

	"github.com/Wikid82/charon/backend/internal/database"
	"github.com/Wikid82/charon/backend/internal/services"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// DBHealthHandler provides database health check endpoints.
type DBHealthHandler struct {
	db            *gorm.DB
	backupService *services.BackupService
	// dbPath is the on-disk path to the main SQLite database file, used so
	// Check can run its integrity scan on a dedicated connection instead of
	// the shared db pool (spec §2.5d/§3.9 — Async Backup/Restore Jobs).
	dbPath string
}

// DBHealthResponse represents the database health check response.
type DBHealthResponse struct {
	Status          string     `json:"status"`
	IntegrityOK     bool       `json:"integrity_ok"`
	IntegrityResult string     `json:"integrity_result"`
	WALMode         bool       `json:"wal_mode"`
	JournalMode     string     `json:"journal_mode"`
	LastBackup      *time.Time `json:"last_backup"`
	CheckedAt       time.Time  `json:"checked_at"`
}

// NewDBHealthHandler creates a new DBHealthHandler. dbPath is the on-disk
// path to the main SQLite database file (spec §2.5d/§3.9) — Check uses it
// to run the integrity scan on a dedicated connection rather than the
// shared db pool.
func NewDBHealthHandler(db *gorm.DB, backupService *services.BackupService, dbPath string) *DBHealthHandler {
	return &DBHealthHandler{
		db:            db,
		backupService: backupService,
		dbPath:        dbPath,
	}
}

// Check performs a database health check.
// GET /api/v1/health/db
// Returns 200 if healthy, 503 if corrupted.
func (h *DBHealthHandler) Check(c *gin.Context) {
	response := DBHealthResponse{
		CheckedAt: time.Now().UTC(),
	}

	// Run integrity check on a dedicated connection (spec §2.5d/§3.9) — this
	// endpoint is registered unauthenticated on the bare router
	// (routes.go), and the previous database.CheckIntegrity(h.db) call ran
	// its PRAGMA quick_check on the shared, SetMaxOpenConns(1) main pool,
	// which could block the entire app for the duration of a multi-minute
	// scan on a large database.
	integrityOK, integrityResult := database.CheckIntegrityDedicated(h.dbPath)
	response.IntegrityOK = integrityOK
	response.IntegrityResult = integrityResult

	// Check journal mode
	var journalMode string
	if err := h.db.Raw("PRAGMA journal_mode").Scan(&journalMode).Error; err == nil {
		response.JournalMode = journalMode
		response.WALMode = journalMode == "wal"
	}

	// Get last backup time
	if h.backupService != nil {
		if lastBackup, err := h.backupService.GetLastBackupTime(); err == nil && !lastBackup.IsZero() {
			response.LastBackup = &lastBackup
		}
	}

	// Determine overall status
	if integrityOK {
		response.Status = "healthy"
		c.JSON(http.StatusOK, response)
	} else {
		response.Status = "corrupted"
		c.JSON(http.StatusServiceUnavailable, response)
	}
}
