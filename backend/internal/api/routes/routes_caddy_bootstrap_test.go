package routes

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
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
	applyInitialCaddyConfig(context.Background(), c, time.Second, time.Millisecond)
	assert.Equal(t, 1, c.applyCalls)
}

func TestApplyInitialCaddyConfig_ApplyErrorIsLogged(t *testing.T) {
	c := &testCaddyBootstrapper{applyErr: errors.New("boom")}
	applyInitialCaddyConfig(context.Background(), c, time.Second, time.Millisecond)
	assert.Equal(t, 1, c.applyCalls)
}

func TestApplyInitialCaddyConfig_TimeoutSkipsApply(t *testing.T) {
	c := &testCaddyBootstrapper{pingErr: errors.New("down")}
	applyInitialCaddyConfig(context.Background(), c, 20*time.Millisecond, time.Millisecond)
	assert.Positive(t, c.pingCalls)
	assert.Zero(t, c.applyCalls)
}

func TestApplyInitialCaddyConfig_CancelledContextSkipsApply(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	c := &testCaddyBootstrapper{}
	applyInitialCaddyConfig(ctx, c, time.Minute, time.Hour)
	assert.Zero(t, c.pingCalls)
	assert.Zero(t, c.applyCalls)
}

func TestApplyInitialCaddyConfig_CancelDuringWaitSkipsApply(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	c := &testCaddyBootstrapper{pingErr: errors.New("down")}
	done := make(chan struct{})
	go func() {
		applyInitialCaddyConfig(ctx, c, time.Minute, time.Millisecond)
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
