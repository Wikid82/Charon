package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Environment variables for the always-on sign-in throttle (issue #1317).
const (
	EnvAuthRateLimitEnabled         = "CHARON_AUTH_RATELIMIT_ENABLED"
	EnvAuthRateLimitLoginRequests   = "CHARON_AUTH_RATELIMIT_LOGIN_REQUESTS"
	EnvAuthRateLimitLoginWindow     = "CHARON_AUTH_RATELIMIT_LOGIN_WINDOW"
	EnvAuthRateLimitSessionRequests = "CHARON_AUTH_RATELIMIT_SESSION_REQUESTS"
	EnvAuthRateLimitSessionWindow   = "CHARON_AUTH_RATELIMIT_SESSION_WINDOW"
)

// Production defaults and bounds for the sign-in throttle.
const (
	DefaultAuthLoginRequests    = 10
	DefaultAuthLoginWindowSec   = 600
	DefaultAuthSessionRequests  = 60
	DefaultAuthSessionWindowSec = 60

	MinAuthRateLimitRequests  = 1
	MaxAuthRateLimitRequests  = 10_000
	MinAuthRateLimitWindowSec = 1
	MaxAuthRateLimitWindowSec = 86_400
)

// AuthRateLimitConfig configures the always-on throttle. Its zero value is the secure
// default (enabled, default budgets), so any hand-built config.Config is protected.
type AuthRateLimitConfig struct {
	Disabled         bool // set only by CHARON_AUTH_RATELIMIT_ENABLED=false
	LoginRequests    int  // 0 = default; -1 = malformed or non-positive env value
	LoginWindowSec   int
	SessionRequests  int
	SessionWindowSec int
}

// normalizeBudgetField resolves one budget field: 0 means "use the default"
// silently; anything else outside [minVal, maxVal] falls back with a warning.
func normalizeBudgetField(val, def, minVal, maxVal int, envVar string) (int, string) {
	switch {
	case val == 0:
		return def, ""
	case val == -1:
		return def, fmt.Sprintf("%s is not a positive integer; using the default %d", envVar, def)
	case val < minVal || val > maxVal:
		return def, fmt.Sprintf("%s must be between %d and %d; using the default %d", envVar, minVal, maxVal, def)
	default:
		return val, ""
	}
}

// Normalize returns the effective configuration (defaults applied, bounds
// enforced) plus one warning per field that fell back to its default. It is
// pure and idempotent.
func (c AuthRateLimitConfig) Normalize() (AuthRateLimitConfig, []string) {
	fields := []struct {
		val      *int
		def      int
		min, max int
		env      string
	}{
		{&c.LoginRequests, DefaultAuthLoginRequests, MinAuthRateLimitRequests, MaxAuthRateLimitRequests, EnvAuthRateLimitLoginRequests},
		{&c.LoginWindowSec, DefaultAuthLoginWindowSec, MinAuthRateLimitWindowSec, MaxAuthRateLimitWindowSec, EnvAuthRateLimitLoginWindow},
		{&c.SessionRequests, DefaultAuthSessionRequests, MinAuthRateLimitRequests, MaxAuthRateLimitRequests, EnvAuthRateLimitSessionRequests},
		{&c.SessionWindowSec, DefaultAuthSessionWindowSec, MinAuthRateLimitWindowSec, MaxAuthRateLimitWindowSec, EnvAuthRateLimitSessionWindow},
	}
	var warnings []string
	for _, f := range fields {
		var warning string
		*f.val, warning = normalizeBudgetField(*f.val, f.def, f.min, f.max, f.env)
		if warning != "" {
			warnings = append(warnings, warning)
		}
	}
	return c, warnings
}

// loadAuthRateLimitConfig reads the raw (un-normalized) throttle settings.
// Only a literal "false" (any case) disables the throttle; any other non-empty
// value other than "true" keeps it on and produces a warning.
func loadAuthRateLimitConfig() (AuthRateLimitConfig, []string) {
	var warnings []string
	cfg := AuthRateLimitConfig{
		LoginRequests:    getEnvIntStrictAny(EnvAuthRateLimitLoginRequests),
		LoginWindowSec:   getEnvIntStrictAny(EnvAuthRateLimitLoginWindow),
		SessionRequests:  getEnvIntStrictAny(EnvAuthRateLimitSessionRequests),
		SessionWindowSec: getEnvIntStrictAny(EnvAuthRateLimitSessionWindow),
	}

	enabled := strings.TrimSpace(os.Getenv(EnvAuthRateLimitEnabled))
	switch {
	case enabled == "", strings.EqualFold(enabled, "true"):
	case strings.EqualFold(enabled, "false"):
		cfg.Disabled = true
	default:
		warnings = append(warnings, fmt.Sprintf(
			"%s has unrecognized value %s; login protection stays on (only \"false\" turns it off)",
			EnvAuthRateLimitEnabled, strconv.Quote(enabled)))
	}
	return cfg, warnings
}

// getEnvIntStrictAny returns the first set value among keys as a positive int:
// unset => 0, a valid positive integer => that value, anything else => -1 (so
// Normalize can warn instead of silently using a default).
func getEnvIntStrictAny(keys ...string) int {
	raw := getEnvAny("", keys...)
	if raw == "" {
		return 0
	}
	v, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || v <= 0 {
		return -1
	}
	return v
}
