package handlers

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
)

// DefaultCrowdsecExecutor implements CrowdsecExecutor using OS processes.
type DefaultCrowdsecExecutor struct {
}

func NewDefaultCrowdsecExecutor() *DefaultCrowdsecExecutor { return &DefaultCrowdsecExecutor{} }

func (e *DefaultCrowdsecExecutor) pidFile(configDir string) string {
	return filepath.Join(configDir, "crowdsec.pid")
}

func (e *DefaultCrowdsecExecutor) Start(ctx context.Context, binPath, configDir string) (int, error) {
	cmd := exec.CommandContext(ctx, binPath, "--config-dir", configDir)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return 0, err
	}
	pid := cmd.Process.Pid
	// write pid file
	if err := os.WriteFile(e.pidFile(configDir), []byte(strconv.Itoa(pid)), 0o644); err != nil {
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

	return true, pid, nil
}
