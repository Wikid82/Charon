package handlers

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/services"
	"github.com/Wikid82/charon/backend/pkg/dnsprovider"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// PluginHandler handles plugin-related API endpoints.
type PluginHandler struct {
	db           *gorm.DB
	pluginLoader *services.PluginLoaderService
}

// NewPluginHandler creates a new plugin handler.
func NewPluginHandler(db *gorm.DB, pluginLoader *services.PluginLoaderService) *PluginHandler {
	return &PluginHandler{
		db:           db,
		pluginLoader: pluginLoader,
	}
}

// PluginInfo represents plugin information for API responses.
type PluginInfo struct {
	ID               uint    `json:"id"`
	UUID             string  `json:"uuid"`
	Name             string  `json:"name"`
	Type             string  `json:"type"`
	Enabled          bool    `json:"enabled"`
	Status           string  `json:"status"`
	Error            string  `json:"error,omitempty"`
	Version          string  `json:"version,omitempty"`
	Author           string  `json:"author,omitempty"`
	IsBuiltIn        bool    `json:"is_built_in"`
	Description      string  `json:"description,omitempty"`
	DocumentationURL string  `json:"documentation_url,omitempty"`
	LoadedAt         *string `json:"loaded_at,omitempty"`
	CreatedAt        string  `json:"created_at"`
	UpdatedAt        string  `json:"updated_at"`
}

// ListPlugins returns all plugins (built-in and external).
// @Summary List all DNS provider plugins
// @Tags Plugins
// @Produce json
// @Success 200 {array} PluginInfo
// @Router /admin/plugins [get]
func (h *PluginHandler) ListPlugins(c *gin.Context) {
	var plugins []PluginInfo

	// Get all registered providers from the registry
	registeredProviders := dnsprovider.Global().List()

	// Create a map for quick lookup
	registeredMap := make(map[string]dnsprovider.ProviderPlugin)
	for _, p := range registeredProviders {
		registeredMap[p.Type()] = p
	}

	// Add all registered providers (built-in and loaded external)
	for providerType, provider := range registeredMap {
		meta := provider.Metadata()

		pluginInfo := PluginInfo{
			Type:             providerType,
			Name:             meta.Name,
			Version:          meta.Version,
			Author:           meta.Author,
			IsBuiltIn:        meta.IsBuiltIn,
			Description:      meta.Description,
			DocumentationURL: meta.DocumentationURL,
			Status:           models.PluginStatusLoaded,
			Enabled:          true,
		}

		// If it's an external plugin, try to get database record
		if !meta.IsBuiltIn {
			var dbPlugin models.Plugin
			if err := h.db.Where("type = ?", providerType).First(&dbPlugin).Error; err == nil {
				pluginInfo.ID = dbPlugin.ID
				pluginInfo.UUID = dbPlugin.UUID
				pluginInfo.Enabled = dbPlugin.Enabled
				pluginInfo.Status = dbPlugin.Status
				pluginInfo.Error = dbPlugin.Error
				pluginInfo.CreatedAt = dbPlugin.CreatedAt.Format("2006-01-02T15:04:05Z")
				pluginInfo.UpdatedAt = dbPlugin.UpdatedAt.Format("2006-01-02T15:04:05Z")
				if dbPlugin.LoadedAt != nil {
					loadedStr := dbPlugin.LoadedAt.Format("2006-01-02T15:04:05Z")
					pluginInfo.LoadedAt = &loadedStr
				}
			}
		}

		plugins = append(plugins, pluginInfo)
	}

	// Add external plugins that failed to load
	var failedPlugins []models.Plugin
	h.db.Where("status = ?", models.PluginStatusError).Find(&failedPlugins)

	for _, dbPlugin := range failedPlugins {
		// Only add if not already in list
		found := false
		for _, p := range plugins {
			if p.Type == dbPlugin.Type {
				found = true
				break
			}
		}

		if !found {
			pluginInfo := PluginInfo{
				ID:        dbPlugin.ID,
				UUID:      dbPlugin.UUID,
				Name:      dbPlugin.Name,
				Type:      dbPlugin.Type,
				Enabled:   dbPlugin.Enabled,
				Status:    dbPlugin.Status,
				Error:     dbPlugin.Error,
				Version:   dbPlugin.Version,
				Author:    dbPlugin.Author,
				IsBuiltIn: false,
				CreatedAt: dbPlugin.CreatedAt.Format("2006-01-02T15:04:05Z"),
				UpdatedAt: dbPlugin.UpdatedAt.Format("2006-01-02T15:04:05Z"),
			}
			if dbPlugin.LoadedAt != nil {
				loadedStr := dbPlugin.LoadedAt.Format("2006-01-02T15:04:05Z")
				pluginInfo.LoadedAt = &loadedStr
			}
			plugins = append(plugins, pluginInfo)
		}
	}

	c.JSON(http.StatusOK, plugins)
}

// GetPlugin returns details for a specific plugin.
// @Summary Get plugin details
// @Tags Plugins
// @Produce json
// @Param id path int true "Plugin ID"
// @Success 200 {object} PluginInfo
// @Router /admin/plugins/{id} [get]
func (h *PluginHandler) GetPlugin(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid plugin ID"})
		return
	}

	var plugin models.Plugin
	if err := h.db.First(&plugin, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Plugin not found"})
			return
		}
		logger.Log().WithError(err).Error("Failed to get plugin")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get plugin"})
		return
	}

	// Get provider metadata if loaded
	var description, docURL string
	if provider, ok := dnsprovider.Global().Get(plugin.Type); ok {
		meta := provider.Metadata()
		description = meta.Description
		docURL = meta.DocumentationURL
	}

	pluginInfo := PluginInfo{
		ID:               plugin.ID,
		UUID:             plugin.UUID,
		Name:             plugin.Name,
		Type:             plugin.Type,
		Enabled:          plugin.Enabled,
		Status:           plugin.Status,
		Error:            plugin.Error,
		Version:          plugin.Version,
		Author:           plugin.Author,
		IsBuiltIn:        false,
		Description:      description,
		DocumentationURL: docURL,
		CreatedAt:        plugin.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt:        plugin.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	}
	if plugin.LoadedAt != nil {
		loadedStr := plugin.LoadedAt.Format("2006-01-02T15:04:05Z")
		pluginInfo.LoadedAt = &loadedStr
	}

	c.JSON(http.StatusOK, pluginInfo)
}

// EnablePlugin enables a disabled plugin.
// @Summary Enable a plugin
// @Tags Plugins
// @Param id path int true "Plugin ID"
// @Success 200 {object} gin.H
// @Router /admin/plugins/{id}/enable [post]
func (h *PluginHandler) EnablePlugin(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid plugin ID"})
		return
	}

	var plugin models.Plugin
	if err := h.db.First(&plugin, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Plugin not found"})
			return
		}
		logger.Log().WithError(err).Error("Failed to get plugin")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get plugin"})
		return
	}

	if plugin.Enabled {
		c.JSON(http.StatusOK, gin.H{"message": "Plugin already enabled"})
		return
	}

	// Update database
	if err := h.db.Model(&plugin).Update("enabled", true).Error; err != nil {
		logger.Log().WithError(err).Error("Failed to enable plugin")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to enable plugin"})
		return
	}

	// Attempt to reload the plugin
	if err := h.pluginLoader.LoadPlugin(plugin.FilePath); err != nil {
		logger.Log().WithError(err).Warnf("Failed to reload enabled plugin: %s", plugin.Type)
		c.JSON(http.StatusOK, gin.H{
			"message": "Plugin enabled but failed to load. Check logs or restart server.",
			"error":   err.Error(),
		})
		return
	}

	logger.Log().Infof("Plugin enabled: %s", plugin.Type)
	c.JSON(http.StatusOK, gin.H{"message": fmt.Sprintf("Plugin %s enabled successfully", plugin.Name)})
}

// DisablePlugin disables an active plugin.
// @Summary Disable a plugin
// @Tags Plugins
// @Param id path int true "Plugin ID"
// @Success 200 {object} gin.H
// @Router /admin/plugins/{id}/disable [post]
func (h *PluginHandler) DisablePlugin(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid plugin ID"})
		return
	}

	var plugin models.Plugin
	if err := h.db.First(&plugin, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Plugin not found"})
			return
		}
		logger.Log().WithError(err).Error("Failed to get plugin")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get plugin"})
		return
	}

	if !plugin.Enabled {
		c.JSON(http.StatusOK, gin.H{"message": "Plugin already disabled"})
		return
	}

	// Check if any DNS providers are using this plugin
	var count int64
	h.db.Model(&models.DNSProvider{}).Where("provider_type = ?", plugin.Type).Count(&count)
	if count > 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("Cannot disable plugin: %d DNS provider(s) are using it", count),
		})
		return
	}

	// Update database
	if err := h.db.Model(&plugin).Update("enabled", false).Error; err != nil {
		logger.Log().WithError(err).Error("Failed to disable plugin")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to disable plugin"})
		return
	}

	// Unload from registry
	if err := h.pluginLoader.UnloadPlugin(plugin.Type); err != nil {
		logger.Log().WithError(err).Warnf("Failed to unload plugin: %s", plugin.Type)
	}

	logger.Log().Infof("Plugin disabled: %s", plugin.Type)
	c.JSON(http.StatusOK, gin.H{
		"message": fmt.Sprintf("Plugin %s disabled successfully. Restart required for full unload.", plugin.Name),
	})
}

// ReloadPlugins reloads all plugins from the plugin directory.
// @Summary Reload all plugins
// @Tags Plugins
// @Success 200 {object} gin.H
// @Router /admin/plugins/reload [post]
func (h *PluginHandler) ReloadPlugins(c *gin.Context) {
	if err := h.pluginLoader.LoadAllPlugins(); err != nil {
		logger.Log().WithError(err).Error("Failed to reload plugins")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to reload plugins", "details": err.Error()})
		return
	}

	loadedPlugins := h.pluginLoader.ListLoadedPlugins()
	logger.Log().Infof("Reloaded %d plugins", len(loadedPlugins))

	c.JSON(http.StatusOK, gin.H{
		"message": "Plugins reloaded successfully",
		"count":   len(loadedPlugins),
	})
}
