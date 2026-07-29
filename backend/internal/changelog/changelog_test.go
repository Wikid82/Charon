package changelog

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setEntriesForTest replaces the package-level parsed data for the
// duration of a test, restoring the original value on cleanup. Thin
// wrapper around the exported SetEntriesForTesting (needed so
// cross-package tests, e.g. handlers.ChangelogHandler's, have a seam
// too — a _test.go file's helpers aren't visible outside this package).
func setEntriesForTest(t *testing.T, entries []Entry) {
	t.Helper()
	t.Cleanup(SetEntriesForTesting(entries))
}

// fixtureEntries returns a deterministic, unordered set of entries used to
// exercise GetEntriesSince/GetAllEntries independent of the real embedded
// (placeholder) data.
func fixtureEntries() []Entry {
	return []Entry{
		{Version: "1.0.0", Date: "2026-01-01", Features: []string{"Initial release"}},
		{
			Version:  "2.0.0",
			Date:     "2026-03-01",
			Features: []string{"Big feature"},
			Security: []SecurityEntry{
				{Summary: "Harden input validation in the API layer", SHA: "a1b2c3d"},
			},
		},
		{Version: "1.1.0", Date: "2026-02-01", Fixes: []string{"Bug fix"}},
	}
}

func TestGetEntriesSince_ReturnsNewerOnly(t *testing.T) {
	svc := NewService("2.0.0")
	setEntriesForTest(t, fixtureEntries())

	result := svc.GetEntriesSince("1.0.0")

	require.Len(t, result, 2)
	assert.Equal(t, "2.0.0", result[0].Version, "expected newest-first ordering")
	assert.Equal(t, "1.1.0", result[1].Version)
	require.Len(t, result[0].Security, 1, "security entries must survive semver filtering unchanged")
	assert.Equal(t, "Harden input validation in the API layer", result[0].Security[0].Summary)
	assert.Equal(t, "a1b2c3d", result[0].Security[0].SHA)
	assert.Empty(t, result[1].Security, "an entry with no security items must yield an empty (not nil-panicking) slice")
}

func TestGetEntriesSince_EmptyLastSeen_ReturnsAll(t *testing.T) {
	svc := NewService("2.0.0")
	setEntriesForTest(t, fixtureEntries())

	result := svc.GetEntriesSince("")

	require.Len(t, result, 3, "empty lastSeen must be treated as behind everything")
	assert.Equal(t, "2.0.0", result[0].Version)
	assert.Equal(t, "1.1.0", result[1].Version)
	assert.Equal(t, "1.0.0", result[2].Version)
}

func TestGetEntriesSince_EqualVersion_ReturnsEmpty(t *testing.T) {
	svc := NewService("2.0.0")
	setEntriesForTest(t, fixtureEntries())

	result := svc.GetEntriesSince("2.0.0")

	assert.Empty(t, result, "the newest entry's own version must not be treated as 'since'")
}

func TestGetAllEntries_ReturnsEverythingNewestFirst(t *testing.T) {
	svc := NewService("2.0.0")
	setEntriesForTest(t, fixtureEntries())

	result := svc.GetAllEntries()

	require.Len(t, result, 3)
	assert.Equal(t, "2.0.0", result[0].Version)
	assert.Equal(t, "1.1.0", result[1].Version)
	assert.Equal(t, "1.0.0", result[2].Version)
}

func TestGetAllEntries_DoesNotMutatePackageState(t *testing.T) {
	svc := NewService("2.0.0")
	setEntriesForTest(t, fixtureEntries())

	result := svc.GetAllEntries()
	result[0].Version = "mutated"

	again := svc.GetAllEntries()
	assert.Equal(t, "2.0.0", again[0].Version, "GetAllEntries must return a copy, not the shared slice")
}

func TestIsDevBuild_TrueForDevSentinel(t *testing.T) {
	svc := NewService("dev")
	assert.True(t, svc.IsDevBuild())
}

func TestIsDevBuild_FalseForRealVersion(t *testing.T) {
	svc := NewService("1.2.3")
	assert.False(t, svc.IsDevBuild())
}

func TestIsDevBuild_FalseForRealPrereleaseVersion(t *testing.T) {
	svc := NewService("1.5.0-beta.1")
	assert.False(t, svc.IsDevBuild())
}

// TestIsDevBuild_TrueForNonSemverVersion covers the CI-produced version
// strings that aren't literally "dev" but also aren't valid semver:
// nightly-build.yml and docker-build.yml tag distributable images with
// VERSION=nightly-<git-sha> or branch-derived docker/metadata-action
// values. semver.Compare treats any invalid string as always "less than"
// a valid one, so without this guard a user seeded with one of these as
// LastSeenVersion would see the entire changelog history on every login,
// forever (see GetEntriesSince_InvalidLastSeen_ReturnsEmpty for the
// second half of this defense).
func TestIsDevBuild_TrueForNonSemverVersion(t *testing.T) {
	for _, v := range []string{"nightly-a1b2c3d", "main", "development", "branch-feat-foo-a1b2c3d", ""} {
		svc := NewService(v)
		assert.Truef(t, svc.IsDevBuild(), "expected IsDevBuild()==true for non-semver version %q", v)
	}
}

func TestNewService_DefaultsCurrentVersion(t *testing.T) {
	svc := NewService("3.4.5")
	assert.Equal(t, "3.4.5", svc.CurrentVersion())
}

func TestSetCurrentVersion_Overrides(t *testing.T) {
	svc := NewService("1.0.0")
	svc.SetCurrentVersion("9.9.9")
	assert.Equal(t, "9.9.9", svc.CurrentVersion())
	assert.False(t, svc.IsDevBuild())
}

// TestGetEntriesSince_InvalidLastSeen_ReturnsEmpty is the service-level
// (belt-and-suspenders) half of the non-semver-version defense. A stored
// LastSeenVersion is invalid semver only if it was written while running
// a non-semver build (nightly-<sha>, branch tags, etc.) — the
// handler-level IsDevBuild() gate prevents that same build from ever
// calling GetEntriesSince, but if the app is later upgraded to a real
// semver release, that user's stale invalid LastSeenVersion must not be
// treated as "behind everything" (that's reserved for lastSeen == "",
// the distinct pre-existing-user catch-up case) — otherwise
// semver.Compare's "invalid < valid always" rule would make every real
// entry look newer forever, and dismiss_permanent could never anchor a
// comparable version.
func TestGetEntriesSince_InvalidLastSeen_ReturnsEmpty(t *testing.T) {
	svc := NewService("2.0.0")
	setEntriesForTest(t, fixtureEntries())

	result := svc.GetEntriesSince("nightly-a1b2c3d")

	assert.Empty(t, result, "invalid non-empty lastSeen must not be treated as behind everything")
}

func TestGetEntriesSince_NoEntries_ReturnsEmpty(t *testing.T) {
	svc := NewService("1.0.0")
	setEntriesForTest(t, nil)

	assert.Empty(t, svc.GetEntriesSince(""))
	assert.Empty(t, svc.GetAllEntries())
}

func TestGetAllEntries_PreservesSecurityField(t *testing.T) {
	svc := NewService("2.0.0")
	setEntriesForTest(t, fixtureEntries())

	result := svc.GetAllEntries()

	require.Len(t, result, 3)
	assert.Equal(t, "2.0.0", result[0].Version)
	require.Len(t, result[0].Security, 1, "security entries must survive GetAllEntries unchanged")
	assert.Equal(t, SecurityEntry{Summary: "Harden input validation in the API layer", SHA: "a1b2c3d"}, result[0].Security[0])
}

// TestSecurityEntry_JSONTags locks in the schema documented in
// docs/superpowers/specs/2026-07-29-security-changelog-category-design.md:
// {"summary": "...", "sha": "..."}. The frontend builds a GitHub commit
// link directly from these field names, so drift here is a silent break.
func TestSecurityEntry_JSONTags(t *testing.T) {
	entry := SecurityEntry{Summary: "Harden input validation in the API layer", SHA: "a1b2c3d"}

	data, err := json.Marshal(entry)
	require.NoError(t, err)
	assert.JSONEq(t, `{"summary":"Harden input validation in the API layer","sha":"a1b2c3d"}`, string(data))

	var roundTripped SecurityEntry
	require.NoError(t, json.Unmarshal(data, &roundTripped))
	assert.Equal(t, entry, roundTripped)
}

// TestEntry_JSONMarshaling_IncludesSecurityArray verifies the Entry-level
// "security" key marshals as an array of the SecurityEntry object shape
// (not a plain string array like features/fixes/other), matching the
// design's schema example.
func TestEntry_JSONMarshaling_IncludesSecurityArray(t *testing.T) {
	entry := Entry{
		Version:  "1.6.0",
		Date:     "2026-08-01",
		Features: []string{},
		Fixes:    []string{},
		Other:    []string{},
		Security: []SecurityEntry{
			{Summary: "Harden input validation in the API layer", SHA: "a1b2c3d"},
		},
	}

	data, err := json.Marshal(entry)
	require.NoError(t, err)
	assert.JSONEq(t, `{
		"version": "1.6.0",
		"date": "2026-08-01",
		"features": [],
		"fixes": [],
		"other": [],
		"security": [
			{"summary": "Harden input validation in the API layer", "sha": "a1b2c3d"}
		]
	}`, string(data))
}

// TestEntry_JSONUnmarshaling_ParsesSecurityArray verifies an embedded
// changelog.json blob shaped like the generator's real output (see
// scripts/generate-changelog.sh) round-trips into the Security field
// correctly, and that an entry with no security-scoped commits parses to
// an empty (not nil-panicking) slice.
func TestEntry_JSONUnmarshaling_ParsesSecurityArray(t *testing.T) {
	raw := `[
		{
			"version": "1.6.0",
			"date": "2026-08-01",
			"features": ["Add widget support"],
			"fixes": [],
			"other": [],
			"security": [
				{"summary": "Harden input validation in the API layer", "sha": "a1b2c3d"},
				{"summary": "Tighten session cookie flags", "sha": "9dbe946"}
			]
		},
		{
			"version": "1.5.0",
			"date": "2026-07-01",
			"features": [],
			"fixes": ["Fix pagination bug"],
			"other": [],
			"security": []
		}
	]`

	var entries []Entry
	require.NoError(t, json.Unmarshal([]byte(raw), &entries))
	require.Len(t, entries, 2)

	require.Len(t, entries[0].Security, 2)
	assert.Equal(t, "Harden input validation in the API layer", entries[0].Security[0].Summary)
	assert.Equal(t, "a1b2c3d", entries[0].Security[0].SHA)
	assert.Equal(t, "Tighten session cookie flags", entries[0].Security[1].Summary)
	assert.Equal(t, "9dbe946", entries[0].Security[1].SHA)

	assert.Empty(t, entries[1].Security, "an entry with no security-scoped commits must parse to an empty slice")
}
