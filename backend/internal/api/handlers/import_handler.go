package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/caddy"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/services"
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
	router.POST("/import/upload-multi", h.UploadMulti)
	router.POST("/import/detect-imports", h.DetectImports)
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
		// No pending/reviewing session, check if there's a mounted Caddyfile available for transient preview
		if h.mountPath != "" {
			if fileInfo, err := os.Stat(h.mountPath); err == nil {
				// Check if this mount has already been committed recently
				var committedSession models.ImportSession
				err := h.db.Where("source_file = ? AND status = ?", h.mountPath, "committed").
					Order("committed_at DESC").
					First(&committedSession).Error

				// Allow re-import if:
				// 1. Never committed before (err == gorm.ErrRecordNotFound), OR
				// 2. File was modified after last commit
				allowImport := err == gorm.ErrRecordNotFound
				if !allowImport && committedSession.CommittedAt != nil {
					fileMod := fileInfo.ModTime()
					commitTime := *committedSession.CommittedAt
					allowImport = fileMod.After(commitTime)
				}

				if allowImport {
					// Mount file is available for import
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
				// Mount file was already committed and hasn't been modified, don't offer it again
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
		if fileInfo, err := os.Stat(h.mountPath); err == nil {
			// Check if this mount has already been committed recently
			var committedSession models.ImportSession
			err := h.db.Where("source_file = ? AND status = ?", h.mountPath, "committed").
				Order("committed_at DESC").
				First(&committedSession).Error

			// Allow preview if:
			// 1. Never committed before (err == gorm.ErrRecordNotFound), OR
			// 2. File was modified after last commit
			allowPreview := err == gorm.ErrRecordNotFound
			if !allowPreview && committedSession.CommittedAt != nil {
				allowPreview = fileInfo.ModTime().After(*committedSession.CommittedAt)
			}

			if !allowPreview {
				// Mount file was already committed and hasn't been modified, don't offer preview again
				c.JSON(http.StatusNotFound, gin.H{"error": "no pending import"})
				return
			}

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

			// Check for conflicts with existing hosts and build conflict details
			existingHosts, _ := h.proxyHostSvc.List()
			existingDomainsMap := make(map[string]models.ProxyHost)
			for _, eh := range existingHosts {
				existingDomainsMap[eh.DomainNames] = eh
			}

			conflictDetails := make(map[string]gin.H)
			for _, ph := range transient.Hosts {
				if existing, found := existingDomainsMap[ph.DomainNames]; found {
					transient.Conflicts = append(transient.Conflicts, ph.DomainNames)
					conflictDetails[ph.DomainNames] = gin.H{
						"existing": gin.H{
							"forward_scheme": existing.ForwardScheme,
							"forward_host":   existing.ForwardHost,
							"forward_port":   existing.ForwardPort,
							"ssl_forced":     existing.SSLForced,
							"websocket":      existing.WebsocketSupport,
							"enabled":        existing.Enabled,
						},
						"imported": gin.H{
							"forward_scheme": ph.ForwardScheme,
							"forward_host":   ph.ForwardHost,
							"forward_port":   ph.ForwardPort,
							"ssl_forced":     ph.SSLForced,
							"websocket":      ph.WebsocketSupport,
						},
					}
				}
			}

			c.JSON(http.StatusOK, gin.H{
				"session":           gin.H{"id": sid, "state": "transient", "source_file": h.mountPath},
				"preview":           transient,
				"caddyfile_content": caddyfileContent,
				"conflict_details":  conflictDetails,
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

	// Check for conflicts with existing hosts and build conflict details
	existingHosts, _ := h.proxyHostSvc.List()
	existingDomainsMap := make(map[string]models.ProxyHost)
	for _, eh := range existingHosts {
		existingDomainsMap[eh.DomainNames] = eh
	}

	conflictDetails := make(map[string]gin.H)
	for _, ph := range result.Hosts {
		if existing, found := existingDomainsMap[ph.DomainNames]; found {
			result.Conflicts = append(result.Conflicts, ph.DomainNames)
			conflictDetails[ph.DomainNames] = gin.H{
				"existing": gin.H{
					"forward_scheme": existing.ForwardScheme,
					"forward_host":   existing.ForwardHost,
					"forward_port":   existing.ForwardPort,
					"ssl_forced":     existing.SSLForced,
					"websocket":      existing.WebsocketSupport,
					"enabled":        existing.Enabled,
				},
				"imported": gin.H{
					"forward_scheme": ph.ForwardScheme,
					"forward_host":   ph.ForwardHost,
					"forward_port":   ph.ForwardPort,
					"ssl_forced":     ph.SSLForced,
					"websocket":      ph.WebsocketSupport,
				},
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"session":          gin.H{"id": sid, "state": "transient", "source_file": tempPath},
		"conflict_details": conflictDetails,
		"preview":          result,
	})
}

// DetectImports analyzes Caddyfile content and returns detected import directives.
func (h *ImportHandler) DetectImports(c *gin.Context) {
	var req struct {
		Content string `json:"content" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	imports := detectImportDirectives(req.Content)
	c.JSON(http.StatusOK, gin.H{
		"has_imports": len(imports) > 0,
		"imports":     imports,
	})
}

// UploadMulti handles upload of main Caddyfile + multiple site files.
func (h *ImportHandler) UploadMulti(c *gin.Context) {
	var req struct {
		Files []struct {
			Filename string `json:"filename" binding:"required"`
			Content  string `json:"content" binding:"required"`
		} `json:"files" binding:"required,min=1"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate: at least one file must be named "Caddyfile" or have no path separator
	hasCaddyfile := false
	for _, f := range req.Files {
		if f.Filename == "Caddyfile" || !strings.Contains(f.Filename, "/") {
			hasCaddyfile = true
			break
		}
	}
	if !hasCaddyfile {
		c.JSON(http.StatusBadRequest, gin.H{"error": "must include a main Caddyfile"})
		return
	}

	// Create session directory
	sid := uuid.NewString()
	sessionDir := filepath.Join(h.importDir, "uploads", sid)
	if err := os.MkdirAll(sessionDir, 0755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create session directory"})
		return
	}

	// Write all files
	mainCaddyfile := ""
	for _, f := range req.Files {
		if strings.TrimSpace(f.Content) == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("file '%s' is empty", f.Filename)})
			return
		}

		// Clean filename and create subdirectories if needed
		cleanName := filepath.Clean(f.Filename)
		targetPath := filepath.Join(sessionDir, cleanName)

		// Create parent directory if file is in a subdirectory
		if dir := filepath.Dir(targetPath); dir != sessionDir {
			if err := os.MkdirAll(dir, 0755); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("failed to create directory for %s", f.Filename)})
				return
			}
		}

		if err := os.WriteFile(targetPath, []byte(f.Content), 0644); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("failed to write file %s", f.Filename)})
			return
		}

		// Track main Caddyfile
		if cleanName == "Caddyfile" || !strings.Contains(cleanName, "/") {
			mainCaddyfile = targetPath
		}
	}

	// Parse the main Caddyfile (which will automatically resolve imports)
	result, err := h.importerservice.ImportFile(mainCaddyfile)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("import failed: %v", err)})
		return
	}

	// Check for conflicts
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
		"session": gin.H{"id": sid, "state": "transient", "source_file": mainCaddyfile},
		"preview": result,
	})
}

// detectImportDirectives scans Caddyfile content for import directives.
func detectImportDirectives(content string) []string {
	imports := []string{}
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "import ") {
			path := strings.TrimSpace(strings.TrimPrefix(trimmed, "import"))
			// Remove any trailing comments
			if idx := strings.Index(path, "#"); idx != -1 {
				path = strings.TrimSpace(path[:idx])
			}
			imports = append(imports, path)
		}
	}
	return imports
}

// Commit finalizes the import with user's conflict resolutions.
func (h *ImportHandler) Commit(c *gin.Context) {
	var req struct {
		SessionUUID string            `json:"session_uuid" binding:"required"`
		Resolutions map[string]string `json:"resolutions"` // domain -> action (keep/skip, overwrite, rename)
		Names       map[string]string `json:"names"`       // domain -> custom name
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
	updated := 0
	skipped := 0
	errors := []string{}

	// Get existing hosts to check for overwrites
	existingHosts, _ := h.proxyHostSvc.List()
	existingMap := make(map[string]*models.ProxyHost)
	for i := range existingHosts {
		existingMap[existingHosts[i].DomainNames] = &existingHosts[i]
	}

	for _, host := range proxyHosts {
		action := req.Resolutions[host.DomainNames]

		// Apply custom name from user input
		if customName, ok := req.Names[host.DomainNames]; ok && customName != "" {
			host.Name = customName
		}

		// "keep" means keep existing (don't import), same as "skip"
		if action == "skip" || action == "keep" {
			skipped++
			continue
		}

		if action == "rename" {
			host.DomainNames += "-imported"
		}

		// Handle overwrite: preserve existing ID, UUID, and certificate
		if action == "overwrite" {
			if existing, found := existingMap[host.DomainNames]; found {
				host.ID = existing.ID
				host.UUID = existing.UUID
				host.CertificateID = existing.CertificateID // Preserve certificate association
				host.CreatedAt = existing.CreatedAt

				if err := h.proxyHostSvc.Update(&host); err != nil {
					errMsg := fmt.Sprintf("%s: %s", host.DomainNames, err.Error())
					errors = append(errors, errMsg)
					log.Printf("Import Commit Error (update): %s", errMsg)
				} else {
					updated++
					log.Printf("Import Commit Success: Updated host %s", host.DomainNames)
				}
				continue
			}
			// If "overwrite" but doesn't exist, fall through to create
		}

		// Create new host
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
		"updated": updated,
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
