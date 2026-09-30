package routes

import (
	"bytes"
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/Wikid82/charon/backend/internal/logger"
)

type testCaddyBootstrapper struct {
	pingErr    error
	applyErr   error
	pingCalls  int
	applyCalls int
}

func (c *testCaddyBootstrapper) Ping(context.Context) error {
	c.pingCalls++
	return c.pingErr
}

func (c *testCaddyBootstrapper) ApplyConfig(context.Context) error {
	c.applyCalls++
	return c.applyErr
}

func TestApplyInitialCaddyConfig_AppliesWhenReady(t *testing.T) {
	c := &testCaddyBootstrapper{}
	rec := &doneRecorder{}
	applyInitialCaddyConfig(context.Background(), c, time.Second, time.Millisecond, rec.onDone)
	assert.Equal(t, 1, c.applyCalls)
	assert.Equal(t, []bool{true}, rec.calls(), "onDone(true) exactly once on success")
}

// doneRecorder records every onDone call.
type doneRecorder struct {
	mu      sync.Mutex
	results []bool
}

func (r *doneRecorder) onDone(applied bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.results = append(r.results, applied)
}

func (r *doneRecorder) calls() []bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]bool(nil), r.results...)
}

func TestApplyInitialCaddyConfig_NilOnDoneIsAllowed(t *testing.T) {
	c := &testCaddyBootstrapper{}
	applyInitialCaddyConfig(context.Background(), c, time.Second, time.Millisecond, nil)
	assert.Equal(t, 1, c.applyCalls)
}

func TestApplyInitialCaddyConfig_ApplyErrorIsLogged(t *testing.T) {
	var out bytes.Buffer
	logger.Init(false, &out)
	t.Cleanup(func() { logger.Init(false, nil) })

	c := &testCaddyBootstrapper{applyErr: errors.New("boom")}
	rec := &doneRecorder{}
	applyInitialCaddyConfig(context.Background(), c, time.Second, time.Millisecond, rec.onDone)
	assert.Equal(t, 1, c.applyCalls)
	assert.Equal(t, []bool{false}, rec.calls(), "onDone(false) exactly once when ApplyConfig fails")
	assert.Contains(t, out.String(), "Failed to apply initial Caddy config")
	assert.Contains(t, out.String(), "boom")
}

func TestApplyInitialCaddyConfig_TimeoutSkipsApply(t *testing.T) {
	c := &testCaddyBootstrapper{pingErr: errors.New("down")}
	rec := &doneRecorder{}
	applyInitialCaddyConfig(context.Background(), c, 20*time.Millisecond, time.Millisecond, rec.onDone)
	assert.Positive(t, c.pingCalls)
	assert.Zero(t, c.applyCalls)
	assert.Equal(t, []bool{false}, rec.calls(), "onDone(false) exactly once on timeout")
}

func TestApplyInitialCaddyConfig_CancelledContextSkipsApply(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	c := &testCaddyBootstrapper{}
	rec := &doneRecorder{}
	applyInitialCaddyConfig(ctx, c, time.Minute, time.Hour, rec.onDone)
	assert.Zero(t, c.pingCalls)
	assert.Zero(t, c.applyCalls)
	assert.Equal(t, []bool{false}, rec.calls(), "onDone(false) exactly once on a cancelled context")
}

func TestApplyInitialCaddyConfig_CancelDuringWaitSkipsApply(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	c := &testCaddyBootstrapper{pingErr: errors.New("down")}
	done := make(chan struct{})
	rec := &doneRecorder{}
	go func() {
		applyInitialCaddyConfig(ctx, c, time.Minute, time.Millisecond, rec.onDone)
		close(done)
	}()
	time.Sleep(10 * time.Millisecond)
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("applyInitialCaddyConfig did not return after cancel")
	}
	assert.Zero(t, c.applyCalls)
}

func TestApplyInitialCaddyConfig_CancelAfterPingSkipsApply(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	c := &cancellingPinger{cancel: cancel}
	rec := &doneRecorder{}
	applyInitialCaddyConfig(ctx, c, time.Minute, time.Millisecond, rec.onDone)
	assert.Zero(t, c.applyCalls)
	assert.Equal(t, []bool{false}, rec.calls())
}

// cancellingPinger cancels the context from inside a successful Ping, the
// window between "Caddy answered" and "apply the config".
type cancellingPinger struct {
	cancel     context.CancelFunc
	applyCalls int
}

func (c *cancellingPinger) Ping(context.Context) error { c.cancel(); return nil }

func (c *cancellingPinger) ApplyConfig(context.Context) error { c.applyCalls++; return nil }
