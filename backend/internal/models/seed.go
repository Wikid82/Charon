package models

import (
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// SeedDefaultSecurityConfig ensures a default SecurityConfig row exists in the database.
// It uses FirstOrCreate so it is safe to call on every startup — existing data is never
// overwritten.  Returns the upserted record and any error encountered.
func SeedDefaultSecurityConfig(db *gorm.DB) (*SecurityConfig, error) {
	record := SecurityConfig{
		UUID:             uuid.NewString(),
		Name:             DefaultSecurityConfigName,
		Enabled:          false,
		CrowdSecMode:     "disabled",
		CrowdSecAPIURL:   "http://127.0.0.1:8085",
		WAFMode:          "disabled",
		WAFParanoiaLevel: 1,
		RateLimitMode:    "disabled",
		RateLimitEnable:  false,
		// Zero values are intentional for the disabled default state. Turning the rate limit on fills each
		// zero threshold with StandardRateLimit* in the same write (SetModuleState). The in-process API limiter
		// falls back to the same values. buildRateLimitHandler (caddy/config.go) returns nil while either proxy
		// threshold is <= 0, so an instance that was enabled with zeros is reported as thresholds_unset.
		RateLimitBurst:     0,
		RateLimitRequests:  0,
		RateLimitWindowSec: 0,
	}

	// FirstOrCreate matches on Name only; if a row with name="default" already exists
	// it is loaded into record without modifying any of its fields.
	result := db.Where(SecurityConfig{Name: DefaultSecurityConfigName}).FirstOrCreate(&record)
	if result.Error != nil {
		return nil, result.Error
	}
	return &record, nil
}
