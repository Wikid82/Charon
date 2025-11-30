package handlers

import (
    "context"
    "fmt"
    "os"
    "os/exec"
    "path/filepath"
    "strconv"
    "syscall"
)

// DefaultCrowdsecExecutor implements CrowdsecExecutor using OS processes.
type DefaultCrowdsecExecutor struct{
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

func (e *DefaultCrowdsecExecutor) Stop(ctx context.Context, configDir string) error {
    b, err := os.ReadFile(e.pidFile(configDir))
    if err != nil {
        return fmt.Errorf("pid file read: %w", err)
    }
    pid, err := strconv.Atoi(string(b))
    if err != nil {
        return fmt.Errorf("invalid pid: %w", err)
    }
    proc, err := os.FindProcess(pid)
    if err != nil {
        return err
    }
    if err := proc.Signal(syscall.SIGTERM); err != nil {
        return err
    }
    // best-effort remove pid file
    _ = os.Remove(e.pidFile(configDir))
    return nil
}

func (e *DefaultCrowdsecExecutor) Status(ctx context.Context, configDir string) (bool, int, error) {
    b, err := os.ReadFile(e.pidFile(configDir))
    if err != nil {
        return false, 0, nil
    }
    pid, err := strconv.Atoi(string(b))
    if err != nil {
        return false, 0, nil
    }
    // Check process exists
    proc, err := os.FindProcess(pid)
    if err != nil {
        return false, pid, nil
    }
    // Sending signal 0 is not portable on Windows, but OK for Linux containers
    if err := proc.Signal(syscall.Signal(0)); err != nil {
        return false, pid, nil
    }
    return true, pid, nil
}
