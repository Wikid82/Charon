package main

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/Wikid82/charon/backend/internal/dbmaint"
)

// stubRunner records when it was waited on and whether it ever returns.
type stubRunner struct {
	log      *[]string
	returned bool
}

func (s stubRunner) WaitRunner(time.Duration) bool {
	*s.log = append(*s.log, "runner")
	return s.returned
}

func TestShutdownDrain_WaitsForTheRunnerBeforeAnyOtherStep(t *testing.T) {
	var order []string
	shutdownDrain(stubRunner{log: &order, returned: true}, time.Second,
		func() { order = append(order, "uptime") },
		func() { order = append(order, "emergency") },
	)
	assert.Equal(t, []string{"runner", "uptime", "emergency"}, order)
}

func TestShutdownDrain_ContinuesWhenTheRunnerNeverReturns(t *testing.T) {
	var order []string
	shutdownDrain(stubRunner{log: &order, returned: false}, time.Second, func() { order = append(order, "uptime") })
	assert.Equal(t, []string{"runner", "uptime"}, order, "the remaining steps still run")
}

func TestShutdownDrain_TheRunnerWaitIsBounded(t *testing.T) {
	gate := dbmaint.NewGate()
	gate.MarkPlanned() // a runner is expected and never exits

	start := time.Now()
	ran := false
	shutdownDrain(gate, 50*time.Millisecond, func() { ran = true })

	elapsed := time.Since(start)
	assert.True(t, ran)
	assert.GreaterOrEqual(t, elapsed, 50*time.Millisecond)
	assert.Less(t, elapsed, 2*time.Second, "bounded at the runner wait, not at the runner")
}

func TestShutdownDrain_NoRunnerReturnsImmediately(t *testing.T) {
	start := time.Now()
	shutdownDrain(dbmaint.NewGate(), dbmaint.ShutdownRunnerWait)
	assert.Less(t, time.Since(start), time.Second)
}

func TestShutdownRunnerWaitFitsTheDockerStopGrace(t *testing.T) {
	// 10 s default grace: ~1 s entrypoint trap latency + the runner wait + the
	// emergency server stop must stay inside it.
	assert.LessOrEqual(t, dbmaint.ShutdownRunnerWait, 4*time.Second)
}
