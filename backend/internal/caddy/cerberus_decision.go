package caddy

import (
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/Wikid82/charon/backend/internal/models"
)

// CerberusEnabledForProxy reports whether the proxy applies the Cerberus security suite. The precedence is the
// static config, then the default SecurityConfig row (row is nil when no row exists), then the feature key
// feature.cerberus.enabled, and only when that key is absent the legacy key security.cerberus.enabled. Only the
// value "true" (case-insensitive) turns a stored switch on.
//
// The caller passes the row it already read, so the proxy apply reads the row once. Boot reads the same decision
// through services.ProxyCerberusCheck.
func CerberusEnabledForProxy(db *gorm.DB, static config.SecurityConfig, row *models.SecurityConfig) bool {
	cerbEnabled := static.CerberusEnabled
	if row != nil {
		cerbEnabled = row.Enabled
	}
	if db == nil {
		return cerbEnabled
	}

	// A stored feature key decides even when its value is blank or garbage (both Off). Only an absent key falls
	// through to the legacy key, unlike the status endpoint, which skips blank values.
	for _, key := range []string{models.CerberusFeatureSettingKey, models.CerberusLegacySettingKey} {
		var s models.Setting
		err := db.Where("key = ?", key).First(&s).Error
		if sw := models.CerberusSwitchFromSetting(s.Value, err == nil); sw != models.CerberusSwitchAbsent {
			return sw == models.CerberusSwitchOn
		}
	}
	return cerbEnabled
}
