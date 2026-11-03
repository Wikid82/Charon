package handlers

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// registrationGateExec answers every cscli call with the key on add. When gate is set, the add call signals
// entered and waits for release before it returns, so a test can hold a registration in flight.
type registrationGateExec struct {
	mu      sync.Mutex
	adds    int
	gate    chan struct{}
	entered chan struct{}
}

func (e *registrationGateExec) Execute(_ context.Context, _ string, args ...string) ([]byte, error) {
	if len(args) < 2 || args[1] != "add" {
		return nil, nil
	}
	e.mu.Lock()
	e.adds++
	e.mu.Unlock()
	if e.gate != nil {
		e.entered <- struct{}{}
		<-e.gate
	}
	return []byte("registered-key-0123456789abcdefghij\n"), nil
}

func (e *registrationGateExec) addCount() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.adds
}

func TestRegistrations_WaitReturnsAtOnceWhenNothingIsInFlight(t *testing.T) {
	h := newTestCrowdsecHandler(t, OpenTestDB(t), &fakeExec{}, "/bin/false", t.TempDir())
	ex := &registrationGateExec{}
	h.CmdExec = ex

	require.NoError(t, h.WaitRegistrations(context.Background()))
	assert.Equal(t, 0, ex.addCount())
}

func TestRegistrations_RefusedOnceTheDrainHasStarted(t *testing.T) {
	h := newTestCrowdsecHandler(t, OpenTestDB(t), &fakeExec{}, "/bin/false", t.TempDir())
	ex := &registrationGateExec{}
	h.CmdExec = ex
	require.NoError(t, h.WaitRegistrations(context.Background()))

	key, err := h.registerBouncer(context.Background())

	require.ErrorIs(t, err, errRegistrationRefused)
	assert.Empty(t, key)
	assert.Equal(t, 0, ex.addCount(), "no cscli call once shutdown has begun")
}

func TestRegistrations_WaitHoldsForAnInFlightReplace(t *testing.T) {
	h := newTestCrowdsecHandler(t, OpenTestDB(t), &fakeExec{}, "/bin/false", t.TempDir())
	ex := &registrationGateExec{gate: make(chan struct{}), entered: make(chan struct{}, 1)}
	h.CmdExec = ex

	type result struct {
		key string
		err error
	}
	registered := make(chan result, 1)
	go func() {
		key, err := h.registerBouncer(context.Background())
		registered <- result{key, err}
	}()
	<-ex.entered

	drained := make(chan error, 1)
	go func() { drained <- h.WaitRegistrations(context.Background()) }()

	select {
	case <-drained:
		t.Fatal("the drain returned while a registration was in flight")
	case <-time.After(50 * time.Millisecond):
	}

	close(ex.gate)
	require.NoError(t, <-drained)
	got := <-registered
	require.NoError(t, got.err)
	assert.Equal(t, "registered-key-0123456789abcdefghij", got.key)
}

func TestRegistrations_WaitGivesUpWhenItsContextEnds(t *testing.T) {
	h := newTestCrowdsecHandler(t, OpenTestDB(t), &fakeExec{}, "/bin/false", t.TempDir())
	ex := &registrationGateExec{gate: make(chan struct{}), entered: make(chan struct{}, 1)}
	h.CmdExec = ex
	go func() { _, _ = h.registerBouncer(context.Background()) }()
	<-ex.entered
	defer close(ex.gate)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	err := h.WaitRegistrations(ctx)

	require.Error(t, err)
	assert.True(t, errors.Is(err, context.DeadlineExceeded), "the wait reports the context that ended it")
}
