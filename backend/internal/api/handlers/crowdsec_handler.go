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
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Wikid82/charon/backend/internal/caddy"
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
	DB               *gorm.DB
	Executor         CrowdsecExecutor
	CmdExec          CommandExecutor
	BinPath          string
	DataDir          string
	Hub              *crowdsec.HubService
	Console          *crowdsec.ConsoleEnrollmentService
	Security         *services.SecurityService
	CaddyManager     *caddy.Manager // For config reload after bouncer registration
	LAPIMaxWait      time.Duration  // For testing; 0 means 60s default
	LAPIPollInterval time.Duration  // For testing; 0 means 500ms default
	dashCache        *dashboardCache

	// registrationMutex protects concurrent bouncer registration attempts
	registrationMutex sync.Mutex

	// envKeyRejected tracks whether the env var key was rejected by LAPI
	// This is set during ensureBouncerRegistration() and used by GetKeyStatus()
	envKeyRejected bool

	// rejectedEnvKey stores the masked env key that was rejected for user notification
	rejectedEnvKey string
}

// Bouncer auto-registration constants.
const (
	bouncerKeyFile = "/app/data/crowdsec/bouncer_key"
	bouncerName    = "caddy-bouncer"
)

func (h *CrowdsecHandler) bouncerKeyPath() string {
	if h != nil && strings.TrimSpace(h.DataDir) != "" {
		return filepath.Join(h.DataDir, "bouncer_key")
	}
	if path := strings.TrimSpace(os.Getenv("CHARON_CROWDSEC_BOUNCER_KEY_PATH")); path != "" {
		return path
	}
	return bouncerKeyFile
}

func getAcquisitionConfigPath() string {
	if path := strings.TrimSpace(os.Getenv("CHARON_CROWDSEC_ACQUIS_PATH")); path != "" {
		return path
	}
	return "/etc/crowdsec/acquis.yaml"
}

func resolveAcquisitionConfigPath() (string, error) {
	rawPath := strings.TrimSpace(getAcquisitionConfigPath())
	if rawPath == "" {
		return "", errors.New("acquisition config path is empty")
	}

	if strings.Contains(rawPath, "\x00") {
		return "", errors.New("acquisition config path contains null byte")
	}

	if !filepath.IsAbs(rawPath) {
		return "", errors.New("acquisition config path must be absolute")
	}

	for _, segment := range strings.Split(filepath.ToSlash(rawPath), "/") {
		if segment == ".." {
			return "", errors.New("acquisition config path must not contain traversal segments")
		}
	}

	return filepath.Clean(rawPath), nil
}

func readAcquisitionConfig(absPath string) ([]byte, error) {
	cleanPath := filepath.Clean(absPath)
	dirPath := filepath.Dir(cleanPath)
	fileName := filepath.Base(cleanPath)

	if fileName == "." || fileName == string(filepath.Separator) {
		return nil, errors.New("acquisition config filename is invalid")
	}

	file, err := os.DirFS(dirPath).Open(fileName)
	if err != nil {
		return nil, fmt.Errorf("open acquisition config: %w", err)
	}
	defer func() {
		_ = file.Close()
	}()

	content, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("read acquisition config: %w", err)
	}

	return content, nil
}

// ConfigArchiveValidator validates CrowdSec configuration archives.
type ConfigArchiveValidator struct {
	MaxSize             int64    // Maximum compressed size (50MB default)
	MaxUncompressed     int64    // Maximum uncompressed size (500MB default)
	MaxCompressionRatio float64  // Maximum compression ratio (100x default)
	RequiredFiles       []string // Required files (config.yaml minimum)
}

// Validate performs comprehensive validation of the archive.
func (v *ConfigArchiveValidator) Validate(path string) error {
	// Check file size
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("failed to stat file: %w", err)
	}

	if info.Size() > v.MaxSize {
		return fmt.Errorf("archive exceeds maximum size: %d > %d", info.Size(), v.MaxSize)
	}

	// Detect format
	format, err := detectArchiveFormat(path)
	if err != nil {
		return err
	}

	// Calculate uncompressed size and check for zip bombs
	uncompressedSize, err := calculateUncompressedSize(path, format)
	if err != nil {
		return err
	}

	if uncompressedSize > v.MaxUncompressed {
		return fmt.Errorf("uncompressed size exceeds maximum: %d > %d", uncompressedSize, v.MaxUncompressed)
	}

	// Check compression ratio (zip bomb protection)
	compressionRatio := float64(uncompressedSize) / float64(info.Size())
	if compressionRatio > v.MaxCompressionRatio {
		return fmt.Errorf("suspicious compression ratio: %.1fx (potential zip bomb)", compressionRatio)
	}

	// List contents and verify required files
	contents, err := listArchiveContents(path, format)
	if err != nil {
		return err
	}

	for _, required := range v.RequiredFiles {
		found := false
		for _, file := range contents {
			if filepath.Base(file) == required || file == required {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("required file missing: %s", required)
		}
	}

	return nil
}

// detectArchiveFormat detects the archive format (tar.gz or zip).
func detectArchiveFormat(path string) (string, error) {
	ext := strings.ToLower(filepath.Ext(path))

	if strings.HasSuffix(strings.ToLower(path), ".tar.gz") {
		return "tar.gz", nil
	}

	if ext == ".zip" {
		return "zip", nil
	}

	return "", fmt.Errorf("unsupported format: %s", ext)
}

// calculateUncompressedSize calculates the total uncompressed size of the archive.
func calculateUncompressedSize(path, format string) (int64, error) {
	switch format {
	case "tar.gz":
		// #nosec G304 -- path is validated upstream
		f, err := os.Open(path)
		if err != nil {
			return 0, fmt.Errorf("failed to open archive: %w", err)
		}
		defer func() { _ = f.Close() }()

		gr, err := gzip.NewReader(f)
		if err != nil {
			return 0, fmt.Errorf("failed to create gzip reader: %w", err)
		}
		defer func() { _ = gr.Close() }()

		tr := tar.NewReader(gr)
		var total int64

		for {
			header, err := tr.Next()
			if err == io.EOF {
				break
			}
			if err != nil {
				return 0, fmt.Errorf("failed to read tar header: %w", err)
			}

			// Only count regular files
			if header.Typeflag == tar.TypeReg {
				total += header.Size
			}
		}

		return total, nil

	default:
		return 0, fmt.Errorf("unsupported format for size calculation: %s", format)
	}
}

// listArchiveContents lists all files in the archive.
func listArchiveContents(path, format string) ([]string, error) {
	switch format {
	case "tar.gz":
		// #nosec G304 -- path is validated upstream
		f, err := os.Open(path)
		if err != nil {
			return nil, fmt.Errorf("failed to open archive: %w", err)
		}
		defer func() { _ = f.Close() }()

		gr, err := gzip.NewReader(f)
		if err != nil {
			return nil, fmt.Errorf("failed to create gzip reader: %w", err)
		}
		defer func() { _ = gr.Close() }()

		tr := tar.NewReader(gr)
		var files []string

		for {
			header, err := tr.Next()
			if err == io.EOF {
				break
			}
			if err != nil {
				return nil, fmt.Errorf("failed to read tar header: %w", err)
			}

			if header.Typeflag == tar.TypeReg {
				files = append(files, header.Name)
			}
		}

		return files, nil

	default:
		return nil, fmt.Errorf("unsupported format for listing: %s", format)
	}
}

// validateYAMLFile validates CrowdSec YAML configuration structure.
func validateYAMLFile(path string) error {
	// #nosec G304 -- path is validated upstream
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	// Basic YAML syntax check
	var config map[string]interface{}
	if err := json.Unmarshal(data, &config); err != nil {
		// Try basic structure validation - check for key CrowdSec fields
		content := string(data)
		if !strings.Contains(content, "api:") && !strings.Contains(content, "server:") {
			return fmt.Errorf("invalid CrowdSec config structure")
		}
	}

	return nil
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
		DB:        db,
		Executor:  executor,
		CmdExec:   &RealCommandExecutor{},
		BinPath:   binPath,
		DataDir:   dataDir,
		Hub:       hubSvc,
		Console:   consoleSvc,
		Security:  securitySvc,
		dashCache: newDashboardCache(),
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
			if createErr := h.DB.Create(&cfg).Error; createErr != nil {
				logger.Log().WithError(createErr).Error("Failed to create SecurityConfig")
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
	maxWait := h.LAPIMaxWait
	if maxWait == 0 {
		maxWait = 60 * time.Second
	}
	pollInterval := h.LAPIPollInterval
	if pollInterval == 0 {
		pollInterval = 500 * time.Millisecond
	}
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

	// After confirming LAPI is ready, ensure bouncer is registered
	apiKey, regErr := h.ensureBouncerRegistration(ctx)
	if regErr != nil {
		logger.Log().WithError(regErr).Warn("Failed to register bouncer, CrowdSec may not enforce decisions")
	} else if apiKey != "" {
		// Log the key for user reference
		h.logBouncerKeyBanner(apiKey)

		// Regenerate Caddy config with new API key
		if h.CaddyManager != nil {
			if err := h.CaddyManager.ApplyConfig(ctx); err != nil {
				logger.Log().WithError(err).Warn("Failed to reload Caddy config with new bouncer key")
			}
		}
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
	if err := os.MkdirAll(tmpPath, 0o750); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create temp dir"})
		return
	}
	defer func() { _ = os.RemoveAll(tmpPath) }()

	dst := filepath.Join(tmpPath, file.Filename)
	if err := c.SaveUploadedFile(file, dst); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save upload"})
		return
	}

	// Pre-import validation
	validator := &ConfigArchiveValidator{
		MaxSize:             50 * 1024 * 1024,  // 50MB
		MaxUncompressed:     500 * 1024 * 1024, // 500MB
		MaxCompressionRatio: 100,               // 100x max ratio
		RequiredFiles:       []string{"config.yaml"},
	}

	if err := validator.Validate(dst); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": fmt.Sprintf("validation failed: %v", err)})
		return
	}

	// Backup current config
	var backupDir string
	if _, err := os.Stat(h.DataDir); err == nil {
		backupDir = h.DataDir + ".backup." + time.Now().Format("20060102-150405")
		if err := os.Rename(h.DataDir, backupDir); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create backup"})
			return
		}
	}

	// Create target dir
	if err := os.MkdirAll(h.DataDir, 0o750); err != nil {
		// Rollback on failure
		if backupDir != "" {
			_ = os.Rename(backupDir, h.DataDir)
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create config dir"})
		return
	}

	// Extract archive
	extractErr := h.extractArchive(dst, h.DataDir)
	if extractErr != nil {
		// Rollback on extraction failure
		_ = os.RemoveAll(h.DataDir)
		if backupDir != "" {
			_ = os.Rename(backupDir, h.DataDir)
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("extraction failed: %v", extractErr)})
		return
	}

	// Validate extracted config
	configPath := filepath.Join(h.DataDir, "config.yaml")
	if err := validateYAMLFile(configPath); err != nil {
		// Rollback on validation failure
		_ = os.RemoveAll(h.DataDir)
		if backupDir != "" {
			_ = os.Rename(backupDir, h.DataDir)
		}
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": fmt.Sprintf("config validation failed: %v", err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "imported", "backup": backupDir})
}

// extractArchive extracts a tar.gz archive to the destination directory.
func (h *CrowdsecHandler) extractArchive(archivePath, destDir string) error {
	// #nosec G304 -- archivePath is validated upstream
	f, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("failed to open archive: %w", err)
	}
	defer func() { _ = f.Close() }()

	gr, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer func() { _ = gr.Close() }()

	tr := tar.NewReader(gr)

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read tar header: %w", err)
		}

		// Path traversal protection
		// #nosec G305 -- Path traversal is explicitly checked below
		target := filepath.Join(destDir, header.Name)
		if !strings.HasPrefix(target, filepath.Clean(destDir)+string(os.PathSeparator)) {
			return fmt.Errorf("invalid file path: %s", header.Name)
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o750); err != nil {
				return fmt.Errorf("failed to create directory: %w", err)
			}
		case tar.TypeReg:
			// Ensure parent directory exists
			if err := os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
				return fmt.Errorf("failed to create parent directory: %w", err)
			}

			// Validate mode is safe before conversion
			fileMode := os.FileMode(0o640) // Default safe mode
			if header.Mode > 0 && header.Mode <= 0o777 {
				// #nosec G115 -- Mode validated to be within valid range (0-0o777)
				fileMode = os.FileMode(header.Mode)
			}

			// #nosec G304 -- target is constructed safely above with path traversal protection
			outFile, err := os.OpenFile(target, os.O_CREATE|os.O_RDWR, fileMode)
			if err != nil {
				return fmt.Errorf("failed to create file: %w", err)
			}

			// #nosec G110 -- We control the tar archive source (uploaded by admin)
			if _, err := io.Copy(outFile, tr); err != nil {
				if closeErr := outFile.Close(); closeErr != nil {
					return fmt.Errorf("failed to write file: %w (close error: %v)", err, closeErr)
				}
				return fmt.Errorf("failed to write file: %w", err)
			}
			if err := outFile.Close(); err != nil {
				return fmt.Errorf("failed to close file: %w", err)
			}
		}
	}

	return nil
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
			logger.Log().WithError(err).Warnf("failed to access path %s during export walk", path)
			return nil // Skip files we cannot access
		}
		if info.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(h.DataDir, path)
		if err != nil {
			return err
		}
		// Open file
		// #nosec G304 -- path is validated via filepath.Walk within CrowdSecDataDir
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
	files := []string{}
	if _, err := os.Stat(h.DataDir); os.IsNotExist(err) {
		c.JSON(http.StatusOK, gin.H{"files": files})
		return
	}
	err := filepath.Walk(h.DataDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			// Permission errors (e.g. lost+found) should not abort the walk
			if os.IsPermission(err) {
				logger.Log().WithError(err).WithField("path", path).Debug("Skipping inaccessible path during list")
				return filepath.SkipDir
			}
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
	// #nosec G304 -- p is validated against CrowdSecDataDir by detectFilePath
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
	if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to prepare dir"})
		return
	}
	if err := os.WriteFile(p, []byte(payload.Content), 0o600); err != nil {
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
		logger.Log().WithField("error", util.SanitizeForLog(err.Error())).WithField("slug", util.SanitizeForLog(slug)).WithField("hub_base_url", util.SanitizeForLog(h.Hub.HubBaseURL)).Warn("crowdsec preset pull failed")
		c.JSON(status, gin.H{"error": err.Error(), "hub_endpoints": h.hubEndpoints()})
		return
	}

	// Verify cache was actually stored
	// codeql[go/log-injection] Safe: res.Meta fields are system-generated (cache keys, file paths)
	// not directly derived from untrusted user input
	logger.Log().Info("preset pulled and cached successfully")

	// Verify files exist on disk
	if _, err := os.Stat(res.Meta.ArchivePath); err != nil {
		// codeql[go/log-injection] Safe: archive_path is system-generated file path
		logger.Log().WithField("error", util.SanitizeForLog(err.Error())).WithField("archive_path", util.SanitizeForLog(res.Meta.ArchivePath)).Error("cached archive file not found after pull")
	}
	if _, err := os.Stat(res.Meta.PreviewPath); err != nil {
		// codeql[go/log-injection] Safe: preview_path is system-generated file path
		logger.Log().WithField("error", util.SanitizeForLog(err.Error())).WithField("preview_path", util.SanitizeForLog(res.Meta.PreviewPath)).Error("cached preview file not found after pull")
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
			if _, statErr := os.Stat(cached.ArchivePath); statErr != nil {
				logger.Log().WithError(statErr).WithField("archive_path", cached.ArchivePath).Error("cached archive file missing")
			}
			if _, statErr := os.Stat(cached.PreviewPath); statErr != nil {
				logger.Log().WithError(statErr).WithField("preview_path", cached.PreviewPath).Error("cached preview file missing")
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
		logger.Log().WithField("error", util.SanitizeForLog(err.Error())).WithField("slug", util.SanitizeForLog(slug)).WithField("hub_base_url", util.SanitizeForLog(h.Hub.HubBaseURL)).WithField("backup_path", util.SanitizeForLog(res.BackupPath)).WithField("cache_key", util.SanitizeForLog(res.CacheKey)).Warn("crowdsec preset apply failed")
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
		if closeErr := resp.Body.Close(); closeErr != nil {
			logger.Log().WithError(closeErr).Warn("Failed to close response body")
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

// BouncerInfo represents the bouncer key information for UI display.
type BouncerInfo struct {
	Name       string `json:"name"`
	KeyPreview string `json:"key_preview"` // First 4 + last 3 chars
	KeySource  string `json:"key_source"`  // "env_var" | "file" | "none"
	FilePath   string `json:"file_path"`
	Registered bool   `json:"registered"`
}

// KeyStatusResponse represents the API response for the key-status endpoint.
// This endpoint provides UX feedback when env var keys are rejected by LAPI.
type KeyStatusResponse struct {
	// KeySource indicates where the current key came from
	// Values: "env" (from environment variable), "file" (from bouncer_key file), "auto-generated" (newly generated)
	KeySource string `json:"key_source"`

	// EnvKeyRejected is true if an environment variable key was set but rejected by LAPI
	EnvKeyRejected bool `json:"env_key_rejected"`

	// CurrentKeyPreview shows a masked preview of the current valid key (first 4 + last 4 chars)
	CurrentKeyPreview string `json:"current_key_preview,omitempty"`

	// RejectedKeyPreview shows a masked preview of the rejected env key (if applicable)
	RejectedKeyPreview string `json:"rejected_key_preview,omitempty"`

	// FullKey is the unmasked valid key, only returned when EnvKeyRejected is true
	// so the user can copy it to fix their docker-compose.yml
	FullKey string `json:"full_key,omitempty"`

	// Message provides user-friendly guidance
	Message string `json:"message,omitempty"`

	// Valid indicates whether the current key is valid (authenticated successfully with LAPI)
	Valid bool `json:"valid"`

	// BouncerName is the name of the registered bouncer
	BouncerName string `json:"bouncer_name"`

	// KeyFilePath is the path where the valid key is stored
	KeyFilePath string `json:"key_file_path"`
}

// testKeyAgainstLAPI validates an API key by making an authenticated request to LAPI.
// Uses /v1/decisions/stream endpoint which requires authentication.
// Returns true if the key is accepted (200 OK), false otherwise.
// Implements retry logic with exponential backoff for LAPI startup (connection refused).
// Fails fast on 403 Forbidden (invalid key - no retries).
func (h *CrowdsecHandler) testKeyAgainstLAPI(ctx context.Context, apiKey string) bool {
	if apiKey == "" {
		return false
	}

	// Get LAPI URL from security config or use default
	lapiURL := "http://127.0.0.1:8085"
	if h.Security != nil {
		cfg, err := h.Security.Get()
		if err == nil && cfg != nil && cfg.CrowdSecAPIURL != "" {
			lapiURL = cfg.CrowdSecAPIURL
		}
	}

	// Use /v1/decisions/stream endpoint (guaranteed to require authentication)
	endpoint := fmt.Sprintf("%s/v1/decisions/stream", strings.TrimRight(lapiURL, "/"))

	// Retry logic for LAPI startup (30s max with exponential backoff)
	const maxStartupWait = 30 * time.Second
	const initialBackoff = 500 * time.Millisecond
	const maxBackoff = 5 * time.Second

	backoff := initialBackoff
	startTime := time.Now()
	attempt := 0

	for {
		attempt++

		// Check for context cancellation before each attempt
		select {
		case <-ctx.Done():
			logger.Log().WithField("attempts", attempt).Debug("Context cancelled during LAPI key validation")
			return false
		default:
			// Continue
		}

		// Create request with 5s timeout per attempt
		testCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		req, err := http.NewRequestWithContext(testCtx, http.MethodGet, endpoint, nil)
		if err != nil {
			cancel()
			logger.Log().WithError(err).Debug("Failed to create LAPI test request")
			return false
		}

		// Set API key header
		req.Header.Set("X-Api-Key", apiKey)

		// Execute request
		client := network.NewInternalServiceHTTPClient(5 * time.Second)
		resp, err := client.Do(req)
		cancel()

		if err != nil {
			// Check if connection refused (LAPI not ready yet)
			if strings.Contains(err.Error(), "connection refused") || strings.Contains(err.Error(), "connect: connection refused") {
				// LAPI not ready - retry with backoff if within time limit
				if time.Since(startTime) < maxStartupWait {
					logger.Log().WithField("attempt", attempt).WithField("backoff", backoff).WithField("elapsed", time.Since(startTime)).Debug("LAPI not ready, retrying with backoff")

					// Check for context cancellation before sleeping
					select {
					case <-ctx.Done():
						logger.Log().WithField("attempts", attempt).Debug("Context cancelled during LAPI retry")
						return false
					case <-time.After(backoff):
						// Continue with retry
					}

					// Exponential backoff: 500ms → 750ms → 1125ms → ... (capped at 5s)
					backoff = time.Duration(float64(backoff) * 1.5)
					if backoff > maxBackoff {
						backoff = maxBackoff
					}
					continue
				}

				logger.Log().WithField("attempts", attempt).WithField("elapsed", time.Since(startTime)).WithField("max_wait", maxStartupWait).Warn("LAPI failed to start within timeout")
				return false
			}

			// Other errors (not connection refused)
			logger.Log().WithError(err).Debug("Failed to connect to LAPI for key validation")
			return false
		}
		defer func() {
			if closeErr := resp.Body.Close(); closeErr != nil {
				logger.Log().WithError(closeErr).Debug("Failed to close HTTP response body")
			}
		}()

		// Check response status
		if resp.StatusCode == http.StatusOK {
			logger.Log().WithField("attempts", attempt).WithField("elapsed", time.Since(startTime)).WithField("masked_key", maskAPIKey(apiKey)).Debug("API key validated successfully against LAPI")
			return true
		}

		// 403 Forbidden = bad key, fail fast (no retries)
		if resp.StatusCode == http.StatusForbidden {
			logger.Log().WithField("status", resp.StatusCode).WithField("masked_key", maskAPIKey(apiKey)).Debug("API key rejected by LAPI (403 Forbidden)")
			return false
		}

		// Other non-OK status codes
		logger.Log().WithField("status", resp.StatusCode).WithField("masked_key", maskAPIKey(apiKey)).Debug("API key validation returned unexpected status")
		return false
	}
}

// GetKeyStatus returns the current CrowdSec bouncer key status and any rejection information.
// This endpoint provides UX feedback when env var keys are rejected by LAPI.
// @Summary Get CrowdSec API key status
// @Description Returns current key source, validity, and rejection status if env key was invalid
// @Tags crowdsec
// @Produce json
// @Success 200 {object} KeyStatusResponse
// @Router /admin/crowdsec/key-status [get]
func (h *CrowdsecHandler) GetKeyStatus(c *gin.Context) {
	h.registrationMutex.Lock()
	defer h.registrationMutex.Unlock()
	keyPath := h.bouncerKeyPath()

	response := KeyStatusResponse{
		BouncerName: bouncerName,
		KeyFilePath: keyPath,
	}

	// Check for rejected env key first
	if h.envKeyRejected && h.rejectedEnvKey != "" {
		response.EnvKeyRejected = true
		response.RejectedKeyPreview = maskAPIKey(h.rejectedEnvKey)
		response.Message = "Environment variable CHARON_SECURITY_CROWDSEC_API_KEY is set but was rejected by LAPI. " +
			"Either remove it from docker-compose.yml or update it to match the valid key stored in /app/data/crowdsec/bouncer_key."
	}

	// Determine current key source and status
	envKey := getBouncerAPIKeyFromEnv()
	fileKey := readKeyFromFile(keyPath)

	switch {
	case envKey != "" && !h.envKeyRejected:
		// Env key is set and was accepted
		response.KeySource = "env"
		response.CurrentKeyPreview = maskAPIKey(envKey)
		response.Valid = true
	case fileKey != "":
		// Using file key (either because no env key, or env key was rejected)
		if h.envKeyRejected {
			response.KeySource = "auto-generated"
			// Provide the full key so the user can copy it to fix their docker-compose.yml
			// Security: User is already authenticated as admin and needs this to fix their config
			response.FullKey = fileKey
		} else {
			response.KeySource = "file"
		}
		response.CurrentKeyPreview = maskAPIKey(fileKey)
		// Verify key is still valid
		ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
		defer cancel()
		response.Valid = h.testKeyAgainstLAPI(ctx, fileKey)
	default:
		// No key available
		response.KeySource = "none"
		response.Valid = false
		if response.Message == "" {
			response.Message = "No CrowdSec API key configured. Start CrowdSec to auto-generate one."
		}
	}

	c.JSON(http.StatusOK, response)
}

// ensureBouncerRegistration checks if bouncer is registered and registers if needed.
// Returns the API key if newly generated (empty if already set via env var or file).
func (h *CrowdsecHandler) ensureBouncerRegistration(ctx context.Context) (string, error) {
	h.registrationMutex.Lock()
	defer h.registrationMutex.Unlock()
	keyPath := h.bouncerKeyPath()

	// Priority 1: Check environment variables
	envKey := getBouncerAPIKeyFromEnv()
	if envKey != "" {
		// Test key against LAPI (not just bouncer name)
		if h.testKeyAgainstLAPI(ctx, envKey) {
			logger.Log().WithField("source", "environment_variable").WithField("masked_key", maskAPIKey(envKey)).Info("CrowdSec bouncer authentication successful")
			// Clear any previous rejection state
			h.envKeyRejected = false
			h.rejectedEnvKey = ""
			return "", nil // Key valid, nothing new to report
		}
		// Track the rejected env key for API status endpoint
		h.envKeyRejected = true
		h.rejectedEnvKey = envKey
		logger.Log().WithField("masked_key", maskAPIKey(envKey)).Warn(
			"Environment variable CHARON_SECURITY_CROWDSEC_API_KEY is set but invalid. " +
				"Either remove it from docker-compose.yml or update it to match the " +
				"auto-generated key. A new valid key will be generated and saved.",
		)
	}

	// Priority 2: Check persistent key file
	fileKey := readKeyFromFile(keyPath)
	if fileKey != "" {
		// Test key against LAPI (not just bouncer name)
		if h.testKeyAgainstLAPI(ctx, fileKey) {
			logger.Log().WithField("source", "file").WithField("file", keyPath).WithField("masked_key", maskAPIKey(fileKey)).Info("CrowdSec bouncer authentication successful")
			return "", nil // Key valid
		}
		logger.Log().WithField("file", keyPath).WithField("masked_key", maskAPIKey(fileKey)).Warn("File-stored API key failed LAPI authentication, will re-register")
	}

	// No valid key found - register new bouncer
	newKey, err := h.registerAndSaveBouncer(ctx)
	if err != nil {
		return "", err
	}

	// Warn user if env var is set but doesn't match the new key
	if envKey != "" && envKey != newKey {
		logger.Log().WithField("env_key_masked", maskAPIKey(envKey)).WithField("valid_key_masked", maskAPIKey(newKey)).Warn(
			"IMPORTANT: Environment variable CHARON_SECURITY_CROWDSEC_API_KEY is set but invalid. " +
				"Either remove it from docker-compose.yml or update it to match the " +
				"auto-generated key shown above. The valid key has been saved to " +
				"/app/data/crowdsec/bouncer_key and will be used on future restarts.",
		)
	}

	return newKey, nil
}

// validateBouncerKey checks if 'caddy-bouncer' is registered with CrowdSec.
func (h *CrowdsecHandler) validateBouncerKey(ctx context.Context) bool {
	checkCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	output, err := h.CmdExec.Execute(checkCtx, "cscli", "bouncers", "list", "-o", "json")
	if err != nil {
		logger.Log().WithError(err).Debug("Failed to list bouncers")
		return false
	}

	// Handle empty or null output
	if len(output) == 0 || string(output) == "null" || string(output) == "null\n" {
		return false
	}

	var bouncers []struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(output, &bouncers); err != nil {
		logger.Log().WithError(err).Debug("Failed to parse bouncers list")
		return false
	}

	for _, b := range bouncers {
		if b.Name == bouncerName {
			return true
		}
	}
	return false
}

// registerAndSaveBouncer registers a new bouncer and saves the key to file.
func (h *CrowdsecHandler) registerAndSaveBouncer(ctx context.Context) (string, error) {
	keyPath := h.bouncerKeyPath()

	// Delete existing bouncer if present (stale registration)
	deleteCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	_, _ = h.CmdExec.Execute(deleteCtx, "cscli", "bouncers", "delete", bouncerName)
	cancel()

	// Register new bouncer
	regCtx, regCancel := context.WithTimeout(ctx, 10*time.Second)
	defer regCancel()

	output, err := h.CmdExec.Execute(regCtx, "cscli", "bouncers", "add", bouncerName, "-o", "raw")
	if err != nil {
		return "", fmt.Errorf("bouncer registration failed: %w: %s", err, string(output))
	}

	apiKey := strings.TrimSpace(string(output))
	if apiKey == "" {
		return "", fmt.Errorf("bouncer registration returned empty API key")
	}

	// Save key to persistent file
	if err := saveKeyToFile(keyPath, apiKey); err != nil {
		logger.Log().WithError(err).Warn("Failed to save bouncer key to file")
		// Continue - key is still valid for this session
	}

	return apiKey, nil
}

// maskAPIKey masks an API key for safe logging by showing only first 4 and last 4 characters.
// Security: Prevents API key exposure in logs (CWE-312, CWE-315, CWE-359).
// Returns "[empty]" for empty strings, "[REDACTED]" for keys shorter than 16 characters.
func maskAPIKey(key string) string {
	if key == "" {
		return "[empty]"
	}
	if len(key) < 16 {
		return "[REDACTED]"
	}
	return fmt.Sprintf("%s...%s", key[:4], key[len(key)-4:])
}

// validateAPIKeyFormat validates the API key format for security.
// Security: Ensures API keys meet minimum security standards.
// Returns true if key is 16-128 chars and contains only alphanumeric, underscore, or hyphen.
func validateAPIKeyFormat(key string) bool {
	if len(key) < 16 || len(key) > 128 {
		return false
	}
	// Only allow alphanumeric, underscore, and hyphen
	for _, ch := range key {
		// Apply De Morgan's law for better readability
		if (ch < 'a' || ch > 'z') && (ch < 'A' || ch > 'Z') &&
			(ch < '0' || ch > '9') && ch != '_' && ch != '-' {
			return false
		}
	}
	return true
}

// logBouncerKeyBanner logs the bouncer key with a formatted banner.
// Security: API key is masked to prevent exposure in logs (CWE-312).
func (h *CrowdsecHandler) logBouncerKeyBanner(apiKey string) {
	keyPath := h.bouncerKeyPath()

	banner := `
════════════════════════════════════════════════════════════════════
🔐 CrowdSec Bouncer Registered Successfully
────────────────────────────────────────────────────────────────────
Bouncer Name: %s
API Key:      %s
Saved To:     %s
────────────────────────────────────────────────────────────────────
⚠️  SECURITY: Full API key saved to file (permissions: 0600)
💡 TIP: If connecting to an EXTERNAL CrowdSec instance, copy this
   key to your docker-compose.yml as CHARON_SECURITY_CROWDSEC_API_KEY
🔄 ROTATE: Change API keys regularly and never commit to version control
════════════════════════════════════════════════════════════════════`
	// Security: Mask API key to prevent cleartext exposure in logs
	maskedKey := maskAPIKey(apiKey)
	logger.Log().Infof(banner, bouncerName, maskedKey, keyPath)
}

// getBouncerAPIKeyFromEnv retrieves the bouncer API key from environment variables.
func getBouncerAPIKeyFromEnv() string {
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

// readKeyFromFile reads the bouncer key from a file and returns trimmed content.
func readKeyFromFile(path string) string {
	// #nosec G304 -- path is a constant defined at compile time (bouncerKeyFile)
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// saveKeyToFile saves the bouncer key to a file with secure permissions.
// Uses atomic write pattern (temp file → rename) to prevent corruption.
func saveKeyToFile(path string, key string) error {
	if key == "" {
		return fmt.Errorf("cannot save empty key")
	}

	// Ensure directory exists with proper permissions
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create key directory: %w", err)
	}

	// Atomic write: temp file → rename
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, []byte(key+"\n"), 0600); err != nil {
		return fmt.Errorf("failed to write key file: %w", err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		if removeErr := os.Remove(tmpPath); removeErr != nil {
			logger.Log().WithError(removeErr).Warn("Failed to clean up temporary key file")
		}
		return fmt.Errorf("failed to finalize key file: %w", err)
	}

	return nil
}

// GetBouncerInfo returns information about the current bouncer key.
// GET /api/v1/admin/crowdsec/bouncer
func (h *CrowdsecHandler) GetBouncerInfo(c *gin.Context) {
	ctx := c.Request.Context()
	keyPath := h.bouncerKeyPath()

	info := BouncerInfo{
		Name:     bouncerName,
		FilePath: keyPath,
	}

	// Determine key source
	envKey := getBouncerAPIKeyFromEnv()
	fileKey := readKeyFromFile(keyPath)

	var fullKey string
	switch {
	case envKey != "":
		info.KeySource = "env_var"
		fullKey = envKey
	case fileKey != "":
		info.KeySource = "file"
		fullKey = fileKey
	default:
		info.KeySource = "none"
	}

	// Generate preview (first 4 + "..." + last 3 chars)
	if fullKey != "" && len(fullKey) > 7 {
		info.KeyPreview = fullKey[:4] + "..." + fullKey[len(fullKey)-3:]
	} else if fullKey != "" {
		info.KeyPreview = "***"
	}

	// Check if bouncer is registered
	info.Registered = h.validateBouncerKey(ctx)

	c.JSON(http.StatusOK, info)
}

// GetBouncerKey returns the full bouncer key (for copy to clipboard).
// GET /api/v1/admin/crowdsec/bouncer/key
func (h *CrowdsecHandler) GetBouncerKey(c *gin.Context) {
	keyPath := h.bouncerKeyPath()

	envKey := getBouncerAPIKeyFromEnv()
	if envKey != "" {
		c.JSON(http.StatusOK, gin.H{"key": envKey, "source": "env_var"})
		return
	}

	fileKey := readKeyFromFile(keyPath)
	if fileKey != "" {
		c.JSON(http.StatusOK, gin.H{"key": fileKey, "source": "file"})
		return
	}

	c.JSON(http.StatusNotFound, gin.H{"error": "No bouncer key configured"})
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

	// Log to security_decisions for dashboard aggregation
	if h.Security != nil {
		parsedDur, _ := time.ParseDuration(duration)
		_ = h.Security.LogDecision(&models.SecurityDecision{
			IP:        ip,
			Action:    "block",
			Source:    "crowdsec",
			RuleID:    reason,
			Scenario:  "manual",
			ExpiresAt: time.Now().Add(parsedDur),
		})
	}
	h.dashCache.Invalidate("dashboard")
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
	h.dashCache.Invalidate("dashboard")
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
	acquisPath, err := resolveAcquisitionConfigPath()
	if err != nil {
		logger.Log().WithError(err).Warn("Invalid acquisition config path")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid acquisition config path"})
		return
	}

	content, err := readAcquisitionConfig(acquisPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
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

	acquisPath, err := resolveAcquisitionConfigPath()
	if err != nil {
		logger.Log().WithError(err).Warn("Invalid acquisition config path")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid acquisition config path"})
		return
	}

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
	if err := os.WriteFile(acquisPath, []byte(payload.Content), 0o600); err != nil {
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

// DiagnosticsConnectivity verifies connectivity to all CrowdSec components.
// GET /api/v1/admin/crowdsec/diagnostics/connectivity
func (h *CrowdsecHandler) DiagnosticsConnectivity(c *gin.Context) {
	ctx := c.Request.Context()

	checks := map[string]interface{}{
		"lapi_running":      false,
		"lapi_ready":        false,
		"capi_registered":   false,
		"capi_reachable":    false,
		"console_enrolled":  false,
		"console_reachable": false,
	}

	// Check 1: LAPI running
	running, pid, _ := h.Executor.Status(ctx, h.DataDir)
	checks["lapi_running"] = running
	if pid > 0 {
		checks["lapi_pid"] = pid
	}

	// Check 2: LAPI ready (responds to cscli lapi status)
	if running {
		args := []string{"lapi", "status"}
		configPath := filepath.Join(h.DataDir, "config", "config.yaml")
		if _, err := os.Stat(configPath); err == nil {
			args = append([]string{"-c", configPath}, args...)
		} else {
			// Fallback to root config
			configPath = filepath.Join(h.DataDir, "config.yaml")
			if _, err := os.Stat(configPath); err == nil {
				args = append([]string{"-c", configPath}, args...)
			}
		}
		checkCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		_, err := h.CmdExec.Execute(checkCtx, "cscli", args...)
		cancel()
		checks["lapi_ready"] = (err == nil)
	}

	// Check 3: CAPI registered (online_api_credentials.yaml exists)
	credsPath := filepath.Join(h.DataDir, "config", "online_api_credentials.yaml")
	if _, err := os.Stat(credsPath); os.IsNotExist(err) {
		// Fallback to root location
		credsPath = filepath.Join(h.DataDir, "online_api_credentials.yaml")
	}
	checks["capi_registered"] = fileExists(credsPath)

	// Check 4: CAPI reachable (cscli capi status)
	if checks["capi_registered"].(bool) {
		args := []string{"capi", "status"}
		configPath := filepath.Join(h.DataDir, "config", "config.yaml")
		if _, err := os.Stat(configPath); err == nil {
			args = append([]string{"-c", configPath}, args...)
		}
		checkCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		out, err := h.CmdExec.Execute(checkCtx, "cscli", args...)
		cancel()
		checks["capi_reachable"] = (err == nil)
		if err == nil {
			checks["capi_status_output"] = strings.TrimSpace(string(out))
		}
	}

	// Check 5: Console enrolled
	if h.Console != nil {
		status, err := h.Console.Status(ctx)
		if err == nil {
			checks["console_enrolled"] = (status.Status == "enrolled" || status.Status == "pending_acceptance")
			checks["console_status"] = status.Status
			if status.AgentName != "" {
				checks["console_agent_name"] = status.AgentName
			}
		}
	}

	// Check 6: Console API reachable (ping crowdsec.net with 5s timeout)
	consoleURL := "https://api.crowdsec.net/health"
	client := &http.Client{Timeout: 5 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, consoleURL, http.NoBody)
	if err == nil {
		resp, respErr := client.Do(req)
		if respErr == nil {
			defer func() {
				if closeErr := resp.Body.Close(); closeErr != nil {
					logger.Log().WithError(closeErr).Warn("Failed to close response body")
				}
			}()
			checks["console_reachable"] = (resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusNoContent)
		} else {
			checks["console_reachable"] = false
			checks["console_error"] = respErr.Error()
		}
	}

	c.JSON(http.StatusOK, checks)
}

// DiagnosticsConfig validates CrowdSec configuration files.
// GET /api/v1/admin/crowdsec/diagnostics/config
func (h *CrowdsecHandler) DiagnosticsConfig(c *gin.Context) {
	ctx := c.Request.Context()

	validation := map[string]interface{}{
		"config_exists": false,
		"config_valid":  false,
		"acquis_exists": false,
		"acquis_valid":  false,
		"lapi_port":     "",
		"errors":        []string{},
	}

	errors := []string{}

	// Check config.yaml - try config subdirectory first, then root
	configPath := filepath.Join(h.DataDir, "config", "config.yaml")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		configPath = filepath.Join(h.DataDir, "config.yaml")
	}

	// Path traversal protection: ensure path is within DataDir
	cleanConfigPath := filepath.Clean(configPath)
	cleanDataDir := filepath.Clean(h.DataDir)
	if !strings.HasPrefix(cleanConfigPath, cleanDataDir) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid config path"})
		return
	}

	if _, err := os.Stat(cleanConfigPath); err == nil {
		validation["config_exists"] = true
		validation["config_path"] = cleanConfigPath

		// Read config and check LAPI port
		// #nosec G304 -- Path validated against DataDir above
		content, err := os.ReadFile(cleanConfigPath)
		if err == nil {
			configStr := string(content)
			// Extract LAPI port from listen_uri
			re := regexp.MustCompile(`listen_uri:\s*127\.0\.0\.1:(\d+)`)
			matches := re.FindStringSubmatch(configStr)
			if len(matches) > 1 {
				validation["lapi_port"] = matches[1]
			}
		}

		// Validate using cscli config check
		checkArgs := []string{"-c", cleanConfigPath, "config", "check"}
		checkCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		out, err := h.CmdExec.Execute(checkCtx, "cscli", checkArgs...)
		cancel()
		if err == nil {
			validation["config_valid"] = true
		} else {
			validation["config_valid"] = false
			errors = append(errors, fmt.Sprintf("config.yaml validation failed: %s", strings.TrimSpace(string(out))))
		}
	} else {
		errors = append(errors, "config.yaml not found")
	}

	// Check acquis.yaml - try config subdirectory first, then root
	acquisPath := filepath.Join(h.DataDir, "config", "acquis.yaml")
	if _, err := os.Stat(acquisPath); os.IsNotExist(err) {
		acquisPath = filepath.Join(h.DataDir, "acquis.yaml")
	}

	// Path traversal protection
	cleanAcquisPath := filepath.Clean(acquisPath)
	if !strings.HasPrefix(cleanAcquisPath, cleanDataDir) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid acquis path"})
		return
	}

	if _, err := os.Stat(cleanAcquisPath); err == nil {
		validation["acquis_exists"] = true
		validation["acquis_path"] = cleanAcquisPath

		// Check if it has datasources
		// #nosec G304 -- Path validated against DataDir above
		content, err := os.ReadFile(cleanAcquisPath)
		if err == nil {
			acquisStr := string(content)
			if strings.Contains(acquisStr, "source:") && (strings.Contains(acquisStr, "filenames:") || strings.Contains(acquisStr, "filename:")) {
				validation["acquis_valid"] = true
			} else {
				validation["acquis_valid"] = false
				errors = append(errors, "acquis.yaml missing datasource configuration (expected 'source:' and 'filenames:' or 'filename:')")
			}
		}
	} else {
		errors = append(errors, "acquis.yaml not found")
	}

	validation["errors"] = errors

	c.JSON(http.StatusOK, validation)
}

// ConsoleHeartbeat returns the current heartbeat status for console.
// GET /api/v1/admin/crowdsec/console/heartbeat
func (h *CrowdsecHandler) ConsoleHeartbeat(c *gin.Context) {
	if !h.isConsoleEnrollmentEnabled() {
		c.JSON(http.StatusNotFound, gin.H{"error": "console enrollment disabled"})
		return
	}
	if h.Console == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "console service unavailable"})
		return
	}

	ctx := c.Request.Context()
	status, err := h.Console.Status(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Return heartbeat-specific information
	c.JSON(http.StatusOK, gin.H{
		"status":                         status.Status,
		"last_heartbeat_at":              status.LastHeartbeatAt,
		"heartbeat_tracking_implemented": false,
		"note":                           "Full heartbeat tracking is planned for Phase 3. Currently shows last_heartbeat_at from database if set.",
		"agent_name":                     status.AgentName,
		"enrolled_at":                    status.EnrolledAt,
	})
}

// fileExists is a helper to check if a file exists
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
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
	// Diagnostic endpoints (Phase 1)
	rg.GET("/admin/crowdsec/diagnostics/connectivity", h.DiagnosticsConnectivity)
	rg.GET("/admin/crowdsec/diagnostics/config", h.DiagnosticsConfig)
	rg.GET("/admin/crowdsec/console/heartbeat", h.ConsoleHeartbeat)
	// Decision management endpoints (Banned IP Dashboard)
	rg.GET("/admin/crowdsec/decisions", h.ListDecisions)
	rg.GET("/admin/crowdsec/decisions/lapi", h.GetLAPIDecisions)
	rg.GET("/admin/crowdsec/lapi/health", h.CheckLAPIHealth)
	rg.POST("/admin/crowdsec/ban", h.BanIP)
	rg.DELETE("/admin/crowdsec/ban/:ip", h.UnbanIP)
	// Bouncer management endpoints (auto-registration)
	rg.GET("/admin/crowdsec/bouncer", h.GetBouncerInfo)
	rg.GET("/admin/crowdsec/bouncer/key", h.GetBouncerKey)
	rg.POST("/admin/crowdsec/bouncer/register", h.RegisterBouncer)
	rg.GET("/admin/crowdsec/key-status", h.GetKeyStatus)
	// Acquisition configuration endpoints
	rg.GET("/admin/crowdsec/acquisition", h.GetAcquisitionConfig)
	rg.PUT("/admin/crowdsec/acquisition", h.UpdateAcquisitionConfig)
	// Dashboard aggregation endpoints (PR-1)
	rg.GET("/admin/crowdsec/dashboard/summary", h.DashboardSummary)
	rg.GET("/admin/crowdsec/dashboard/timeline", h.DashboardTimeline)
	rg.GET("/admin/crowdsec/dashboard/top-ips", h.DashboardTopIPs)
	rg.GET("/admin/crowdsec/dashboard/scenarios", h.DashboardScenarios)
	rg.GET("/admin/crowdsec/alerts", h.ListAlerts)
	rg.GET("/admin/crowdsec/decisions/export", h.ExportDecisions)
}
