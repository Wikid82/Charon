package crowdsec

import "testing"

// skipUntilLaterCommit marks a test that describes behavior a later commit of the CrowdSec robustness series
// adds. Each later commit removes the calls for the behavior it delivers; the last one deletes this helper.
func skipUntilLaterCommit(t *testing.T) {
	t.Helper()
	t.Skip("enabled by a later commit in this series")
}
