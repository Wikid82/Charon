package cerberus_test

import (
	"testing"
)

// TestCerberusWAFMetric_FollowsResolvedModuleState describes the WAF metric in the API middleware reading the same
// resolved module state as the proxy, not only the static configuration.
func TestCerberusWAFMetric_FollowsResolvedModuleState(t *testing.T) {
	t.Skip("enabled in commit 2 of this PR: API middleware reads the shared module resolver")
}
