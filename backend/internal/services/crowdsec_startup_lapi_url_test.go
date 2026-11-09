package services

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/crowdsec"
	"github.com/Wikid82/charon/backend/internal/crowdsec/bouncerkey"
)

// pointStartupKeyChecksAt makes the boot key check answer from baseURL, for the test only. The production options
// always address the local engine; this only redirects the request to a stub server.
func pointStartupKeyChecksAt(t *testing.T, baseURL string) {
	t.Helper()
	prev := startupKeyCheckOptions
	startupKeyCheckOptions = func() bouncerkey.CheckOptions {
		opts := prev()
		opts.BaseURL = baseURL
		return opts
	}
	t.Cleanup(func() { startupKeyCheckOptions = prev })
}

func TestStartupKeyCheckOptions_AddressesOnlyTheLocalEngine(t *testing.T) {
	t.Setenv(lapiURLEnvVar, "http://10.255.255.1:9")

	opts := startupKeyCheckOptions()

	assert.Equal(t, crowdsec.LocalLAPIURL, opts.BaseURL)
	assert.Equal(t, startupKeyCheckTiming.Wait, opts.Wait, "the boot retry policy is unchanged")
	assert.Equal(t, startupKeyCheckTiming.PerAttempt, opts.PerAttempt)
	assert.Equal(t, startupKeyCheckTiming.FirstBackoff, opts.FirstBackoff)
	assert.Equal(t, startupKeyCheckTiming.MaxBackoff, opts.MaxBackoff)
}

func TestStartupKeyValid_UsesTheCheckOptions(t *testing.T) {
	stub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	t.Cleanup(stub.Close)
	pointStartupKeyChecksAt(t, stub.URL)

	assert.Equal(t, bouncerkey.VerdictRejected, startupKeyValid(context.Background(), "a-key"))
}

func TestWarnIgnoredLAPIURLEnv(t *testing.T) {
	const want = "CHARON_SECURITY_CROWDSEC_API_URL is ignored for key checks; remove it from docker-compose.yml"
	cases := []struct {
		name    string
		value   string
		wantLog bool
	}{
		{name: "unset", value: "", wantLog: false},
		{name: "local engine", value: crowdsec.LocalLAPIURL, wantLog: false},
		{name: "localhost on the engine port", value: "http://localhost:8085/", wantLog: false},
		{name: "external address", value: "http://10.255.255.1:9", wantLog: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := newBouncerHarness(t)
			t.Setenv(lapiURLEnvVar, tc.value)

			warnIgnoredLAPIURLEnv()

			var warnings []string
			for _, e := range h.drain() {
				if e.Level == logrus.WarnLevel {
					warnings = append(warnings, e.Message)
				}
			}
			if !tc.wantLog {
				assert.Empty(t, warnings)
				return
			}
			require.Len(t, warnings, 1, "one line per call")
			assert.Equal(t, want, warnings[0])
			assert.NotContains(t, strings.Join(warnings, "\n"), "10.255.255.1", "the address value is never logged")
		})
	}
}

func TestReconcileCrowdSecOnStartup_WarnsOncePerStart(t *testing.T) {
	h := newBouncerHarness(t)
	t.Setenv(lapiURLEnvVar, "http://10.255.255.1:9")

	// A nil database returns right after the entry log, so this exercises only the once-per-start line.
	ReconcileCrowdSecOnStartup(nil, nil, "", h.dataDir, nil)

	count := 0
	for _, e := range h.drain() {
		if strings.Contains(e.Message, "is ignored for key checks") {
			count++
		}
	}
	assert.Equal(t, 1, count)
}
