package services

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// keyDecisionLAPI records every key presented to it. Keys prefixed "rejected-" are refused.
func keyDecisionLAPI(t *testing.T) (mu *sync.Mutex, seen *[]string) {
	t.Helper()
	mu = &sync.Mutex{}
	seen = new([]string)
	lapi := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.Header.Get("X-Api-Key")
		mu.Lock()
		*seen = append(*seen, key)
		mu.Unlock()
		if strings.HasPrefix(key, "rejected-") {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(lapi.Close)
	t.Setenv("CHARON_SECURITY_CROWDSEC_API_URL", lapi.URL)
	return mu, seen
}

// TestEnsureBouncerRegistration_KeyDecisionRows covers the four sampled saved/env combinations
// of the startup decision, mirroring the handler-level precedence rows.
func TestEnsureBouncerRegistration_KeyDecisionRows(t *testing.T) {
	const invalidSaved = "rejected-saved-key-0123456789abcdef"

	tests := []struct {
		name         string
		saved        string
		env          string
		wantFile     string
		wantRegister bool
		wantValidate []string
	}{
		{
			name:         "valid saved and stale env keeps saved",
			saved:        testFileKey,
			env:          testRejectedEnvKey,
			wantFile:     testFileKey,
			wantValidate: []string{testFileKey},
		},
		{
			name:         "invalid saved and valid env saves env",
			saved:        invalidSaved,
			env:          testFileKey,
			wantFile:     testFileKey + "\n",
			wantValidate: []string{invalidSaved, testFileKey},
		},
		{
			name:         "both invalid registers new key",
			saved:        invalidSaved,
			env:          testRejectedEnvKey,
			wantFile:     testGeneratedKey + "\n",
			wantRegister: true,
			wantValidate: []string{invalidSaved, testRejectedEnvKey},
		},
		{
			name:         "none registers new key",
			wantFile:     testGeneratedKey + "\n",
			wantRegister: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := newBouncerHarness(t)
			mu, seen := keyDecisionLAPI(t)
			if tc.saved != "" {
				h.writeFileKey(t, tc.saved)
			}
			if tc.env != "" {
				t.Setenv("CHARON_SECURITY_CROWDSEC_API_KEY", tc.env)
			}
			exec := &scriptedBouncerExecutor{addOutput: []byte(testGeneratedKey + "\n")}

			require.NoError(t, ensureBouncerRegistration(h.dataDir, h.keyFile, exec))

			got, err := os.ReadFile(h.keyFile)
			require.NoError(t, err)
			assert.Equal(t, tc.wantFile, string(got), "key file contents")

			mu.Lock()
			assert.Equal(t, tc.wantValidate, *seen, "keys validated against LAPI, in order")
			mu.Unlock()

			assert.Equal(t, tc.wantRegister, exec.called("bouncers add"), "registration")
			assert.Equal(t, tc.wantRegister, exec.called("bouncers delete"), "stale bouncer delete")
			for _, c := range exec.calls {
				assert.Equal(t, "cscli", c[0], "startup registration must only run cscli, never a Caddy apply")
			}
		})
	}
}

func TestEnsureBouncerRegistration_EnvKeyRejectedSuccessLogNamesKeyFile(t *testing.T) {
	h := newBouncerHarness(t)
	keyDecisionLAPI(t)
	t.Setenv("CHARON_SECURITY_CROWDSEC_API_KEY", testRejectedEnvKey)
	exec := &scriptedBouncerExecutor{addOutput: []byte(testGeneratedKey + "\n")}

	require.NoError(t, ensureBouncerRegistration(h.dataDir, h.keyFile, exec))

	out := strings.Join(h.channels(h.drain()), "\n")
	assert.Contains(t, out, "Charon uses the key file (permissions 0600)")
	assert.NotContains(t, out, "could not save")
}

func TestEnsureBouncerRegistration_EnvKeyRejectedSaveFailureLogsAccurately(t *testing.T) {
	h := newBouncerHarness(t)
	keyDecisionLAPI(t)
	t.Setenv("CHARON_SECURITY_CROWDSEC_API_KEY", testRejectedEnvKey)
	blocker := filepath.Join(h.dataDir, "blocker")
	require.NoError(t, os.WriteFile(blocker, []byte("x"), 0o600))
	keyFile := filepath.Join(blocker, "bouncer_key")
	exec := &scriptedBouncerExecutor{addOutput: []byte(testGeneratedKey)}

	require.NoError(t, ensureBouncerRegistration(h.dataDir, keyFile, exec))

	entries := h.drain()
	out := strings.Join(h.channels(entries), "\n")
	assert.Contains(t, out, "Charon could not save its new key to "+keyFile)
	assert.NotContains(t, out, "Charon uses the key file", "a failed save must not claim the file holds the key in use")

	var sawSaveError bool
	for _, e := range entries {
		if e.Level == logrus.ErrorLevel && strings.Contains(e.Message, "failed to save the key file") {
			sawSaveError = true
		}
	}
	assert.True(t, sawSaveError, "the save failure must be logged at Error level")
	assertNoSubstring(t, h.channels(entries), testGeneratedKey)
}
