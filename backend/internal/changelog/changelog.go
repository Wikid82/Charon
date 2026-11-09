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
	"regexp"
	"sort"

	"github.com/Wikid82/charon/backend/internal/version"
)

//go:embed data/changelog.json
var rawData []byte

// firstViewEntryLimit caps every status response at the newest N entries,
// whatever the user's last-seen version is. Without the cap, a user seeded
// long ago (or never acked) would be shown the whole history on login. The
// full history stays available through GetAllEntries (Settings browse mode).
const firstViewEntryLimit = 3

// commitSHAPattern is the only shape accepted for a security entry's commit
// link. Anything else is dropped so the frontend never builds a link from
// unvalidated text.
var commitSHAPattern = regexp.MustCompile(`^[0-9a-f]{7,40}$`)

// Entry describes one released version's categorized, novice-friendly
// changelog content.
type Entry struct {
	Version  string          `json:"version"`
	Date     string          `json:"date"`
	Features []string        `json:"features"`
	Fixes    []string        `json:"fixes"`
	Other    []string        `json:"other"`
	Security []SecurityEntry `json:"security"`
}

// SecurityEntry describes one security-relevant changelog item. Unlike
// the plain-string Features/Fixes/Other categories, security entries
// carry a commit SHA so the frontend can link to the underlying commit
// for users who want to dig deeper — see
// docs/superpowers/specs/2026-07-29-security-changelog-category-design.md.
// Summary must be a vague-by-default description of the category of
// issue and its mitigation (enforced by commit-writing convention, not
// this type) — never the specific vulnerability class or attack vector.
type SecurityEntry struct {
	Summary string `json:"summary"`
	SHA     string `json:"sha"`
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

// IsDevBuild reports whether the effective current version is unsuitable
// as a changelog "since" comparison anchor: the unversioned "dev"
// sentinel (version.Version's default), or any other non-SemVer string
// (including the legacy nightly-<sha> format). Such values can come from
// distributable builds that are non-"dev" but non-semver. Seeding a user with one of those would
// leave them with an incomparable LastSeenVersion that could never be
// dismissed. The check itself lives in version.IsUnversioned, the single
// shared authority; callers (the /changelog/status handler) must gate
// show_changelog on this, not just on currentVersion == "dev".
func (s *Service) IsDevBuild() bool {
	return version.IsUnversioned(s.currentVersion)
}

// AnchorVersion returns the bare SemVer form of the current version, the
// value dismiss_permanent stores in LastSeenVersion. It returns "" for an
// unversioned build, which must never be written as an anchor.
func (s *Service) AnchorVersion() string {
	anchor, _ := version.Canonical(s.currentVersion)
	return anchor
}

// withinCeiling reports whether an entry may be shown at all. A versioned
// build never shows entries newer than itself, so an upgrade-then-downgrade
// or a pre-release can't surface notes for code it doesn't run. An
// unversioned build (local UI iteration) has no ceiling.
func (s *Service) withinCeiling(entryVersion string) bool {
	if s.IsDevBuild() {
		return true
	}
	cmp, ok := version.Compare(entryVersion, s.currentVersion)
	return ok && cmp <= 0
}

// GetEntriesSince returns the entries a user has not yet seen, newest-first,
// capped at firstViewEntryLimit whatever lastSeen holds.
//
// Empty lastSeen is treated as "behind everything" (all entries are
// candidates), the pre-existing-user catch-up case. The cap then keeps the
// first view short, and the rest of the history is reachable through
// GetAllEntries.
//
// A non-empty lastSeen that is not valid semver is treated as "already
// seen everything" (no entries returned). This is a defense-in-depth check:
// an unversioned comparison would otherwise make every real entry look newer
// forever, and dismiss_permanent could never fix it for a non-semver build.
// Entries above the running version are excluded (see withinCeiling).
func (s *Service) GetEntriesSince(lastSeen string) []Entry {
	if lastSeen != "" && version.IsUnversioned(lastSeen) {
		return []Entry{}
	}
	result := []Entry{}
	for _, e := range allEntries {
		if !s.withinCeiling(e.Version) {
			continue
		}
		if lastSeen != "" {
			cmp, ok := version.Compare(e.Version, lastSeen)
			if !ok || cmp <= 0 {
				continue
			}
		}
		result = append(result, normalizeEntry(e))
	}
	sortNewestFirst(result)
	if len(result) > firstViewEntryLimit {
		result = result[:firstViewEntryLimit]
	}
	return result
}

// GetAllEntries returns the full changelog history, newest-first, as a
// copy safe for callers to mutate. It applies the version ceiling when the
// build is versioned, and it is never capped: this is the opt-in browse view.
func (s *Service) GetAllEntries() []Entry {
	result := []Entry{}
	for _, e := range allEntries {
		if s.withinCeiling(e.Version) {
			result = append(result, normalizeEntry(e))
		}
	}
	sortNewestFirst(result)
	return result
}

// normalizeEntry returns a copy of e that is safe to serve: every array field
// is non-nil, each list is deduplicated while keeping its order (defense in
// depth against a duplicated React key, F18), and a security entry whose SHA
// is not a plain hex commit id is dropped (F17).
//
// The embedded data/changelog.json is always generator-produced
// (scripts/generate-changelog.sh's jq program builds these via `map`,
// which yields `[]` rather than omitting the key even for zero matches),
// so in practice the real release pipeline never triggers this. But
// data/changelog.json's committed placeholder is deliberately
// hand-editable for local "What's New" UI iteration (see this file's
// package doc and the SetEntriesForTesting doc comment), and a
// hand-edited entry that omits an array key entirely leaves the
// corresponding Go field at its json.Unmarshal zero value: nil. A nil
// slice marshals to JSON `null`, but every one of these fields is typed
// on the frontend as always being an array (e.g. WhatsNewModal.tsx does
// `entry.security.length`) — returning `null` for any of them is an API
// contract violation that crashes the client. This is the single choke
// point both GetEntriesSince and GetAllEntries pass every Entry through
// before returning, regardless of whether it originated from the
// embedded init() parse or a test's SetEntriesForTesting override.
func normalizeEntry(e Entry) Entry {
	return Entry{
		Version:  e.Version,
		Date:     e.Date,
		Features: dedupeKeepOrder(e.Features),
		Fixes:    dedupeKeepOrder(e.Fixes),
		Other:    dedupeKeepOrder(e.Other),
		Security: validSecurityEntries(e.Security),
	}
}

// dedupeKeepOrder returns a new non-nil slice with the first occurrence of
// each value, in its original order.
func dedupeKeepOrder[T comparable](in []T) []T {
	out := make([]T, 0, len(in))
	seen := make(map[T]struct{}, len(in))
	for _, v := range in {
		if _, dup := seen[v]; dup {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}

// validSecurityEntries drops security entries whose SHA is not a plain hex
// commit id, then dedupes the rest. The result is never nil.
func validSecurityEntries(in []SecurityEntry) []SecurityEntry {
	valid := make([]SecurityEntry, 0, len(in))
	for _, s := range in {
		if commitSHAPattern.MatchString(s.SHA) {
			valid = append(valid, s)
		}
	}
	return dedupeKeepOrder(valid)
}

func sortNewestFirst(entries []Entry) {
	sort.Slice(entries, func(i, j int) bool {
		cmp, ok := version.Compare(entries[i].Version, entries[j].Version)
		return ok && cmp > 0
	})
}
