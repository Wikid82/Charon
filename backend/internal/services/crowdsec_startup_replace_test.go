package services

import (
	"context"
	"errors"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// addStep is the scripted answer to one "bouncers add" call.
type addStep struct {
	out []byte
	err error
}

// sequencedAddExecutor answers "bouncers add" calls from steps in order and records every call.
type sequencedAddExecutor struct {
	mu    sync.Mutex
	steps []addStep
	calls []string
}

func (e *sequencedAddExecutor) Execute(_ context.Context, _ string, args ...string) ([]byte, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	joined := strings.Join(args, " ")
	e.calls = append(e.calls, joined)
	if strings.HasPrefix(joined, "bouncers add") && len(e.steps) > 0 {
		next := e.steps[0]
		e.steps = e.steps[1:]
		return next.out, next.err
	}
	return nil, nil
}

func TestEnsureBouncerRegistration_FailedAddIsRetriedWithADelete(t *testing.T) {
	h := newBouncerHarness(t)
	exec := &sequencedAddExecutor{steps: []addStep{
		{err: errors.New("exit status 1")},
		{out: []byte(testGeneratedKey + "\n")},
	}}

	require.NoError(t, ensureBouncerRegistration(h.dataDir, h.keyFile, exec))

	got, err := os.ReadFile(h.keyFile)
	require.NoError(t, err)
	assert.Equal(t, testGeneratedKey+"\n", string(got), "the key from the retried add is the one saved")

	var bouncerCalls []string
	for _, c := range exec.calls {
		if strings.Contains(c, "bouncers") {
			bouncerCalls = append(bouncerCalls, c)
		}
	}
	assert.Equal(t, []string{
		"bouncers delete " + startupBouncerName,
		"bouncers add " + startupBouncerName + " -o raw",
		"bouncers delete " + startupBouncerName,
		"bouncers add " + startupBouncerName + " -o raw",
	}, bouncerCalls)
}
