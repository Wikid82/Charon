package services

import (
	"bytes"
	"context"
	"errors"
	"fmt"
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

	"github.com/Wikid82/charon/backend/internal/crowdsec/bouncerkey"
	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/internal/util"
)

const (
	testRejectedEnvKey = "rejected-env-key-0123456789abcdefghij"
	testFileKey        = "valid-file-key-0123456789abcdefghijkl"
	testGeneratedKey   = "ZZsentinel0123456789abcdefghijklmnopqrstuvwx"
)

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

	for _, name := range bouncerkey.EnvNames() {
		t.Setenv(name, "")
	}

	lapi := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.Header.Get("X-Api-Key"), "rejected-") {
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

// channels flattens message, data, error field and raw output of every captured entry.
func (h *bouncerHarness) channels(entries []*logrus.Entry) []string {
	out := []string{h.buf.String()}
	for _, e := range entries {
		out = append(out, e.Message, fmt.Sprint(e.Data))
		if v, ok := e.Data[logrus.ErrorKey]; ok {
			out = append(out, fmt.Sprint(v))
		}
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

	// Intended flip: a valid env key is saved to the key file so the proxy reads it.
	assert.False(t, exec.called("bouncers add"))
	got, err := os.ReadFile(h.keyFile)
	require.NoError(t, err)
	assert.Equal(t, testFileKey+"\n", string(got), "saved with one trailing newline, like every key writer")
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
	assert.Equal(t, testGeneratedKey+"\n", string(got), "saved with one trailing newline, like every key writer")
	info, err := os.Stat(h.keyFile)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())

	for _, e := range h.drain() {
		assert.NotEqual(t, logrus.WarnLevel, e.Level, "no warning expected without an env key: %s", e.Message)
	}
}

func TestEnsureBouncerRegistration_ReplacesSymlinkedKeyWithRegularFile(t *testing.T) {
	h := newBouncerHarness(t)
	target := filepath.Join(h.dataDir, "elsewhere")
	require.NoError(t, os.WriteFile(target, []byte("rejected-linked-key\n"), 0o600))
	require.NoError(t, os.MkdirAll(filepath.Dir(h.keyFile), 0o750))
	require.NoError(t, os.Symlink(target, h.keyFile))
	exec := &scriptedBouncerExecutor{addOutput: []byte(testGeneratedKey + "\n")}

	require.NoError(t, ensureBouncerRegistration(h.dataDir, h.keyFile, exec))

	info, err := os.Lstat(h.keyFile)
	require.NoError(t, err)
	assert.True(t, info.Mode().IsRegular(), "the link is replaced by a regular file")
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
	got, err := os.ReadFile(h.keyFile)
	require.NoError(t, err)
	assert.Equal(t, testGeneratedKey+"\n", string(got))
	linked, err := os.ReadFile(target) //nolint:gosec // G304: t.TempDir-derived path
	require.NoError(t, err)
	assert.Equal(t, "rejected-linked-key\n", string(linked), "the link target is never written")
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

	// Intended flip: a failed save is not fatal; the key is valid for this run and the Error log names the path.
	require.NoError(t, err)
	assert.Contains(t, strings.Join(h.channels(h.drain()), "\n"), blocker)
	_, statErr := os.Stat(filepath.Join(blocker, "bouncer_key"))
	assert.Error(t, statErr)
}

func assertNoSubstring(t *testing.T, channels []string, secret string) {
	t.Helper()
	for i, ch := range channels {
		assert.NotContains(t, ch, secret, "channel %d", i)
	}
}

func TestEnsureBouncerRegistration_EnvKeyRejected_DiagnosticsRedacted(t *testing.T) {
	for _, envName := range bouncerkey.EnvNames() {
		t.Run(envName, func(t *testing.T) {
			h := newBouncerHarness(t)
			t.Setenv(envName, testRejectedEnvKey)
			exec := &scriptedBouncerExecutor{addOutput: []byte(testGeneratedKey + "\n")}

			require.NoError(t, ensureBouncerRegistration(h.dataDir, h.keyFile, exec))

			// The generated value is persisted to the key file with restricted permissions.
			got, err := os.ReadFile(h.keyFile)
			require.NoError(t, err)
			require.Equal(t, testGeneratedKey+"\n", string(got))
			info, err := os.Stat(h.keyFile)
			require.NoError(t, err)
			require.Equal(t, os.FileMode(0o600), info.Mode().Perm())

			entries := h.drain()
			channels := h.channels(entries)
			assertNoSubstring(t, channels, testGeneratedKey)
			assertNoSubstring(t, channels, testRejectedEnvKey)

			joined := strings.Join(channels, "\n")
			assert.Contains(t, joined, h.keyFile)
			assert.Contains(t, joined, "0600")
			assert.Contains(t, joined, util.MaskAPIKey(testRejectedEnvKey))
		})
	}
}

func TestEnsureBouncerRegistration_EnvKeyRejected_ShortKeyRedacted(t *testing.T) {
	h := newBouncerHarness(t)
	const shortKey = "rejected-short"
	t.Setenv("CHARON_SECURITY_CROWDSEC_API_KEY", shortKey)
	exec := &scriptedBouncerExecutor{addOutput: []byte(testGeneratedKey)}

	require.NoError(t, ensureBouncerRegistration(h.dataDir, h.keyFile, exec))

	channels := h.channels(h.drain())
	assertNoSubstring(t, channels, shortKey)
	assertNoSubstring(t, channels, testGeneratedKey)
	assert.Contains(t, strings.Join(channels, "\n"), "[REDACTED]")
}

func TestEnsureBouncerRegistration_RegistrationError_DiagnosticsRedacted(t *testing.T) {
	h := newBouncerHarness(t)
	output := testGeneratedKey + " boom"
	exec := &scriptedBouncerExecutor{addOutput: []byte(output), addErr: errors.New("exit status 1")}

	err := ensureBouncerRegistration(h.dataDir, h.keyFile, exec)

	require.Error(t, err)
	assert.NotContains(t, err.Error(), testGeneratedKey)
	channels := h.channels(h.drain())
	assertNoSubstring(t, channels, testGeneratedKey)
	assert.Contains(t, strings.Join(channels, "\n"), fmt.Sprintf("%d bytes of output omitted", len(output)))
	_, statErr := os.Stat(h.keyFile)
	assert.True(t, os.IsNotExist(statErr))
}

// TestEnsureBouncerRegistration_CharonNameWinsOverUnprefixedName checks the startup
// path reads the shared order: CHARON_SECURITY_CROWDSEC_API_KEY before CROWDSEC_API_KEY.
// The unprefixed name holds a rejected key, so a registration attempt would show up as a bouncers add call.
func TestEnsureBouncerRegistration_CharonNameWinsOverUnprefixedName(t *testing.T) {
	h := newBouncerHarness(t)
	t.Setenv("CROWDSEC_API_KEY", testRejectedEnvKey)
	t.Setenv("CHARON_SECURITY_CROWDSEC_API_KEY", testFileKey)
	exec := &scriptedBouncerExecutor{addOutput: []byte(testGeneratedKey + "\n")}

	require.NoError(t, ensureBouncerRegistration(h.dataDir, h.keyFile, exec))
	require.False(t, exec.called("bouncers add"), "the valid CHARON key must be used without registering")
}
