package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/crowdsec/bouncerkey"
)

// unreachableLAPIURL returns a base URL with nothing listening, so every connection is refused.
func unreachableLAPIURL(t *testing.T) string {
	t.Helper()
	srv := httptest.NewServer(http.NotFoundHandler())
	url := srv.URL
	srv.Close()
	return url
}

// statusLAPI answers every key check with status.
func statusLAPI(t *testing.T, status int) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(status)
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

// keyCheckCmdRecorder records every command and answers nil, so a test can assert that no bouncer command ran.
type keyCheckCmdRecorder struct {
	mu    sync.Mutex
	calls []string
}

func (r *keyCheckCmdRecorder) Execute(_ context.Context, name string, args ...string) ([]byte, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, name+" "+strings.Join(args, " "))
	return nil, nil
}

func (r *keyCheckCmdRecorder) bouncerCalls() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []string
	for _, c := range r.calls {
		if strings.Contains(c, "bouncers") {
			out = append(out, c)
		}
	}
	return out
}

// keyStatusBody runs GetKeyStatus and returns the status code and decoded body. It makes no assertions,
// so it is safe to call from a goroutine.
func keyStatusBody(h *CrowdsecHandler) (code int, body map[string]any) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/admin/crowdsec/key-status", http.NoBody)
	h.GetKeyStatus(c)
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	return w.Code, body
}

// keyStatusFor runs GetKeyStatus and decodes the response.
func keyStatusFor(t *testing.T, h *CrowdsecHandler) map[string]any {
	t.Helper()
	code, body := keyStatusBody(h)
	require.Equal(t, http.StatusOK, code)
	return body
}

// newKeyCheckHandler builds a handler whose LAPI key check gives up after a short window, so an unreachable
// or erroring LAPI does not hold a test for the production 30 s window.
func newKeyCheckHandler(t *testing.T, lapiURL, dataDir string, cmd CommandExecutor) *CrowdsecHandler {
	t.Helper()
	h := newPrecedenceHandler(t, lapiURL, dataDir, cmd)
	h.keyCheckWait = 300 * time.Millisecond
	return h
}

func TestKeyCheckUnreachableStoredURLIsUnknown(t *testing.T) {
	h := newKeyCheckHandler(t, unreachableLAPIURL(t), t.TempDir(), noCSCLIMock())

	require.Equal(t, bouncerkey.VerdictUnknown, h.testKeyAgainstLAPI(context.Background(), precSavedKey),
		"a refused connection is no evidence about the key")
}

func TestKeyCheckUnauthorizedIsUnknown(t *testing.T) {
	h := newKeyCheckHandler(t, statusLAPI(t, http.StatusUnauthorized), t.TempDir(), noCSCLIMock())

	require.Equal(t, bouncerkey.VerdictUnknown, h.testKeyAgainstLAPI(context.Background(), precSavedKey),
		"only 403 is a rejection; 401 stays unknown")
}

func TestKeyCheckServerErrorIsUnknown(t *testing.T) {
	h := newKeyCheckHandler(t, statusLAPI(t, http.StatusInternalServerError), t.TempDir(), noCSCLIMock())

	require.Equal(t, bouncerkey.VerdictUnknown, h.testKeyAgainstLAPI(context.Background(), precSavedKey))
}

func TestKeyCheckForbiddenIsRejected(t *testing.T) {
	h := newKeyCheckHandler(t, statusLAPI(t, http.StatusForbidden), t.TempDir(), noCSCLIMock())

	require.Equal(t, bouncerkey.VerdictRejected, h.testKeyAgainstLAPI(context.Background(), precSavedKey))
}

func TestKeyCheckCancelledContextIsUnknownWithoutRequest(t *testing.T) {
	var requests atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)
	h := newKeyCheckHandler(t, srv.URL, t.TempDir(), noCSCLIMock())
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	require.Equal(t, bouncerkey.VerdictUnknown, h.testKeyAgainstLAPI(ctx, precSavedKey),
		"a shut-down context is never evidence about the key")
	require.Zero(t, requests.Load())
}

// TestEnsureBouncerRegistrationUnreachableKeepsSavedKey: an unreachable LAPI leaves the saved key, the bouncer
// and the key file alone, and the rejection facts untouched.
func TestEnsureBouncerRegistrationUnreachableKeepsSavedKey(t *testing.T) {
	clearKeyEnv(t, "")
	dataDir := t.TempDir()
	keyPath := filepath.Join(dataDir, "bouncer_key")
	writeSavedKey(t, keyPath, precSavedKey)
	before, err := os.ReadFile(keyPath)
	require.NoError(t, err)
	h := newKeyCheckHandler(t, unreachableLAPIURL(t), dataDir, noCSCLIMock())
	h.envKeyRejected = true
	h.rejectedEnvKey = precEnvKeyOther
	h.rejectedEnvSaveFailed = true

	res, regErr := h.ensureBouncerRegistration(context.Background())

	require.NoError(t, regErr)
	require.Equal(t, bouncerkey.OutcomeUnverified, res.Outcome)
	require.Equal(t, precSavedKey, res.Key)
	after, err := os.ReadFile(keyPath)
	require.NoError(t, err)
	require.Equal(t, before, after, "the key file is byte-for-byte unchanged")
	require.True(t, h.envKeyRejected, "an unverified check leaves the rejection state as it was")
	require.Equal(t, precEnvKeyOther, h.rejectedEnvKey)
	require.True(t, h.rejectedEnvSaveFailed)
}

// TestEnsureBouncerRegistrationShutdownDoesNotRotate: a shutdown during the key check reaches neither the
// delete nor the add, even when LAPI would reject the key.
func TestEnsureBouncerRegistrationShutdownDoesNotRotate(t *testing.T) {
	clearKeyEnv(t, "")
	dataDir := t.TempDir()
	keyPath := filepath.Join(dataDir, "bouncer_key")
	writeSavedKey(t, keyPath, precSavedKey)
	lapiURL, _ := newKeyLAPI(t) // accepts no key: every check is a definite 403
	cmd := &keyCheckCmdRecorder{}
	h := newKeyCheckHandler(t, lapiURL, dataDir, cmd)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	res, err := h.ensureBouncerRegistration(ctx)

	require.NoError(t, err)
	require.Equal(t, bouncerkey.OutcomeUnverified, res.Outcome)
	require.Empty(t, cmd.bouncerCalls(), "a shutdown must not delete or add a bouncer")
	require.Equal(t, precSavedKey, bouncerkey.ReadFile(keyPath))
}

// TestStartUnreachableStoredURLWarnsWithoutRotating: Start with a stored LAPI URL that cannot be reached keeps
// the saved key, runs no bouncer command and says so in the response.
func TestStartUnreachableStoredURLWarnsWithoutRotating(t *testing.T) {
	clearKeyEnv(t, "")
	dataDir := t.TempDir()
	keyPath := filepath.Join(dataDir, "bouncer_key")
	writeSavedKey(t, keyPath, precSavedKey)
	cmd := &keyCheckCmdRecorder{}
	applier := &recordingApplier{}
	h := newStartApplyHandler(t, unreachableLAPIURL(t), dataDir, cmd, &fakeExec{}, applier)
	h.keyCheckWait = 300 * time.Millisecond

	w := startRequest(t, h, context.Background())

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var body map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.Equal(t, true, body["lapi_ready"])
	require.Equal(t, bouncerkey.UnverifiedMessage, body["warning"])
	require.Empty(t, cmd.bouncerCalls(), "no bouncer delete or add while the key is unverified")
	require.Equal(t, precSavedKey, bouncerkey.ReadFile(keyPath))
	require.Equal(t, 1, applier.count(), "the mode is set, so the proxy is applied once")
}

// TestGetKeyStatusKeyCheckUnreachableIsUnknown: the status reports the check as unknown, not as valid or
// rejected, and leaves the rejection state alone.
func TestGetKeyStatusKeyCheckUnreachableIsUnknown(t *testing.T) {
	clearKeyEnv(t, "")
	dataDir := t.TempDir()
	writeSavedKey(t, filepath.Join(dataDir, "bouncer_key"), precSavedKey)
	h := newKeyCheckHandler(t, unreachableLAPIURL(t), dataDir, noCSCLIMock())

	body := keyStatusFor(t, h)

	require.Equal(t, "unknown", body["key_check"])
	require.Equal(t, false, body["valid"])
	require.Equal(t, false, body["env_key_rejected"])
	require.Equal(t, "file", body["key_source"])
}

// TestGetKeyStatusKeyCheckStates: accepted, rejected and none, with valid equal to accepted.
func TestGetKeyStatusKeyCheckStates(t *testing.T) {
	t.Run("accepted", func(t *testing.T) {
		clearKeyEnv(t, "")
		dataDir := t.TempDir()
		writeSavedKey(t, filepath.Join(dataDir, "bouncer_key"), precSavedKey)
		lapiURL, _ := newKeyLAPI(t, precSavedKey)
		h := newKeyCheckHandler(t, lapiURL, dataDir, noCSCLIMock())

		body := keyStatusFor(t, h)

		require.Equal(t, "accepted", body["key_check"])
		require.Equal(t, true, body["valid"])
	})
	t.Run("rejected", func(t *testing.T) {
		clearKeyEnv(t, "")
		dataDir := t.TempDir()
		writeSavedKey(t, filepath.Join(dataDir, "bouncer_key"), precSavedKey)
		lapiURL, _ := newKeyLAPI(t)
		h := newKeyCheckHandler(t, lapiURL, dataDir, noCSCLIMock())

		body := keyStatusFor(t, h)

		require.Equal(t, "rejected", body["key_check"])
		require.Equal(t, false, body["valid"])
	})
	t.Run("none", func(t *testing.T) {
		clearKeyEnv(t, "")
		h := newKeyCheckHandler(t, unreachableLAPIURL(t), t.TempDir(), noCSCLIMock())

		body := keyStatusFor(t, h)

		require.Equal(t, "none", body["key_check"])
		require.Equal(t, false, body["valid"])
		require.Equal(t, "none", body["key_source"])
	})
}

// TestGetKeyStatusAnswersWhileStartIsChecking: a status request is not held behind a Start that is waiting on
// LAPI. The first LAPI request blocks until released; every later request answers at once.
func TestGetKeyStatusAnswersWhileStartIsChecking(t *testing.T) {
	clearKeyEnv(t, "")
	dataDir := t.TempDir()
	writeSavedKey(t, filepath.Join(dataDir, "bouncer_key"), precSavedKey)

	entered := make(chan struct{})
	release := make(chan struct{})
	var first atomic.Bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if first.CompareAndSwap(false, true) {
			close(entered)
			<-release
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)
	h := newKeyCheckHandler(t, srv.URL, dataDir, noCSCLIMock())

	startDone := make(chan struct{})
	go func() {
		defer close(startDone)
		_, _ = h.ensureBouncerRegistration(context.Background())
	}()
	t.Cleanup(func() {
		select {
		case <-release:
		default:
			close(release)
		}
		<-startDone
	})

	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("the Start key check never reached LAPI")
	}

	type statusReply struct {
		code int
		body map[string]any
	}
	statusDone := make(chan statusReply, 1)
	started := time.Now()
	go func() {
		code, body := keyStatusBody(h)
		statusDone <- statusReply{code: code, body: body}
	}()

	select {
	case reply := <-statusDone:
		require.Equal(t, http.StatusOK, reply.code)
		require.Less(t, time.Since(started), 5*time.Second, "the status must answer within its own 5 s budget")
		require.Equal(t, "accepted", reply.body["key_check"])
	case <-time.After(5 * time.Second):
		t.Fatal("GetKeyStatus waited for the Start key check; it must snapshot under the lock and release it first")
	}
}
