// Package version provides build version information.
package version

const (
	// Name of the application
	Name = "Charon"
)

var (
	// Version is the semantic version, set during release builds via ldflags.
	// The "dev" fallback marks local/untagged builds so they are never
	// mistaken for a release.
	Version = "dev"
	// BuildTime is set during build via ldflags
	BuildTime = "unknown"
	// GitCommit is set during build via ldflags
	GitCommit = "unknown"
)

// Full returns the complete version string.
func Full() string {
	if BuildTime != "unknown" && GitCommit != "unknown" {
		return Version + " (commit: " + GitCommit + ", built: " + BuildTime + ")"
	}
	return Version
}
