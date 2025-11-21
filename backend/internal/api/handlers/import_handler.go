package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/Wikid82/CaddyProxyManagerPlus/backend/internal/caddy"
	"github.com/Wikid82/CaddyProxyManagerPlus/backend/internal/models"
	"github.com/Wikid82/CaddyProxyManagerPlus/backend/internal/services"
)

// ImportHandler handles Caddyfile import operations.
type ImportHandler struct {
	db              *gorm.DB
	proxyHostSvc    *services.ProxyHostService
	importerservice *caddy.Importer
	importDir       string
}

// NewImportHandler creates a new import handler.
func NewImportHandler(db *gorm.DB, caddyBinary, importDir string) *ImportHandler {
	return &ImportHandler{
		db:              db,
		proxyHostSvc:    services.NewProxyHostService(db),
		importerservice: caddy.NewImporter(caddyBinary),
		importDir:       importDir,
	}
}

// RegisterRoutes registers import-related routes.
func (h *ImportHandler) RegisterRoutes(router *gin.RouterGroup) {
	router.GET("/import/status", h.GetStatus)
	router.GET("/import/preview", h.GetPreview)
	router.POST("/import/upload", h.Upload)
	router.POST("/import/commit", h.Commit)
	router.DELETE("/import/cancel", h.Cancel)
}

// GetStatus returns current import session status.
func (h *ImportHandler) GetStatus(c *gin.Context) {
	var session models.ImportSession
	err := h.db.Where("status IN ?", []string{"pending", "reviewing"}).
		Order("created_at DESC").
		First(&session).Error

	if err == gorm.ErrRecordNotFound {
		c.JSON(http.StatusOK, gin.H{"has_pending": false})
		return
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"has_pending": true,
		"session": gin.H{
			"id":         session.UUID,
			"state":      session.Status,
			"created_at": session.CreatedAt,
			"updated_at": session.UpdatedAt,
		},
	})
}

// GetPreview returns parsed hosts and conflicts for review.
func (h *ImportHandler) GetPreview(c *gin.Context) {
	var session models.ImportSession
	err := h.db.Where("status IN ?", []string{"pending", "reviewing"}).
		Order("created_at DESC").
		First(&session).Error

	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "no pending import"})
		return
	}

	var result caddy.ImportResult
	if err := json.Unmarshal([]byte(session.ParsedData), &result); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to parse import data"})
		return
	}

	// Update status to reviewing
	session.Status = "reviewing"
	h.db.Save(&session)

	c.JSON(http.StatusOK, gin.H{
		"session": gin.H{
			"id":         session.UUID,
			"state":      session.Status,
			"created_at": session.CreatedAt,
			"updated_at": session.UpdatedAt,
		},
		"preview": result,
	})
}

// Upload handles manual Caddyfile upload or paste.
func (h *ImportHandler) Upload(c *gin.Context) {
	var req struct {
		Content  string `json:"content" binding:"required"`
		Filename string `json:"filename"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Create temporary file
	tempPath := filepath.Join(h.importDir, fmt.Sprintf("upload-%s.caddyfile", uuid.NewString()))
	if err := os.MkdirAll(h.importDir, 0755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create import directory"})
		return
	}

	if err := os.WriteFile(tempPath, []byte(req.Content), 0644); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to write upload"})
		return
	}

	// Process the uploaded file
	if err := h.processImport(tempPath, req.Filename); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "upload processed, ready for review"})
}

// Commit finalizes the import with user's conflict resolutions.
func (h *ImportHandler) Commit(c *gin.Context) {
	var req struct {
		SessionUUID string            `json:"session_uuid" binding:"required"`
		Resolutions map[string]string `json:"resolutions"` // domain -> action (skip, rename, merge)
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var session models.ImportSession
	if err := h.db.Where("uuid = ? AND status = ?", req.SessionUUID, "reviewing").First(&session).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "session not found or not in reviewing state"})
		return
	}

	var result caddy.ImportResult
	if err := json.Unmarshal([]byte(session.ParsedData), &result); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to parse import data"})
		return
	}

	// Convert parsed hosts to ProxyHost models
	proxyHosts := caddy.ConvertToProxyHosts(result.Hosts)

	created := 0
	skipped := 0
	errors := []string{}

	for _, host := range proxyHosts {
		action := req.Resolutions[host.DomainNames]

		if action == "skip" {
			skipped++
			continue
		}

		if action == "rename" {
			host.DomainNames = host.DomainNames + "-imported"
		}

		host.UUID = uuid.NewString()

		if err := h.proxyHostSvc.Create(&host); err != nil {
			errors = append(errors, fmt.Sprintf("%s: %s", host.DomainNames, err.Error()))
		} else {
			created++
		}
	}

	// Mark session as committed
	now := time.Now()
	session.Status = "committed"
	session.CommittedAt = &now
	session.UserResolutions = string(mustMarshal(req.Resolutions))
	h.db.Save(&session)

	c.JSON(http.StatusOK, gin.H{
		"created": created,
		"skipped": skipped,
		"errors":  errors,
	})
}

// Cancel discards a pending import session.
func (h *ImportHandler) Cancel(c *gin.Context) {
	sessionUUID := c.Query("session_uuid")
	if sessionUUID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "session_uuid required"})
		return
	}

	var session models.ImportSession
	if err := h.db.Where("uuid = ?", sessionUUID).First(&session).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "session not found"})
		return
	}

	session.Status = "rejected"
	h.db.Save(&session)

	c.JSON(http.StatusOK, gin.H{"message": "import cancelled"})
}

// processImport handles the import logic for both mounted and uploaded files.
func (h *ImportHandler) processImport(caddyfilePath, originalName string) error {
	// Validate Caddy binary
	if err := h.importerservice.ValidateCaddyBinary(); err != nil {
		return fmt.Errorf("caddy binary not available: %w", err)
	}

	// Parse and extract hosts
	result, err := h.importerservice.ImportFile(caddyfilePath)
	if err != nil {
		return fmt.Errorf("import failed: %w", err)
	}

	// Check for conflicts with existing hosts
	existingHosts, _ := h.proxyHostSvc.List()
	existingDomains := make(map[string]bool)
	for _, host := range existingHosts {
		existingDomains[host.DomainNames] = true
	}

	for _, parsed := range result.Hosts {
		if existingDomains[parsed.DomainNames] {
			result.Conflicts = append(result.Conflicts,
				fmt.Sprintf("Domain '%s' already exists in CPM+", parsed.DomainNames))
		}
	}

	// Create import session
	session := models.ImportSession{
		UUID:           uuid.NewString(),
		SourceFile:     originalName,
		Status:         "pending",
		ParsedData:     string(mustMarshal(result)),
		ConflictReport: string(mustMarshal(result.Conflicts)),
	}

	if err := h.db.Create(&session).Error; err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}

	// Backup original file
	if _, err := caddy.BackupCaddyfile(caddyfilePath, filepath.Join(h.importDir, "backups")); err != nil {
		// Non-fatal, log and continue
		fmt.Printf("Warning: failed to backup Caddyfile: %v\n", err)
	}

	return nil
}

// CheckMountedImport checks for mounted Caddyfile on startup.
func CheckMountedImport(db *gorm.DB, mountPath, caddyBinary, importDir string) error {
	if _, err := os.Stat(mountPath); os.IsNotExist(err) {
		return nil // No mounted file, skip
	}

	// Check if already processed
	var count int64
	db.Model(&models.ImportSession{}).Where("source_file = ? AND status IN ?",
		mountPath, []string{"pending", "reviewing", "committed"}).Count(&count)

	if count > 0 {
		return nil // Already processed
	}

	handler := NewImportHandler(db, caddyBinary, importDir)
	return handler.processImport(mountPath, mountPath)
}

func mustMarshal(v interface{}) []byte {
	b, _ := json.Marshal(v)
	return b
}
