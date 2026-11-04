package handlers

import (
	"os"
	"os/signal"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// signalOSProcess refuses pids Charon may never signal: 0 (the caller's process group), 1 (init) and its own pid.
// Signal 0 is used for the first two so that a missing guard cannot disturb the test runner; the own-pid case
// uses SIGHUP under signal.Notify so a delivered signal is observed instead of ending the test process.
func TestSignalOSProcess_RefusesUnsignalablePids(t *testing.T) {
	for _, pid := range []int{0, 1} {
		err := signalOSProcess(pid, syscall.Signal(0))
		require.Error(t, err, "pid %d must be refused", pid)
		assert.Contains(t, err.Error(), "refusing to signal")
	}
}

func TestSignalOSProcess_RefusesOwnPidWithoutSignalling(t *testing.T) {
	received := make(chan os.Signal, 1)
	signal.Notify(received, syscall.SIGHUP)
	defer signal.Stop(received)

	err := signalOSProcess(os.Getpid(), syscall.SIGHUP)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "refusing to signal")

	select {
	case sig := <-received:
		t.Fatalf("own pid received %v although the signal was refused", sig)
	case <-time.After(200 * time.Millisecond):
	}
}
