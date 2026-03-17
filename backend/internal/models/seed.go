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
		UUID:               uuid.NewString(),
		Name:               "default",
		Enabled:            false,
		CrowdSecMode:       "disabled",
		CrowdSecAPIURL:     "http://127.0.0.1:8085",
		WAFMode:            "disabled",
		WAFParanoiaLevel:   1,
		RateLimitMode:   "disabled",
		RateLimitEnable: false,
		// Zero values are intentional for the disabled default state.
		// cerberus.RateLimitMiddleware guards against zero/negative values by falling
		// back to safe operational defaults (requests=100, window=60s, burst=20) before
		// computing the token-bucket rate.  buildRateLimitHandler (caddy/config.go) also
		// returns nil — skipping rate-limit injection — when either value is ≤ 0.
		// A user enabling rate limiting via the UI without configuring thresholds will
		// therefore receive the safe hardcoded defaults, not a zero-rate limit.
		RateLimitBurst:     0,
		RateLimitRequests:  0,
		RateLimitWindowSec: 0,
	}

	// FirstOrCreate matches on Name only; if a row with name="default" already exists
	// it is loaded into record without modifying any of its fields.
	result := db.Where(SecurityConfig{Name: "default"}).FirstOrCreate(&record)
	if result.Error != nil {
		return nil, result.Error
	}
	return &record, nil
}
