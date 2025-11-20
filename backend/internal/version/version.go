package version

const (
	// Name of the application
	Name = "CPMP"
	// Version is the semantic version
	Version = "0.1.0"
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
