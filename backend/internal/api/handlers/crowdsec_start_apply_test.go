package handlers

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/crowdsec/bouncerkey"
	"github.com/Wikid82/charon/backend/internal/models"
)

// recordingApplier stands in for the Caddy manager. It counts applies and records, at call time, the
// context state each apply saw (applyProxyConfig cancels its context once the apply returns).
type recordingApplier struct {
	mu    sync.Mutex
	calls int
	seen  []applySeen
	err   error
}

// applySeen is what one ApplyConfig call observed on its context.
type applySeen struct {
	ctxErr      error
	deadline    time.Time
	hasDeadline bool
}

func (r *recordingApplier) ApplyConfig(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls++
	deadline, ok := ctx.Deadline()
	r.seen = append(r.seen, applySeen{ctxErr: ctx.Err(), deadline: deadline, hasDeadline: ok})
	return r.err
}

func (r *recordingApplier) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.calls
}

// lapiMock answers every "cscli lapi status" call with err (nil means LAPI is ready).
func lapiMock(m *MockCommandExecutor, err error) *MockCommandExecutor {
	m.On("Execute", mock.Anything, "cscli", mock.MatchedBy(func(args []string) bool {
		return slices.Contains(args, "lapi")
	})).Return([]byte("ok"), err)
	return m
}

// isBouncersCall matches any cscli call that manages bouncers.
func isBouncersCall(args []string) bool {
	return slices.Contains(args, "bouncers")
}

// newStartApplyHandler builds a handler for Start tests with the given engine executor and Caddy applier.
func newStartApplyHandler(t *testing.T, lapiURL, dataDir string, cmd CommandExecutor, exec CrowdsecExecutor, applier CaddyConfigManager) *CrowdsecHandler {
	t.Helper()
	h := newPrecedenceHandler(t, lapiURL, dataDir, cmd)
	h.Executor = exec
	h.caddyManager = applier
	h.LAPIMaxWait = 200 * time.Millisecond
	h.LAPIPollInterval = 10 * time.Millisecond
	return h
}

// startRequest runs Start with the given request context and returns the recorded response.
func startRequest(t *testing.T, h *CrowdsecHandler, reqCtx context.Context) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/admin/crowdsec/start", http.NoBody).WithContext(reqCtx)
	h.Start(c)
	return w
}

// Start applies the proxy configuration exactly once after every outcome that changes or confirms the CrowdSec state.

func TestStartApply_NewStartLAPIReadySavedKeyAppliesOnce(t *testing.T) {
	lapiURL, _ := newKeyLAPI(t, precSavedKey)
	clearKeyEnv(t, "")
	dataDir := t.TempDir()
	writeSavedKey(t, filepath.Join(dataDir, "bouncer_key"), precSavedKey)
	cmd := lapiMock(new(MockCommandExecutor), nil)
	applier := &recordingApplier{}
	h := newStartApplyHandler(t, lapiURL, dataDir, cmd, &fakeExec{}, applier)

	w := startRequest(t, h, context.Background())

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.JSONEq(t, `{"status":"started","pid":12345,"lapi_ready":true}`, w.Body.String())
	require.Equal(t, 1, applier.count(), "a valid saved key still changes the mode, so one apply")
}

func TestStartApply_NewStartLAPIReadyEnvKeySavedAppliesOnce(t *testing.T) {
	lapiURL, _ := newKeyLAPI(t, precEnvKey)
	clearKeyEnv(t, precEnvKey)
	dataDir := t.TempDir()
	keyPath := filepath.Join(dataDir, "bouncer_key")
	writeSavedKey(t, keyPath, "saved-key-stale-000001")
	applier := &recordingApplier{}
	h := newStartApplyHandler(t, lapiURL, dataDir, lapiMock(new(MockCommandExecutor), nil), &fakeExec{}, applier)

	w := startRequest(t, h, context.Background())

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.Equal(t, precEnvKey, bouncerkey.ReadFile(keyPath))
	require.Equal(t, 1, applier.count())
}

func TestStartApply_NewStartLAPIReadyRegisteredAppliesOnce(t *testing.T) {
	lapiURL, _ := newKeyLAPI(t)
	clearKeyEnv(t, "")
	dataDir := t.TempDir()
	applier := &recordingApplier{}
	h := newStartApplyHandler(t, lapiURL, dataDir, lapiMock(bouncerRegisterMock(), nil), &fakeExec{}, applier)

	w := startRequest(t, h, context.Background())

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.Equal(t, precNewKey, bouncerkey.ReadFile(filepath.Join(dataDir, "bouncer_key")))
	require.Equal(t, 1, applier.count())
}

func TestStartApply_NewStartRegisteredButSaveFailedAppliesOnce(t *testing.T) {
	lapiURL, _ := newKeyLAPI(t)
	clearKeyEnv(t, "")
	// A regular file where the key directory should be makes the save fail.
	dataDir := filepath.Join(t.TempDir(), "not-a-directory")
	require.NoError(t, os.WriteFile(dataDir, []byte("x"), 0o600))
	applier := &recordingApplier{}
	h := newStartApplyHandler(t, lapiURL, dataDir, lapiMock(bouncerRegisterMock(), nil), &fakeExec{}, applier)

	w := startRequest(t, h, context.Background())

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.Equal(t, 1, applier.count(), "the mode changed even though the new key was not saved")
}

func TestStartApply_NewStartRegistrationFailedAppliesOnce(t *testing.T) {
	lapiURL, _ := newKeyLAPI(t)
	clearKeyEnv(t, "")
	applier := &recordingApplier{}
	h := newStartApplyHandler(t, lapiURL, t.TempDir(), lapiMock(bouncerRegisterFailMock(), nil), &fakeExec{}, applier)

	w := startRequest(t, h, context.Background())

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.Equal(t, 1, applier.count())
}

func TestStartApply_LAPINeverReadyAppliesOnceWithoutRegistration(t *testing.T) {
	lapiURL, _ := newKeyLAPI(t)
	clearKeyEnv(t, "")
	cmd := lapiMock(new(MockCommandExecutor), errors.New("lapi unreachable"))
	applier := &recordingApplier{}
	h := newStartApplyHandler(t, lapiURL, t.TempDir(), cmd, &fakeExec{}, applier)

	w := startRequest(t, h, context.Background())

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.JSONEq(t, `{"status":"started","pid":12345,"lapi_ready":false,"warning":"Process started but LAPI initialization may take additional time"}`, w.Body.String())
	require.Equal(t, 1, applier.count(), "the mode is local even though LAPI is not ready")
	cmd.AssertNotCalled(t, "Execute", mock.Anything, "cscli", mock.MatchedBy(isBouncersCall))
}

func TestStartApply_AlreadyRunningAppliesOnce(t *testing.T) {
	lapiURL, _ := newKeyLAPI(t)
	clearKeyEnv(t, "")
	applier := &recordingApplier{}
	h := newStartApplyHandler(t, lapiURL, t.TempDir(), lapiMock(new(MockCommandExecutor), nil), &fakeExec{started: true}, applier)

	w := startRequest(t, h, context.Background())

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.JSONEq(t, `{"status":"already_running","pid":12345,"lapi_ready":true}`, w.Body.String())
	require.Equal(t, 1, applier.count(), "a confirmed running engine still gets its mode applied")
}

func TestStartApply_EngineStartFailureDoesNotApply(t *testing.T) {
	lapiURL, _ := newKeyLAPI(t)
	clearKeyEnv(t, "")
	applier := &recordingApplier{}
	exec := &fakeExec{startErr: errors.New("spawn failed")}
	h := newStartApplyHandler(t, lapiURL, t.TempDir(), lapiMock(new(MockCommandExecutor), nil), exec, applier)

	w := startRequest(t, h, context.Background())

	require.Equal(t, http.StatusInternalServerError, w.Code, w.Body.String())
	require.JSONEq(t, `{"error":"failed to start CrowdSec"}`, w.Body.String())
	require.Zero(t, applier.count(), "a failed start leaves the state reverted, so nothing is applied")
}

func TestStartApply_BusyRefusalDoesNotApply(t *testing.T) {
	lapiURL, _ := newKeyLAPI(t)
	clearKeyEnv(t, "")
	applier := &recordingApplier{}
	h := newStartApplyHandler(t, lapiURL, t.TempDir(), lapiMock(new(MockCommandExecutor), nil), &fakeExec{}, applier)
	setBackgroundInstallActive(t, h)

	w := startRequest(t, h, context.Background())

	require.Equal(t, http.StatusConflict, w.Code, w.Body.String())
	require.Zero(t, applier.count())
}

func TestStartApply_ApplyFailureKeepsResponseAndState(t *testing.T) {
	lapiURL, _ := newKeyLAPI(t, precSavedKey)
	clearKeyEnv(t, "")
	dataDir := t.TempDir()
	writeSavedKey(t, filepath.Join(dataDir, "bouncer_key"), precSavedKey)
	logs := captureLogs(t)
	applier := &recordingApplier{err: errors.New("caddy admin unreachable")}
	h := newStartApplyHandler(t, lapiURL, dataDir, lapiMock(new(MockCommandExecutor), nil), &fakeExec{}, applier)

	w := startRequest(t, h, context.Background())

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.Contains(t, w.Body.String(), `"status":"started"`)
	require.Equal(t, 1, applier.count())
	require.Contains(t, logs.String(), "Failed to apply the proxy configuration after a CrowdSec state change")

	var cfg models.SecurityConfig
	require.NoError(t, h.DB.Where("name = ?", models.DefaultSecurityConfigName).First(&cfg).Error)
	require.Equal(t, "local", cfg.CrowdSecMode, "a failed apply does not revert the started engine's state")
}

func TestStartApply_NilManagerLogsNotWired(t *testing.T) {
	lapiURL, _ := newKeyLAPI(t, precSavedKey)
	clearKeyEnv(t, "")
	dataDir := t.TempDir()
	writeSavedKey(t, filepath.Join(dataDir, "bouncer_key"), precSavedKey)
	logs := captureLogs(t)
	h := newStartApplyHandler(t, lapiURL, dataDir, lapiMock(new(MockCommandExecutor), nil), &fakeExec{}, nil)

	w := startRequest(t, h, context.Background())

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.False(t, h.HasProxyApplier())
	require.Contains(t, logs.String(), errCaddyNotWired.Error())
	require.ErrorIs(t, h.reapplyCaddyConfig(context.Background()), errCaddyNotWired)
}

// TestStartApply_ApplyRunsAfterClientDisconnect pins M2: a cancelled request must not cancel the apply.
func TestStartApply_ApplyRunsAfterClientDisconnect(t *testing.T) {
	lapiURL, _ := newKeyLAPI(t, precSavedKey)
	clearKeyEnv(t, "")
	dataDir := t.TempDir()
	writeSavedKey(t, filepath.Join(dataDir, "bouncer_key"), precSavedKey)
	applier := &recordingApplier{}
	// A cancelled request also fails the saved-key check, so the test registers a bouncer as well.
	h := newStartApplyHandler(t, lapiURL, dataDir, lapiMock(bouncerRegisterMock(), nil), &fakeExec{}, applier)

	reqCtx, cancel := context.WithCancel(context.Background())
	cancel()
	w := startRequest(t, h, reqCtx)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.Equal(t, 1, applier.count())
	require.Len(t, applier.seen, 1)
	seen := applier.seen[0]
	require.NoError(t, seen.ctxErr, "the apply must not inherit the client's cancellation")
	require.True(t, seen.hasDeadline, "the apply must be bounded")
	require.LessOrEqual(t, time.Until(seen.deadline), crowdsecApplyTimeout)
}

func TestNilIfTypedNil(t *testing.T) {
	var typedNil *recordingApplier
	assert.Nil(t, nilIfTypedNil(nil))
	assert.Nil(t, nilIfTypedNil(typedNil), "a typed nil pointer must count as not wired")

	applier := &recordingApplier{}
	assert.NotNil(t, nilIfTypedNil(applier))
}

func TestNewCrowdsecHandler_ReportsProxyApplierWiring(t *testing.T) {
	var typedNil *recordingApplier

	assert.True(t, NewCrowdsecHandler(nil, &fakeExec{}, "/bin/false", t.TempDir(), &recordingApplier{}).HasProxyApplier())
	assert.False(t, NewCrowdsecHandler(nil, &fakeExec{}, "/bin/false", t.TempDir(), typedNil).HasProxyApplier())
	assert.False(t, NewCrowdsecHandler(nil, &fakeExec{}, "/bin/false", t.TempDir(), nil).HasProxyApplier())
}
