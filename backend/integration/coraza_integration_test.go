//go:build integration
// +build integration

package integration

import (
    "context"
    "os/exec"
    "strings"
    "testing"
    "time"
)

// TestCorazaIntegration runs the scripts/coraza_integration.sh and ensures it completes successfully.
// This test requires Docker and docker compose access locally; it is gated behind build tag `integration`.
func TestCorazaIntegration(t *testing.T) {
    t.Parallel()

    // Ensure the script exists
    cmd := exec.CommandContext(context.Background(), "bash", "./scripts/coraza_integration.sh")
    // set a timeout in case something hangs
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
    defer cancel()
    cmd = exec.CommandContext(ctx, "bash", "./scripts/coraza_integration.sh")

    out, err := cmd.CombinedOutput()
    t.Logf("coraza_integration script output:\n%s", string(out))
    if err != nil {
        t.Fatalf("coraza integration failed: %v", err)
    }
    if !strings.Contains(string(out), "Coraza WAF blocked payload as expected") {
        t.Fatalf("unexpected script output, expected blocking assertion not found")
    }
}
