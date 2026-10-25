package models

import (
	"fmt"

	"gorm.io/gorm"
)

// DefaultSecurityConfigName is the Name of the singleton SecurityConfig row that the
// proxy, Cerberus, the security handlers and the CrowdSec lifecycle all read and write.
const DefaultSecurityConfigName = "default"

// FindDefaultSecurityConfig loads the singleton SecurityConfig row by name.
// Every reader goes through this helper so that no caller can drift onto a different row.
// A missing row is returned as an error wrapping gorm.ErrRecordNotFound; use errors.Is to test for it.
func FindDefaultSecurityConfig(db *gorm.DB) (SecurityConfig, error) {
	var cfg SecurityConfig
	if err := db.Where("name = ?", DefaultSecurityConfigName).First(&cfg).Error; err != nil {
		return SecurityConfig{}, fmt.Errorf("find default security config: %w", err)
	}
	return cfg, nil
}
