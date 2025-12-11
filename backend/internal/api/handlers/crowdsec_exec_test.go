package handlers

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDefaultCrowdsecExecutorPidFile(t *testing.T) {
	e := NewDefaultCrowdsecExecutor()
	tmp := t.TempDir()
	expected := filepath.Join(tmp, "crowdsec.pid")
	if p := e.pidFile(tmp); p != expected {
		t.Fatalf("pidFile mismatch got %s expected %s", p, expected)
	}
}

func TestDefaultCrowdsecExecutorStartStatusStop(t *testing.T) {
	e := NewDefaultCrowdsecExecutor()
	tmp := t.TempDir()

	// create a tiny script that sleeps and traps TERM
	script := filepath.Join(tmp, "runscript.sh")
	content := `#!/bin/sh
trap 'exit 0' TERM INT
while true; do sleep 1; done
`
	if err := os.WriteFile(script, []byte(content), 0o755); err != nil {
		t.Fatalf("write script: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	pid, err := e.Start(ctx, script, tmp)
	if err != nil {
		t.Fatalf("start err: %v", err)
	}
	if pid <= 0 {
		t.Fatalf("invalid pid %d", pid)
	}

	// ensure pid file exists and content matches
	pidB, err := os.ReadFile(e.pidFile(tmp))
	if err != nil {
		t.Fatalf("read pid file: %v", err)
	}
	gotPid, _ := strconv.Atoi(string(pidB))
	if gotPid != pid {
		t.Fatalf("pid file mismatch got %d expected %d", gotPid, pid)
	}

	// Status should return running
	running, got, err := e.Status(ctx, tmp)
	if err != nil {
		t.Fatalf("status err: %v", err)
	}
	if !running || got != pid {
		t.Fatalf("status expected running for %d got %d running=%v", pid, got, running)
	}

	// Stop should terminate and remove pid file
	if err := e.Stop(ctx, tmp); err != nil {
		t.Fatalf("stop err: %v", err)
	}

	// give a little time for process to exit
	time.Sleep(200 * time.Millisecond)

	running2, _, _ := e.Status(ctx, tmp)
	if running2 {
		t.Fatalf("process still running after stop")
	}
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
	os.WriteFile(filepath.Join(tmpDir, "crowdsec.pid"), []byte("invalid"), 0o644)

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
	os.WriteFile(filepath.Join(tmpDir, "crowdsec.pid"), []byte("999999999"), 0o644)

	running, pid, err := exec.Status(context.Background(), tmpDir)

	assert.NoError(t, err)
	assert.False(t, running)
	assert.Equal(t, 999999999, pid)
}

func TestDefaultCrowdsecExecutor_Stop_NoPidFile(t *testing.T) {
	exec := NewDefaultCrowdsecExecutor()
	tmpDir := t.TempDir()

	err := exec.Stop(context.Background(), tmpDir)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "pid file read")
}

func TestDefaultCrowdsecExecutor_Stop_InvalidPid(t *testing.T) {
	exec := NewDefaultCrowdsecExecutor()
	tmpDir := t.TempDir()

	// Write invalid pid
	os.WriteFile(filepath.Join(tmpDir, "crowdsec.pid"), []byte("invalid"), 0o644)

	err := exec.Stop(context.Background(), tmpDir)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid pid")
}

func TestDefaultCrowdsecExecutor_Stop_NonExistentProcess(t *testing.T) {
	exec := NewDefaultCrowdsecExecutor()
	tmpDir := t.TempDir()

	// Write a pid that doesn't exist
	os.WriteFile(filepath.Join(tmpDir, "crowdsec.pid"), []byte("999999999"), 0o644)

	err := exec.Stop(context.Background(), tmpDir)

	// Should fail with signal error
	assert.Error(t, err)
}

func TestDefaultCrowdsecExecutor_Start_InvalidBinary(t *testing.T) {
	exec := NewDefaultCrowdsecExecutor()
	tmpDir := t.TempDir()

	pid, err := exec.Start(context.Background(), "/nonexistent/binary", tmpDir)

	assert.Error(t, err)
	assert.Equal(t, 0, pid)
}
