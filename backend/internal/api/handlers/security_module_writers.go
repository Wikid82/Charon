package handlers

import (
	"context"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/logger"
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
	codeRulesetEmpty          = "ruleset_empty"
	msgRulesetEmpty           = "A rule set needs at least one rule. Add a SecRule line."
	msgModuleApplyFailed      = "Failed to reload configuration"
	msgModuleWriteFailed      = "Failed to update security module"
)

// writeModuleToggle writes one switch inside tx: a module key goes through SetModuleState, which also enables the
// framework when the module is switched on. Any other key is the Cerberus switch, written on both of its keys.
func writeModuleToggle(tx *gorm.DB, settingKey string, enabled bool, actor string) error {
	if models.IsSecurityModuleKey(settingKey) {
		return models.SetModuleState(tx, settingKey, enabled, actor)
	}
	return models.SetCerberusSwitch(tx, enabled)
}

// errSwitchApplyFailed is returned by switchWriter.write when the proxy refuses the change. The pre-image is restored
// before it is returned.
var errSwitchApplyFailed = errors.New("security switch proxy apply failed")

// switchWriter writes one security switch, a module key or the Cerberus switch, with its proxy apply as one unit: the
// write and its snapshot share a write-locked transaction, the Cerberus cache is invalidated and the proxy is applied,
// and a refused apply restores the pre-image.
type switchWriter struct {
	db       *gorm.DB
	caddy    CaddyConfigManager
	cerberus CacheInvalidator
}

// write stores the switch and applies the proxy. It returns the stored setting. A storage failure is returned as it
// is. A refused apply restores the pre-image and returns errSwitchApplyFailed.
func (w switchWriter) write(ctx context.Context, key string, on bool, actor string) (models.Setting, error) {
	if err := w.writeBatch(ctx, actor, key, on, nil, nil); err != nil {
		return models.Setting{}, err
	}
	stored, category := "false", "security"
	if on {
		stored = "true"
	}
	if key == models.CerberusFeatureSettingKey {
		category = "feature"
	}
	return models.Setting{Key: key, Value: stored, Category: category, Type: "bool"}, nil
}

// writeBatch stores the switch together with the extra writes, in one write-locked transaction, and then applies the
// proxy. The extra writes run in the same transaction, so a storage failure in them rolls the switch back too. The keys
// in extraKeys are imaged with the switch, so a refused apply restores them as well. A refused apply restores the
// pre-image and returns errSwitchApplyFailed.
func (w switchWriter) writeBatch(ctx context.Context, actor, key string, on bool, extraKeys []string, extra func(tx *gorm.DB) error) error {
	var snapshot models.ModuleStateSnapshot
	err := services.WithWriteLock(w.db, &models.Setting{}, func(tx *gorm.DB) error {
		row, err := models.ReadModuleRow(tx, models.DefaultSecurityConfigName)
		if err != nil {
			return err
		}
		if snapshot, err = models.CaptureModuleState(tx, models.DefaultSecurityConfigName, row, append([]string{key}, extraKeys...)...); err != nil {
			return err
		}
		if err := writeModuleToggle(tx, key, on, actor); err != nil {
			return err
		}
		if extra != nil {
			if err := extra(tx); err != nil {
				return err
			}
		}
		return snapshot.Seal(tx)
	})
	if err != nil {
		return err
	}

	if w.cerberus != nil {
		w.cerberus.InvalidateCache()
	}
	if w.caddy != nil {
		if err := applyProxyConfigDetached(ctx, w.caddy, proxyApplyTimeout); err != nil {
			logger.Log().WithError(err).Warn("Failed to reload Caddy config after security setting change")
			restoreModuleState(w.db, w.cerberus, snapshot)
			return errSwitchApplyFailed
		}
		logger.Log().WithField("setting_key", sanitizeForLog(key)).Info("Caddy config reloaded after security setting change")
	}
	return nil
}

// restoreModuleState puts back the values a failed module write changed and drops the Cerberus cache so the restored
// state is what the middleware reads. Only the keys and columns the write touched are restored, and only while they
// still hold the value the write left: a newer concurrent write is kept (see models.ModuleStateSnapshot). The settings
// keys and the default row are restored in separate write-locked transactions, so a failure in one does not undo the
// other. Restore errors are logged: the caller answers with the apply failure either way.
func restoreModuleState(db *gorm.DB, cerberus CacheInvalidator, snapshot models.ModuleStateSnapshot) {
	err := services.WithWriteLock(db, &models.Setting{}, snapshot.RestoreSettings)
	if err != nil {
		logger.Log().WithError(err).Error("Failed to restore settings after the proxy apply failed")
	}
	err = services.WithWriteLock(db, &models.Setting{}, snapshot.RestoreRow)
	if err != nil {
		logger.Log().WithError(err).Error("Failed to restore security config after the proxy apply failed")
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
		logger.Log().WithError(err).Error("Failed to save security config")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save security config"})
	}
}
