package changelog

import (
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
		{Version: "2.0.0", Date: "2026-03-01", Features: []string{"Big feature"}},
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

func TestGetEntriesSince_NoEntries_ReturnsEmpty(t *testing.T) {
	svc := NewService("1.0.0")
	setEntriesForTest(t, nil)

	assert.Empty(t, svc.GetEntriesSince(""))
	assert.Empty(t, svc.GetAllEntries())
}
