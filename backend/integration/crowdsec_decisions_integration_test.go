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

// TestCrowdsecDecisionsIntegration runs the scripts/crowdsec_decision_integration.sh and ensures it completes successfully.
// This test requires Docker access locally; it is gated behind build tag `integration`.
//
// The test verifies:
// - CrowdSec status endpoint works correctly
// - Decisions list endpoint returns valid response
// - Ban IP operation works (or gracefully handles missing cscli)
// - Unban IP operation works (or gracefully handles missing cscli)
// - Export endpoint returns valid response
// - LAPI health endpoint returns valid response
//
// Note: CrowdSec binary may not be available in the test container.
// Tests gracefully handle this scenario and skip operations requiring cscli.
func TestCrowdsecDecisionsIntegration(t *testing.T) {
	t.Parallel()

	// Set a timeout for the entire test
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	// Run the integration script from the repo root
	cmd := exec.CommandContext(ctx, "bash", "../scripts/crowdsec_decision_integration.sh")
	cmd.Dir = ".." // Run from repo root

	out, err := cmd.CombinedOutput()
	t.Logf("crowdsec_decision_integration script output:\n%s", string(out))

	if err != nil {
		t.Fatalf("crowdsec decision integration failed: %v", err)
	}

	// Verify key assertions are present in output
	if !strings.Contains(string(out), "Passed:") {
		t.Fatalf("unexpected script output: pass count not found")
	}

	if !strings.Contains(string(out), "ALL CROWDSEC DECISION TESTS PASSED") {
		t.Fatalf("unexpected script output: final success message not found")
	}
}
