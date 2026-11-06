package handlers

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestSeedLastSeenVersion_WritesCanonicalForm ensures a build carrying SemVer
// build metadata seeds the bare core version, so later comparisons and the
// dismiss-permanent anchor agree on the same stored form.
func TestSeedLastSeenVersion_WritesCanonicalForm(t *testing.T) {
	tests := map[string]string{
		"0.46.5+nightly.a1b2c3d": "0.46.5",
		"1.2.0+dev":              "1.2.0",
		"0.47.0-rc.1":            "0.47.0-rc.1",
		"0.46.5":                 "0.46.5",
	}
	for v, want := range tests {
		withVersion(t, v)
		assert.Equalf(t, want, seedLastSeenVersion(), "seedLastSeenVersion() for %q", v)
	}
}
