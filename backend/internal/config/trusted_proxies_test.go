package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateTrustedProxies_AllValidKeepsTrimmedStrings(t *testing.T) {
	effective, warnings := ValidateTrustedProxies([]string{" 172.20.0.5 ", "::1", "10.0.0.0/24", "fd00::/8"})
	assert.Equal(t, []string{"172.20.0.5", "::1", "10.0.0.0/24", "fd00::/8"}, effective, "bare IPs are not rewritten to CIDRs")
	assert.Empty(t, warnings)
}

func TestValidateTrustedProxies_EmptyInput(t *testing.T) {
	effective, warnings := ValidateTrustedProxies(nil)
	assert.Empty(t, effective)
	assert.Empty(t, warnings)
}

func TestValidateTrustedProxies_InvalidEntryTrustsNothing(t *testing.T) {
	for _, bad := range []string{"not-a-cidr", "10.0.0.0/33", "fe80::1%eth0", "300.1.1.1"} {
		t.Run(bad, func(t *testing.T) {
			effective, warnings := ValidateTrustedProxies([]string{"10.0.0.0/8", bad, "192.168.1.1"})
			assert.Empty(t, effective)
			require.Len(t, warnings, 1)
			assert.Contains(t, warnings[0], "CHARON_TRUSTED_PROXIES")
			assert.Contains(t, warnings[0], "no proxy is trusted")
		})
	}
}

func TestValidateTrustedProxies_TrustAllWarns(t *testing.T) {
	for _, entry := range []string{"0.0.0.0/0", "::/0", "0.0.0.0/1", "0.0.0.0"} {
		t.Run(entry, func(t *testing.T) {
			effective, warnings := ValidateTrustedProxies([]string{"10.0.0.0/8", entry})
			assert.Equal(t, []string{"10.0.0.0/8", entry}, effective, "the operator's explicit choice is still used")
			require.Len(t, warnings, 1)
			assert.Contains(t, warnings[0], "every address")
		})
	}
}

func TestLoad_TrustedProxiesInvalidEntryWarns(t *testing.T) {
	clearAuthRateLimitEnv(t)
	setLoadTempDirs(t)
	t.Setenv("CHARON_TRUSTED_PROXIES", "10.0.0.0/8, bogus")
	cfg, err := Load()
	require.NoError(t, err)
	assert.Empty(t, cfg.Security.TrustedProxies)
	assert.True(t, warningsMention(cfg.StartupWarnings, "CHARON_TRUSTED_PROXIES"))
}
