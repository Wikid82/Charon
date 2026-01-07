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

// TestRateLimitIntegration runs the scripts/rate_limit_integration.sh and ensures it completes successfully.
// This test requires Docker and docker compose access locally; it is gated behind build tag `integration`.
//
// The test verifies:
// - Rate limiting is correctly applied to proxy hosts
// - Requests within the limit return HTTP 200
// - Requests exceeding the limit return HTTP 429
// - Rate limit window resets correctly
func TestRateLimitIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	// Set a timeout for the entire test (rate limit tests need time for window resets)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	// Run the integration script from the repo root
	cmd := exec.CommandContext(ctx, "bash", "../scripts/rate_limit_integration.sh")
	cmd.Dir = ".." // Run from repo root

	out, err := cmd.CombinedOutput()
	t.Logf("rate_limit_integration script output:\n%s", string(out))

	if err != nil {
		t.Fatalf("rate limit integration failed: %v", err)
	}

	// Verify key assertions are present in output
	if !strings.Contains(string(out), "Rate limit enforcement succeeded") {
		t.Fatalf("unexpected script output: rate limit enforcement assertion not found")
	}

	if !strings.Contains(string(out), "ALL RATE LIMIT TESTS PASSED") {
		t.Fatalf("unexpected script output: final success message not found")
	}
}
