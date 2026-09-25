package config

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var authRateLimitEnvVars = []string{
	EnvAuthRateLimitEnabled,
	EnvAuthRateLimitLoginRequests,
	EnvAuthRateLimitLoginWindow,
	EnvAuthRateLimitSessionRequests,
	EnvAuthRateLimitSessionWindow,
}

// clearAuthRateLimitEnv isolates tests from any ambient auth throttle settings.
func clearAuthRateLimitEnv(t *testing.T) {
	t.Helper()
	for _, k := range authRateLimitEnvVars {
		t.Setenv(k, "")
	}
}

func setLoadTempDirs(t *testing.T) {
	t.Helper()
	tempDir := t.TempDir()
	t.Setenv("CHARON_DB_PATH", filepath.Join(tempDir, "test.db"))
	t.Setenv("CHARON_CADDY_CONFIG_DIR", filepath.Join(tempDir, "caddy"))
	t.Setenv("CHARON_IMPORT_DIR", filepath.Join(tempDir, "imports"))
}

func secureDefaults() AuthRateLimitConfig {
	return AuthRateLimitConfig{
		LoginRequests:    10,
		LoginWindowSec:   600,
		SessionRequests:  60,
		SessionWindowSec: 60,
	}
}

func warningsMention(warnings []string, s string) bool {
	for _, w := range warnings {
		if strings.Contains(w, s) {
			return true
		}
	}
	return false
}

func TestLoadAuthRateLimitConfig_Defaults(t *testing.T) {
	clearAuthRateLimitEnv(t)
	raw, warnings := loadAuthRateLimitConfig()
	assert.Equal(t, AuthRateLimitConfig{}, raw)
	assert.Empty(t, warnings)

	setLoadTempDirs(t)
	cfg, err := Load()
	require.NoError(t, err)
	assert.Equal(t, secureDefaults(), cfg.Security.AuthRateLimit, "production defaults are pinned here")
	assert.False(t, cfg.Security.AuthRateLimit.Disabled)
	assert.Empty(t, cfg.StartupWarnings)
}

func TestLoadAuthRateLimitConfig_Overrides(t *testing.T) {
	clearAuthRateLimitEnv(t)
	t.Setenv(EnvAuthRateLimitLoginRequests, "300")
	t.Setenv(EnvAuthRateLimitLoginWindow, "60")
	t.Setenv(EnvAuthRateLimitSessionRequests, "600")
	t.Setenv(EnvAuthRateLimitSessionWindow, " 120 ")

	raw, warnings := loadAuthRateLimitConfig()
	assert.Empty(t, warnings)
	norm, warnings := raw.Normalize()
	assert.Empty(t, warnings)
	assert.Equal(t, AuthRateLimitConfig{LoginRequests: 300, LoginWindowSec: 60, SessionRequests: 600, SessionWindowSec: 120}, norm)
}

func TestLoadAuthRateLimitConfig_MalformedSentinel(t *testing.T) {
	for _, bad := range []string{"0", "-5", "10m", "abc", "1.5", "99999999999999999999"} {
		t.Run(bad, func(t *testing.T) {
			clearAuthRateLimitEnv(t)
			t.Setenv(EnvAuthRateLimitLoginRequests, bad)
			raw, _ := loadAuthRateLimitConfig()
			assert.Equal(t, -1, raw.LoginRequests)

			norm, warnings := raw.Normalize()
			assert.Equal(t, DefaultAuthLoginRequests, norm.LoginRequests, "never a burst of 0 and never unlimited")
			require.Len(t, warnings, 1)
			assert.Contains(t, warnings[0], EnvAuthRateLimitLoginRequests)
		})
	}
}

func TestLoadAuthRateLimitConfig_EnabledTrueFalseOnly(t *testing.T) {
	cases := []struct {
		val          string
		wantDisabled bool
	}{{"false", true}, {"FALSE", true}, {" False ", true}, {"true", false}, {"TRUE", false}, {"", false}}
	for _, tc := range cases {
		val, wantDisabled := tc.val, tc.wantDisabled
		t.Run(val, func(t *testing.T) {
			clearAuthRateLimitEnv(t)
			t.Setenv(EnvAuthRateLimitEnabled, val)
			raw, warnings := loadAuthRateLimitConfig()
			assert.Equal(t, wantDisabled, raw.Disabled)
			assert.Empty(t, warnings)
		})
	}
}

func TestLoadAuthRateLimitConfig_EnabledUnrecognizedWarnsAndStaysOn(t *testing.T) {
	for _, val := range []string{"no", "0", "off", "disabled", "yes"} {
		t.Run(val, func(t *testing.T) {
			clearAuthRateLimitEnv(t)
			t.Setenv(EnvAuthRateLimitEnabled, val)
			raw, warnings := loadAuthRateLimitConfig()
			assert.False(t, raw.Disabled)
			require.Len(t, warnings, 1)
			assert.Contains(t, warnings[0], EnvAuthRateLimitEnabled)
			assert.Contains(t, warnings[0], "unrecognized value")
		})
	}
}

func TestAuthRateLimitConfig_NormalizeZeroValueIsSecureDefault(t *testing.T) {
	norm, warnings := AuthRateLimitConfig{}.Normalize()
	assert.Empty(t, warnings)
	assert.Equal(t, secureDefaults(), norm)
	assert.False(t, norm.Disabled, "the zero value must never fail open")
}

func TestAuthRateLimitConfig_NormalizeBounds(t *testing.T) {
	cases := []struct {
		name     string
		in       AuthRateLimitConfig
		want     AuthRateLimitConfig
		warnVars []string
	}{
		{
			name: "upper bounds accepted",
			in:   AuthRateLimitConfig{LoginRequests: 10_000, LoginWindowSec: 86_400, SessionRequests: 10_000, SessionWindowSec: 86_400},
			want: AuthRateLimitConfig{LoginRequests: 10_000, LoginWindowSec: 86_400, SessionRequests: 10_000, SessionWindowSec: 86_400},
		},
		{
			name: "lower bounds accepted",
			in:   AuthRateLimitConfig{LoginRequests: 1, LoginWindowSec: 1, SessionRequests: 1, SessionWindowSec: 1},
			want: AuthRateLimitConfig{LoginRequests: 1, LoginWindowSec: 1, SessionRequests: 1, SessionWindowSec: 1},
		},
		{
			name: "over bounds fall back with warnings",
			in:   AuthRateLimitConfig{LoginRequests: 10_001, LoginWindowSec: 86_401, SessionRequests: 20_000, SessionWindowSec: 100_000},
			want: secureDefaults(),
			warnVars: []string{
				EnvAuthRateLimitLoginRequests, EnvAuthRateLimitLoginWindow,
				EnvAuthRateLimitSessionRequests, EnvAuthRateLimitSessionWindow,
			},
		},
		{
			name:     "negative hand-built values fall back with warnings",
			in:       AuthRateLimitConfig{LoginRequests: -3, SessionWindowSec: -1},
			want:     secureDefaults(),
			warnVars: []string{EnvAuthRateLimitLoginRequests, EnvAuthRateLimitSessionWindow},
		},
		{
			name: "disabled flag preserved",
			in:   AuthRateLimitConfig{Disabled: true},
			want: AuthRateLimitConfig{Disabled: true, LoginRequests: 10, LoginWindowSec: 600, SessionRequests: 60, SessionWindowSec: 60},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			norm, warnings := tc.in.Normalize()
			assert.Equal(t, tc.want, norm)
			assert.Len(t, warnings, len(tc.warnVars))
			for _, v := range tc.warnVars {
				assert.True(t, warningsMention(warnings, v), "warning should name %s: %v", v, warnings)
			}

			again, warnings := norm.Normalize()
			assert.Equal(t, norm, again, "Normalize is idempotent")
			assert.Empty(t, warnings)
		})
	}
}

func TestLoad_StartupWarningsCollected(t *testing.T) {
	clearAuthRateLimitEnv(t)
	setLoadTempDirs(t)
	t.Setenv(EnvAuthRateLimitEnabled, "off")
	t.Setenv(EnvAuthRateLimitSessionWindow, "1h")

	cfg, err := Load()
	require.NoError(t, err)
	assert.False(t, cfg.Security.AuthRateLimit.Disabled)
	assert.Equal(t, DefaultAuthSessionWindowSec, cfg.Security.AuthRateLimit.SessionWindowSec)
	assert.True(t, warningsMention(cfg.StartupWarnings, EnvAuthRateLimitEnabled))
	assert.True(t, warningsMention(cfg.StartupWarnings, EnvAuthRateLimitSessionWindow))
}

func TestLoad_AuthRateLimitDisabled(t *testing.T) {
	clearAuthRateLimitEnv(t)
	setLoadTempDirs(t)
	t.Setenv(EnvAuthRateLimitEnabled, "false")
	cfg, err := Load()
	require.NoError(t, err)
	assert.True(t, cfg.Security.AuthRateLimit.Disabled)
	assert.Empty(t, cfg.StartupWarnings)
}

func TestGetEnvIntStrictAny(t *testing.T) {
	t.Setenv("TEST_STRICT_A", "")
	t.Setenv("TEST_STRICT_B", "")
	assert.Equal(t, 0, getEnvIntStrictAny("TEST_STRICT_A", "TEST_STRICT_B"), "unset => 0")

	t.Setenv("TEST_STRICT_B", "42")
	assert.Equal(t, 42, getEnvIntStrictAny("TEST_STRICT_A", "TEST_STRICT_B"))

	t.Setenv("TEST_STRICT_A", "7")
	assert.Equal(t, 7, getEnvIntStrictAny("TEST_STRICT_A", "TEST_STRICT_B"), "first set key wins")

	for _, bad := range []string{"0", "-1", "x", "5s", "  "} {
		t.Setenv("TEST_STRICT_A", bad)
		assert.Equal(t, -1, getEnvIntStrictAny("TEST_STRICT_A"), "value %q", bad)
	}
}
