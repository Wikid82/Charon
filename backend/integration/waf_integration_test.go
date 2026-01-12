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

// TestWAFIntegration runs the scripts/waf_integration.sh and ensures it completes successfully.
func TestWAFIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx, "bash", "./scripts/waf_integration.sh")
	cmd.Dir = "../.."

	out, err := cmd.CombinedOutput()
	t.Logf("waf_integration script output:\n%s", string(out))

	if err != nil {
		t.Fatalf("waf integration failed: %v", err)
	}

	if !strings.Contains(string(out), "ALL WAF TESTS PASSED") {
		t.Fatalf("unexpected script output, expected pass assertion not found")
	}
}
