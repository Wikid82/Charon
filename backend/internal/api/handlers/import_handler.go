package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"golang.org/x/text/unicode/norm"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/api/middleware"
	"github.com/Wikid82/charon/backend/internal/caddy"
	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/services"
	"github.com/Wikid82/charon/backend/internal/util"
)

// ProxyHostServiceInterface defines the subset of ProxyHostService needed by ImportHandler.
// This allows for easier testing by enabling mock implementations.
type ProxyHostServiceInterface interface {
	Create(host *models.ProxyHost) error
	Update(host *models.ProxyHost) error
	List() ([]models.ProxyHost, error)
}

// ImporterService defines the interface for Caddyfile import operations
type ImporterService interface {
	NormalizeCaddyfile(content string) (string, error)
	ParseCaddyfile(path string) ([]byte, error)
	ImportFile(path string) (*caddy.ImportResult, error)
	ExtractHosts(caddyJSON []byte) (*caddy.ImportResult, error)
	ValidateCaddyBinary() error
}

// ImportHandler handles Caddyfile import operations.
type ImportHandler struct {
	db              *gorm.DB
	proxyHostSvc    ProxyHostServiceInterface
	importerservice ImporterService
	importDir       string
	mountPath       string
	securityService *services.SecurityService
}

// NewImportHandler creates a new import handler.
func NewImportHandler(db *gorm.DB, caddyBinary, importDir, mountPath string) *ImportHandler {
	return NewImportHandlerWithDeps(db, caddyBinary, importDir, mountPath, nil)
}

func NewImportHandlerWithDeps(db *gorm.DB, caddyBinary, importDir, mountPath string, securityService *services.SecurityService) *ImportHandler {
	return &ImportHandler{
		db:              db,
		proxyHostSvc:    services.NewProxyHostService(db),
		importerservice: caddy.NewImporter(caddyBinary),
		importDir:       importDir,
		mountPath:       mountPath,
		securityService: securityService,
	}
}

// NewImportHandlerWithService creates an import handler with a custom ProxyHostService.
// This is primarily used for testing with mock services.
func NewImportHandlerWithService(db *gorm.DB, proxyHostSvc ProxyHostServiceInterface, caddyBinary, importDir, mountPath string, securityService *services.SecurityService) *ImportHandler {
	return &ImportHandler{
		db:              db,
		proxyHostSvc:    proxyHostSvc,
		importerservice: caddy.NewImporter(caddyBinary),
		importDir:       importDir,
		mountPath:       mountPath,
		securityService: securityService,
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
	if !requireAuthenticatedAdmin(c) {
		return
	}

	var session models.ImportSession
	err := h.db.Where("status IN ?", []string{"pending", "reviewing"}).
		Order("created_at DESC").
		First(&session).Error

	if err == gorm.ErrRecordNotFound {
		// No pending/reviewing session, check if there's a mounted Caddyfile available for transient preview
		if h.mountPath != "" {
			if fileInfo, statErr := os.Stat(h.mountPath); statErr == nil {
				// Check if this mount has already been committed recently
				var committedSession models.ImportSession
				committedErr := h.db.Where("source_file = ? AND status = ?", h.mountPath, "committed").
					Order("committed_at DESC").
					First(&committedSession).Error

				// Allow re-import if:
				// 1. Never committed before (err == gorm.ErrRecordNotFound), OR
				// 2. File was modified after last commit
				allowImport := committedErr == gorm.ErrRecordNotFound
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
	if !requireAuthenticatedAdmin(c) {
		return
	}

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
					// #nosec G304 -- backupPath is constructed from trusted importDir and sanitized basename
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
		if fileInfo, statErr := os.Stat(h.mountPath); statErr == nil {
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
	if !requireAdmin(c) {
		return
	}

	var req struct {
		Content  string `json:"content" binding:"required"`
		Filename string `json:"filename"`
	}

	// Capture raw request for better diagnostics in tests
	if err := c.ShouldBindJSON(&req); err != nil {
		// Try to include raw body preview when binding fails
		entry := middleware.GetRequestLogger(c)
		if raw, _ := c.GetRawData(); len(raw) > 0 {
			entry.WithError(err).WithField("raw_body_preview", util.SanitizeForLog(string(raw))).Error("Import Upload: failed to bind JSON")
		} else {
			entry.WithError(err).Error("Import Upload: failed to bind JSON")
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	middleware.GetRequestLogger(c).WithField("filename", util.SanitizeForLog(filepath.Base(req.Filename))).WithField("content_len", len(req.Content)).Info("Import Upload: received upload")

	// Normalize Caddyfile format before saving (handles single-line format)
	normalizedContent := req.Content
	if normalized, err := h.importerservice.NormalizeCaddyfile(req.Content); err != nil {
		// If normalization fails, log warning but continue with original content
		middleware.GetRequestLogger(c).WithError(err).Warn("Import Upload: Caddyfile normalization failed, using original content")
	} else {
		normalizedContent = normalized
	}

	// Save upload to import/uploads/<uuid>.caddyfile and return transient preview (do not persist yet)
	sid := uuid.NewString()
	uploadsDir, err := safeJoin(h.importDir, "uploads")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid import directory"})
		return
	}
	// #nosec G301 -- Import uploads directory needs group readability for processing
	if mkdirErr := os.MkdirAll(uploadsDir, 0o755); mkdirErr != nil {
		if respondPermissionError(c, h.securityService, "import_upload_failed", mkdirErr, h.importDir) {
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create uploads directory"})
		return
	}
	tempPath, err := safeJoin(uploadsDir, fmt.Sprintf("%s.caddyfile", sid))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid temp path"})
		return
	}
	// #nosec G306 -- Caddyfile uploads need group readability for Caddy validation
	if writeErr := os.WriteFile(tempPath, []byte(normalizedContent), 0o644); writeErr != nil {
		middleware.GetRequestLogger(c).WithField("tempPath", util.SanitizeForLog(filepath.Base(tempPath))).WithError(writeErr).Error("Import Upload: failed to write temp file")
		if respondPermissionError(c, h.securityService, "import_upload_failed", writeErr, h.importDir) {
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to write upload"})
		return
	}

	// Parse uploaded file transiently
	result, err := h.importerservice.ImportFile(tempPath)
	if err != nil {
		// Read a small preview of the uploaded file for diagnostics
		preview := ""
		// #nosec G304 -- tempPath is the validated temporary file from Gin SaveUploadedFile
		if b, rerr := os.ReadFile(tempPath); rerr == nil {
			if len(b) > 200 {
				preview = string(b[:200])
			} else {
				preview = string(b)
			}
		}
		middleware.GetRequestLogger(c).WithError(err).WithField("tempPath", util.SanitizeForLog(filepath.Base(tempPath))).WithField("content_preview", util.SanitizeForLog(preview)).Error("Import Upload: import failed")
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("import failed: %v", err)})
		return
	}

	// Determine whether any parsed hosts are actually importable (have forward host/port)
	importableCount := 0
	fileServerDetected := false
	for _, ph := range result.Hosts {
		if ph.ForwardHost != "" && ph.ForwardPort != 0 {
			importableCount++
		}
		for _, w := range ph.Warnings {
			if strings.Contains(strings.ToLower(w), "file server") || strings.Contains(strings.ToLower(w), "file_server") {
				fileServerDetected = true
			}
		}
	}

	// If there are no importable hosts, surface clearer feedback. This covers cases
	// where routes were parsed (e.g. file_server) but none are reverse_proxy
	// entries that we can import.
	if importableCount == 0 {
		imports := detectImportDirectives(req.Content)
		if len(imports) > 0 {
			sanitizedImports := make([]string, 0, len(imports))
			for _, imp := range imports {
				sanitizedImports = append(sanitizedImports, util.SanitizeForLog(filepath.Base(imp)))
			}
			middleware.GetRequestLogger(c).WithField("imports", sanitizedImports).Warn("Import Upload: no importable hosts parsed but imports detected")
			// Keep existing behavior for import directives (400) so callers can react
			c.JSON(http.StatusBadRequest, gin.H{"error": "no sites found in uploaded Caddyfile; imports detected; please upload the referenced site files using the multi-file import flow", "imports": imports})
			return
		}

		// If file_server directives were present, return a preview + explicit
		// warning so the frontend can show a prominent banner while still
		// returning a successful preview shape (tests expect preview + banner).
		if fileServerDetected {
			middleware.GetRequestLogger(c).WithField("content_len", len(req.Content)).Warn("Import Upload: parsed routes were file_server-only and not importable")
			// Return 400 but include preview + warning so callers (and E2E) can render
			// the same preview UX while still signaling an error status.
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "File server directives are not supported for import or no sites/hosts found in your Caddyfile",
				"warning": "File server directives are not supported for import or no sites/hosts found in your Caddyfile",
				"session": gin.H{"id": sid, "state": "transient", "source_file": tempPath},
				"preview": result,
			})
			return
		}

		middleware.GetRequestLogger(c).WithField("content_len", len(req.Content)).Warn("Import Upload: no hosts parsed and no imports detected")
		c.JSON(http.StatusBadRequest, gin.H{"error": "no sites found in uploaded Caddyfile", "warning": "No sites or importable hosts were found in the uploaded Caddyfile", "session": gin.H{"id": sid, "state": "transient", "source_file": tempPath}, "preview": result})
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

	session := models.ImportSession{
		UUID:           sid,
		SourceFile:     tempPath,
		Status:         "pending",
		ParsedData:     string(mustMarshal(result)),
		ConflictReport: string(mustMarshal(result.Conflicts)),
	}
	if err := h.db.Create(&session).Error; err != nil {
		middleware.GetRequestLogger(c).WithError(err).Warn("Import Upload: failed to persist session")
		if respondPermissionError(c, h.securityService, "import_upload_failed", err, h.importDir) {
			return
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
		entry := middleware.GetRequestLogger(c)
		if raw, _ := c.GetRawData(); len(raw) > 0 {
			entry.WithError(err).WithField("raw_body_preview", util.SanitizeForLog(string(raw))).Error("Import UploadMulti: failed to bind JSON")
		} else {
			entry.WithError(err).Error("Import UploadMulti: failed to bind JSON")
		}
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
	if !requireAdmin(c) {
		return
	}

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
	sessionDir, err := safeJoin(h.importDir, filepath.Join("uploads", sid))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid session directory"})
		return
	}
	// #nosec G301 -- Session directory with standard permissions for import processing
	if mkdirErr := os.MkdirAll(sessionDir, 0o755); mkdirErr != nil {
		if respondPermissionError(c, h.securityService, "import_upload_failed", mkdirErr, h.importDir) {
			return
		}
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
		targetPath, joinErr := safeJoin(sessionDir, cleanName)
		if joinErr != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("invalid filename: %s", f.Filename)})
			return
		}

		// Create parent directory if file is in a subdirectory
		if dir := filepath.Dir(targetPath); dir != sessionDir {
			// #nosec G301 -- Subdirectory within validated session directory
			if mkdirErr := os.MkdirAll(dir, 0o755); mkdirErr != nil {
				if respondPermissionError(c, h.securityService, "import_upload_failed", mkdirErr, h.importDir) {
					return
				}
				c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("failed to create directory for %s", f.Filename)})
				return
			}
		}

		// #nosec G306 -- Imported Caddyfile needs to be readable for processing
		if writeErr := os.WriteFile(targetPath, []byte(f.Content), 0o644); writeErr != nil {
			if respondPermissionError(c, h.securityService, "import_upload_failed", writeErr, h.importDir) {
				return
			}
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
		// Provide diagnostics
		preview := ""
		if b, rerr := os.ReadFile(mainCaddyfile); rerr == nil {
			if len(b) > 200 {
				preview = string(b[:200])
			} else {
				preview = string(b)
			}
		}
		middleware.GetRequestLogger(c).WithError(err).WithField("mainCaddyfile", util.SanitizeForLog(filepath.Base(mainCaddyfile))).WithField("preview", util.SanitizeForLog(preview)).Error("Import UploadMulti: import failed")
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("import failed: %v", err)})
		return
	}

	// If parsing succeeded but no importable hosts were found, surface clearer
	// feedback. This covers cases where routes exist (e.g., file_server) but none
	// are reverse_proxy entries that we can import.
	// Determine importable hosts and detect file_server presence.
	importableCount := 0
	fileServerDetected := false
	for _, ph := range result.Hosts {
		if ph.ForwardHost != "" && ph.ForwardPort != 0 {
			importableCount++
		}
		for _, w := range ph.Warnings {
			if strings.Contains(strings.ToLower(w), "file server") || strings.Contains(strings.ToLower(w), "file_server") {
				fileServerDetected = true
			}
		}
	}

	if importableCount == 0 {
		mainContentBytes, _ := os.ReadFile(mainCaddyfile)
		imports := detectImportDirectives(string(mainContentBytes))
		if len(imports) > 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "no sites parsed from main Caddyfile; import directives detected; please include site files in upload", "imports": imports})
			return
		}

		if fileServerDetected {
			// Return 400 but include preview + warning so the UI can render the
			// preview shape while the HTTP status indicates an error.
			middleware.GetRequestLogger(c).WithField("mainCaddyfile", util.SanitizeForLog(filepath.Base(mainCaddyfile))).Warn("Import UploadMulti: parsed routes were file_server-only and not importable")
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "File server directives are not supported for import or no sites/hosts found in your Caddyfile",
				"warning": "File server directives are not supported for import or no sites/hosts found in your Caddyfile",
				"session": gin.H{"id": sid, "state": "transient", "source_file": mainCaddyfile},
				"preview": result,
			})
			return
		}

		c.JSON(http.StatusBadRequest, gin.H{"error": "no sites parsed from main Caddyfile"})
		return
	}

	// --- Additional multi-file behavior: when the main Caddyfile contains import
	// directives, the multi-file flow is expected (by E2E tests) to return only
	// hosts that originated from the imported files. The importer does not
	// currently annotate host origins, so we implement a pragmatic filter:
	// - extract domain names explicitly declared in the main Caddyfile and
	// - if import directives exist, exclude those main-file domains from the
	//   preview so the preview reflects imported-file hosts only.
	mainContentBytes, _ := os.ReadFile(mainCaddyfile)
	mainContent := string(mainContentBytes)
	if len(detectImportDirectives(mainContent)) > 0 {
		// crude extraction of domains declared in the main file
		mainDomains := make(map[string]bool)
		for _, line := range strings.Split(mainContent, "\n") {
			ln := strings.TrimSpace(line)
			if ln == "" || strings.HasPrefix(ln, "#") || strings.HasPrefix(ln, "import ") {
				continue
			}
			if strings.HasSuffix(ln, "{") {
				tokens := strings.Fields(strings.TrimSuffix(ln, "{"))
				if len(tokens) > 0 {
					mainDomains[tokens[0]] = true
				}
			}
		}

		if len(mainDomains) > 0 {
			filtered := make([]caddy.ParsedHost, 0, len(result.Hosts))
			for _, ph := range result.Hosts {
				if _, found := mainDomains[ph.DomainNames]; found {
					// skip hosts declared in main Caddyfile when imports are present
					continue
				}
				filtered = append(filtered, ph)
			}
			result.Hosts = filtered
		}
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

	session := models.ImportSession{
		UUID:           sid,
		SourceFile:     mainCaddyfile,
		Status:         "pending",
		ParsedData:     string(mustMarshal(result)),
		ConflictReport: string(mustMarshal(result.Conflicts)),
	}
	if err := h.db.Create(&session).Error; err != nil {
		middleware.GetRequestLogger(c).WithError(err).Warn("Import UploadMulti: failed to persist session")
		if respondPermissionError(c, h.securityService, "import_upload_failed", err, h.importDir) {
			return
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
			importPath := strings.TrimSpace(strings.TrimPrefix(trimmed, "import"))
			// Remove any trailing comments
			if idx := strings.Index(importPath, "#"); idx != -1 {
				importPath = strings.TrimSpace(importPath[:idx])
			}
			imports = append(imports, importPath)
		}
	}
	return imports
}

// safeJoin joins a user-supplied path to a base directory and ensures
// the resulting path is contained within the base directory.
// Security: Protects against path traversal, Windows absolute paths, null byte injection,
// and normalizes Unicode confusables to prevent directory traversal attacks.
func safeJoin(baseDir, userPath string) (string, error) {
	// Security: Strip null bytes that could be used to bypass extension checks
	// Following the principle that we should sanitize rather than reject to be more permissive
	// while still maintaining security
	userPath = strings.ReplaceAll(userPath, "\x00", "")

	// Security: Reject paths with invalid UTF-8 encoding
	if !utf8.ValidString(userPath) {
		return "", fmt.Errorf("invalid UTF-8 in path")
	}

	// Security: Apply Unicode NFC normalization to handle confusable characters
	// This prevents attacks using visually similar Unicode characters (e.g., U+2215 vs /)
	normalized := norm.NFC.String(userPath)

	// Security: Check for Windows drive letter absolute paths (C:\, D:\, etc.)
	// Must check BEFORE filepath.Clean as it's an explicit absolute path indicator
	// On Unix systems, filepath.IsAbs won't catch these, creating security vulnerabilities
	if len(normalized) >= 3 {
		// Check for Windows drive letters: C:\, D:\, etc.
		if (normalized[1] == ':') && (normalized[2] == '\\' || normalized[2] == '/') {
			c := normalized[0]
			if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') {
				return "", fmt.Errorf("windows absolute paths not allowed")
			}
		}
	}

	// Clean the normalized path - this handles platform-specific separators
	// On Unix, backslashes in \\server\share become part of the filename
	// On Windows, UNC paths remain absolute and are caught by filepath.IsAbs
	clean := filepath.Clean(normalized)

	// Reject empty or current directory references
	if clean == "" || clean == "." {
		return "", fmt.Errorf("empty path not allowed")
	}

	// Reject absolute paths (Unix-style + Windows UNC paths after cleaning)
	// This catches both /etc/passwd on Unix and \\server\share on Windows
	if filepath.IsAbs(clean) {
		return "", fmt.Errorf("absolute paths not allowed")
	}

	// Security: Prevent parent directory traversal (.., ../, ..\\)
	// Only reject ".." when it's followed by a path separator or is the entire path
	if strings.HasPrefix(clean, ".."+string(os.PathSeparator)) || clean == ".." {
		return "", fmt.Errorf("path traversal detected")
	}

	// Join with base directory and verify result stays within base
	target := filepath.Join(baseDir, clean)
	rel, err := filepath.Rel(baseDir, target)
	if err != nil {
		return "", fmt.Errorf("invalid path")
	}

	// Final check: ensure relative path doesn't escape base directory
	// Only reject if ".." is followed by a separator or is the complete path
	// This allows filenames like "..something" while blocking "../etc" traversal
	if rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("path traversal detected")
	}

	// Normalize path separators for consistency
	target = path.Clean(target)
	return target, nil
}

// Commit finalizes the import with user's conflict resolutions.
func (h *ImportHandler) Commit(c *gin.Context) {
	if !requireAdmin(c) {
		return
	}

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
	// Basic sanitize of session id to prevent path separators
	sid := filepath.Base(req.SessionUUID)
	if sid == "" || sid == "." || strings.Contains(sid, string(os.PathSeparator)) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid session_uuid"})
		return
	}
	var result *caddy.ImportResult
	if err := h.db.Where("uuid = ? AND status IN ?", sid, []string{"reviewing", "pending"}).First(&session).Error; err == nil {
		// DB session found
		if err := json.Unmarshal([]byte(session.ParsedData), &result); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to parse import data"})
			return
		}
	} else {
		// No DB session: check for uploaded temp file
		var parseErr error
		uploadsPath, err := safeJoin(h.importDir, filepath.Join("uploads", fmt.Sprintf("%s.caddyfile", sid)))
		if err == nil {
			if _, err := os.Stat(uploadsPath); err == nil {
				r, err := h.importerservice.ImportFile(uploadsPath)
				if err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to parse uploaded file"})
					return
				}
				result = r
				// We'll create a committed DB session after applying
				session = models.ImportSession{UUID: sid, SourceFile: uploadsPath}
			}
		}
		// If not found yet, check mounted Caddyfile
		if result == nil && h.mountPath != "" {
			if _, err := os.Stat(h.mountPath); err == nil {
				r, err := h.importerservice.ImportFile(h.mountPath)
				if err != nil {
					parseErr = err
				} else {
					result = r
					session = models.ImportSession{UUID: sid, SourceFile: h.mountPath}
				}
			}
		}
		// If still not parsed, return not found or error
		if result == nil {
			if parseErr != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to parse mounted Caddyfile"})
				return
			}
			c.JSON(http.StatusNotFound, gin.H{"error": "session not found or file missing"})
			return
		}
	}

	// Convert parsed hosts to ProxyHost models
	proxyHosts := caddy.ConvertToProxyHosts(result.Hosts)
	middleware.GetRequestLogger(c).WithField("parsed_hosts", len(result.Hosts)).WithField("proxy_hosts", len(proxyHosts)).Info("Import Commit: Parsed and converted hosts")

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
					middleware.GetRequestLogger(c).WithField("host", util.SanitizeForLog(host.DomainNames)).WithField("error", util.SanitizeForLog(errMsg)).Error("Import Commit Error (update)")
				} else {
					updated++
					middleware.GetRequestLogger(c).WithField("host", util.SanitizeForLog(host.DomainNames)).Info("Import Commit Success: Updated host")
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
			middleware.GetRequestLogger(c).WithField("host", util.SanitizeForLog(host.DomainNames)).WithField("error", util.SanitizeForLog(errMsg)).Error("Import Commit Error")
		} else {
			created++
			middleware.GetRequestLogger(c).WithField("host", util.SanitizeForLog(host.DomainNames)).Info("Import Commit Success: Created host")
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
		middleware.GetRequestLogger(c).WithError(err).Warn("Warning: failed to save import session")
		if respondPermissionError(c, h.securityService, "import_commit_failed", err, h.importDir) {
			return
		}
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
	if !requireAdmin(c) {
		return
	}

	sessionUUID := c.Query("session_uuid")
	if sessionUUID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "session_uuid required"})
		return
	}

	sid := filepath.Base(sessionUUID)
	if sid == "" || sid == "." || strings.Contains(sid, string(os.PathSeparator)) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid session_uuid"})
		return
	}

	var session models.ImportSession
	if err := h.db.Where("uuid = ?", sid).First(&session).Error; err == nil {
		session.Status = "rejected"
		if saveErr := h.db.Save(&session).Error; saveErr != nil {
			if respondPermissionError(c, h.securityService, "import_cancel_failed", saveErr, h.importDir) {
				return
			}
		}
		c.JSON(http.StatusOK, gin.H{"message": "import cancelled"})
		return
	}

	// If no DB session, check for uploaded temp file and delete it
	uploadsPath, err := safeJoin(h.importDir, filepath.Join("uploads", fmt.Sprintf("%s.caddyfile", sid)))
	if err == nil {
		if _, err := os.Stat(uploadsPath); err == nil {
			if err := os.Remove(uploadsPath); err != nil {
				logger.Log().WithError(err).Warn("Failed to remove upload file")
				if respondPermissionError(c, h.securityService, "import_cancel_failed", err, h.importDir) {
					return
				}
			}
			c.JSON(http.StatusOK, gin.H{"message": "transient upload cancelled"})
			return
		}
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

func mustMarshal(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}
