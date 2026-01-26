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

// TestCrowdsecStartup runs the scripts/crowdsec_startup_test.sh and ensures
// CrowdSec can start successfully without the fatal "no datasource enabled" error.
// This is a focused test for verifying basic CrowdSec initialization.
//
// The test verifies:
// - No "no datasource enabled" fatal error
// - LAPI health endpoint responds (if CrowdSec is installed)
// - Acquisition config exists with datasource definition
// - Parsers and scenarios are installed (if cscli is available)
//
// This test requires Docker access and is gated behind build tag `integration`.
func TestCrowdsecStartup(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	// Set a timeout for the entire test
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Run the startup test script from the repo root
	cmd := exec.CommandContext(ctx, "bash", "../scripts/crowdsec_startup_test.sh")
	cmd.Dir = ".." // Run from repo root

	out, err := cmd.CombinedOutput()
	t.Logf("crowdsec_startup_test script output:\n%s", string(out))

	// Check for the specific fatal error that indicates CrowdSec is broken
	if strings.Contains(string(out), "no datasource enabled") {
		t.Fatal("CRITICAL: CrowdSec failed with 'no datasource enabled' - acquis.yaml is missing or empty")
	}

	if err != nil {
		t.Fatalf("crowdsec startup test failed: %v", err)
	}

	// Verify success message is present
	if !strings.Contains(string(out), "ALL CROWDSEC STARTUP TESTS PASSED") {
		t.Fatalf("unexpected script output: final success message not found")
	}
}

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
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	// Set a timeout for the entire test
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	// Run the integration script from the repo root
	cmd := exec.CommandContext(ctx, "bash", "../scripts/crowdsec_decision_integration.sh")
	cmd.Dir = ".." // Run from repo root

	out, err := cmd.CombinedOutput()
	t.Logf("crowdsec_decision_integration script output:\n%s", string(out))

	// Check for the specific fatal error that indicates CrowdSec is broken
	if strings.Contains(string(out), "no datasource enabled") {
		t.Fatal("CRITICAL: CrowdSec failed with 'no datasource enabled' - acquis.yaml is missing or empty")
	}

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
