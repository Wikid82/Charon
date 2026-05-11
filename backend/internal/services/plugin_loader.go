package services

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"plugin"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/pkg/dnsprovider"
	"gorm.io/gorm"
)

// PluginLoaderService manages loading and unloading of DNS provider plugins.
type PluginLoaderService struct {
	pluginDir     string
	allowedSigs   map[string]string // plugin name -> expected signature
	loadedPlugins map[string]string // plugin type -> file path
	db            *gorm.DB
	mu            sync.RWMutex
}

// NewPluginLoaderService creates a new plugin loader.
func NewPluginLoaderService(db *gorm.DB, pluginDir string, allowedSignatures map[string]string) *PluginLoaderService {
	return &PluginLoaderService{
		pluginDir:     pluginDir,
		allowedSigs:   allowedSignatures,
		loadedPlugins: make(map[string]string),
		db:            db,
	}
}

// LoadAllPlugins loads all .so files from the plugin directory.
func (s *PluginLoaderService) LoadAllPlugins() error {
	// Check if plugins are supported on this platform
	if runtime.GOOS == "windows" {
		logger.Log().Warn("Go plugins are not supported on Windows - only built-in providers available")
		return nil
	}

	if s.pluginDir == "" {
		logger.Log().Info("Plugin directory not configured, skipping plugin loading")
		return nil
	}

	// Check if directory exists
	entries, err := os.ReadDir(s.pluginDir)
	if err != nil {
		if os.IsNotExist(err) {
			logger.Log().Info("Plugin directory does not exist, skipping plugin loading")
			return nil
		}
		return fmt.Errorf("failed to read plugin directory: %w", err)
	}

	// Verify directory permissions (security requirement)
	if err := s.verifyDirectoryPermissions(s.pluginDir); err != nil {
		return fmt.Errorf("plugin directory has insecure permissions: %w", err)
	}

	loadedCount := 0
	failedCount := 0

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".so") {
			continue
		}

		pluginPath := filepath.Join(s.pluginDir, entry.Name())
		if err := s.LoadPlugin(pluginPath); err != nil {
			logger.Log().WithError(err).Warnf("Failed to load plugin: %s", entry.Name())
			failedCount++

			// Update plugin status in database
			s.updatePluginStatus(pluginPath, models.PluginStatusError, err.Error())
			continue
		}
		loadedCount++
	}

	logger.Log().Infof("Loaded %d external DNS provider plugins (%d failed)", loadedCount, failedCount)
	return nil
}

// LoadPlugin loads a single plugin from the specified path.
func (s *PluginLoaderService) LoadPlugin(path string) error {
	// Verify signature if allowlist is configured (nil = permissive, non-nil = strict)
	if s.allowedSigs != nil {
		pluginName := strings.TrimSuffix(filepath.Base(path), ".so")
		expectedSig, ok := s.allowedSigs[pluginName]
		if !ok {
			return fmt.Errorf("%w: %s", dnsprovider.ErrPluginNotInAllowlist, pluginName)
		}

		actualSig, err := s.computeSignature(path)
		if err != nil {
			return fmt.Errorf("failed to compute signature: %w", err)
		}

		if actualSig != expectedSig {
			return fmt.Errorf("%w: expected %s, got %s", dnsprovider.ErrSignatureMismatch, expectedSig, actualSig)
		}
	}

	// Load the plugin
	p, err := plugin.Open(path)
	if err != nil {
		return fmt.Errorf("%w: %v", dnsprovider.ErrPluginLoadFailed, err)
	}

	// Look up the Plugin symbol (support both T and *T)
	symbol, err := p.Lookup("Plugin")
	if err != nil {
		return fmt.Errorf("%w: missing 'Plugin' symbol: %v", dnsprovider.ErrInvalidPlugin, err)
	}

	// Assert the interface (handle both value and pointer)
	var provider dnsprovider.ProviderPlugin

	// Try direct interface assertion first
	provider, ok := symbol.(dnsprovider.ProviderPlugin)
	if !ok {
		// Try pointer to interface
		providerPtr, ok := symbol.(*dnsprovider.ProviderPlugin)
		if !ok {
			return fmt.Errorf("%w: 'Plugin' symbol does not implement ProviderPlugin interface", dnsprovider.ErrInvalidPlugin)
		}
		provider = *providerPtr
	}

	// Validate provider metadata
	meta := provider.Metadata()
	if meta.Type == "" || meta.Name == "" {
		return fmt.Errorf("%w: invalid metadata (missing type or name)", dnsprovider.ErrInvalidPlugin)
	}

	// Verify Go version compatibility
	if meta.GoVersion != "" && meta.GoVersion != runtime.Version() {
		logger.Log().Warnf("Plugin %s built with Go %s, host is %s - may cause compatibility issues",
			meta.Type, meta.GoVersion, runtime.Version())
	}

	// Verify interface version compatibility
	if meta.InterfaceVersion != "" && meta.InterfaceVersion != dnsprovider.InterfaceVersion {
		return fmt.Errorf("%w: plugin interface %s, host requires %s",
			dnsprovider.ErrInterfaceVersionMismatch, meta.InterfaceVersion, dnsprovider.InterfaceVersion)
	}

	// Initialize the provider
	if err := provider.Init(); err != nil {
		return fmt.Errorf("%w: %v", dnsprovider.ErrPluginInitFailed, err)
	}

	// Register with global registry
	if err := dnsprovider.Global().Register(provider); err != nil {
		// Cleanup on registration failure
		_ = provider.Cleanup()
		return fmt.Errorf("failed to register plugin: %w", err)
	}

	s.mu.Lock()
	s.loadedPlugins[provider.Type()] = path
	s.mu.Unlock()

	// Update database with success status
	now := time.Now()
	s.updatePluginRecord(path, meta, models.PluginStatusLoaded, "", &now)

	logger.Log().WithFields(map[string]interface{}{
		"type":    meta.Type,
		"name":    meta.Name,
		"version": meta.Version,
		"author":  meta.Author,
	}).Info("Loaded DNS provider plugin")

	return nil
}

// computeSignature calculates SHA-256 hash of plugin file.
func (s *PluginLoaderService) computeSignature(path string) (string, error) {
	// #nosec G304 -- path is from ReadDir iteration within pluginDir
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	hash := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(hash[:]), nil
}

// verifyDirectoryPermissions checks that the plugin directory has secure permissions.
func (s *PluginLoaderService) verifyDirectoryPermissions(dir string) error {
	info, err := os.Stat(dir)
	if err != nil {
		return err
	}

	// On Unix-like systems, check that directory is not world-writable
	if runtime.GOOS != "windows" {
		mode := info.Mode().Perm()
		if mode&0o002 != 0 { // Check if world-writable bit is set
			return fmt.Errorf("directory is world-writable (mode: %o) - this is a security risk", mode)
		}
	}

	return nil
}

// updatePluginStatus updates the status of a plugin in the database.
func (s *PluginLoaderService) updatePluginStatus(filePath, status, errorMsg string) {
	if s.db == nil {
		return
	}

	var pluginRecord models.Plugin
	result := s.db.Where("file_path = ?", filePath).First(&pluginRecord)
	if result.Error != nil {
		// Plugin not in database yet, skip
		return
	}

	updates := map[string]interface{}{
		"status": status,
		"error":  errorMsg,
	}

	if status == models.PluginStatusLoaded {
		now := time.Now()
		updates["loaded_at"] = &now
	}

	s.db.Model(&pluginRecord).Updates(updates)
}

// updatePluginRecord creates or updates a plugin record in the database.
func (s *PluginLoaderService) updatePluginRecord(filePath string, meta dnsprovider.ProviderMetadata, status, errorMsg string, loadedAt *time.Time) {
	if s.db == nil {
		return
	}

	var pluginRecord models.Plugin
	result := s.db.Where("file_path = ?", filePath).First(&pluginRecord)

	if result.Error != nil {
		// Create new record
		pluginRecord = models.Plugin{
			UUID:     generateUUID(),
			Name:     meta.Name,
			Type:     meta.Type,
			FilePath: filePath,
			Enabled:  true,
			Status:   status,
			Error:    errorMsg,
			Version:  meta.Version,
			Author:   meta.Author,
			LoadedAt: loadedAt,
		}
		s.db.Create(&pluginRecord)
	} else {
		// Update existing record
		updates := map[string]interface{}{
			"name":      meta.Name,
			"type":      meta.Type,
			"status":    status,
			"error":     errorMsg,
			"version":   meta.Version,
			"author":    meta.Author,
			"loaded_at": loadedAt,
		}
		s.db.Model(&pluginRecord).Updates(updates)
	}
}

// ListLoadedPlugins returns information about loaded external plugins.
func (s *PluginLoaderService) ListLoadedPlugins() []dnsprovider.ProviderMetadata {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var plugins []dnsprovider.ProviderMetadata
	for providerType := range s.loadedPlugins {
		if provider, ok := dnsprovider.Global().Get(providerType); ok {
			plugins = append(plugins, provider.Metadata())
		}
	}
	return plugins
}

// IsPluginLoaded checks if a provider type was loaded from an external plugin.
func (s *PluginLoaderService) IsPluginLoaded(providerType string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.loadedPlugins[providerType]
	return ok
}

// UnloadPlugin removes a plugin from the registry.
// Note: Go plugins cannot be truly unloaded from memory.
func (s *PluginLoaderService) UnloadPlugin(providerType string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Get provider for cleanup
	provider, ok := dnsprovider.Global().Get(providerType)
	if ok {
		// Call cleanup hook
		if err := provider.Cleanup(); err != nil {
			logger.Log().WithError(err).Warnf("Error during plugin cleanup: %s", providerType)
		}
	}

	// Unregister from global registry
	dnsprovider.Global().Unregister(providerType)

	// Remove from loaded plugins map
	delete(s.loadedPlugins, providerType)

	logger.Log().Infof("Unloaded plugin: %s (memory not reclaimed - restart required for full unload)", providerType)
	return nil
}

// Cleanup calls Cleanup() on all loaded plugins.
func (s *PluginLoaderService) Cleanup() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for providerType := range s.loadedPlugins {
		if provider, ok := dnsprovider.Global().Get(providerType); ok {
			if err := provider.Cleanup(); err != nil {
				logger.Log().WithError(err).Warnf("Error during plugin cleanup: %s", providerType)
			}
		}
	}

	return nil
}

// generateUUID generates a simple UUID (using timestamp-based approach for simplicity).
// In production, consider using github.com/google/uuid package.
func generateUUID() string {
	return fmt.Sprintf("%d-%d", time.Now().UnixNano(), time.Now().Unix())
}
