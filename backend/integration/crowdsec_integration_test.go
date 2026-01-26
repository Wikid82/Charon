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

// TestCrowdsecIntegration runs scripts/crowdsec_integration.sh and ensures it completes successfully.
func TestCrowdsecIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	cmd := exec.CommandContext(context.Background(), "bash", "./scripts/crowdsec_integration.sh")
	// Ensure script runs from repo root so relative paths in scripts work reliably
	cmd.Dir = "../../"
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()
	cmd = exec.CommandContext(ctx, "bash", "./scripts/crowdsec_integration.sh")
	cmd.Dir = "../../"

	out, err := cmd.CombinedOutput()
	t.Logf("crowdsec_integration script output:\n%s", string(out))
	if err != nil {
		t.Fatalf("crowdsec integration failed: %v", err)
	}
	if !strings.Contains(string(out), "Apply response: ") {
		t.Fatalf("unexpected script output, expected Apply response in output")
	}
}
