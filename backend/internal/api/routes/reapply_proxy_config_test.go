package routes

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// fakeReapplier counts the re-applies it is asked to run.
type fakeReapplier struct {
	calls int
	err   error
}

func (f *fakeReapplier) ApplyConfig(context.Context) error {
	f.calls++
	return f.err
}

func TestReapplyProxyConfig_AppliesOnce(t *testing.T) {
	f := &fakeReapplier{}
	reapplyProxyConfig(f)
	assert.Equal(t, 1, f.calls)
}

func TestReapplyProxyConfig_ToleratesFailure(t *testing.T) {
	f := &fakeReapplier{err: errors.New("caddy unreachable")}
	reapplyProxyConfig(f)
	assert.Equal(t, 1, f.calls)
}

// signalReapplier reports each re-apply on a channel.
type signalReapplier struct{ done chan struct{} }

func (s signalReapplier) ApplyConfig(context.Context) error {
	s.done <- struct{}{}
	return nil
}

func TestBackgroundReapply_RunsApplyOnItsOwnGoroutine(t *testing.T) {
	s := signalReapplier{done: make(chan struct{})}
	hook := backgroundReapply(s)
	// The hook returns before the apply finishes: the unbuffered send blocks
	// until this test receives it.
	hook()
	select {
	case <-s.done:
	case <-time.After(2 * time.Second):
		t.Fatal("background re-apply did not run")
	}
}
