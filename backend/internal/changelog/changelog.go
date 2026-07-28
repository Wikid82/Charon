// Package changelog provides build-time-generated "What's New" data,
// embedded into the binary at compile time — no runtime file I/O or
// network calls. The embedded data/changelog.json is regenerated from
// conventional-commit git history by scripts/generate-changelog.sh
// immediately before release builds; local/dev/PR builds embed the
// committed placeholder ([]).
package changelog

import (
	_ "embed"
	"encoding/json"
	"sort"

	"golang.org/x/mod/semver"
)

//go:embed data/changelog.json
var rawData []byte

// Entry describes one released version's categorized, novice-friendly
// changelog content.
type Entry struct {
	Version  string   `json:"version"`
	Date     string   `json:"date"`
	Features []string `json:"features"`
	Fixes    []string `json:"fixes"`
	Other    []string `json:"other"`
}

// allEntries holds the parsed embedded changelog data. Parsed once at
// package init; only ever reassigned by tests via SetEntriesForTesting.
var allEntries []Entry

func init() {
	// Malformed/missing embedded data degrades to "no changelog" rather
	// than panicking at boot — the placeholder `[]` always parses cleanly,
	// and a bad CI-generated file should never crash the server.
	_ = json.Unmarshal(rawData, &allEntries)
}

// SetEntriesForTesting overrides the package-level parsed changelog data
// and returns a function that restores the original value. Test-only —
// production code must never call this.
//
// Exported (rather than an unexported seam living in a _test.go file)
// because a _test.go file's identifiers are only visible within that
// package's own test binary; other packages' tests (e.g.
// handlers.ChangelogHandler's tests, which exercise a real
// *changelog.Service against deterministic fixtures rather than the real
// embedded placeholder) need a cross-package seam. Same spirit as
// Service.SetCurrentVersion, which already serves double duty as both a
// test seam and the CHARON_CHANGELOG_VERSION dev-override hook.
func SetEntriesForTesting(entries []Entry) (restore func()) {
	original := allEntries
	allEntries = entries
	return func() { allEntries = original }
}

// Service resolves the "current version" used for since-comparisons via
// an injectable seam, mirroring services.UpdateService.SetCurrentVersion.
type Service struct {
	currentVersion string
}

// NewService constructs a Service using the given current version — pass
// version.Version at the call site (see routes.go), which defaults to
// "dev" for local/untagged builds.
func NewService(currentVersion string) *Service {
	return &Service{currentVersion: currentVersion}
}

// SetCurrentVersion overrides the effective current version — used by
// tests and by the CHARON_CHANGELOG_VERSION dev-only override wired in
// routes.go.
func (s *Service) SetCurrentVersion(v string) { s.currentVersion = v }

// CurrentVersion returns the effective current version.
func (s *Service) CurrentVersion() string { return s.currentVersion }

// IsDevBuild reports whether the effective current version is the
// unversioned "dev" sentinel (version.Version's default).
func (s *Service) IsDevBuild() bool { return s.currentVersion == "dev" }

// GetEntriesSince returns entries newer than lastSeen, newest-first.
// Empty lastSeen is treated as "behind everything" (all entries
// returned) — the pre-existing-user catch-up case: a user who has never
// seen any version is behind every released version, not just the
// newest one.
func (s *Service) GetEntriesSince(lastSeen string) []Entry {
	var result []Entry
	for _, e := range allEntries {
		if lastSeen == "" || semver.Compare("v"+e.Version, "v"+lastSeen) > 0 {
			result = append(result, e)
		}
	}
	sortNewestFirst(result)
	return result
}

// GetAllEntries returns the full changelog history, newest-first, as a
// copy safe for callers to mutate.
func (s *Service) GetAllEntries() []Entry {
	result := make([]Entry, len(allEntries))
	copy(result, allEntries)
	sortNewestFirst(result)
	return result
}

func sortNewestFirst(entries []Entry) {
	sort.Slice(entries, func(i, j int) bool {
		return semver.Compare("v"+entries[i].Version, "v"+entries[j].Version) > 0
	})
}
