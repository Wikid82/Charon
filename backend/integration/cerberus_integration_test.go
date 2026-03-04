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

// TestCerberusIntegration runs the scripts/cerberus_integration.sh
// to verify all security features work together without conflicts.
func TestCerberusIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx, "bash", "./scripts/cerberus_integration.sh")
	cmd.Dir = "../.."

	out, err := cmd.CombinedOutput()
	t.Logf("cerberus_integration script output:\n%s", string(out))

	if err != nil {
		t.Fatalf("cerberus integration failed: %v", err)
	}

	if !strings.Contains(string(out), "ALL CERBERUS INTEGRATION TESTS PASSED") {
		t.Fatalf("unexpected script output, expected pass assertion not found")
	}
}
