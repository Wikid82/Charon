package handlers

import (
	"context"
	"errors"
	"path/filepath"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/models"
)

// lifecycleLAPIFake is the cscli stand-in for the Start lifecycle tests. Every "lapi" call answers ok from call
// readyAt on (0 means LAPI never answers), and fails at once when its context is done, as the real process does.
// beforeLAPI, when set, runs on each lapi call before it answers, so a test can act while the wait is running.
type lifecycleLAPIFake struct {
	mu         sync.Mutex
	readyAt    int
	calls      int
	ctxErrs    []error
	beforeLAPI func(call int)
}

func (f *lifecycleLAPIFake) Execute(ctx context.Context, _ string, args ...string) ([]byte, error) {
	if !slices.Contains(args, "lapi") {
		return nil, errors.New("unexpected cscli call")
	}
	f.mu.Lock()
	f.calls++
	call := f.calls
	f.ctxErrs = append(f.ctxErrs, ctx.Err())
	before := f.beforeLAPI
	f.mu.Unlock()

	if before != nil {
		before(call)
	}
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if f.readyAt == 0 || call < f.readyAt {
		return nil, errors.New("LAPI not ready")
	}
	return []byte("ok"), nil
}

// sawCancelledContext reports whether any lapi call ran on a context that was already done.
func (f *lifecycleLAPIFake) sawCancelledContext() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Contains(f.ctxErrs, context.Canceled)
}

// blockingStopExecutor blocks in Stop until release is closed, then records what its context looked like.
type blockingStopExecutor struct {
	entered     chan struct{}
	release     chan struct{}
	mu          sync.Mutex
	ctxErr      error
	hasDeadline bool
}

func newBlockingStopExecutor() *blockingStopExecutor {
	return &blockingStopExecutor{entered: make(chan struct{}), release: make(chan struct{})}
}

func (b *blockingStopExecutor) Start(_ context.Context, _, _ string) (int, error) {
	return 0, nil
}

func (b *blockingStopExecutor) Stop(ctx context.Context, _ string) error {
	_, hasDeadline := ctx.Deadline()
	close(b.entered)
	select {
	case <-b.release:
	case <-time.After(5 * time.Second):
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.ctxErr = ctx.Err()
	b.hasDeadline = hasDeadline
	return nil
}

func (b *blockingStopExecutor) Status(_ context.Context, _ string) (running bool, pid int, err error) {
	return false, 0, nil
}

// lifecycleSettingValue returns the stored security.crowdsec.enabled value.
func lifecycleSettingValue(t *testing.T, db *gorm.DB) string {
	t.Helper()
	var s models.Setting
	require.NoError(t, db.Where("key = ?", "security.crowdsec.enabled").First(&s).Error)
	return s.Value
}

// newLifecycleStopHandler builds a handler whose default row and enabled setting say CrowdSec is on.
func newLifecycleStopHandler(t *testing.T, exec CrowdsecExecutor) *CrowdsecHandler {
	t.Helper()
	db := setupCrowdDB(t)
	require.NoError(t, db.Create(&models.SecurityConfig{Name: models.DefaultSecurityConfigName, Enabled: true, CrowdSecMode: "local"}).Error)
	require.NoError(t, db.Create(&models.Setting{Key: "security.crowdsec.enabled", Value: "true", Category: "security", Type: "bool"}).Error)
	return &CrowdsecHandler{DB: db, Executor: exec, CmdExec: &mockCommandExecutor{}, DataDir: t.TempDir()}
}

// TestStartCrowdSec_RequestCancelledDuringLAPIWaitStillFinishes: a client that leaves during the LAPI wait must not
// cut the Start short. LAPI answers on the second poll, so the wait has to keep polling after the cancel.
func TestStartCrowdSec_RequestCancelledDuringLAPIWaitStillFinishes(t *testing.T) {
	lapiURL, _ := newKeyLAPI(t, precSavedKey)
	clearKeyEnv(t, "")
	dataDir := t.TempDir()
	writeSavedKey(t, filepath.Join(dataDir, "bouncer_key"), precSavedKey)

	reqCtx, cancelReq := context.WithCancel(context.Background())
	defer cancelReq()
	cmd := &lifecycleLAPIFake{readyAt: 2, beforeLAPI: func(call int) {
		if call == 1 {
			cancelReq()
		}
	}}
	applier := &recordingApplier{}
	h := newStartApplyHandler(t, lapiURL, dataDir, cmd, &fakeExec{}, applier)

	res, err := h.StartCrowdSec(reqCtx)

	require.NoError(t, err)
	require.True(t, res.LAPIReady, "the LAPI wait must not stop when the client leaves")
	require.False(t, cmd.sawCancelledContext(), "lapi polls must run on the operation context, not the request")
	require.Equal(t, 1, applier.count(), "the proxy is applied once")
	require.Equal(t, "true", lifecycleSettingValue(t, h.DB))
}

// TestStopCrowdSec_RequestCancelledDuringStopCompletes: a client that leaves during Stop must not cut the engine
// stop in half. The executor sees a live context with its own deadline, and the stop writes disabled and false.
func TestStopCrowdSec_RequestCancelledDuringStopCompletes(t *testing.T) {
	exec := newBlockingStopExecutor()
	h := newLifecycleStopHandler(t, exec)
	reqCtx, cancelReq := context.WithCancel(context.Background())
	defer cancelReq()

	done := make(chan error, 1)
	go func() {
		_, err := h.StopCrowdSec(reqCtx)
		done <- err
	}()
	<-exec.entered
	cancelReq()
	close(exec.release)

	require.NoError(t, <-done)
	requireStopCompletedOnOwnContext(t, exec, h.DB)
}

// TestStartCrowdSec_AppCancelledDuringLAPIWaitEndsWait: shutdown ends a LAPI wait at once, with the same outcome as
// a LAPI timeout, instead of holding the process for the full bound.
func TestStartCrowdSec_AppCancelledDuringLAPIWaitEndsWait(t *testing.T) {
	appCtx, cancelApp := context.WithCancel(context.Background())
	defer cancelApp()
	cmd := &lifecycleLAPIFake{readyAt: 0, beforeLAPI: func(call int) {
		if call == 2 {
			cancelApp()
		}
	}}
	applier := &recordingApplier{}
	h := newStartApplyHandler(t, "", t.TempDir(), cmd, &fakeExec{}, applier)
	h.LAPIMaxWait = 5 * time.Second
	h.WithAppContext(appCtx)

	start := time.Now()
	res, err := h.StartCrowdSec(context.Background())
	elapsed := time.Since(start)

	require.NoError(t, err)
	require.False(t, res.LAPIReady)
	require.NotEmpty(t, res.Warning, "the timeout outcome carries the LAPI warning")
	require.Less(t, elapsed, 2*time.Second, "shutdown must end the LAPI wait, not the 5 s bound")
	require.Equal(t, 1, applier.count(), "the proxy is applied once")
}

// TestStopCrowdSec_AppCancelledDuringStopCompletes: shutdown during Stop must not skip the SIGKILL path, so the
// executor's context stays live and bounded while the appCtx is cancelled.
func TestStopCrowdSec_AppCancelledDuringStopCompletes(t *testing.T) {
	appCtx, cancelApp := context.WithCancel(context.Background())
	defer cancelApp()
	exec := newBlockingStopExecutor()
	h := newLifecycleStopHandler(t, exec)
	h.WithAppContext(appCtx)

	done := make(chan error, 1)
	go func() {
		_, err := h.StopCrowdSec(context.Background())
		done <- err
	}()
	<-exec.entered
	cancelApp()
	close(exec.release)

	require.NoError(t, <-done)
	requireStopCompletedOnOwnContext(t, exec, h.DB)
}

// requireStopCompletedOnOwnContext asserts the executor's Stop context was live and bounded, and that the stop
// wrote disabled and false.
func requireStopCompletedOnOwnContext(t *testing.T, exec *blockingStopExecutor, db *gorm.DB) {
	t.Helper()
	exec.mu.Lock()
	defer exec.mu.Unlock()
	require.NoError(t, exec.ctxErr, "Executor.Stop must run on a context that no client or shutdown cancels")
	require.True(t, exec.hasDeadline, "Executor.Stop must run under its own bound")

	var cfg models.SecurityConfig
	require.NoError(t, db.Where("name = ?", models.DefaultSecurityConfigName).First(&cfg).Error)
	require.Equal(t, "disabled", cfg.CrowdSecMode)
	require.Equal(t, "false", lifecycleSettingValue(t, db))
}
