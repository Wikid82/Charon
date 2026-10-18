package handlers

import (
	"context"
	"os"
	osexec "os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const eventuallyTimeout = 10 * time.Second

func TestDefaultCrowdsecExecutorPidFile(t *testing.T) {
	e := NewDefaultCrowdsecExecutor()
	tmp := t.TempDir()
	expected := filepath.Join(tmp, "crowdsec.pid")
	if p := e.pidFile(tmp); p != expected {
		t.Fatalf("pidFile mismatch got %s expected %s", p, expected)
	}
}

func TestDefaultCrowdsecExecutorStartStatusStop(t *testing.T) {
	e, bin, dir := newStubExecutor(t)
	ctx, cancel := context.WithTimeout(context.Background(), eventuallyTimeout)
	defer cancel()

	pid, err := e.Start(ctx, bin, dir)
	require.NoError(t, err)
	require.Positive(t, pid)

	pidB, err := os.ReadFile(e.pidFile(dir))
	require.NoError(t, err)
	assert.Equal(t, strconv.Itoa(pid), string(pidB))

	running, got, err := e.Status(ctx, dir)
	require.NoError(t, err)
	assert.True(t, running)
	assert.Equal(t, pid, got)

	require.NoError(t, e.Stop(ctx, dir))
	assert.False(t, e.isAlive(pid))

	running, _, _ = e.Status(ctx, dir)
	assert.False(t, running, "process still running after stop")
}

// Additional coverage tests for error paths

func TestDefaultCrowdsecExecutor_Status_NoPidFile(t *testing.T) {
	exec := NewDefaultCrowdsecExecutor()
	tmpDir := t.TempDir()

	running, pid, err := exec.Status(context.Background(), tmpDir)

	assert.NoError(t, err)
	assert.False(t, running)
	assert.Equal(t, 0, pid)
}

func TestDefaultCrowdsecExecutor_Status_InvalidPid(t *testing.T) {
	exec := NewDefaultCrowdsecExecutor()
	tmpDir := t.TempDir()

	// Write invalid pid
	_ = os.WriteFile(filepath.Join(tmpDir, "crowdsec.pid"), []byte("invalid"), 0o600) // #nosec G306 -- test fixture

	running, pid, err := exec.Status(context.Background(), tmpDir)

	assert.NoError(t, err)
	assert.False(t, running)
	assert.Equal(t, 0, pid)
}

func TestDefaultCrowdsecExecutor_Status_NonExistentProcess(t *testing.T) {
	exec := NewDefaultCrowdsecExecutor()
	tmpDir := t.TempDir()

	// Write a pid that doesn't exist
	// Use a very high PID that's unlikely to exist
	_ = os.WriteFile(filepath.Join(tmpDir, "crowdsec.pid"), []byte("999999999"), 0o600) // #nosec G306 -- test fixture

	running, pid, err := exec.Status(context.Background(), tmpDir)

	assert.NoError(t, err)
	assert.False(t, running)
	assert.Equal(t, 999999999, pid)
}

func TestDefaultCrowdsecExecutor_Stop_NoPidFile(t *testing.T) {
	exec := NewDefaultCrowdsecExecutor()
	tmpDir := t.TempDir()

	err := exec.Stop(context.Background(), tmpDir)

	// Stop should be idempotent - no PID file means already stopped
	assert.NoError(t, err)
}

func TestDefaultCrowdsecExecutor_Stop_InvalidPid(t *testing.T) {
	exec := NewDefaultCrowdsecExecutor()
	tmpDir := t.TempDir()

	// Write invalid pid
	_ = os.WriteFile(filepath.Join(tmpDir, "crowdsec.pid"), []byte("invalid"), 0o600) // #nosec G306 -- test fixture

	err := exec.Stop(context.Background(), tmpDir)

	// Stop should clean up malformed PID file and succeed
	assert.NoError(t, err)

	// Verify PID file was cleaned up
	_, statErr := os.Stat(filepath.Join(tmpDir, "crowdsec.pid"))
	assert.True(t, os.IsNotExist(statErr), "PID file should be removed after Stop with invalid PID")
}

func TestDefaultCrowdsecExecutor_Stop_NonExistentProcess(t *testing.T) {
	exec := NewDefaultCrowdsecExecutor()
	tmpDir := t.TempDir()

	// Write a pid that doesn't exist
	_ = os.WriteFile(filepath.Join(tmpDir, "crowdsec.pid"), []byte("999999999"), 0o600) // #nosec G306 -- test fixture

	err := exec.Stop(context.Background(), tmpDir)

	// Stop should be idempotent - stale PID file means process already dead
	assert.NoError(t, err)

	// Verify PID file was cleaned up
	_, statErr := os.Stat(filepath.Join(tmpDir, "crowdsec.pid"))
	assert.True(t, os.IsNotExist(statErr), "Stale PID file should be cleaned up after Stop")
}

func TestDefaultCrowdsecExecutor_Stop_Idempotent(t *testing.T) {
	exec := NewDefaultCrowdsecExecutor()
	tmpDir := t.TempDir()

	// Stop should succeed even when called multiple times
	err1 := exec.Stop(context.Background(), tmpDir)
	err2 := exec.Stop(context.Background(), tmpDir)
	err3 := exec.Stop(context.Background(), tmpDir)

	assert.NoError(t, err1)
	assert.NoError(t, err2)
	assert.NoError(t, err3)
}

func TestDefaultCrowdsecExecutor_Start_InvalidBinary(t *testing.T) {
	exec := NewDefaultCrowdsecExecutor()
	tmpDir := t.TempDir()

	pid, err := exec.Start(context.Background(), "/nonexistent/binary", tmpDir)

	assert.Error(t, err)
	assert.Equal(t, 0, pid)
}

// Tests for process identification (PID reuse and look-alike processes)

func TestDefaultCrowdsecExecutor_isCrowdSecProcess(t *testing.T) {
	const cfg = "/data/config/config.yaml"
	cases := []struct {
		name string
		args []string
		want bool
	}{
		{"engine by absolute path", []string{"/usr/local/bin/crowdsec", "-c", cfg}, true},
		{"engine by bare name", []string{"crowdsec", "-c", cfg}, true},
		{"engine of another config", []string{"/usr/local/bin/crowdsec", "-c", "/elsewhere/config/config.yaml"}, false},
		{"cscli with the config path", []string{"/usr/local/bin/cscli", "-c", cfg, "lapi", "status"}, false},
		{"tail of the config file", []string{"tail", "-f", cfg}, false},
		{"executable merely containing crowdsec", []string{"/opt/crowdsec-tools/run", "-c", cfg}, false},
		{"debugger", []string{"/usr/local/bin/dlv", "--telemetry", "--headless"}, false},
		{"no arguments", []string{"crowdsec"}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e, proc, _ := newProcExecutor(t)
			writeFakeProc(t, proc, 12345, tc.args...)
			assert.Equal(t, tc.want, e.isCrowdSecProcess(12345, cfg))
		})
	}
}

func TestDefaultCrowdsecExecutor_isCrowdSecProcess_UnreadableProcess(t *testing.T) {
	e, proc, _ := newProcExecutor(t)
	assert.False(t, e.isCrowdSecProcess(99999, "cfg"), "non-existent process")

	dir := filepath.Join(proc, "12345")
	require.NoError(t, os.MkdirAll(dir, 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "cmdline"), nil, 0o600)) // #nosec G306 -- test fixture
	assert.False(t, e.isCrowdSecProcess(12345, "cfg"), "empty cmdline")
}

func TestDefaultCrowdsecExecutor_Status_PIDReuse_DifferentProcess(t *testing.T) {
	e, proc, dir := newProcExecutor(t)
	pid := spawnSleeper(t)
	writeFakeProc(t, proc, pid, "/usr/local/bin/dlv", "debug")
	require.NoError(t, e.writePidFile(dir, pid))

	running, got, err := e.Status(context.Background(), dir)
	require.NoError(t, err)
	assert.False(t, running, "Should detect PID reuse and return not running")
	assert.Equal(t, pid, got)
}

func TestDefaultCrowdsecExecutor_Status_PIDReuse_IsCrowdSec(t *testing.T) {
	e, proc, dir := newProcExecutor(t)
	pid := spawnSleeper(t)
	writeFakeProc(t, proc, pid, "/usr/bin/crowdsec", "-c", configFile(dir))
	require.NoError(t, e.writePidFile(dir, pid))

	running, got, err := e.Status(context.Background(), dir)
	require.NoError(t, err)
	assert.True(t, running, "Should return running when process is CrowdSec")
	assert.Equal(t, pid, got)
}

func TestDefaultCrowdsecExecutor_Status_DoesNotAdoptLookalikes(t *testing.T) {
	e, proc, dir := newProcExecutor(t)
	cscli := spawnSleeper(t)
	tail := spawnSleeper(t)
	writeFakeProc(t, proc, cscli, "/usr/local/bin/cscli", "-c", configFile(dir), "lapi", "status")
	writeFakeProc(t, proc, tail, "tail", "-f", configFile(dir))

	running, _, err := e.Status(context.Background(), dir)
	require.NoError(t, err)
	assert.False(t, running)
	_, statErr := os.Stat(e.pidFile(dir))
	assert.True(t, os.IsNotExist(statErr), "look-alike must not be recorded in the pid file")
}

func TestDefaultCrowdsecExecutor_Stop_DoesNotSignalLookalikes(t *testing.T) {
	e, proc, dir := newProcExecutor(t)
	cscli := spawnSleeper(t)
	tail := spawnSleeper(t)
	writeFakeProc(t, proc, cscli, "/usr/local/bin/cscli", "-c", configFile(dir), "lapi", "status")
	writeFakeProc(t, proc, tail, "tail", "-f", configFile(dir))
	require.NoError(t, e.writePidFile(dir, tail))

	require.NoError(t, e.Stop(context.Background(), dir))
	assert.True(t, e.isAlive(cscli), "cscli must not be signalled")
	assert.True(t, e.isAlive(tail), "tail must not be signalled")
}

func TestDefaultCrowdsecExecutor_Start_DoesNotMistakeLookalikeForRunningEngine(t *testing.T) {
	e, bin, dir := newStubExecutor(t)
	tail := spawnSleeper(t)
	// A look-alike recorded in the pid file must not block a real start.
	require.NoError(t, e.writePidFile(dir, tail))

	pid, err := e.Start(context.Background(), bin, dir)
	require.NoError(t, err)
	assert.NotEqual(t, tail, pid)
	t.Cleanup(func() { _ = e.Stop(context.Background(), dir) })
}

// Pids that Charon must never signal are treated as "not running".
func TestDefaultCrowdsecExecutor_RefusesUnsignalablePids(t *testing.T) {
	for _, pid := range []int{0, 1, os.Getpid()} {
		t.Run(strconv.Itoa(pid), func(t *testing.T) {
			e, proc, dir := newProcExecutor(t)
			writeFakeProc(t, proc, pid, "crowdsec", "-c", configFile(dir))
			require.NoError(t, e.writePidFile(dir, pid))

			running, _, err := e.Status(context.Background(), dir)
			require.NoError(t, err)
			assert.False(t, running)

			require.NoError(t, e.Stop(context.Background(), dir))
			_, statErr := os.Stat(e.pidFile(dir))
			assert.True(t, os.IsNotExist(statErr), "stale pid file should be removed")
		})
	}
}

func TestDefaultCrowdsecExecutor_isAlive_TreatsZombieAsDead(t *testing.T) {
	e, proc, _ := newProcExecutor(t)
	pid := spawnSleeper(t)
	assert.True(t, e.isAlive(pid), "no stat entry: signal 0 decides")

	writeFakeStat(t, proc, pid, "123 (crowdsec (x) y) S 1 2 3")
	assert.True(t, e.isAlive(pid))

	writeFakeStat(t, proc, pid, "123 (crowdsec (x) y) Z 1 2 3")
	assert.False(t, e.isAlive(pid), "zombie state means the process is gone")

	writeFakeStat(t, proc, pid, "garbage")
	assert.True(t, e.isAlive(pid), "unparseable stat falls back to signal 0")

	assert.False(t, e.isAlive(999999999))
}

// Process lifecycle against a real stub engine

func TestDefaultCrowdsecExecutor_Start_ConcurrentCallsSpawnOnce(t *testing.T) {
	e, bin, dir := newStubExecutor(t)
	spawnLog := filepath.Join(dir, "spawns.log")
	t.Setenv(stubSpawnLogEnv, spawnLog)
	t.Cleanup(func() { _ = e.Stop(context.Background(), dir) })

	const callers = 8
	pids := make([]int, callers)
	errs := make([]error, callers)
	var wg sync.WaitGroup
	for i := range callers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			pids[i], errs[i] = e.Start(context.Background(), bin, dir)
		}()
	}
	wg.Wait()

	for i := range callers {
		require.NoError(t, errs[i])
		assert.Equal(t, pids[0], pids[i], "every caller must see the same engine")
	}
	assert.Eventually(t, func() bool {
		b, err := os.ReadFile(spawnLog) // #nosec G304 -- test path
		return err == nil && strings.Count(string(b), "spawn") >= 1
	}, eventuallyTimeout, 10*time.Millisecond)
	b, err := os.ReadFile(spawnLog) // #nosec G304 -- test path
	require.NoError(t, err)
	assert.Equal(t, 1, strings.Count(string(b), "spawn"), "exactly one engine process must be spawned")
}

func TestDefaultCrowdsecExecutor_Start_ReturnsExistingPid(t *testing.T) {
	e, bin, dir := newStubExecutor(t)
	t.Cleanup(func() { _ = e.Stop(context.Background(), dir) })

	first, err := e.Start(context.Background(), bin, dir)
	require.NoError(t, err)
	second, err := e.Start(context.Background(), bin, dir)
	require.NoError(t, err)
	assert.Equal(t, first, second)
}

func TestDefaultCrowdsecExecutor_Start_PidFileWriteFailureStillReapsProcess(t *testing.T) {
	e, bin, dir := newStubExecutor(t)
	// A directory where the pid file belongs makes the write fail.
	require.NoError(t, os.Mkdir(e.pidFile(dir), 0o750))

	pid, err := e.Start(context.Background(), bin, dir)
	require.Error(t, err)
	require.Positive(t, pid)
	require.NoError(t, syscall.Kill(pid, syscall.SIGTERM))
	// The background Wait must still reap the child, leaving no zombie behind.
	assert.Eventually(t, func() bool { return syscall.Kill(pid, 0) != nil }, eventuallyTimeout, 10*time.Millisecond)
}

func TestDefaultCrowdsecExecutor_Start_LateExitKeepsNewerPidFile(t *testing.T) {
	e, bin, dir := newStubExecutor(t)
	ready := filepath.Join(dir, "ready")
	t.Setenv(stubReadyFileEnv, ready)

	pid, err := e.Start(context.Background(), bin, dir)
	require.NoError(t, err)
	waitForFile(t, ready)
	// Simulate a surviving engine whose pid replaced ours in the file.
	require.NoError(t, e.writePidFile(dir, pid+1))

	require.NoError(t, syscall.Kill(pid, syscall.SIGTERM))
	// Once the child is reaped the Wait goroutine's cleanup runs right after;
	// the file must keep the newer pid throughout.
	assert.Eventually(t, func() bool { return syscall.Kill(pid, 0) != nil }, eventuallyTimeout, 10*time.Millisecond)
	assert.Never(t, func() bool {
		b, readErr := os.ReadFile(e.pidFile(dir))
		return readErr != nil || string(b) != strconv.Itoa(pid+1)
	}, 300*time.Millisecond, 10*time.Millisecond)
}

func TestDefaultCrowdsecExecutor_Start_ExitRemovesOwnPidFile(t *testing.T) {
	e, bin, dir := newStubExecutor(t)

	pid, err := e.Start(context.Background(), bin, dir)
	require.NoError(t, err)
	require.NoError(t, syscall.Kill(pid, syscall.SIGTERM))
	assert.Eventually(t, func() bool {
		_, statErr := os.Stat(e.pidFile(dir))
		return os.IsNotExist(statErr)
	}, eventuallyTimeout, 10*time.Millisecond)
}

func TestDefaultCrowdsecExecutor_Status_AdoptsUntrackedEngine(t *testing.T) {
	e, proc, dir := newProcExecutor(t)
	pid := spawnSleeper(t)
	writeFakeProc(t, proc, pid, "/usr/local/bin/crowdsec", "-c", configFile(dir))

	running, got, err := e.Status(context.Background(), dir)
	require.NoError(t, err)
	assert.True(t, running)
	assert.Equal(t, pid, got)
	b, err := os.ReadFile(e.pidFile(dir))
	require.NoError(t, err)
	assert.Equal(t, strconv.Itoa(pid), string(b), "pid file should be rewritten on adoption")
}

func TestDefaultCrowdsecExecutor_Status_AdoptionToleratesUnwritablePidFile(t *testing.T) {
	e, proc, dir := newProcExecutor(t)
	pid := spawnSleeper(t)
	writeFakeProc(t, proc, pid, "crowdsec", "-c", configFile(dir))
	require.NoError(t, os.Mkdir(e.pidFile(dir), 0o750)) // pid file cannot be written

	running, got, err := e.Status(context.Background(), dir)
	require.NoError(t, err)
	assert.True(t, running)
	assert.Equal(t, pid, got)
}

func TestDefaultCrowdsecExecutor_Status_IgnoresEngineOfOtherConfig(t *testing.T) {
	e, proc, dir := newProcExecutor(t)
	pid := spawnSleeper(t)
	writeFakeProc(t, proc, pid, "/usr/local/bin/crowdsec", "-c", "/elsewhere/config/config.yaml")

	running, _, err := e.Status(context.Background(), dir)
	require.NoError(t, err)
	assert.False(t, running)
}

func TestDefaultCrowdsecExecutor_Status_StalePidFileFallsBackToScan(t *testing.T) {
	e, proc, dir := newProcExecutor(t)
	engine := spawnSleeper(t)
	other := spawnSleeper(t)
	writeFakeProc(t, proc, engine, "crowdsec", "-c", configFile(dir))
	writeFakeProc(t, proc, other, "sleep", "30")
	require.NoError(t, e.writePidFile(dir, other))

	running, got, err := e.Status(context.Background(), dir)
	require.NoError(t, err)
	assert.True(t, running)
	assert.Equal(t, engine, got)
}

func TestDefaultCrowdsecExecutor_Stop_SignalsUntrackedEngine(t *testing.T) {
	e, proc, dir := newProcExecutor(t)
	pid := spawnSleeper(t)
	writeFakeProc(t, proc, pid, "crowdsec", "-c", configFile(dir))

	require.NoError(t, e.Stop(context.Background(), dir))
	assert.False(t, e.isAlive(pid), "Stop must wait for the engine to exit")
	_, statErr := os.Stat(e.pidFile(dir))
	assert.True(t, os.IsNotExist(statErr))
}

func TestDefaultCrowdsecExecutor_Stop_DoesNotKillRecycledPid(t *testing.T) {
	e, proc, dir := newProcExecutor(t)
	pid := spawnSleeper(t)
	writeFakeProc(t, proc, pid, "sleep", "30")
	require.NoError(t, e.writePidFile(dir, pid))

	require.NoError(t, e.Stop(context.Background(), dir))
	assert.True(t, e.isAlive(pid), "non-CrowdSec process must not be signalled")
	_, statErr := os.Stat(e.pidFile(dir))
	assert.True(t, os.IsNotExist(statErr), "stale pid file should be removed")
}

func TestDefaultCrowdsecExecutor_Stop_WaitsForGracefulExit(t *testing.T) {
	e, bin, dir := newStubExecutor(t)
	e.stopTimeout = 5 * time.Second
	ready := filepath.Join(dir, "ready")
	t.Setenv(stubReadyFileEnv, ready)
	t.Setenv(stubTermDelayEnv, "500ms")

	pid, err := e.Start(context.Background(), bin, dir)
	require.NoError(t, err)
	waitForFile(t, ready) // the stub's SIGTERM handler is installed

	start := time.Now()
	require.NoError(t, e.Stop(context.Background(), dir))
	assert.GreaterOrEqual(t, time.Since(start), 400*time.Millisecond, "Stop should wait for the process to exit")
	assert.False(t, e.isAlive(pid))
}

func TestDefaultCrowdsecExecutor_Stop_KillsStubbornEngine(t *testing.T) {
	e, bin, dir := newStubExecutor(t)
	ready := filepath.Join(dir, "ready")
	t.Setenv(stubReadyFileEnv, ready)
	t.Setenv(stubIgnoreTermEnv, "1")

	pid, err := e.Start(context.Background(), bin, dir)
	require.NoError(t, err)
	waitForFile(t, ready)

	require.NoError(t, e.Stop(context.Background(), dir))
	assert.False(t, e.isAlive(pid), "engine ignoring SIGTERM must be killed")
	_, statErr := os.Stat(e.pidFile(dir))
	assert.True(t, os.IsNotExist(statErr))
}

func TestDefaultCrowdsecExecutor_Stop_CancelledContextDoesNotEscalate(t *testing.T) {
	e, bin, dir := newStubExecutor(t)
	e.stopTimeout = 30 * time.Second
	ready := filepath.Join(dir, "ready")
	t.Setenv(stubReadyFileEnv, ready)
	t.Setenv(stubIgnoreTermEnv, "1")

	pid, err := e.Start(context.Background(), bin, dir)
	require.NoError(t, err)
	t.Cleanup(func() { _ = syscall.Kill(pid, syscall.SIGKILL) })
	waitForFile(t, ready)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err = e.Stop(ctx, dir)
	require.ErrorIs(t, err, context.Canceled)
	assert.True(t, e.isAlive(pid), "a cancelled request must not escalate to SIGKILL")
	_, statErr := os.Stat(e.pidFile(dir))
	assert.NoError(t, statErr, "pid file must be kept while the engine is still running")
}

// Helpers

// spawnSleeper starts a real child process and reaps it in the background so a
// signalled child does not linger as a zombie (which still answers signal 0).
func spawnSleeper(t *testing.T) int {
	t.Helper()
	cmd := osexec.Command("sleep", "30")
	require.NoError(t, cmd.Start())
	t.Cleanup(func() { _ = cmd.Process.Kill() })
	go func() { _ = cmd.Wait() }()
	return cmd.Process.Pid
}

// writeFakeProc registers a fake /proc/<pid>/cmdline entry.
func writeFakeProc(t *testing.T, procDir string, pid int, args ...string) {
	t.Helper()
	dir := filepath.Join(procDir, strconv.Itoa(pid))
	require.NoError(t, os.MkdirAll(dir, 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "cmdline"), []byte(strings.Join(args, "\x00")+"\x00"), 0o600)) // #nosec G306 -- test fixture
}

// writeFakeStat registers a fake /proc/<pid>/stat entry.
func writeFakeStat(t *testing.T, procDir string, pid int, content string) {
	t.Helper()
	dir := filepath.Join(procDir, strconv.Itoa(pid))
	require.NoError(t, os.MkdirAll(dir, 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "stat"), []byte(content), 0o600)) // #nosec G306 -- test fixture
}

// waitForFile blocks until path exists.
func waitForFile(t *testing.T, path string) {
	t.Helper()
	require.Eventually(t, func() bool {
		_, err := os.Stat(path)
		return err == nil
	}, eventuallyTimeout, 5*time.Millisecond)
}

// newProcExecutor returns an executor reading a fake /proc, for tests that
// register look-alike processes by hand.
func newProcExecutor(t *testing.T) (e *DefaultCrowdsecExecutor, procDir, configDir string) {
	t.Helper()
	e = NewDefaultCrowdsecExecutor()
	e.procPath = t.TempDir()
	e.stopTimeout = 300 * time.Millisecond
	return e, e.procPath, t.TempDir()
}

// newStubExecutor returns an executor on the real /proc plus the path of a
// stub engine (this test binary under the name "crowdsec", see TestMain).
func newStubExecutor(t *testing.T) (e *DefaultCrowdsecExecutor, binPath, configDir string) {
	t.Helper()
	self, err := os.Executable()
	require.NoError(t, err)
	binPath = filepath.Join(t.TempDir(), crowdsecExecutableName)
	require.NoError(t, os.Symlink(self, binPath))
	e = NewDefaultCrowdsecExecutor()
	e.stopTimeout = 2 * time.Second
	return e, binPath, t.TempDir()
}
