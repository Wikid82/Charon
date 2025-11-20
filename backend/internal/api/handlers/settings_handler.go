package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/Wikid82/CaddyProxyManagerPlus/backend/internal/models"
)

type SettingsHandler struct {
	DB *gorm.DB
}

func NewSettingsHandler(db *gorm.DB) *SettingsHandler {
	return &SettingsHandler{DB: db}
}

// GetSettings returns all settings.
func (h *SettingsHandler) GetSettings(c *gin.Context) {
	var settings []models.Setting
	if err := h.DB.Find(&settings).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch settings"})
		return
	}

	// Convert to map for easier frontend consumption
	settingsMap := make(map[string]string)
	for _, s := range settings {
		settingsMap[s.Key] = s.Value
	}

	c.JSON(http.StatusOK, settingsMap)
}

type UpdateSettingRequest struct {
	Key      string `json:"key" binding:"required"`
	Value    string `json:"value" binding:"required"`
	Category string `json:"category"`
	Type     string `json:"type"`
}

// UpdateSetting updates or creates a setting.
func (h *SettingsHandler) UpdateSetting(c *gin.Context) {
	var req UpdateSettingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	setting := models.Setting{
		Key:   req.Key,
		Value: req.Value,
	}

	if req.Category != "" {
		setting.Category = req.Category
	}
	if req.Type != "" {
		setting.Type = req.Type
	}

	// Upsert
	if err := h.DB.Where(models.Setting{Key: req.Key}).Assign(setting).FirstOrCreate(&setting).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save setting"})
		return
	}

	c.JSON(http.StatusOK, setting)
}
