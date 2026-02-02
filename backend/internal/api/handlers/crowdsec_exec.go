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

	"github.com/Wikid82/charon/backend/internal/logger"
)

// DefaultCrowdsecExecutor implements CrowdsecExecutor using OS processes.
type DefaultCrowdsecExecutor struct {
	// procPath allows overriding /proc for testing
	procPath string
}

func NewDefaultCrowdsecExecutor() *DefaultCrowdsecExecutor {
	return &DefaultCrowdsecExecutor{
		procPath: "/proc",
	}
}

// isCrowdSecProcess checks if the given PID is actually a CrowdSec process
// by reading /proc/{pid}/cmdline and verifying it contains "crowdsec".
// This prevents false positives when PIDs are recycled by the OS.
func (e *DefaultCrowdsecExecutor) isCrowdSecProcess(pid int) bool {
	cmdlinePath := filepath.Join(e.procPath, strconv.Itoa(pid), "cmdline")
	// #nosec G304 -- Reading process cmdline for PID validation, path constructed from trusted procPath and pid
	data, err := os.ReadFile(cmdlinePath)
	if err != nil {
		// Process doesn't exist or can't read - not CrowdSec
		return false
	}
	// cmdline is null-separated, but strings.Contains works on the raw bytes
	return strings.Contains(string(data), "crowdsec")
}

func (e *DefaultCrowdsecExecutor) pidFile(configDir string) string {
	return filepath.Join(configDir, "crowdsec.pid")
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
	if err := os.WriteFile(e.pidFile(configDir), []byte(strconv.Itoa(pid)), 0o600); err != nil {
		return pid, fmt.Errorf("failed to write pid file: %w", err)
	}
	// wait in background
	go func() {
		_ = cmd.Wait()
		_ = os.Remove(e.pidFile(configDir))
	}()
	return pid, nil
}

// Stop stops the CrowdSec process. It is idempotent - stopping an already-stopped
// service or one that was never started will succeed without error.
func (e *DefaultCrowdsecExecutor) Stop(ctx context.Context, configDir string) error {
	pidFilePath := e.pidFile(configDir)
	// #nosec G304 -- Reading PID file for CrowdSec process, path controlled by configDir parameter
	b, err := os.ReadFile(pidFilePath)
	if err != nil {
		// If PID file doesn't exist, service is already stopped - return success
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("pid file read: %w", err)
	}

	pid, err := strconv.Atoi(string(b))
	if err != nil {
		// Malformed PID file - clean it up and return success
		_ = os.Remove(pidFilePath)
		return nil
	}

	proc, err := os.FindProcess(pid)
	if err != nil {
		// Process lookup failed - clean up PID file and return success
		_ = os.Remove(pidFilePath)
		return nil
	}

	if err := proc.Signal(syscall.SIGTERM); err != nil {
		// Check if process is already dead (ESRCH = no such process)
		if errors.Is(err, syscall.ESRCH) || errors.Is(err, os.ErrProcessDone) {
			_ = os.Remove(pidFilePath)
			return nil
		}
		return err
	}

	// Successfully sent signal - remove PID file
	_ = os.Remove(pidFilePath)
	return nil
}

func (e *DefaultCrowdsecExecutor) Status(ctx context.Context, configDir string) (running bool, pid int, err error) {
	b, err := os.ReadFile(e.pidFile(configDir))
	if err != nil {
		// Missing pid file is treated as not running
		return false, 0, nil
	}

	pid, err = strconv.Atoi(string(b))
	if err != nil {
		// Malformed pid file is treated as not running
		return false, 0, nil
	}

	proc, err := os.FindProcess(pid)
	if err != nil {
		// Process lookup failures are treated as not running
		return false, pid, nil
	}

	// Sending signal 0 is not portable on Windows, but OK for Linux containers
	if err = proc.Signal(syscall.Signal(0)); err != nil {
		if errors.Is(err, os.ErrProcessDone) {
			return false, pid, nil
		}
		// ESRCH or other errors mean process isn't running
		return false, pid, nil
	}

	// After successful Signal(0) check, verify it's actually CrowdSec
	// This prevents false positives when PIDs are recycled by the OS
	if !e.isCrowdSecProcess(pid) {
		logger.Log().WithField("pid", pid).Warn("PID exists but is not CrowdSec (PID recycled)")
		return false, pid, nil
	}

	return true, pid, nil
}
