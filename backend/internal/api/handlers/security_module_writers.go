package handlers

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/services"
)

// Coded errors of the security config and ruleset writers. Each code is answered through respondCoded.
const (
	codeInvalidWAFMode        = "invalid_waf_mode"
	msgInvalidWAFMode         = "Choose disabled, monitor or block for the WAF mode."
	codeWAFRulesSourceUnknown = "waf_rules_source_unknown"
	msgWAFRulesSourceUnknown  = "That rule set does not exist. Choose one from the list."
	codeWAFRulesetInUse       = "waf_ruleset_in_use"
	msgWAFRulesetInUse        = "This rule set is the one the WAF uses. Choose another rule set first."
	msgModuleApplyFailed      = "Failed to reload configuration"
	msgModuleWriteFailed      = "Failed to update security module"
)

// writeModuleToggle writes one switch for the toggle routes inside tx. The module keys go through SetModuleState, which
// also enables the framework when the module is switched on. The Cerberus feature switch keeps its own two keys.
func writeModuleToggle(tx *gorm.DB, settingKey string, enabled bool, actor string) error {
	if models.IsSecurityModuleKey(settingKey) {
		return models.SetModuleState(tx, settingKey, enabled, actor)
	}

	value := "false"
	if enabled {
		value = "true"
	}
	category := "security"
	if settingKey == models.CerberusFeatureSettingKey {
		category = "feature"
	}
	if err := upsertSetting(tx, models.Setting{Key: settingKey, Value: value, Category: category, Type: "bool"}); err != nil {
		return err
	}
	return upsertSetting(tx, models.Setting{Key: models.CerberusLegacySettingKey, Value: value, Category: "security", Type: "bool"})
}

// restoreModuleState puts the pre-image of a failed module write back and drops the Cerberus cache so the restored
// state is what the middleware reads. The settings keys and the default row are restored in separate write-locked
// transactions, so a failure in one does not undo the other. Restore errors are logged: the caller answers with the
// apply failure either way.
func restoreModuleState(db *gorm.DB, cerberus CacheInvalidator, snapshot models.ModuleStateSnapshot) {
	err := services.WithWriteLock(db, &models.Setting{}, snapshot.RestoreSettings)
	if err != nil {
		log.WithError(err).Error("Failed to restore settings after the proxy apply failed")
	}
	err = services.WithWriteLock(db, &models.Setting{}, snapshot.RestoreRow)
	if err != nil {
		log.WithError(err).Error("Failed to restore security config after the proxy apply failed")
	}
	if cerberus != nil {
		cerberus.InvalidateCache()
	}
}

// respondConfigUpdateError answers a failed config save: a refused value with its code, a validation refusal with 400,
// and any storage failure with 500.
func respondConfigUpdateError(c *gin.Context, err error) {
	var invalid *services.InvalidConfigError
	switch {
	case errors.Is(err, services.ErrInvalidWAFMode):
		respondCoded(c, http.StatusBadRequest, codeInvalidWAFMode, msgInvalidWAFMode, nil)
	case errors.Is(err, services.ErrWAFRulesSourceUnknown):
		respondCoded(c, http.StatusUnprocessableEntity, codeWAFRulesSourceUnknown, msgWAFRulesSourceUnknown, nil)
	case errors.As(err, &invalid):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	default:
		log.WithError(err).Error("Failed to save security config")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save security config"})
	}
}
