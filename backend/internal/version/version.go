package version

const (
	// Name of the application
	Name = "CPMP"
)

var (
	// Version is the semantic version
	Version = "0.2.0-beta.1"
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
