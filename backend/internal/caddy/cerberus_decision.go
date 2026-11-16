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

	// A stored switch decides, whatever its value. A failed read falls through to the row and the static configuration.
	if sw, err := models.ResolveCerberusSwitchKeys(db); err == nil && sw != models.CerberusSwitchAbsent {
		return sw == models.CerberusSwitchOn
	}
	return cerbEnabled
}
