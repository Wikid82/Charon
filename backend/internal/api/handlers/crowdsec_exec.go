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

// DefaultCrowdsecExecutor implements CrowdsecExecutor using OS processes.
type DefaultCrowdsecExecutor struct {
	// procPath allows overriding /proc for testing
	procPath string
	// stopTimeout is how long Stop waits after SIGTERM before sending SIGKILL
	stopTimeout time.Duration
}

func NewDefaultCrowdsecExecutor() *DefaultCrowdsecExecutor {
	return &DefaultCrowdsecExecutor{
		procPath:    "/proc",
		stopTimeout: defaultStopTimeout,
	}
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

// isCrowdSecProcess checks if the given PID is actually a CrowdSec process
// by reading /proc/{pid}/cmdline and verifying it contains "crowdsec".
// This prevents false positives when PIDs are recycled by the OS.
func (e *DefaultCrowdsecExecutor) isCrowdSecProcess(pid int) bool {
	args, ok := e.readCmdline(pid)
	return ok && strings.Contains(strings.Join(args, "\x00"), "crowdsec")
}

// findProcess scans the process table for a CrowdSec engine started with this
// config directory's config file. It finds engines that the pid file does not
// track, e.g. after the file was lost or overwritten.
func (e *DefaultCrowdsecExecutor) findProcess(configDir string) (int, bool) {
	entries, err := os.ReadDir(e.procPath)
	if err != nil {
		return 0, false
	}
	configFile := filepath.Join(configDir, "config", "config.yaml")
	for _, entry := range entries {
		pid, convErr := strconv.Atoi(entry.Name())
		if convErr != nil || !e.isCrowdSecProcess(pid) {
			continue
		}
		args, _ := e.readCmdline(pid)
		for _, arg := range args {
			if arg == configFile {
				return pid, true
			}
		}
	}
	return 0, false
}

// isAlive reports whether a process with the given pid exists.
func isAlive(pid int) bool {
	return syscall.Kill(pid, syscall.Signal(0)) == nil
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
// stale. An untracked engine that is found gets the pid file rewritten.
func (e *DefaultCrowdsecExecutor) trackedPid(configDir string) (pid int, running bool) {
	// #nosec G304 -- Reading PID file for CrowdSec process, path controlled by configDir parameter
	if b, err := os.ReadFile(e.pidFile(configDir)); err == nil {
		if filePid, convErr := strconv.Atoi(strings.TrimSpace(string(b))); convErr == nil && isAlive(filePid) {
			if e.isCrowdSecProcess(filePid) {
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

func (e *DefaultCrowdsecExecutor) Start(ctx context.Context, binPath, configDir string) (int, error) {
	configFile := filepath.Join(configDir, "config", "config.yaml")

	// Use exec.Command (not CommandContext) to avoid context cancellation killing the process
	// CrowdSec should run independently of the startup goroutine's lifecycle
	//
	// #nosec G204 -- binPath is server-controlled: sourced from CHARON_CROWDSEC_BIN env var
	// or defaults to "/usr/local/bin/crowdsec". Not user input. Arguments are static.
	cmd := exec.Command(binPath, "-c", configFile)

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
	// write pid file
	if err := e.writePidFile(configDir, pid); err != nil {
		return pid, fmt.Errorf("failed to write pid file: %w", err)
	}
	// wait in background
	go func() {
		_ = cmd.Wait()
		e.removePidFileIfOwnedBy(configDir, pid)
	}()
	return pid, nil
}

// Stop stops the CrowdSec process: SIGTERM, then SIGKILL if it has not exited
// within the stop timeout. It is idempotent - stopping an already-stopped
// service or one that was never started will succeed without error. A pid
// file pointing at a recycled, non-CrowdSec process is removed without
// signalling anything.
func (e *DefaultCrowdsecExecutor) Stop(ctx context.Context, configDir string) error {
	pidFilePath := e.pidFile(configDir)
	pid, running := e.trackedPid(configDir)
	if !running {
		_ = os.Remove(pidFilePath)
		return nil
	}

	if err := syscall.Kill(pid, syscall.SIGTERM); err != nil {
		if errors.Is(err, syscall.ESRCH) {
			_ = os.Remove(pidFilePath)
			return nil
		}
		return fmt.Errorf("signal crowdsec: %w", err)
	}

	if !e.waitForExit(ctx, pid, e.stopTimeout) {
		logger.Log().WithField("pid", pid).Warn("CrowdSec did not exit after SIGTERM, sending SIGKILL")
		_ = syscall.Kill(pid, syscall.SIGKILL)
		e.waitForExit(ctx, pid, e.stopTimeout)
	}

	_ = os.Remove(pidFilePath)
	return nil
}

// waitForExit polls until the process is gone, the timeout elapses or ctx is
// cancelled. It reports whether the process exited.
func (e *DefaultCrowdsecExecutor) waitForExit(ctx context.Context, pid int, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for isAlive(pid) {
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
