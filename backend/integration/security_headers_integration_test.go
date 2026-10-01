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

// TestSecurityHeadersIntegration runs scripts/security_headers_integration.sh and ensures it completes successfully.
// This test requires Docker access locally; it is gated behind build tag `integration`.
//
// The test verifies:
// - A proxy host's security header profile emits each header it controls exactly once
// - Headers the profile does not control pass through from the upstream
// - Hosts without a profile leave upstream headers untouched
// - The generated Caddy config emits each headers handler as a non-deferred + deferred pair
// - Streaming responses stay incremental and Caddy-generated 502s carry the profile headers
func TestSecurityHeadersIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	// Run the integration script from the repo root
	cmd := exec.CommandContext(ctx, "bash", "../scripts/security_headers_integration.sh")
	cmd.Dir = ".." // Run from repo root

	out, err := cmd.CombinedOutput()
	t.Logf("security_headers_integration script output:\n%s", string(out))

	if err != nil {
		t.Fatalf("security headers integration failed: %v", err)
	}

	if !strings.Contains(string(out), "Security header profile handling succeeded") {
		t.Fatalf("unexpected script output: security header handling assertion not found")
	}

	if !strings.Contains(string(out), "ALL SECURITY HEADERS TESTS PASSED") {
		t.Fatalf("unexpected script output: final success message not found")
	}
}
