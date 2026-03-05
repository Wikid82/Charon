package handlers

import (
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/models"
)

// FeatureFlagsHandler exposes simple DB-backed feature flags with env fallback.
type FeatureFlagsHandler struct {
	DB *gorm.DB
}

func NewFeatureFlagsHandler(db *gorm.DB) *FeatureFlagsHandler {
	return &FeatureFlagsHandler{DB: db}
}

// defaultFlags lists the canonical feature flags we expose.
var defaultFlags = []string{
	"feature.cerberus.enabled",
	"feature.uptime.enabled",
	"feature.crowdsec.console_enrollment",
	"feature.notifications.engine.notify_v1.enabled",
	"feature.notifications.service.discord.enabled",
	"feature.notifications.service.email.enabled",
	"feature.notifications.service.gotify.enabled",
	"feature.notifications.service.webhook.enabled",
	"feature.notifications.security_provider_events.enabled", // Blocker 3: Add security_provider_events gate
}

var defaultFlagValues = map[string]bool{
	"feature.cerberus.enabled":                               false, // Cerberus OFF by default (per diagnostic fix)
	"feature.uptime.enabled":                                 true,  // Uptime enabled by default
	"feature.crowdsec.console_enrollment":                    false,
	"feature.notifications.engine.notify_v1.enabled":         false,
	"feature.notifications.service.discord.enabled":          false,
	"feature.notifications.service.email.enabled":            false,
	"feature.notifications.service.gotify.enabled":           false,
	"feature.notifications.service.webhook.enabled":          false,
	"feature.notifications.security_provider_events.enabled": false, // Blocker 3: Default disabled for this stage
}

// GetFlags returns a map of feature flag -> bool. DB setting takes precedence
// and falls back to environment variables if present.
func (h *FeatureFlagsHandler) GetFlags(c *gin.Context) {
	// Phase 0: Performance instrumentation
	startTime := time.Now()
	defer func() {
		latency := time.Since(startTime).Milliseconds()
		log.Printf("[METRICS] GET /feature-flags: %dms", latency)
	}()

	result := make(map[string]bool)

	// Phase 1: Batch query optimization - fetch all flags in single query (eliminating N+1)
	var settings []models.Setting
	if err := h.DB.Where("key IN ?", defaultFlags).Find(&settings).Error; err != nil {
		log.Printf("[ERROR] Failed to fetch feature flags: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch feature flags"})
		return
	}

	// Build map for O(1) lookup
	settingsMap := make(map[string]models.Setting)
	for _, s := range settings {
		settingsMap[s.Key] = s
	}

	// Process all flags using the map
	for _, key := range defaultFlags {
		defaultVal := true
		if v, ok := defaultFlagValues[key]; ok {
			defaultVal = v
		}

		// Check if flag exists in DB
		if s, exists := settingsMap[key]; exists {
			v := strings.ToLower(strings.TrimSpace(s.Value))
			b := v == "1" || v == "true" || v == "yes"
			result[key] = b
			continue
		}

		// Fallback to env vars. Try FEATURE_... and also stripped service name e.g. CERBERUS_ENABLED
		envKey := strings.ToUpper(strings.ReplaceAll(key, ".", "_"))
		if ev, ok := os.LookupEnv(envKey); ok {
			if bv, err := strconv.ParseBool(ev); err == nil {
				result[key] = bv
				continue
			}
			// accept 1/0
			result[key] = ev == "1"
			continue
		}

		// Try shorter variant after removing leading "feature."
		if strings.HasPrefix(key, "feature.") {
			short := strings.ToUpper(strings.ReplaceAll(strings.TrimPrefix(key, "feature."), ".", "_"))
			if ev, ok := os.LookupEnv(short); ok {
				if bv, err := strconv.ParseBool(ev); err == nil {
					result[key] = bv
					continue
				}
				result[key] = ev == "1"
				continue
			}
		}

		// Default based on declared flag value
		result[key] = defaultVal
	}

	c.JSON(http.StatusOK, result)
}

// UpdateFlags accepts a JSON object map[string]bool and upserts settings.
func (h *FeatureFlagsHandler) UpdateFlags(c *gin.Context) {
	// Phase 0: Performance instrumentation
	startTime := time.Now()
	defer func() {
		latency := time.Since(startTime).Milliseconds()
		log.Printf("[METRICS] PUT /feature-flags: %dms", latency)
	}()

	var payload map[string]bool
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Phase 1: Transaction wrapping - all updates in single atomic transaction
	if err := h.DB.Transaction(func(tx *gorm.DB) error {
		for k, v := range payload {
			// Only allow keys in the default list to avoid arbitrary settings
			allowed := false
			for _, ak := range defaultFlags {
				if ak == k {
					allowed = true
					break
				}
			}
			if !allowed {
				continue
			}

			s := models.Setting{Key: k, Value: strconv.FormatBool(v), Type: "bool", Category: "feature"}
			if err := tx.Where(models.Setting{Key: k}).Assign(s).FirstOrCreate(&s).Error; err != nil {
				return err // Rollback on error
			}
		}
		return nil
	}); err != nil {
		log.Printf("[ERROR] Failed to update feature flags: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update feature flags"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
