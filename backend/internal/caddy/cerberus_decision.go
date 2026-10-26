package caddy

import (
	"strings"

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

	var s models.Setting
	if err := db.Where("key = ?", "feature.cerberus.enabled").First(&s).Error; err == nil {
		return strings.EqualFold(s.Value, "true")
	}
	if err := db.Where("key = ?", "security.cerberus.enabled").First(&s).Error; err == nil {
		return strings.EqualFold(s.Value, "true")
	}
	return cerbEnabled
}
