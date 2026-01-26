package handlers

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/Wikid82/charon/backend/internal/crowdsec"
	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/network"
	"github.com/Wikid82/charon/backend/internal/security"
	"github.com/Wikid82/charon/backend/internal/services"
	"github.com/Wikid82/charon/backend/internal/util"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// CrowdsecExecutor abstracts starting/stopping CrowdSec so tests can mock it.
type CrowdsecExecutor interface {
	Start(ctx context.Context, binPath, configDir string) (int, error)
	Stop(ctx context.Context, configDir string) error
	Status(ctx context.Context, configDir string) (running bool, pid int, err error)
}

// CommandExecutor abstracts command execution for testing.
type CommandExecutor interface {
	Execute(ctx context.Context, name string, args ...string) ([]byte, error)
}

// RealCommandExecutor executes commands using os/exec.
type RealCommandExecutor struct{}

// Execute runs a command and returns its combined output (stdout/stderr)
func (r *RealCommandExecutor) Execute(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	return cmd.CombinedOutput()
}

// CrowdsecHandler manages CrowdSec process and config imports.
type CrowdsecHandler struct {
	DB       *gorm.DB
	Executor CrowdsecExecutor
	CmdExec  CommandExecutor
	BinPath  string
	DataDir  string
	Hub      *crowdsec.HubService
	Console  *crowdsec.ConsoleEnrollmentService
	Security *services.SecurityService
}

func ttlRemainingSeconds(now, retrievedAt time.Time, ttl time.Duration) *int64 {
	if retrievedAt.IsZero() || ttl <= 0 {
		return nil
	}
	remaining := retrievedAt.Add(ttl).Sub(now)
	if remaining < 0 {
		var zero int64
		return &zero
	}
	secs := int64(remaining.Seconds())
	return &secs
}

func mapCrowdsecStatus(err error, defaultCode int) int {
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return http.StatusGatewayTimeout
	}
	return defaultCode
}

func NewCrowdsecHandler(db *gorm.DB, executor CrowdsecExecutor, binPath, dataDir string) *CrowdsecHandler {
	cacheDir := filepath.Join(dataDir, "hub_cache")
	cache, err := crowdsec.NewHubCache(cacheDir, 24*time.Hour)
	if err != nil {
		logger.Log().WithError(err).Warn("failed to init crowdsec hub cache")
	}
	hubSvc := crowdsec.NewHubService(&RealCommandExecutor{}, cache, dataDir)
	consoleSecret := os.Getenv("CHARON_CONSOLE_ENCRYPTION_KEY")
	if consoleSecret == "" {
		consoleSecret = os.Getenv("CHARON_JWT_SECRET")
	}
	var securitySvc *services.SecurityService
	var consoleSvc *crowdsec.ConsoleEnrollmentService
	if db != nil {
		securitySvc = services.NewSecurityService(db)
		consoleSvc = crowdsec.NewConsoleEnrollmentService(db, &crowdsec.SecureCommandExecutor{}, dataDir, consoleSecret)
	}
	return &CrowdsecHandler{
		DB:       db,
		Executor: executor,
		CmdExec:  &RealCommandExecutor{},
		BinPath:  binPath,
		DataDir:  dataDir,
		Hub:      hubSvc,
		Console:  consoleSvc,
		Security: securitySvc,
	}
}

// isCerberusEnabled returns true when Cerberus is enabled via DB or env flag.
func (h *CrowdsecHandler) isCerberusEnabled() bool {
	if h.DB != nil && h.DB.Migrator().HasTable(&models.Setting{}) {
		var s models.Setting
		if err := h.DB.Where("key = ?", "feature.cerberus.enabled").First(&s).Error; err == nil {
			v := strings.ToLower(strings.TrimSpace(s.Value))
			return v == "true" || v == "1" || v == "yes"
		}
	}

	if envVal, ok := os.LookupEnv("FEATURE_CERBERUS_ENABLED"); ok {
		if b, err := strconv.ParseBool(envVal); err == nil {
			return b
		}
		return envVal == "1"
	}

	if envVal, ok := os.LookupEnv("CERBERUS_ENABLED"); ok {
		if b, err := strconv.ParseBool(envVal); err == nil {
			return b
		}
		return envVal == "1"
	}

	return true
}

// isConsoleEnrollmentEnabled toggles console enrollment via DB or env flag.
func (h *CrowdsecHandler) isConsoleEnrollmentEnabled() bool {
	const key = "feature.crowdsec.console_enrollment"
	if h.DB != nil && h.DB.Migrator().HasTable(&models.Setting{}) {
		var s models.Setting
		if err := h.DB.Where("key = ?", key).First(&s).Error; err == nil {
			v := strings.ToLower(strings.TrimSpace(s.Value))
			return v == "true" || v == "1" || v == "yes"
		}
	}

	if envVal, ok := os.LookupEnv("FEATURE_CROWDSEC_CONSOLE_ENROLLMENT"); ok {
		if b, err := strconv.ParseBool(envVal); err == nil {
			return b
		}
		return envVal == "1"
	}

	return false
}

func actorFromContext(c *gin.Context) string {
	if id, ok := c.Get("userID"); ok {
		return fmt.Sprintf("user:%v", id)
	}
	return "unknown"
}

func (h *CrowdsecHandler) hubEndpoints() []string {
	if h.Hub == nil {
		return nil
	}
	set := make(map[string]struct{})
	for _, e := range []string{h.Hub.HubBaseURL, h.Hub.MirrorBaseURL} {
		if e == "" {
			continue
		}
		set[e] = struct{}{}
	}
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	return out
}

// Start starts the CrowdSec process and waits for LAPI to be ready.
func (h *CrowdsecHandler) Start(c *gin.Context) {
	ctx := c.Request.Context()

	// UPDATE SecurityConfig to persist user's intent
	var cfg models.SecurityConfig
	if err := h.DB.First(&cfg).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			// Create default config with CrowdSec enabled
			cfg = models.SecurityConfig{
				UUID:         "default",
				Name:         "Default Security Config",
				Enabled:      true,
				CrowdSecMode: "local",
			}
			if err := h.DB.Create(&cfg).Error; err != nil {
				logger.Log().WithError(err).Error("Failed to create SecurityConfig")
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to persist configuration"})
				return
			}
		} else {
			logger.Log().WithError(err).Error("Failed to read SecurityConfig")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read configuration"})
			return
		}
	} else {
		// Update existing config
		cfg.CrowdSecMode = "local"
		cfg.Enabled = true
		if err := h.DB.Save(&cfg).Error; err != nil {
			logger.Log().WithError(err).Error("Failed to update SecurityConfig")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to persist configuration"})
			return
		}
	}

	// After updating SecurityConfig, also sync settings table for state consistency
	if h.DB != nil {
		setting := models.Setting{Key: "security.crowdsec.enabled", Value: "true", Category: "security", Type: "bool"}
		h.DB.Where(models.Setting{Key: "security.crowdsec.enabled"}).Assign(setting).FirstOrCreate(&setting)
	}

	// Start the process
	pid, err := h.Executor.Start(ctx, h.BinPath, h.DataDir)
	if err != nil {
		// Revert config on failure
		cfg.CrowdSecMode = "disabled"
		cfg.Enabled = false
		h.DB.Save(&cfg)
		// Also revert settings table
		if h.DB != nil {
			revertSetting := models.Setting{Key: "security.crowdsec.enabled", Value: "false", Category: "security", Type: "bool"}
			h.DB.Where(models.Setting{Key: "security.crowdsec.enabled"}).Assign(revertSetting).FirstOrCreate(&revertSetting)
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Wait for LAPI to be ready (with timeout)
	lapiReady := false
	maxWait := 60 * time.Second
	pollInterval := 500 * time.Millisecond
	deadline := time.Now().Add(maxWait)

	for time.Now().Before(deadline) {
		// Check LAPI status using cscli
		args := []string{"lapi", "status"}
		if _, err := os.Stat(filepath.Join(h.DataDir, "config.yaml")); err == nil {
			args = append([]string{"-c", filepath.Join(h.DataDir, "config.yaml")}, args...)
		}

		checkCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		_, err := h.CmdExec.Execute(checkCtx, "cscli", args...)
		cancel()

		if err == nil {
			lapiReady = true
			break
		}

		time.Sleep(pollInterval)
	}

	if !lapiReady {
		logger.Log().WithField("pid", pid).Warn("CrowdSec started but LAPI not ready within timeout")
		c.JSON(http.StatusOK, gin.H{
			"status":     "started",
			"pid":        pid,
			"lapi_ready": false,
			"warning":    "Process started but LAPI initialization may take additional time",
		})
		return
	}

	logger.Log().WithField("pid", pid).Info("CrowdSec started and LAPI is ready")
	c.JSON(http.StatusOK, gin.H{
		"status":     "started",
		"pid":        pid,
		"lapi_ready": true,
	})
}

// Stop stops the CrowdSec process.
func (h *CrowdsecHandler) Stop(c *gin.Context) {
	ctx := c.Request.Context()
	if err := h.Executor.Stop(ctx, h.DataDir); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// UPDATE SecurityConfig to persist user's intent
	var cfg models.SecurityConfig
	if err := h.DB.First(&cfg).Error; err == nil {
		cfg.CrowdSecMode = "disabled"
		cfg.Enabled = false
		if err := h.DB.Save(&cfg).Error; err != nil {
			logger.Log().WithError(err).Warn("Failed to update SecurityConfig after stopping CrowdSec")
		}
	}

	// After updating SecurityConfig, also sync settings table for state consistency
	if h.DB != nil {
		setting := models.Setting{Key: "security.crowdsec.enabled", Value: "false", Category: "security", Type: "bool"}
		h.DB.Where(models.Setting{Key: "security.crowdsec.enabled"}).Assign(setting).FirstOrCreate(&setting)
	}

	c.JSON(http.StatusOK, gin.H{"status": "stopped"})
}

// Status returns running state including LAPI availability check.
func (h *CrowdsecHandler) Status(c *gin.Context) {
	ctx := c.Request.Context()
	running, pid, err := h.Executor.Status(ctx, h.DataDir)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Check LAPI connectivity if process is running
	lapiReady := false
	if running {
		args := []string{"lapi", "status"}
		if _, err := os.Stat(filepath.Join(h.DataDir, "config.yaml")); err == nil {
			args = append([]string{"-c", filepath.Join(h.DataDir, "config.yaml")}, args...)
		}
		checkCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		_, checkErr := h.CmdExec.Execute(checkCtx, "cscli", args...)
		cancel()
		lapiReady = (checkErr == nil)
	}

	c.JSON(http.StatusOK, gin.H{
		"running":    running,
		"pid":        pid,
		"lapi_ready": lapiReady,
	})
}

// ImportConfig accepts a tar.gz or zip upload and extracts into DataDir (backing up existing config).
func (h *CrowdsecHandler) ImportConfig(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file required"})
		return
	}

	// Save to temp file
	tmpDir := os.TempDir()
	tmpPath := filepath.Join(tmpDir, fmt.Sprintf("crowdsec-import-%d", time.Now().UnixNano()))
	if err := os.MkdirAll(tmpPath, 0o755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create temp dir"})
		return
	}

	dst := filepath.Join(tmpPath, file.Filename)
	if err := c.SaveUploadedFile(file, dst); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save upload"})
		return
	}

	// For safety, do minimal validation: ensure file non-empty
	fi, err := os.Stat(dst)
	if err != nil || fi.Size() == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "empty upload"})
		return
	}

	// Backup current config
	backupDir := h.DataDir + ".backup." + time.Now().Format("20060102-150405")
	if _, err := os.Stat(h.DataDir); err == nil {
		_ = os.Rename(h.DataDir, backupDir)
	}
	// Create target dir
	if err := os.MkdirAll(h.DataDir, 0o755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create config dir"})
		return
	}

	// For now, simply copy uploaded file into data dir for operator to handle extraction
	target := filepath.Join(h.DataDir, file.Filename)
	in, err := os.Open(dst)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to open temp file"})
		return
	}
	defer func() {
		if err := in.Close(); err != nil {
			logger.Log().WithError(err).Warn("failed to close temp file")
		}
	}()
	out, err := os.Create(target)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create target file"})
		return
	}
	defer func() {
		if err := out.Close(); err != nil {
			logger.Log().WithError(err).Warn("failed to close target file")
		}
	}()
	if _, err := io.Copy(out, in); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to write config"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "imported", "backup": backupDir})
}

// ExportConfig creates a tar.gz archive of the CrowdSec data directory and streams it
// back to the client as a downloadable file.
func (h *CrowdsecHandler) ExportConfig(c *gin.Context) {
	// Ensure DataDir exists
	if _, err := os.Stat(h.DataDir); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, gin.H{"error": "crowdsec config not found"})
		return
	}

	// Create a gzip writer and tar writer that stream directly to the response
	c.Header("Content-Type", "application/gzip")
	filename := fmt.Sprintf("crowdsec-config-%s.tar.gz", time.Now().Format("20060102-150405"))
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	gw := gzip.NewWriter(c.Writer)
	defer func() {
		if err := gw.Close(); err != nil {
			logger.Log().WithError(err).Warn("Failed to close gzip writer")
		}
	}()
	tw := tar.NewWriter(gw)
	defer func() {
		if err := tw.Close(); err != nil {
			logger.Log().WithError(err).Warn("Failed to close tar writer")
		}
	}()

	// Walk the DataDir and add files to the archive
	err := filepath.Walk(h.DataDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(h.DataDir, path)
		if err != nil {
			return err
		}
		// Open file
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer func() {
			if err := f.Close(); err != nil {
				logger.Log().WithError(err).Warn("failed to close file while archiving", "path", util.SanitizeForLog(path))
			}
		}()

		hdr := &tar.Header{
			Name:    rel,
			Size:    info.Size(),
			Mode:    int64(info.Mode()),
			ModTime: info.ModTime(),
		}
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		if _, err := io.Copy(tw, f); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		// If any error occurred while creating the archive, return 500
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
}

// ListFiles returns a flat list of files under the CrowdSec DataDir.
func (h *CrowdsecHandler) ListFiles(c *gin.Context) {
	var files []string
	if _, err := os.Stat(h.DataDir); os.IsNotExist(err) {
		c.JSON(http.StatusOK, gin.H{"files": files})
		return
	}
	err := filepath.Walk(h.DataDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			rel, err := filepath.Rel(h.DataDir, path)
			if err != nil {
				return err
			}
			files = append(files, rel)
		}
		return nil
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"files": files})
}

// ReadFile returns the contents of a specific file under DataDir. Query param 'path' required.
func (h *CrowdsecHandler) ReadFile(c *gin.Context) {
	rel := c.Query("path")
	if rel == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "path required"})
		return
	}
	clean := filepath.Clean(rel)
	// prevent directory traversal
	p := filepath.Join(h.DataDir, clean)
	if !strings.HasPrefix(p, filepath.Clean(h.DataDir)) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid path"})
		return
	}
	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": "file not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"content": string(data)})
}

// WriteFile writes content to a file under the CrowdSec DataDir, creating a backup before doing so.
// JSON body: { "path": "relative/path.conf", "content": "..." }
func (h *CrowdsecHandler) WriteFile(c *gin.Context) {
	var payload struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}
	if payload.Path == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "path required"})
		return
	}
	clean := filepath.Clean(payload.Path)
	p := filepath.Join(h.DataDir, clean)
	if !strings.HasPrefix(p, filepath.Clean(h.DataDir)) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid path"})
		return
	}
	// Backup existing DataDir
	backupDir := h.DataDir + ".backup." + time.Now().Format("20060102-150405")
	if _, err := os.Stat(h.DataDir); err == nil {
		if err := os.Rename(h.DataDir, backupDir); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create backup"})
			return
		}
	}
	// Recreate DataDir and write file
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to prepare dir"})
		return
	}
	if err := os.WriteFile(p, []byte(payload.Content), 0o644); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to write file"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "written", "backup": backupDir})
}

// ListPresets returns the curated preset catalog when Cerberus is enabled.
func (h *CrowdsecHandler) ListPresets(c *gin.Context) {
	if !h.isCerberusEnabled() {
		c.JSON(http.StatusNotFound, gin.H{"error": "cerberus disabled"})
		return
	}

	type presetInfo struct {
		crowdsec.Preset
		Available           bool       `json:"available"`
		Cached              bool       `json:"cached"`
		CacheKey            string     `json:"cache_key,omitempty"`
		Etag                string     `json:"etag,omitempty"`
		RetrievedAt         *time.Time `json:"retrieved_at,omitempty"`
		TTLRemainingSeconds *int64     `json:"ttl_remaining_seconds,omitempty"`
	}

	result := map[string]*presetInfo{}
	for _, p := range crowdsec.ListCuratedPresets() {
		cp := p
		result[p.Slug] = &presetInfo{Preset: cp, Available: true}
	}

	// Merge hub index when available
	if h.Hub != nil {
		ctx := c.Request.Context()
		if idx, err := h.Hub.FetchIndex(ctx); err == nil {
			for _, item := range idx.Items {
				slug := strings.TrimSpace(item.Name)
				if slug == "" {
					continue
				}
				if _, ok := result[slug]; !ok {
					result[slug] = &presetInfo{Preset: crowdsec.Preset{
						Slug:        slug,
						Title:       item.Title,
						Summary:     item.Description,
						Source:      "hub",
						Tags:        []string{item.Type},
						RequiresHub: true,
					}, Available: true}
				} else {
					result[slug].Available = true
				}
			}
		} else {
			logger.Log().WithError(err).Warn("crowdsec hub index unavailable")
		}
	}

	// Merge cache metadata
	if h.Hub != nil && h.Hub.Cache != nil {
		ctx := c.Request.Context()
		if cached, err := h.Hub.Cache.List(ctx); err == nil {
			cacheTTL := h.Hub.Cache.TTL()
			now := time.Now().UTC()
			for _, entry := range cached {
				if _, ok := result[entry.Slug]; !ok {
					result[entry.Slug] = &presetInfo{Preset: crowdsec.Preset{Slug: entry.Slug, Title: entry.Slug, Summary: "cached preset", Source: "hub", RequiresHub: true}}
				}
				result[entry.Slug].Cached = true
				result[entry.Slug].CacheKey = entry.CacheKey
				result[entry.Slug].Etag = entry.Etag
				if !entry.RetrievedAt.IsZero() {
					val := entry.RetrievedAt
					result[entry.Slug].RetrievedAt = &val
				}
				result[entry.Slug].TTLRemainingSeconds = ttlRemainingSeconds(now, entry.RetrievedAt, cacheTTL)
			}
		} else {
			logger.Log().WithError(err).Warn("crowdsec hub cache list failed")
		}
	}

	list := make([]presetInfo, 0, len(result))
	for _, v := range result {
		list = append(list, *v)
	}

	c.JSON(http.StatusOK, gin.H{"presets": list})
}

// PullPreset downloads and caches a hub preset while returning a preview.
func (h *CrowdsecHandler) PullPreset(c *gin.Context) {
	if !h.isCerberusEnabled() {
		c.JSON(http.StatusNotFound, gin.H{"error": "cerberus disabled"})
		return
	}

	var payload struct {
		Slug string `json:"slug"`
	}
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}
	slug := strings.TrimSpace(payload.Slug)
	if slug == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "slug required"})
		return
	}
	if h.Hub == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "hub service unavailable"})
		return
	}

	// Check for curated preset that doesn't require hub
	if preset, ok := crowdsec.FindPreset(slug); ok && !preset.RequiresHub {
		c.JSON(http.StatusOK, gin.H{
			"status":       "pulled",
			"slug":         preset.Slug,
			"preview":      "# Curated preset: " + preset.Title + "\n# " + preset.Summary,
			"cache_key":    "curated-" + preset.Slug,
			"etag":         "curated",
			"retrieved_at": time.Now(),
			"source":       "charon-curated",
		})
		return
	}

	ctx := c.Request.Context()
	// Log cache directory before pull
	if h.Hub != nil && h.Hub.Cache != nil {
		cacheDir := filepath.Join(h.DataDir, "hub_cache")
		logger.Log().WithField("cache_dir", util.SanitizeForLog(cacheDir)).WithField("slug", util.SanitizeForLog(slug)).Info("attempting to pull preset")
		if stat, err := os.Stat(cacheDir); err == nil {
			logger.Log().WithField("cache_dir_mode", stat.Mode()).WithField("cache_dir_writable", stat.Mode().Perm()&0o200 != 0).Debug("cache directory exists")
		} else {
			logger.Log().WithError(err).Warn("cache directory stat failed")
		}
	}

	res, err := h.Hub.Pull(ctx, slug)
	if err != nil {
		status := mapCrowdsecStatus(err, http.StatusBadGateway)
		// codeql[go/log-injection] Safe: User input sanitized via util.SanitizeForLog()
		// which removes control characters (0x00-0x1F, 0x7F) including CRLF
		logger.Log().WithError(err).WithField("slug", util.SanitizeForLog(slug)).WithField("hub_base_url", h.Hub.HubBaseURL).Warn("crowdsec preset pull failed")
		c.JSON(status, gin.H{"error": err.Error(), "hub_endpoints": h.hubEndpoints()})
		return
	}

	// Verify cache was actually stored
	// codeql[go/log-injection] Safe: res.Meta fields are system-generated (cache keys, file paths)
	// not directly derived from untrusted user input
	logger.Log().WithField("slug", res.Meta.Slug).WithField("cache_key", res.Meta.CacheKey).WithField("archive_path", res.Meta.ArchivePath).WithField("preview_path", res.Meta.PreviewPath).Info("preset pulled and cached successfully")

	// Verify files exist on disk
	if _, err := os.Stat(res.Meta.ArchivePath); err != nil {
		// codeql[go/log-injection] Safe: archive_path is system-generated file path
		logger.Log().WithError(err).WithField("archive_path", res.Meta.ArchivePath).Error("cached archive file not found after pull")
	}
	if _, err := os.Stat(res.Meta.PreviewPath); err != nil {
		// codeql[go/log-injection] Safe: preview_path is system-generated file path
		logger.Log().WithError(err).WithField("preview_path", res.Meta.PreviewPath).Error("cached preview file not found after pull")
	}

	c.JSON(http.StatusOK, gin.H{
		"status":       "pulled",
		"slug":         res.Meta.Slug,
		"preview":      res.Preview,
		"cache_key":    res.Meta.CacheKey,
		"etag":         res.Meta.Etag,
		"retrieved_at": res.Meta.RetrievedAt,
		"source":       res.Meta.Source,
	})
}

// ApplyPreset installs a pulled preset from cache or via cscli.
func (h *CrowdsecHandler) ApplyPreset(c *gin.Context) {
	if !h.isCerberusEnabled() {
		c.JSON(http.StatusNotFound, gin.H{"error": "cerberus disabled"})
		return
	}

	var payload struct {
		Slug string `json:"slug"`
	}
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}

	slug := strings.TrimSpace(payload.Slug)
	if slug == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "slug required"})
		return
	}
	if h.Hub == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "hub service unavailable"})
		return
	}

	// Check for curated preset that doesn't require hub
	if preset, ok := crowdsec.FindPreset(slug); ok && !preset.RequiresHub {
		if h.DB != nil {
			_ = h.DB.Create(&models.CrowdsecPresetEvent{
				Slug:       slug,
				Action:     "apply",
				Status:     "applied",
				CacheKey:   "curated-" + slug,
				BackupPath: "",
			}).Error
		}

		c.JSON(http.StatusOK, gin.H{
			"status":      "applied",
			"backup":      "",
			"reload_hint": true,
			"used_cscli":  false,
			"cache_key":   "curated-" + slug,
			"slug":        slug,
		})
		return
	}

	ctx := c.Request.Context()

	// Log cache status before apply
	if h.Hub != nil && h.Hub.Cache != nil {
		cacheDir := filepath.Join(h.DataDir, "hub_cache")
		logger.Log().WithField("cache_dir", util.SanitizeForLog(cacheDir)).WithField("slug", util.SanitizeForLog(slug)).Info("attempting to apply preset")

		// Check if cached
		if cached, err := h.Hub.Cache.Load(ctx, slug); err == nil {
			logger.Log().WithField("slug", util.SanitizeForLog(slug)).WithField("cache_key", cached.CacheKey).WithField("archive_path", cached.ArchivePath).WithField("preview_path", cached.PreviewPath).Info("preset found in cache")
			// Verify files still exist
			if _, err := os.Stat(cached.ArchivePath); err != nil {
				logger.Log().WithError(err).WithField("archive_path", cached.ArchivePath).Error("cached archive file missing")
			}
			if _, err := os.Stat(cached.PreviewPath); err != nil {
				logger.Log().WithError(err).WithField("preview_path", cached.PreviewPath).Error("cached preview file missing")
			}
		} else {
			logger.Log().WithError(err).WithField("slug", util.SanitizeForLog(slug)).Warn("preset not found in cache before apply")
			// List what's actually in the cache
			if entries, listErr := h.Hub.Cache.List(ctx); listErr == nil {
				slugs := make([]string, len(entries))
				for i, e := range entries {
					slugs[i] = e.Slug
				}
				logger.Log().WithField("cached_slugs", slugs).Info("current cache contents")
			}
		}
	}

	res, err := h.Hub.Apply(ctx, slug)
	if err != nil {
		status := mapCrowdsecStatus(err, http.StatusInternalServerError)
		// codeql[go/log-injection] Safe: User input (slug) sanitized via util.SanitizeForLog();
		// backup_path and cache_key are system-generated values
		logger.Log().WithError(err).WithField("slug", util.SanitizeForLog(slug)).WithField("hub_base_url", h.Hub.HubBaseURL).WithField("backup_path", res.BackupPath).WithField("cache_key", res.CacheKey).Warn("crowdsec preset apply failed")
		if h.DB != nil {
			_ = h.DB.Create(&models.CrowdsecPresetEvent{Slug: slug, Action: "apply", Status: "failed", CacheKey: res.CacheKey, BackupPath: res.BackupPath, Error: err.Error()}).Error
		}
		// Build detailed error response
		errorMsg := err.Error()
		// Add actionable guidance based on error type
		if errors.Is(err, crowdsec.ErrCacheMiss) || strings.Contains(errorMsg, "cache miss") {
			errorMsg = "Preset cache missing or expired. Pull the preset again, then retry apply."
		} else if strings.Contains(errorMsg, "cscli unavailable") && strings.Contains(errorMsg, "no cached preset") {
			errorMsg = "CrowdSec preset not cached. Pull the preset first by clicking 'Pull Preview', then try applying again."
		}
		errorResponse := gin.H{"error": errorMsg, "hub_endpoints": h.hubEndpoints()}
		if res.BackupPath != "" {
			errorResponse["backup"] = res.BackupPath
		}
		if res.CacheKey != "" {
			errorResponse["cache_key"] = res.CacheKey
		}
		c.JSON(status, errorResponse)
		return
	}

	if h.DB != nil {
		status := res.Status
		if status == "" {
			status = "applied"
		}
		slugVal := res.AppliedPreset
		if slugVal == "" {
			slugVal = slug
		}
		_ = h.DB.Create(&models.CrowdsecPresetEvent{Slug: slugVal, Action: "apply", Status: status, CacheKey: res.CacheKey, BackupPath: res.BackupPath}).Error
	}

	c.JSON(http.StatusOK, gin.H{
		"status":      res.Status,
		"backup":      res.BackupPath,
		"reload_hint": res.ReloadHint,
		"used_cscli":  res.UsedCSCLI,
		"cache_key":   res.CacheKey,
		"slug":        res.AppliedPreset,
	})
}

// ConsoleEnroll enrolls the local engine with CrowdSec console.
func (h *CrowdsecHandler) ConsoleEnroll(c *gin.Context) {
	if !h.isConsoleEnrollmentEnabled() {
		c.JSON(http.StatusNotFound, gin.H{"error": "console enrollment disabled"})
		return
	}
	if h.Console == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "console enrollment unavailable"})
		return
	}

	var payload struct {
		EnrollmentKey string `json:"enrollment_key"`
		Tenant        string `json:"tenant"`
		AgentName     string `json:"agent_name"`
		Force         bool   `json:"force"`
	}
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}

	ctx := c.Request.Context()
	status, err := h.Console.Enroll(ctx, crowdsec.ConsoleEnrollRequest{
		EnrollmentKey: payload.EnrollmentKey,
		Tenant:        payload.Tenant,
		AgentName:     payload.AgentName,
		Force:         payload.Force,
	})

	if err != nil {
		httpStatus := mapCrowdsecStatus(err, http.StatusBadGateway)
		if strings.Contains(strings.ToLower(err.Error()), "progress") {
			httpStatus = http.StatusConflict
		} else if strings.Contains(strings.ToLower(err.Error()), "required") {
			httpStatus = http.StatusBadRequest
		}
		logger.Log().WithError(err).WithField("tenant", util.SanitizeForLog(payload.Tenant)).WithField("agent", util.SanitizeForLog(payload.AgentName)).WithField("correlation_id", status.CorrelationID).Warn("crowdsec console enrollment failed")
		if h.Security != nil {
			_ = h.Security.LogAudit(&models.SecurityAudit{Actor: actorFromContext(c), Action: "crowdsec_console_enroll_failed", Details: fmt.Sprintf("status=%s tenant=%s agent=%s correlation_id=%s", status.Status, payload.Tenant, payload.AgentName, status.CorrelationID)})
		}
		resp := gin.H{"error": err.Error(), "status": status.Status}
		if status.CorrelationID != "" {
			resp["correlation_id"] = status.CorrelationID
		}
		c.JSON(httpStatus, resp)
		return
	}

	if h.Security != nil {
		_ = h.Security.LogAudit(&models.SecurityAudit{Actor: actorFromContext(c), Action: "crowdsec_console_enroll_succeeded", Details: fmt.Sprintf("status=%s tenant=%s agent=%s correlation_id=%s", status.Status, status.Tenant, status.AgentName, status.CorrelationID)})
	}

	c.JSON(http.StatusOK, status)
}

// ConsoleStatus returns the current console enrollment status without secrets.
func (h *CrowdsecHandler) ConsoleStatus(c *gin.Context) {
	if !h.isConsoleEnrollmentEnabled() {
		c.JSON(http.StatusNotFound, gin.H{"error": "console enrollment disabled"})
		return
	}
	if h.Console == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "console enrollment unavailable"})
		return
	}

	status, err := h.Console.Status(c.Request.Context())
	if err != nil {
		logger.Log().WithError(err).Warn("failed to read console enrollment status")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read enrollment status"})
		return
	}
	c.JSON(http.StatusOK, status)
}

// DeleteConsoleEnrollment clears the local enrollment state to allow fresh enrollment.
// DELETE /api/v1/admin/crowdsec/console/enrollment
// Note: This does NOT unenroll from crowdsec.net - that must be done manually on the console.
func (h *CrowdsecHandler) DeleteConsoleEnrollment(c *gin.Context) {
	if !h.isConsoleEnrollmentEnabled() {
		c.JSON(http.StatusNotFound, gin.H{"error": "console enrollment disabled"})
		return
	}
	if h.Console == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "console enrollment service not available"})
		return
	}

	ctx := c.Request.Context()
	if err := h.Console.ClearEnrollment(ctx); err != nil {
		logger.Log().WithError(err).Warn("failed to clear console enrollment state")
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "enrollment state cleared"})
}

// GetCachedPreset returns cached preview for a slug when available.
func (h *CrowdsecHandler) GetCachedPreset(c *gin.Context) {
	if !h.isCerberusEnabled() {
		c.JSON(http.StatusNotFound, gin.H{"error": "cerberus disabled"})
		return
	}
	if h.Hub == nil || h.Hub.Cache == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "hub cache unavailable"})
		return
	}
	ctx := c.Request.Context()
	slug := strings.TrimSpace(c.Param("slug"))
	if slug == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "slug required"})
		return
	}
	preview, err := h.Hub.Cache.LoadPreview(ctx, slug)
	if err != nil {
		if errors.Is(err, crowdsec.ErrCacheMiss) || errors.Is(err, crowdsec.ErrCacheExpired) {
			c.JSON(http.StatusNotFound, gin.H{"error": "cache miss"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	meta, metaErr := h.Hub.Cache.Load(ctx, slug)
	if metaErr != nil && !errors.Is(metaErr, crowdsec.ErrCacheMiss) && !errors.Is(metaErr, crowdsec.ErrCacheExpired) {
		c.JSON(http.StatusInternalServerError, gin.H{"error": metaErr.Error()})
		return
	}
	cacheTTL := h.Hub.Cache.TTL()
	now := time.Now().UTC()
	c.JSON(http.StatusOK, gin.H{
		"preview":               preview,
		"cache_key":             meta.CacheKey,
		"etag":                  meta.Etag,
		"retrieved_at":          meta.RetrievedAt,
		"ttl_remaining_seconds": ttlRemainingSeconds(now, meta.RetrievedAt, cacheTTL),
	})
}

// CrowdSecDecision represents a ban decision from CrowdSec
type CrowdSecDecision struct {
	ID        int64     `json:"id"`
	Origin    string    `json:"origin"`
	Type      string    `json:"type"`
	Scope     string    `json:"scope"`
	Value     string    `json:"value"`
	Duration  string    `json:"duration"`
	Scenario  string    `json:"scenario"`
	CreatedAt time.Time `json:"created_at"`
	Until     string    `json:"until,omitempty"`
}

// cscliDecision represents the JSON output from cscli decisions list
type cscliDecision struct {
	ID        int64  `json:"id"`
	Origin    string `json:"origin"`
	Type      string `json:"type"`
	Scope     string `json:"scope"`
	Value     string `json:"value"`
	Duration  string `json:"duration"`
	Scenario  string `json:"scenario"`
	CreatedAt string `json:"created_at"`
	Until     string `json:"until"`
}

// lapiDecision represents the JSON structure from CrowdSec LAPI /v1/decisions
type lapiDecision struct {
	ID        int64  `json:"id"`
	Origin    string `json:"origin"`
	Type      string `json:"type"`
	Scope     string `json:"scope"`
	Value     string `json:"value"`
	Duration  string `json:"duration"`
	Scenario  string `json:"scenario"`
	CreatedAt string `json:"created_at,omitempty"`
	Until     string `json:"until,omitempty"`
}

const (
	// Default CrowdSec LAPI port to avoid conflict with Charon management API on port 8080.
	defaultCrowdsecLAPIPort = 8085
)

// validateCrowdsecLAPIBaseURLFunc is a variable holding the LAPI URL validation function.
// This indirection allows tests to inject a permissive validator for mock servers.
var validateCrowdsecLAPIBaseURLFunc = validateCrowdsecLAPIBaseURLDefault

func validateCrowdsecLAPIBaseURLDefault(raw string) (*url.URL, error) {
	return security.ValidateInternalServiceBaseURL(raw, defaultCrowdsecLAPIPort, security.InternalServiceHostAllowlist())
}

func validateCrowdsecLAPIBaseURL(raw string) (*url.URL, error) {
	return validateCrowdsecLAPIBaseURLFunc(raw)
}

// GetLAPIDecisions queries CrowdSec LAPI directly for current decisions.
// This is an alternative to ListDecisions which uses cscli.
// Query params:
//   - ip: filter by specific IP address
//   - scope: filter by scope (e.g., "ip", "range")
//   - type: filter by decision type (e.g., "ban", "captcha")
func (h *CrowdsecHandler) GetLAPIDecisions(c *gin.Context) {
	// Get LAPI URL from security config or use default
	// Default port is 8085 to avoid conflict with Charon management API on port 8080
	lapiURL := "http://127.0.0.1:8085"
	if h.Security != nil {
		cfg, err := h.Security.Get()
		if err == nil && cfg != nil && cfg.CrowdSecAPIURL != "" {
			lapiURL = cfg.CrowdSecAPIURL
		}
	}

	baseURL, err := validateCrowdsecLAPIBaseURL(lapiURL)
	if err != nil {
		logger.Log().WithError(err).WithField("lapi_url", lapiURL).Warn("Blocked CrowdSec LAPI URL by internal allowlist policy")
		// Fallback to cscli-based method.
		h.ListDecisions(c)
		return
	}

	q := url.Values{}
	if ip := strings.TrimSpace(c.Query("ip")); ip != "" {
		q.Set("ip", ip)
	}
	if scope := strings.TrimSpace(c.Query("scope")); scope != "" {
		q.Set("scope", scope)
	}
	if decisionType := strings.TrimSpace(c.Query("type")); decisionType != "" {
		q.Set("type", decisionType)
	}

	endpoint := baseURL.ResolveReference(&url.URL{Path: "/v1/decisions"})
	endpoint.RawQuery = q.Encode()
	// Use validated+rebuilt URL for request construction (taint break).
	reqURL := endpoint.String()

	// Get API key
	apiKey := getLAPIKey()

	// Create HTTP request with timeout
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, http.NoBody)
	if err != nil {
		logger.Log().WithError(err).Warn("Failed to create LAPI decisions request")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create request"})
		return
	}

	// Add authentication header if API key is available
	if apiKey != "" {
		req.Header.Set("X-Api-Key", apiKey)
	}
	req.Header.Set("Accept", "application/json")

	// Execute request
	client := network.NewInternalServiceHTTPClient(10 * time.Second)
	resp, err := client.Do(req)
	if err != nil {
		logger.Log().WithError(err).WithField("lapi_url", baseURL.String()).Warn("Failed to query LAPI decisions")
		// Fallback to cscli-based method
		h.ListDecisions(c)
		return
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			logger.Log().WithError(err).Warn("Failed to close response body")
		}
	}()

	// Handle non-200 responses
	if resp.StatusCode == http.StatusUnauthorized {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "LAPI authentication failed - check API key configuration"})
		return
	}
	if resp.StatusCode != http.StatusOK {
		logger.Log().WithField("status", resp.StatusCode).WithField("lapi_url", baseURL.String()).Warn("LAPI returned non-OK status")
		// Fallback to cscli-based method
		h.ListDecisions(c)
		return
	}

	// Check content-type to ensure we're getting JSON (not HTML from a proxy/frontend)
	contentType := resp.Header.Get("Content-Type")
	if contentType != "" && !strings.Contains(contentType, "application/json") {
		logger.Log().WithField("content_type", contentType).WithField("lapi_url", baseURL.String()).Warn("LAPI returned non-JSON content-type, falling back to cscli")
		// Fallback to cscli-based method
		h.ListDecisions(c)
		return
	}

	// Parse response body
	body, err := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024)) // 10MB limit
	if err != nil {
		logger.Log().WithError(err).Warn("Failed to read LAPI response")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read response"})
		return
	}

	// Handle null/empty responses
	if len(body) == 0 || string(body) == "null" || string(body) == "null\n" {
		c.JSON(http.StatusOK, gin.H{"decisions": []CrowdSecDecision{}, "total": 0, "source": "lapi"})
		return
	}

	// Parse JSON
	var lapiDecisions []lapiDecision
	if err := json.Unmarshal(body, &lapiDecisions); err != nil {
		logger.Log().WithError(err).WithField("body", string(body)).Warn("Failed to parse LAPI decisions")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to parse LAPI response"})
		return
	}

	// Convert to our format
	decisions := make([]CrowdSecDecision, 0, len(lapiDecisions))
	for _, d := range lapiDecisions {
		var createdAt time.Time
		if d.CreatedAt != "" {
			createdAt, _ = time.Parse(time.RFC3339, d.CreatedAt)
		}
		decisions = append(decisions, CrowdSecDecision{
			ID:        d.ID,
			Origin:    d.Origin,
			Type:      d.Type,
			Scope:     d.Scope,
			Value:     d.Value,
			Duration:  d.Duration,
			Scenario:  d.Scenario,
			CreatedAt: createdAt,
			Until:     d.Until,
		})
	}

	c.JSON(http.StatusOK, gin.H{"decisions": decisions, "total": len(decisions), "source": "lapi"})
}

// getLAPIKey retrieves the LAPI API key from environment variables.
func getLAPIKey() string {
	envVars := []string{
		"CROWDSEC_API_KEY",
		"CROWDSEC_BOUNCER_API_KEY",
		"CERBERUS_SECURITY_CROWDSEC_API_KEY",
		"CHARON_SECURITY_CROWDSEC_API_KEY",
		"CPM_SECURITY_CROWDSEC_API_KEY",
	}
	for _, key := range envVars {
		if val := os.Getenv(key); val != "" {
			return val
		}
	}
	return ""
}

// CheckLAPIHealth verifies that CrowdSec LAPI is responding.
func (h *CrowdsecHandler) CheckLAPIHealth(c *gin.Context) {
	// Get LAPI URL from security config or use default
	// Default port is 8085 to avoid conflict with Charon management API on port 8080
	lapiURL := "http://127.0.0.1:8085"
	if h.Security != nil {
		cfg, err := h.Security.Get()
		if err == nil && cfg != nil && cfg.CrowdSecAPIURL != "" {
			lapiURL = cfg.CrowdSecAPIURL
		}
	}

	// Create health check request
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	baseURL, err := validateCrowdsecLAPIBaseURL(lapiURL)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"healthy": false, "error": "invalid LAPI URL (blocked by SSRF policy)", "lapi_url": lapiURL})
		return
	}

	healthURL := baseURL.ResolveReference(&url.URL{Path: "/health"}).String()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, healthURL, http.NoBody)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"healthy": false, "error": "failed to create request"})
		return
	}

	client := network.NewInternalServiceHTTPClient(5 * time.Second)
	resp, err := client.Do(req)
	if err != nil {
		// Try decisions endpoint as fallback health check
		decisionsURL := baseURL.ResolveReference(&url.URL{Path: "/v1/decisions"}).String()
		req2, _ := http.NewRequestWithContext(ctx, http.MethodHead, decisionsURL, http.NoBody)
		resp2, err2 := client.Do(req2)
		if err2 != nil {
			c.JSON(http.StatusOK, gin.H{"healthy": false, "error": "LAPI unreachable", "lapi_url": baseURL.String()})
			return
		}
		defer func() {
			if err := resp2.Body.Close(); err != nil {
				logger.Log().WithError(err).Warn("Failed to close response body")
			}
		}()
		// 401 is expected without auth but indicates LAPI is running
		if resp2.StatusCode == http.StatusOK || resp2.StatusCode == http.StatusUnauthorized {
			c.JSON(http.StatusOK, gin.H{"healthy": true, "lapi_url": baseURL.String(), "note": "health endpoint unavailable, verified via decisions endpoint"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"healthy": false, "error": "unexpected status", "status": resp2.StatusCode, "lapi_url": baseURL.String()})
		return
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			logger.Log().WithError(err).Warn("Failed to close response body")
		}
	}()

	c.JSON(http.StatusOK, gin.H{"healthy": resp.StatusCode == http.StatusOK, "lapi_url": baseURL.String(), "status": resp.StatusCode})
}

// ListDecisions calls cscli to get current decisions (banned IPs)
func (h *CrowdsecHandler) ListDecisions(c *gin.Context) {
	ctx := c.Request.Context()
	args := []string{"decisions", "list", "-o", "json"}
	if _, err := os.Stat(filepath.Join(h.DataDir, "config.yaml")); err == nil {
		args = append([]string{"-c", filepath.Join(h.DataDir, "config.yaml")}, args...)
	}
	output, err := h.CmdExec.Execute(ctx, "cscli", args...)
	if err != nil {
		// If cscli is not available or returns error, return empty list with warning
		logger.Log().WithError(err).Warn("Failed to execute cscli decisions list")
		c.JSON(http.StatusOK, gin.H{"decisions": []CrowdSecDecision{}, "error": "cscli not available or failed"})
		return
	}

	// Handle empty output (no decisions)
	if len(output) == 0 || string(output) == "null" || string(output) == "null\n" {
		c.JSON(http.StatusOK, gin.H{"decisions": []CrowdSecDecision{}, "total": 0})
		return
	}

	// Parse JSON output
	var rawDecisions []cscliDecision
	if err := json.Unmarshal(output, &rawDecisions); err != nil {
		logger.Log().WithError(err).WithField("output", string(output)).Warn("Failed to parse cscli decisions output")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to parse decisions"})
		return
	}

	// Convert to our format
	decisions := make([]CrowdSecDecision, 0, len(rawDecisions))
	for _, d := range rawDecisions {
		var createdAt time.Time
		if d.CreatedAt != "" {
			createdAt, _ = time.Parse(time.RFC3339, d.CreatedAt)
		}
		decisions = append(decisions, CrowdSecDecision{
			ID:        d.ID,
			Origin:    d.Origin,
			Type:      d.Type,
			Scope:     d.Scope,
			Value:     d.Value,
			Duration:  d.Duration,
			Scenario:  d.Scenario,
			CreatedAt: createdAt,
			Until:     d.Until,
		})
	}

	c.JSON(http.StatusOK, gin.H{"decisions": decisions, "total": len(decisions)})
}

// BanIPRequest represents the request body for banning an IP
type BanIPRequest struct {
	IP       string `json:"ip" binding:"required"`
	Duration string `json:"duration"`
	Reason   string `json:"reason"`
}

// BanIP adds a manual ban for an IP address
func (h *CrowdsecHandler) BanIP(c *gin.Context) {
	var req BanIPRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ip is required"})
		return
	}

	// Validate IP format (basic check)
	ip := strings.TrimSpace(req.IP)
	if ip == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ip cannot be empty"})
		return
	}

	// Default duration to 24h if not specified
	duration := req.Duration
	if duration == "" {
		duration = "24h"
	}

	// Build reason string
	reason := "manual ban"
	if req.Reason != "" {
		reason = fmt.Sprintf("manual ban: %s", req.Reason)
	}

	ctx := c.Request.Context()
	args := []string{"decisions", "add", "-i", ip, "-d", duration, "-R", reason, "-t", "ban"}
	if _, err := os.Stat(filepath.Join(h.DataDir, "config.yaml")); err == nil {
		args = append([]string{"-c", filepath.Join(h.DataDir, "config.yaml")}, args...)
	}
	_, err := h.CmdExec.Execute(ctx, "cscli", args...)
	if err != nil {
		logger.Log().WithError(err).WithField("ip", util.SanitizeForLog(ip)).Warn("Failed to execute cscli decisions add")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to ban IP"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "banned", "ip": ip, "duration": duration})
}

// UnbanIP removes a ban for an IP address
func (h *CrowdsecHandler) UnbanIP(c *gin.Context) {
	ip := c.Param("ip")
	if ip == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ip parameter required"})
		return
	}

	// Sanitize IP
	ip = strings.TrimSpace(ip)

	ctx := c.Request.Context()
	args := []string{"decisions", "delete", "-i", ip}
	if _, err := os.Stat(filepath.Join(h.DataDir, "config.yaml")); err == nil {
		args = append([]string{"-c", filepath.Join(h.DataDir, "config.yaml")}, args...)
	}
	_, err := h.CmdExec.Execute(ctx, "cscli", args...)
	if err != nil {
		logger.Log().WithError(err).WithField("ip", util.SanitizeForLog(ip)).Warn("Failed to execute cscli decisions delete")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to unban IP"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "unbanned", "ip": ip})
}

// RegisterBouncer registers a new bouncer or returns existing bouncer status.
// POST /api/v1/admin/crowdsec/bouncer/register
func (h *CrowdsecHandler) RegisterBouncer(c *gin.Context) {
	ctx := c.Request.Context()

	// Check if register_bouncer.sh script exists
	scriptPath := "/usr/local/bin/register_bouncer.sh"
	if _, err := os.Stat(scriptPath); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, gin.H{"error": "bouncer registration script not found"})
		return
	}

	// Run the registration script
	output, err := h.CmdExec.Execute(ctx, "bash", scriptPath)
	if err != nil {
		logger.Log().WithError(err).WithField("output", string(output)).Warn("Failed to register bouncer")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to register bouncer", "details": string(output)})
		return
	}

	// Parse output for API key (last line typically contains the key)
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	var apiKeyPreview string
	for _, line := range lines {
		// Look for lines that appear to be an API key (long alphanumeric string)
		line = strings.TrimSpace(line)
		if len(line) >= 32 && !strings.Contains(line, " ") && !strings.Contains(line, ":") {
			// Found what looks like an API key, show preview
			if len(line) > 8 {
				apiKeyPreview = line[:8] + "..."
			} else {
				apiKeyPreview = line + "..."
			}
			break
		}
	}

	// Check if bouncer is actually registered by querying cscli
	checkOutput, checkErr := h.CmdExec.Execute(ctx, "cscli", "bouncers", "list", "-o", "json")
	registered := false
	if checkErr == nil && len(checkOutput) > 0 && string(checkOutput) != "null" {
		if strings.Contains(string(checkOutput), "caddy-bouncer") {
			registered = true
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"status":          "registered",
		"bouncer_name":    "caddy-bouncer",
		"api_key_preview": apiKeyPreview,
		"registered":      registered,
	})
}

// GetAcquisitionConfig returns the current CrowdSec acquisition configuration.
// GET /api/v1/admin/crowdsec/acquisition
func (h *CrowdsecHandler) GetAcquisitionConfig(c *gin.Context) {
	acquisPath := "/etc/crowdsec/acquis.yaml"

	content, err := os.ReadFile(acquisPath)
	if err != nil {
		if os.IsNotExist(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": "acquisition config not found", "path": acquisPath})
			return
		}
		logger.Log().WithError(err).WithField("path", acquisPath).Warn("Failed to read acquisition config")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read acquisition config"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"content": string(content),
		"path":    acquisPath,
	})
}

// UpdateAcquisitionConfig updates the CrowdSec acquisition configuration.
// PUT /api/v1/admin/crowdsec/acquisition
func (h *CrowdsecHandler) UpdateAcquisitionConfig(c *gin.Context) {
	var payload struct {
		Content string `json:"content" binding:"required"`
	}
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "content is required"})
		return
	}

	acquisPath := "/etc/crowdsec/acquis.yaml"

	// Create backup of existing config if it exists
	var backupPath string
	if _, err := os.Stat(acquisPath); err == nil {
		backupPath = fmt.Sprintf("%s.backup.%s", acquisPath, time.Now().Format("20060102-150405"))
		if err := os.Rename(acquisPath, backupPath); err != nil {
			logger.Log().WithError(err).WithField("path", acquisPath).Warn("Failed to backup acquisition config")
			// Continue anyway - we'll try to write the new config
		}
	}

	// Write new config
	if err := os.WriteFile(acquisPath, []byte(payload.Content), 0o644); err != nil {
		logger.Log().WithError(err).WithField("path", acquisPath).Warn("Failed to write acquisition config")
		// Try to restore backup if it exists
		if backupPath != "" {
			_ = os.Rename(backupPath, acquisPath)
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to write acquisition config"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":      "updated",
		"backup":      backupPath,
		"reload_hint": true,
	})
}

// RegisterRoutes registers crowdsec admin routes under protected group
func (h *CrowdsecHandler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.POST("/admin/crowdsec/start", h.Start)
	rg.POST("/admin/crowdsec/stop", h.Stop)
	rg.GET("/admin/crowdsec/status", h.Status)
	rg.POST("/admin/crowdsec/import", h.ImportConfig)
	rg.GET("/admin/crowdsec/export", h.ExportConfig)
	rg.GET("/admin/crowdsec/files", h.ListFiles)
	rg.GET("/admin/crowdsec/file", h.ReadFile)
	rg.POST("/admin/crowdsec/file", h.WriteFile)
	rg.GET("/admin/crowdsec/presets", h.ListPresets)
	rg.POST("/admin/crowdsec/presets/pull", h.PullPreset)
	rg.POST("/admin/crowdsec/presets/apply", h.ApplyPreset)
	rg.GET("/admin/crowdsec/presets/cache/:slug", h.GetCachedPreset)
	rg.POST("/admin/crowdsec/console/enroll", h.ConsoleEnroll)
	rg.GET("/admin/crowdsec/console/status", h.ConsoleStatus)
	rg.DELETE("/admin/crowdsec/console/enrollment", h.DeleteConsoleEnrollment)
	// Decision management endpoints (Banned IP Dashboard)
	rg.GET("/admin/crowdsec/decisions", h.ListDecisions)
	rg.GET("/admin/crowdsec/decisions/lapi", h.GetLAPIDecisions)
	rg.GET("/admin/crowdsec/lapi/health", h.CheckLAPIHealth)
	rg.POST("/admin/crowdsec/ban", h.BanIP)
	rg.DELETE("/admin/crowdsec/ban/:ip", h.UnbanIP)
	// Bouncer registration endpoint
	rg.POST("/admin/crowdsec/bouncer/register", h.RegisterBouncer)
	// Acquisition configuration endpoints
	rg.GET("/admin/crowdsec/acquisition", h.GetAcquisitionConfig)
	rg.PUT("/admin/crowdsec/acquisition", h.UpdateAcquisitionConfig)
}
