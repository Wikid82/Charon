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
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/Wikid82/charon/backend/internal/crowdsec"
	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/internal/models"

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

// Execute runs a command and returns its output
func (r *RealCommandExecutor) Execute(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	return cmd.Output()
}

// CrowdsecHandler manages CrowdSec process and config imports.
type CrowdsecHandler struct {
	DB       *gorm.DB
	Executor CrowdsecExecutor
	CmdExec  CommandExecutor
	BinPath  string
	DataDir  string
	Hub      *crowdsec.HubService
}

func NewCrowdsecHandler(db *gorm.DB, executor CrowdsecExecutor, binPath, dataDir string) *CrowdsecHandler {
	cacheDir := filepath.Join(dataDir, "hub_cache")
	cache, err := crowdsec.NewHubCache(cacheDir, 24*time.Hour)
	if err != nil {
		logger.Log().WithError(err).Warn("failed to init crowdsec hub cache")
	}
	hubSvc := crowdsec.NewHubService(&RealCommandExecutor{}, cache, dataDir)
	return &CrowdsecHandler{
		DB:       db,
		Executor: executor,
		CmdExec:  &RealCommandExecutor{},
		BinPath:  binPath,
		DataDir:  dataDir,
		Hub:      hubSvc,
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

// Start starts the CrowdSec process.
func (h *CrowdsecHandler) Start(c *gin.Context) {
	ctx := c.Request.Context()
	pid, err := h.Executor.Start(ctx, h.BinPath, h.DataDir)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "started", "pid": pid})
}

// Stop stops the CrowdSec process.
func (h *CrowdsecHandler) Stop(c *gin.Context) {
	ctx := c.Request.Context()
	if err := h.Executor.Stop(ctx, h.DataDir); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "stopped"})
}

// Status returns simple running state.
func (h *CrowdsecHandler) Status(c *gin.Context) {
	ctx := c.Request.Context()
	running, pid, err := h.Executor.Status(ctx, h.DataDir)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"running": running, "pid": pid})
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
				logger.Log().WithError(err).Warn("failed to close file while archiving", "path", path)
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
		Available   bool       `json:"available"`
		Cached      bool       `json:"cached"`
		CacheKey    string     `json:"cache_key,omitempty"`
		Etag        string     `json:"etag,omitempty"`
		RetrievedAt *time.Time `json:"retrieved_at,omitempty"`
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

	ctx := c.Request.Context()
	res, err := h.Hub.Pull(ctx, slug)
	if err != nil {
		logger.Log().WithError(err).WithField("slug", slug).Warn("crowdsec preset pull failed")
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
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

	ctx := c.Request.Context()
	res, err := h.Hub.Apply(ctx, slug)
	if err != nil {
		logger.Log().WithError(err).WithField("slug", slug).Warn("crowdsec preset apply failed")
		if h.DB != nil {
			_ = h.DB.Create(&models.CrowdsecPresetEvent{Slug: slug, Action: "apply", Status: "failed", CacheKey: res.CacheKey, BackupPath: res.BackupPath, Error: err.Error()}).Error
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "backup": res.BackupPath})
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
	meta, _ := h.Hub.Cache.Load(ctx, slug)
	c.JSON(http.StatusOK, gin.H{"preview": preview, "cache_key": meta.CacheKey, "etag": meta.Etag})
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

// ListDecisions calls cscli to get current decisions (banned IPs)
func (h *CrowdsecHandler) ListDecisions(c *gin.Context) {
	ctx := c.Request.Context()
	output, err := h.CmdExec.Execute(ctx, "cscli", "decisions", "list", "-o", "json")
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
	_, err := h.CmdExec.Execute(ctx, "cscli", args...)
	if err != nil {
		logger.Log().WithError(err).WithField("ip", ip).Warn("Failed to execute cscli decisions add")
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
	_, err := h.CmdExec.Execute(ctx, "cscli", args...)
	if err != nil {
		logger.Log().WithError(err).WithField("ip", ip).Warn("Failed to execute cscli decisions delete")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to unban IP"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "unbanned", "ip": ip})
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
	// Decision management endpoints (Banned IP Dashboard)
	rg.GET("/admin/crowdsec/decisions", h.ListDecisions)
	rg.POST("/admin/crowdsec/ban", h.BanIP)
	rg.DELETE("/admin/crowdsec/ban/:ip", h.UnbanIP)
}
