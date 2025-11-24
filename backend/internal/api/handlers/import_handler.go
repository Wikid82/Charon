package handlers

import (
	"encoding/json"
	"fmt"
	"log"
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
	mountPath       string
}

// NewImportHandler creates a new import handler.
func NewImportHandler(db *gorm.DB, caddyBinary, importDir, mountPath string) *ImportHandler {
	return &ImportHandler{
		db:              db,
		proxyHostSvc:    services.NewProxyHostService(db),
		importerservice: caddy.NewImporter(caddyBinary),
		importDir:       importDir,
		mountPath:       mountPath,
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
		// No DB session, check if there's a mounted Caddyfile available for transient preview
		if h.mountPath != "" {
			if _, err := os.Stat(h.mountPath); err == nil {
				c.JSON(http.StatusOK, gin.H{
					"has_pending": true,
					"session": gin.H{
						"id":          "transient",
						"state":       "transient",
						"source_file": h.mountPath,
					},
				})
				return
			}
		}
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

	if err == nil {
		// DB session found
		var result caddy.ImportResult
		if err := json.Unmarshal([]byte(session.ParsedData), &result); err == nil {
			// Update status to reviewing
			session.Status = "reviewing"
			h.db.Save(&session)

			// Read original Caddyfile content if available
			var caddyfileContent string
			if session.SourceFile != "" {
				if content, err := os.ReadFile(session.SourceFile); err == nil {
					caddyfileContent = string(content)
				} else {
					backupPath := filepath.Join(h.importDir, "backups", filepath.Base(session.SourceFile))
					if content, err := os.ReadFile(backupPath); err == nil {
						caddyfileContent = string(content)
					}
				}
			}

			c.JSON(http.StatusOK, gin.H{
				"session": gin.H{
					"id":          session.UUID,
					"state":       session.Status,
					"created_at":  session.CreatedAt,
					"updated_at":  session.UpdatedAt,
					"source_file": session.SourceFile,
				},
				"preview":           result,
				"caddyfile_content": caddyfileContent,
			})
			return
		}
	}

	// No DB session found or failed to parse session. Try transient preview from mountPath.
	if h.mountPath != "" {
		if _, err := os.Stat(h.mountPath); err == nil {
			// Parse mounted Caddyfile transiently
			transient, err := h.importerservice.ImportFile(h.mountPath)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to parse mounted Caddyfile"})
				return
			}

			// Build a transient session id (not persisted)
			sid := uuid.NewString()
			var caddyfileContent string
			if content, err := os.ReadFile(h.mountPath); err == nil {
				caddyfileContent = string(content)
			}

			// Check for conflicts with existing hosts and append raw domain names
			existingHosts, _ := h.proxyHostSvc.List()
			existingDomains := make(map[string]bool)
			for _, eh := range existingHosts {
				existingDomains[eh.DomainNames] = true
			}
			for _, ph := range transient.Hosts {
				if existingDomains[ph.DomainNames] {
					transient.Conflicts = append(transient.Conflicts, ph.DomainNames)
				}
			}

			c.JSON(http.StatusOK, gin.H{
				"session":           gin.H{"id": sid, "state": "transient", "source_file": h.mountPath},
				"preview":           transient,
				"caddyfile_content": caddyfileContent,
			})
			return
		}
	}

	c.JSON(http.StatusNotFound, gin.H{"error": "no pending import"})
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

	// Save upload to import/uploads/<uuid>.caddyfile and return transient preview (do not persist yet)
	sid := uuid.NewString()
	uploadsDir := filepath.Join(h.importDir, "uploads")
	if err := os.MkdirAll(uploadsDir, 0755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create uploads directory"})
		return
	}

	tempPath := filepath.Join(uploadsDir, fmt.Sprintf("%s.caddyfile", sid))
	if err := os.WriteFile(tempPath, []byte(req.Content), 0644); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to write upload"})
		return
	}

	// Parse uploaded file transiently
	result, err := h.importerservice.ImportFile(tempPath)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("import failed: %v", err)})
		return
	}

	// Check for conflicts with existing hosts and append raw domain names
	existingHosts, _ := h.proxyHostSvc.List()
	existingDomains := make(map[string]bool)
	for _, eh := range existingHosts {
		existingDomains[eh.DomainNames] = true
	}
	for _, ph := range result.Hosts {
		if existingDomains[ph.DomainNames] {
			result.Conflicts = append(result.Conflicts, ph.DomainNames)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"session": gin.H{"id": sid, "state": "transient", "source_file": tempPath},
		"preview": result,
	})
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

	// Try to find a DB-backed session first
	var session models.ImportSession
	var result *caddy.ImportResult
	if err := h.db.Where("uuid = ? AND status = ?", req.SessionUUID, "reviewing").First(&session).Error; err == nil {
		// DB session found
		if err := json.Unmarshal([]byte(session.ParsedData), &result); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to parse import data"})
			return
		}
	} else {
		// No DB session: check for uploaded temp file
		uploadsPath := filepath.Join(h.importDir, "uploads", fmt.Sprintf("%s.caddyfile", req.SessionUUID))
		if _, err := os.Stat(uploadsPath); err == nil {
			r, err := h.importerservice.ImportFile(uploadsPath)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to parse uploaded file"})
				return
			}
			result = r
			// We'll create a committed DB session after applying
			session = models.ImportSession{UUID: req.SessionUUID, SourceFile: uploadsPath}
		} else if h.mountPath != "" {
			if _, err := os.Stat(h.mountPath); err == nil {
				r, err := h.importerservice.ImportFile(h.mountPath)
				if err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to parse mounted Caddyfile"})
					return
				}
				result = r
				session = models.ImportSession{UUID: req.SessionUUID, SourceFile: h.mountPath}
			} else {
				c.JSON(http.StatusNotFound, gin.H{"error": "session not found or file missing"})
				return
			}
		} else {
			c.JSON(http.StatusNotFound, gin.H{"error": "session not found"})
			return
		}
	}

	// Convert parsed hosts to ProxyHost models
	proxyHosts := caddy.ConvertToProxyHosts(result.Hosts)
	log.Printf("Import Commit: Parsed %d hosts, converted to %d proxy hosts", len(result.Hosts), len(proxyHosts))

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
			errMsg := fmt.Sprintf("%s: %s", host.DomainNames, err.Error())
			errors = append(errors, errMsg)
			log.Printf("Import Commit Error: %s", errMsg)
		} else {
			created++
			log.Printf("Import Commit Success: Created host %s", host.DomainNames)
		}
	}

	// Persist an import session record now that user confirmed
	now := time.Now()
	session.Status = "committed"
	session.CommittedAt = &now
	session.UserResolutions = string(mustMarshal(req.Resolutions))
	// If ParsedData/ConflictReport not set, fill from result
	if session.ParsedData == "" {
		session.ParsedData = string(mustMarshal(result))
	}
	if session.ConflictReport == "" {
		session.ConflictReport = string(mustMarshal(result.Conflicts))
	}
	if err := h.db.Save(&session).Error; err != nil {
		log.Printf("Warning: failed to save import session: %v", err)
	}

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
	if err := h.db.Where("uuid = ?", sessionUUID).First(&session).Error; err == nil {
		session.Status = "rejected"
		h.db.Save(&session)
		c.JSON(http.StatusOK, gin.H{"message": "import cancelled"})
		return
	}

	// If no DB session, check for uploaded temp file and delete it
	uploadsPath := filepath.Join(h.importDir, "uploads", fmt.Sprintf("%s.caddyfile", sessionUUID))
	if _, err := os.Stat(uploadsPath); err == nil {
		os.Remove(uploadsPath)
		c.JSON(http.StatusOK, gin.H{"message": "transient upload cancelled"})
		return
	}

	// If neither exists, return not found
	c.JSON(http.StatusNotFound, gin.H{"error": "session not found"})
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
			// Append the raw domain name so frontend can match conflicts against domain strings
			result.Conflicts = append(result.Conflicts, parsed.DomainNames)
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
		// If mount is gone, remove any pending/reviewing sessions created previously for this mount
		db.Where("source_file = ? AND status IN ?", mountPath, []string{"pending", "reviewing"}).Delete(&models.ImportSession{})
		return nil // No mounted file, nothing to import
	}

	// Check if already processed (includes committed to avoid re-imports)
	var count int64
	db.Model(&models.ImportSession{}).Where("source_file = ? AND status IN ?",
		mountPath, []string{"pending", "reviewing", "committed"}).Count(&count)

	if count > 0 {
		return nil // Already processed
	}

	// Do not create a DB session automatically for mounted imports; preview will be transient.
	return nil
}

func mustMarshal(v interface{}) []byte {
	b, _ := json.Marshal(v)
	return b
}
