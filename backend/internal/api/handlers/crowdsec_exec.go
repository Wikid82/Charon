package handlers

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/Wikid82/charon/backend/internal/logger"
)

// Defaults for how long Stop waits for the engine to exit after SIGTERM, and
// how often it checks.
const (
	defaultStopTimeout = 5 * time.Second
	stopPollInterval   = 50 * time.Millisecond
)

// crowdsecExecutableName is the base name of the CrowdSec engine executable.
const crowdsecExecutableName = "crowdsec"

// DefaultCrowdsecExecutor implements CrowdsecExecutor using OS processes.
type DefaultCrowdsecExecutor struct {
	// mu serialises Start and Stop so concurrent callers cannot spawn two
	// engines or stop one while another is starting. It is never held while
	// waiting for LAPI (that wait lives in the handler).
	mu sync.Mutex
	// procPath allows overriding /proc for testing
	procPath string
	// stopTimeout is how long Stop waits after SIGTERM before sending SIGKILL
	stopTimeout time.Duration
	// kill delivers a signal to a process; tests replace it to exercise
	// signalling failures without privileges.
	kill func(pid int, sig syscall.Signal) error
}

func NewDefaultCrowdsecExecutor() *DefaultCrowdsecExecutor {
	return &DefaultCrowdsecExecutor{
		procPath:    "/proc",
		stopTimeout: defaultStopTimeout,
		kill:        syscall.Kill,
	}
}

// configFile returns the config file path Charon passes to the engine.
func configFile(configDir string) string {
	return filepath.Join(configDir, "config", "config.yaml")
}

// signalable reports whether pid is one Charon may ever signal: never init,
// the kernel's pid 0 / negative group targets, or Charon itself.
func signalable(pid int) bool {
	return pid > 1 && pid != os.Getpid()
}

// readCmdline returns the null-separated arguments of the process, or false
// when the process does not exist or cannot be read.
func (e *DefaultCrowdsecExecutor) readCmdline(pid int) ([]string, bool) {
	cmdlinePath := filepath.Join(e.procPath, strconv.Itoa(pid), "cmdline")
	// #nosec G304 -- Reading process cmdline for PID validation, path constructed from trusted procPath and pid
	data, err := os.ReadFile(cmdlinePath)
	if err != nil {
		return nil, false
	}
	return strings.Split(strings.TrimRight(string(data), "\x00"), "\x00"), true
}

// isCrowdSecProcess checks that pid is a CrowdSec engine started with the
// given config file: its executable (argv[0]) must be named "crowdsec" and one
// of its arguments must be exactly the config file. Matching on the executable
// rather than a substring keeps tools that merely mention the config path
// (cscli -c <config>, tail -f <config>) from being mistaken for the engine,
// and checking the config keeps a recycled pid from matching.
func (e *DefaultCrowdsecExecutor) isCrowdSecProcess(pid int, cfgFile string) bool {
	args, ok := e.readCmdline(pid)
	if !ok || len(args) < 2 || filepath.Base(args[0]) != crowdsecExecutableName {
		return false
	}
	for _, arg := range args[1:] {
		if arg == cfgFile {
			return true
		}
	}
	return false
}

// findProcess scans the process table for a CrowdSec engine started with this
// config directory's config file. It finds engines that the pid file does not
// track, e.g. after the file was lost or overwritten.
func (e *DefaultCrowdsecExecutor) findProcess(configDir string) (int, bool) {
	entries, err := os.ReadDir(e.procPath)
	if err != nil {
		return 0, false
	}
	cfgFile := configFile(configDir)
	for _, entry := range entries {
		pid, convErr := strconv.Atoi(entry.Name())
		if convErr == nil && signalable(pid) && e.isCrowdSecProcess(pid, cfgFile) {
			return pid, true
		}
	}
	return 0, false
}

// isAlive reports whether a live process with the given pid exists. A zombie
// (exited but not yet reaped) still answers signal 0, so it counts as dead.
func (e *DefaultCrowdsecExecutor) isAlive(pid int) bool {
	if syscall.Kill(pid, syscall.Signal(0)) != nil {
		return false
	}
	// #nosec G304 -- path constructed from trusted procPath and pid
	stat, err := os.ReadFile(filepath.Join(e.procPath, strconv.Itoa(pid), "stat"))
	if err != nil {
		return true
	}
	// Format: "pid (comm) S ..."; comm may contain spaces and parentheses, so
	// the state is the first field after the last ")".
	idx := strings.LastIndex(string(stat), ")")
	if idx < 0 {
		return true
	}
	fields := strings.Fields(string(stat)[idx+1:])
	return len(fields) == 0 || fields[0] != "Z"
}

func (e *DefaultCrowdsecExecutor) pidFile(configDir string) string {
	return filepath.Join(configDir, "crowdsec.pid")
}

func (e *DefaultCrowdsecExecutor) writePidFile(configDir string, pid int) error {
	return os.WriteFile(e.pidFile(configDir), []byte(strconv.Itoa(pid)), 0o600)
}

// removePidFileIfOwnedBy removes the pid file only while it still records pid,
// so a process that exits late cannot erase the record of a newer engine.
func (e *DefaultCrowdsecExecutor) removePidFileIfOwnedBy(configDir string, pid int) {
	// #nosec G304 -- path controlled by configDir parameter
	b, err := os.ReadFile(e.pidFile(configDir))
	if err != nil || strings.TrimSpace(string(b)) != strconv.Itoa(pid) {
		return
	}
	_ = os.Remove(e.pidFile(configDir))
}

// trackedPid returns the running CrowdSec pid, preferring the pid file and
// falling back to a process-table scan when the file is missing, malformed or
// stale. An untracked engine that is found gets the pid file rewritten, so
// even read-only callers such as Status can modify the pid file.
func (e *DefaultCrowdsecExecutor) trackedPid(configDir string) (pid int, running bool) {
	cfgFile := configFile(configDir)
	// #nosec G304 -- Reading PID file for CrowdSec process, path controlled by configDir parameter
	if b, err := os.ReadFile(e.pidFile(configDir)); err == nil {
		if filePid, convErr := strconv.Atoi(strings.TrimSpace(string(b))); convErr == nil && signalable(filePid) && e.isAlive(filePid) {
			if e.isCrowdSecProcess(filePid, cfgFile) {
				return filePid, true
			}
			logger.Log().WithField("pid", filePid).Warn("PID exists but is not CrowdSec (PID recycled)")
		}
	}
	found, ok := e.findProcess(configDir)
	if !ok {
		return 0, false
	}
	if err := e.writePidFile(configDir, found); err != nil {
		logger.Log().WithError(err).Warn("failed to record adopted CrowdSec pid")
	}
	logger.Log().WithField("pid", found).Info("adopted CrowdSec process not tracked by pid file")
	return found, true
}

// Start launches the CrowdSec engine, or returns the pid of the one already
// running for this config directory. The check and the spawn happen under one
// lock, so concurrent calls never start two engines.
func (e *DefaultCrowdsecExecutor) Start(ctx context.Context, binPath, configDir string) (int, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if pid, running := e.trackedPid(configDir); running {
		return pid, nil
	}

	// Use exec.Command (not CommandContext) to avoid context cancellation killing the process
	// CrowdSec should run independently of the startup goroutine's lifecycle
	//
	// #nosec G204 -- binPath is server-controlled: sourced from CHARON_CROWDSEC_BIN env var
	// or defaults to "/usr/local/bin/crowdsec". Not user input. Arguments are static.
	cmd := exec.Command(binPath, "-c", configFile(configDir))

	// Detach the process so it doesn't get killed when the parent exits
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true, // Create new process group
	}

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return 0, err
	}
	pid := cmd.Process.Pid
	// Reap the process whatever happens next so it never lingers as a zombie.
	go func() {
		_ = cmd.Wait()
		e.removePidFileIfOwnedBy(configDir, pid)
	}()
	if err := e.writePidFile(configDir, pid); err != nil {
		return pid, fmt.Errorf("failed to write pid file: %w", err)
	}
	return pid, nil
}

// Stop stops the CrowdSec process: SIGTERM, then SIGKILL if it has not exited
// within the stop timeout. It is idempotent - stopping an already-stopped
// service or one that was never started will succeed without error. A pid
// file pointing at a recycled, non-CrowdSec process is removed without
// signalling anything. If ctx is cancelled while waiting, Stop gives up with
// the context error instead of escalating to SIGKILL; if the process survives
// SIGKILL, Stop returns an error and keeps the pid file.
func (e *DefaultCrowdsecExecutor) Stop(ctx context.Context, configDir string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	pidFilePath := e.pidFile(configDir)
	pid, running := e.trackedPid(configDir)
	if !running {
		_ = os.Remove(pidFilePath)
		return nil
	}

	if err := e.kill(pid, syscall.SIGTERM); err != nil {
		if errors.Is(err, syscall.ESRCH) {
			_ = os.Remove(pidFilePath)
			return nil
		}
		return fmt.Errorf("signal crowdsec: %w", err)
	}

	if !e.waitForExit(ctx, pid, e.stopTimeout) {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("waiting for crowdsec to exit: %w", err)
		}
		logger.Log().WithField("pid", pid).Warn("CrowdSec did not exit after SIGTERM, sending SIGKILL")
		if err := e.kill(pid, syscall.SIGKILL); err != nil && !errors.Is(err, syscall.ESRCH) {
			return fmt.Errorf("kill crowdsec: %w", err)
		}
		if !e.waitForExit(ctx, pid, e.stopTimeout) {
			return fmt.Errorf("crowdsec (pid %d) is still running after SIGKILL", pid)
		}
	}

	_ = os.Remove(pidFilePath)
	return nil
}

// waitForExit polls until the process is gone, the timeout elapses or ctx is
// cancelled. It reports whether the process exited.
func (e *DefaultCrowdsecExecutor) waitForExit(ctx context.Context, pid int, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for e.isAlive(pid) {
		if time.Now().After(deadline) || ctx.Err() != nil {
			return false
		}
		time.Sleep(stopPollInterval)
	}
	return true
}

// Status reports whether CrowdSec is running, falling back to a process-table
// scan when the pid file does not point at a live CrowdSec process. When no
// engine is running it returns the (possibly stale) pid recorded in the file.
func (e *DefaultCrowdsecExecutor) Status(ctx context.Context, configDir string) (running bool, pid int, err error) {
	if pid, running = e.trackedPid(configDir); running {
		return true, pid, nil
	}
	// #nosec G304 -- Reading PID file for CrowdSec process, path controlled by configDir parameter
	if b, readErr := os.ReadFile(e.pidFile(configDir)); readErr == nil {
		pid, _ = strconv.Atoi(strings.TrimSpace(string(b)))
	}
	return false, pid, nil
}
