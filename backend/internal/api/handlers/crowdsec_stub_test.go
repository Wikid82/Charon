package handlers

import (
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"
)

// Environment variables that steer the stub engine (see runStubCrowdsec).
const (
	stubReadyFileEnv  = "CHARON_TEST_STUB_READY_FILE"
	stubSpawnLogEnv   = "CHARON_TEST_STUB_SPAWN_LOG"
	stubIgnoreTermEnv = "CHARON_TEST_STUB_IGNORE_TERM"
	stubTermDelayEnv  = "CHARON_TEST_STUB_TERM_DELAY"
)

// isStubCrowdsec reports whether this test binary was launched through a
// symlink named "crowdsec", i.e. as the stand-in engine for executor tests.
func isStubCrowdsec() bool {
	return filepath.Base(os.Args[0]) == crowdsecExecutableName
}

// runStubCrowdsec behaves like a minimal engine: it records its start, signals
// readiness once its SIGTERM handler is installed, then waits for SIGTERM. It
// ignores SIGTERM when stubIgnoreTermEnv is set and sleeps stubTermDelayEnv
// before exiting. Because it is a real process named "crowdsec" with
// "-c <config>" arguments, the executor tests can use the real /proc.
func runStubCrowdsec() {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGTERM)

	if p := os.Getenv(stubSpawnLogEnv); p != "" {
		if f, err := os.OpenFile(p, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600); err == nil { //nolint:gosec // G304/G703: test-controlled path
			_, _ = f.WriteString("spawn\n")
			_ = f.Close()
		}
	}
	if p := os.Getenv(stubReadyFileEnv); p != "" {
		_ = os.WriteFile(p, []byte("ready"), 0o600) //nolint:gosec // G306/G703: test-controlled path
	}
	for range sigs {
		if os.Getenv(stubIgnoreTermEnv) != "" {
			continue
		}
		if d, err := time.ParseDuration(os.Getenv(stubTermDelayEnv)); err == nil {
			time.Sleep(d)
		}
		os.Exit(0)
	}
}
