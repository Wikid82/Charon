package services

import "testing"

// TestAgentImageTag covers every row of the version-to-tag mapping in
// docs/plans/pr33_orthrus_transport_plan.md section 4.9, plus the edge cases
// that must fall back to the unpinned "latest" tag.
func TestAgentImageTag(t *testing.T) {
	tests := []struct {
		name       string
		version    string
		wantTag    string
		wantPinned bool
	}{
		// Release and main channels: strict X.Y.Z with no build metadata.
		{name: "release version is pinned", version: "0.49.1", wantTag: "0.49.1", wantPinned: true},
		{name: "major zero release is pinned", version: "1.0.0", wantTag: "1.0.0", wantPinned: true},

		// Development and nightly channels follow a moving tag.
		{name: "dev channel", version: "0.49.1+dev.abc1234", wantTag: "dev"},
		{name: "nightly channel", version: "0.49.1+nightly.abc1234", wantTag: "nightly"},

		// Every other build has no release version and uses latest.
		{name: "pull request build", version: "0.49.1+pr.42", wantTag: "latest"},
		{name: "local build", version: "0.49.1+local.abc1234", wantTag: "latest"},
		{name: "local dirty build", version: "0.49.1+local.abc1234.dirty", wantTag: "latest"},
		{name: "branch build", version: "0.49.1+branch.abc1234", wantTag: "latest"},
		{name: "dev without version", version: "dev", wantTag: "latest"},
		{name: "empty version", version: "", wantTag: "latest"},
		{name: "unparsable version", version: "garbage", wantTag: "latest"},
		{name: "leading v is not strict semver", version: "v0.49.1", wantTag: "latest"},
		{name: "shorthand is not strict semver", version: "0.49", wantTag: "latest"},
		{name: "unknown metadata channel", version: "0.49.1+other.abc1234", wantTag: "latest"},

		// Pre-releases are not published as agent images; they map to latest.
		{name: "prerelease", version: "0.49.1-rc.1", wantTag: "latest"},
		{name: "prerelease with dev metadata", version: "0.49.1-rc.1+dev.abc1234", wantTag: "latest"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotTag, gotPinned := agentImageTag(tt.version)
			if gotTag != tt.wantTag {
				t.Errorf("agentImageTag(%q) tag = %q, want %q", tt.version, gotTag, tt.wantTag)
			}
			if gotPinned != tt.wantPinned {
				t.Errorf("agentImageTag(%q) pinned = %v, want %v", tt.version, gotPinned, tt.wantPinned)
			}
		})
	}
}

// TestAgentImageNote checks the snippet comment for each tag.
func TestAgentImageNote(t *testing.T) {
	tests := []struct {
		name string
		tag  string
		want string
	}{
		{name: "pinned release has no comment", tag: "0.49.1", want: ""},
		{name: "dev tag", tag: "dev", want: "Development build: this tag follows the development branch."},
		{name: "nightly tag", tag: "nightly", want: "Nightly build: this tag follows the nightly channel."},
		{
			name: "latest tag",
			tag:  "latest",
			want: "This Charon build has no release version; use the agent tag that matches your Charon release.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := agentImageNote(tt.tag); got != tt.want {
				t.Errorf("agentImageNote(%q) = %q, want %q", tt.tag, got, tt.want)
			}
		})
	}
}
