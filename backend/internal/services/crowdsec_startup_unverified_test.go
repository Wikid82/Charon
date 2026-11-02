package services

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/crowdsec/bouncerkey"
)

// shortenStartupKeyCheck makes the startup key check give up quickly, so tests with an unreachable,
// slow or erroring LAPI do not wait for the production 10 s window.
func shortenStartupKeyCheck(t *testing.T) {
	t.Helper()
	saved := startupKeyCheckTiming
	startupKeyCheckTiming.Wait = 300 * time.Millisecond
	startupKeyCheckTiming.PerAttempt = 100 * time.Millisecond
	startupKeyCheckTiming.FirstBackoff = 5 * time.Millisecond
	startupKeyCheckTiming.MaxBackoff = 20 * time.Millisecond
	t.Cleanup(func() { startupKeyCheckTiming = saved })
}

// lapiAnswering starts a LAPI stub that answers every key check with status.
func lapiAnswering(t *testing.T, status int) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(status)
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

// unreachableURL returns a base URL with nothing listening, so every connection is refused.
func unreachableURL(t *testing.T) string {
	t.Helper()
	srv := httptest.NewServer(http.NotFoundHandler())
	url := srv.URL
	srv.Close()
	return url
}

// assertNoKeyFragments fails if any 4-character window of key appears in the captured output.
func assertNoKeyFragments(t *testing.T, out []string, key string) {
	t.Helper()
	for i := 0; i+4 <= len(key); i++ {
		fragment := key[i : i+4]
		for _, s := range out {
			require.NotContains(t, s, fragment, "log output must carry no key characters")
		}
	}
}

// TestStartupKeyCheckWindowIsTenSeconds pins the boot window (Q-7): one shared retry rule with a 10 s budget.
func TestStartupKeyCheckWindowIsTenSeconds(t *testing.T) {
	require.Equal(t, 10*time.Second, startupKeyCheckTiming.Wait)
}

// logProbeKey is written as the saved key in the log-scan tests. Its characters do not form words, so a
// 4-character window cannot match ordinary log text.
const logProbeKey = "Q7zX9kP2mW4nB8vC1hL5jR3tY6gF0dS"

// TestEnsureBouncerRegistrationUnreachableLAPIKeepsSavedKey: a LAPI that cannot be reached at boot leaves the
// saved key, the bouncer and the key file byte-for-byte alone, logs one neutral Warn, and lets boot continue.
func TestEnsureBouncerRegistrationUnreachableLAPIKeepsSavedKey(t *testing.T) {
	h := newBouncerHarness(t)
	shortenStartupKeyCheck(t)
	t.Setenv("CHARON_SECURITY_CROWDSEC_API_URL", unreachableURL(t))
	h.writeFileKey(t, logProbeKey)
	before, err := os.ReadFile(h.keyFile)
	require.NoError(t, err)
	exec := &scriptedBouncerExecutor{}

	require.NoError(t, ensureBouncerRegistration(h.dataDir, h.keyFile, exec))

	require.False(t, exec.called("bouncers"), "no bouncer delete or add while LAPI is unreachable")
	after, err := os.ReadFile(h.keyFile)
	require.NoError(t, err)
	require.Equal(t, before, after, "the key file is byte-for-byte unchanged")
	out := h.channels(h.drain())
	require.Contains(t, strings.Join(out, "\n"), bouncerkey.UnverifiedMessage)
	assertNoKeyFragments(t, out, logProbeKey)
}

// TestEnsureBouncerRegistrationUnverifiedOutcomesNeverRotate runs the other non-definite answers through the
// same boot decision: a slow LAPI, a server error, an unauthorized answer and a redirect. None may delete or add.
func TestEnsureBouncerRegistrationUnverifiedOutcomesNeverRotate(t *testing.T) {
	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(slow.Close)

	cases := []struct {
		name string
		url  func(t *testing.T) string
	}{
		{name: "slow", url: func(*testing.T) string { return slow.URL }},
		{name: "server error", url: func(t *testing.T) string { return lapiAnswering(t, http.StatusInternalServerError) }},
		{name: "unauthorized", url: func(t *testing.T) string { return lapiAnswering(t, http.StatusUnauthorized) }},
		{name: "redirect", url: func(t *testing.T) string { return lapiAnswering(t, http.StatusFound) }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := newBouncerHarness(t)
			shortenStartupKeyCheck(t)
			t.Setenv("CHARON_SECURITY_CROWDSEC_API_URL", tc.url(t))
			h.writeFileKey(t, testFileKey)
			exec := &scriptedBouncerExecutor{}

			require.NoError(t, ensureBouncerRegistration(h.dataDir, h.keyFile, exec))

			require.False(t, exec.called("bouncers"), "%s is not a definite rejection", tc.name)
			require.Equal(t, testFileKey, bouncerkey.ReadFile(h.keyFile))
		})
	}
}

// TestEnsureBouncerRegistrationRecoversWithinWindow: a 500 followed by a 200 inside the window is accepted, so the
// saved key is kept without any bouncer command.
func TestEnsureBouncerRegistrationRecoversWithinWindow(t *testing.T) {
	h := newBouncerHarness(t)
	shortenStartupKeyCheck(t)
	var requests atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if requests.Add(1) == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)
	t.Setenv("CHARON_SECURITY_CROWDSEC_API_URL", srv.URL)
	h.writeFileKey(t, testFileKey)
	exec := &scriptedBouncerExecutor{}

	require.NoError(t, ensureBouncerRegistration(h.dataDir, h.keyFile, exec))

	require.GreaterOrEqual(t, requests.Load(), int32(2), "the check retries inside the window")
	require.False(t, exec.called("bouncers"))
	require.Equal(t, testFileKey, bouncerkey.ReadFile(h.keyFile))
}

// TestEnsureBouncerRegistrationDefiniteRejectionStillRotates: a 403 is still the one definite rejection, so the
// saved key is replaced by a registered one.
func TestEnsureBouncerRegistrationDefiniteRejectionStillRotates(t *testing.T) {
	h := newBouncerHarness(t)
	shortenStartupKeyCheck(t)
	h.writeFileKey(t, testRejectedEnvKey)
	exec := &scriptedBouncerExecutor{addOutput: []byte(testGeneratedKey + "\n")}

	require.NoError(t, ensureBouncerRegistration(h.dataDir, h.keyFile, exec))

	require.True(t, exec.called("bouncers delete"))
	require.True(t, exec.called("bouncers add"))
	require.Equal(t, testGeneratedKey, bouncerkey.ReadFile(h.keyFile))
}

// TestLogStartupKeyDecisionUnverifiedIsNeutral: the unverified outcome has its own log line, with no key text.
func TestLogStartupKeyDecisionUnverifiedIsNeutral(t *testing.T) {
	h := newBouncerHarness(t)

	logStartupKeyDecision(bouncerkey.EnsureResult{Outcome: bouncerkey.OutcomeUnverified, Key: logProbeKey}, h.keyFile, nil)

	out := h.channels(h.drain())
	require.Contains(t, strings.Join(out, "\n"), bouncerkey.UnverifiedMessage)
	assertNoKeyFragments(t, out, logProbeKey)
}
