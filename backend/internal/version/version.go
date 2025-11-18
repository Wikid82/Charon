package version

var (
	// Name identifies the service in logs and telemetry.
	Name = "caddy-proxy-manager-plus"
	// SemVer captures the backend semantic version (injected at build time via ldflags).
	SemVer = "0.1.0-alpha"
	// GitCommit is the git commit SHA (injected at build time via ldflags).
	GitCommit = "unknown"
	// BuildDate is the build timestamp (injected at build time via ldflags).
	BuildDate = "unknown"
)

// Full returns the complete version string with commit and build date.
func Full() string {
	if GitCommit != "unknown" && BuildDate != "unknown" {
		return SemVer + " (" + GitCommit[:7] + ", " + BuildDate + ")"
	}
	return SemVer
}
