package services

import (
	"bytes"
	"context"
	"io"
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

	"github.com/Wikid82/charon/backend/internal/logger"
)

const (
	testRejectedEnvKey = "rejected-env-key-0123456789abcdefghij"
	testFileKey        = "valid-file-key-0123456789abcdefghijkl"
	testGeneratedKey   = "ZZsentinel0123456789abcdefghijklmnopqrstuvwx"
)

var bouncerKeyEnvNames = []string{
	"CROWDSEC_API_KEY",
	"CROWDSEC_BOUNCER_API_KEY",
	"CERBERUS_SECURITY_CROWDSEC_API_KEY",
	"CHARON_SECURITY_CROWDSEC_API_KEY",
	"CPM_SECURITY_CROWDSEC_API_KEY",
}

// scriptedBouncerExecutor answers the cscli calls made during startup registration.
type scriptedBouncerExecutor struct {
	mu        sync.Mutex
	calls     [][]string
	addOutput []byte
	addErr    error
}

func (e *scriptedBouncerExecutor) Execute(_ context.Context, name string, args ...string) ([]byte, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.calls = append(e.calls, append([]string{name}, args...))
	joined := strings.Join(args, " ")
	if strings.Contains(joined, "bouncers add") {
		return e.addOutput, e.addErr
	}
	return nil, nil
}

func (e *scriptedBouncerExecutor) called(sub string) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, c := range e.calls {
		if strings.Contains(strings.Join(c, " "), sub) {
			return true
		}
	}
	return false
}

// bouncerHarness wires a fake LAPI, a clean environment and log capture.
type bouncerHarness struct {
	dataDir string
	keyFile string
	buf     *bytes.Buffer
	entries <-chan *logrus.Entry
	hook    *logger.BroadcastHook
	id      string
}

// newBouncerHarness clears all key env names, points the LAPI URL at a stub that
// accepts every key except testRejectedEnvKey, and captures log output.
func newBouncerHarness(t *testing.T) *bouncerHarness {
	t.Helper()

	for _, name := range bouncerKeyEnvNames {
		t.Setenv(name, "")
	}

	lapi := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Api-Key") == testRejectedEnvKey {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(lapi.Close)
	t.Setenv("CHARON_SECURITY_CROWDSEC_API_URL", lapi.URL)

	h := &bouncerHarness{buf: &bytes.Buffer{}}
	logger.Init(false, h.buf)
	h.hook = logger.GetBroadcastHook()
	h.id = t.Name()
	h.entries = h.hook.Subscribe(h.id)
	t.Cleanup(func() {
		logger.Init(false, io.Discard)
	})

	dir := t.TempDir()
	h.dataDir = dir
	h.keyFile = filepath.Join(dir, "crowdsec", "bouncer_key")
	return h
}

// drain returns every broadcast entry captured so far.
func (h *bouncerHarness) drain() []*logrus.Entry {
	h.hook.Unsubscribe(h.id)
	var out []*logrus.Entry
	for e := range h.entries {
		out = append(out, e)
	}
	return out
}

func (h *bouncerHarness) writeFileKey(t *testing.T, key string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(h.keyFile), 0o750))
	require.NoError(t, os.WriteFile(h.keyFile, []byte(key), 0o600))
}

func TestEnsureBouncerRegistration_ValidEnvKey_NoRegistration(t *testing.T) {
	h := newBouncerHarness(t)
	t.Setenv("CHARON_SECURITY_CROWDSEC_API_KEY", testFileKey)
	exec := &scriptedBouncerExecutor{}

	require.NoError(t, ensureBouncerRegistration(h.dataDir, h.keyFile, exec))

	assert.False(t, exec.called("bouncers add"))
	_, err := os.Stat(h.keyFile)
	assert.True(t, os.IsNotExist(err))
}

func TestEnsureBouncerRegistration_ValidFileKey_NoReRegistration(t *testing.T) {
	h := newBouncerHarness(t)
	t.Setenv("CROWDSEC_API_KEY", testRejectedEnvKey)
	h.writeFileKey(t, testFileKey)
	exec := &scriptedBouncerExecutor{}

	require.NoError(t, ensureBouncerRegistration(h.dataDir, h.keyFile, exec))

	assert.False(t, exec.called("bouncers add"))
	got, err := os.ReadFile(h.keyFile)
	require.NoError(t, err)
	assert.Equal(t, testFileKey, string(got))
}

func TestEnsureBouncerRegistration_NoEnvKey_RegistersAndSavesKey(t *testing.T) {
	h := newBouncerHarness(t)
	exec := &scriptedBouncerExecutor{addOutput: []byte(testGeneratedKey + "\n")}

	require.NoError(t, ensureBouncerRegistration(h.dataDir, h.keyFile, exec))

	assert.True(t, exec.called("bouncers delete"))
	assert.True(t, exec.called("bouncers add"))
	got, err := os.ReadFile(h.keyFile)
	require.NoError(t, err)
	assert.Equal(t, testGeneratedKey, string(got))
	info, err := os.Stat(h.keyFile)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())

	for _, e := range h.drain() {
		assert.NotEqual(t, logrus.WarnLevel, e.Level, "no warning expected without an env key: %s", e.Message)
	}
}

func TestEnsureBouncerRegistration_EmptyRegistrationOutput(t *testing.T) {
	h := newBouncerHarness(t)
	exec := &scriptedBouncerExecutor{addOutput: []byte("  \n")}

	err := ensureBouncerRegistration(h.dataDir, h.keyFile, exec)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty key")
}

func TestEnsureBouncerRegistration_KeyDirectoryUnavailable(t *testing.T) {
	h := newBouncerHarness(t)
	blocker := filepath.Join(h.dataDir, "blocker")
	require.NoError(t, os.WriteFile(blocker, []byte("x"), 0o600))
	exec := &scriptedBouncerExecutor{addOutput: []byte(testGeneratedKey)}

	err := ensureBouncerRegistration(h.dataDir, filepath.Join(blocker, "bouncer_key"), exec)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create key directory")
}
