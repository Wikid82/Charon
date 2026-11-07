package version

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFull(t *testing.T) {
	t.Parallel()
	// Default
	assert.Contains(t, Full(), Version)

	// With build info
	originalBuildTime := BuildTime
	originalGitCommit := GitCommit
	defer func() {
		BuildTime = originalBuildTime
		GitCommit = originalGitCommit
	}()

	BuildTime = "2023-01-01"
	GitCommit = "abcdef"

	full := Full()
	assert.Contains(t, full, "2023-01-01")
	assert.Contains(t, full, "abcdef")
}

func TestCanonical(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		in     string
		want   string
		wantOK bool
	}{
		{name: "plain release", in: "0.46.4", want: "0.46.4", wantOK: true},
		{name: "prerelease kept", in: "0.47.0-rc.1", want: "0.47.0-rc.1", wantOK: true},
		{name: "build metadata stripped", in: "0.46.5+nightly.abc", want: "0.46.5", wantOK: true},
		{name: "prerelease and metadata", in: "0.47.0-rc.1+build.5", want: "0.47.0-rc.1", wantOK: true},
		{name: "dev sentinel", in: "dev", want: "", wantOK: false},
		{name: "empty", in: "", want: "", wantOK: false},
		{name: "leading v", in: "v1.2.3", want: "", wantOK: false},
		{name: "major only shorthand", in: "1", want: "", wantOK: false},
		{name: "major.minor shorthand", in: "1.2", want: "", wantOK: false},
		{name: "non-semver nightly tag", in: "nightly-a1b2c3d", want: "", wantOK: false},
		{name: "leading zero component", in: "01.2.3", want: "", wantOK: false},
		{name: "four components", in: "1.2.3.4", want: "", wantOK: false},
		{name: "dangling prerelease separator", in: "1.2.3-", want: "", wantOK: false},
		{name: "empty build metadata", in: "1.2.3+", want: "", wantOK: false},
		{name: "numeric prerelease leading zero", in: "1.2.3-01", want: "", wantOK: false},
		{name: "numeric prerelease zero", in: "1.2.3-0", want: "1.2.3-0", wantOK: true},
		{name: "rc prerelease", in: "1.2.3-rc.1", want: "1.2.3-rc.1", wantOK: true},
		{name: "alphanumeric prerelease starting with zero", in: "1.2.3-0a", want: "1.2.3-0a", wantOK: true},
		{name: "prerelease with metadata stripped", in: "1.2.3-0a+build.1", want: "1.2.3-0a", wantOK: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, ok := Canonical(tt.in)
			assert.Equal(t, tt.wantOK, ok)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestIsUnversioned(t *testing.T) {
	t.Parallel()
	tests := []struct {
		in   string
		want bool
	}{
		{in: "0.46.4", want: false},
		{in: "0.47.0-rc.1", want: false},
		{in: "0.46.5+nightly.abc", want: false},
		{in: "dev", want: true},
		{in: "", want: true},
		{in: "v1.2.3", want: true},
		{in: "1", want: true},
		{in: "1.2", want: true},
		{in: "nightly-a1b2c3d", want: true},
		{in: "1.2.3-01", want: true},
		{in: "1.2.3-0", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, IsUnversioned(tt.in))
		})
	}
}

func TestCompare(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		a, b    string
		wantCmp int
		wantOK  bool
	}{
		{name: "prerelease newer than older release", a: "0.47.0-rc.1", b: "0.46.5+nightly.abc", wantCmp: 1, wantOK: true},
		{name: "prerelease older than its release", a: "0.47.0-rc.1", b: "0.47.0", wantCmp: -1, wantOK: true},
		{name: "build metadata ignored", a: "0.46.5+nightly.abc", b: "0.46.5", wantCmp: 0, wantOK: true},
		{name: "patch older", a: "0.46.4", b: "0.46.5", wantCmp: -1, wantOK: true},
		{name: "major dominates minor", a: "1.0.0", b: "0.99.99", wantCmp: 1, wantOK: true},
		{name: "equal", a: "0.47.0", b: "0.47.0", wantCmp: 0, wantOK: true},
		{name: "numeric prerelease identifiers", a: "0.47.0-rc.2", b: "0.47.0-rc.10", wantCmp: -1, wantOK: true},
		{name: "dev left side", a: "dev", b: "0.46.5", wantCmp: 0, wantOK: false},
		{name: "shorthand right side", a: "0.46.5", b: "1.2", wantCmp: 0, wantOK: false},
		{name: "leading v left side", a: "v0.46.5", b: "0.46.5", wantCmp: 0, wantOK: false},
		{name: "both unversioned", a: "nightly-x", b: "dev", wantCmp: 0, wantOK: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cmp, ok := Compare(tt.a, tt.b)
			assert.Equal(t, tt.wantOK, ok)
			assert.Equal(t, tt.wantCmp, cmp)
		})
	}
}
