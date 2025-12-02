package handlers

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"
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
