package changelog

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// latestHistoryMinor is the newest minor in the fixture history: 0.46.0.
const latestHistoryMinor = 46

// releasesFrom builds one entry per release "0.<minor>.0" for the minors in
// [from, latestHistoryMinor], so cap and ceiling tests can use realistic history.
func releasesFrom(from int) []Entry {
	entries := make([]Entry, 0, latestHistoryMinor-from+1)
	for minor := from; minor <= latestHistoryMinor; minor++ {
		v := fmt.Sprintf("0.%d.0", minor)
		entries = append(entries, Entry{Version: v, Features: []string{"release " + v}})
	}
	return entries
}

func versionsOf(entries []Entry) []string {
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		out = append(out, e.Version)
	}
	return out
}

func TestGetEntriesSince_CapAndCeiling(t *testing.T) {
	// 0.36.0 .. 0.46.0 gives 11 history entries, enough to exercise the cap.
	history := releasesFrom(36)

	tests := []struct {
		name     string
		current  string
		lastSeen string
		entries  []Entry
		want     []string
	}{
		{
			name:     "stale release user gets exactly the newest three",
			current:  "0.46.6",
			lastSeen: "0.36.1",
			entries:  append(releasesFrom(36), Entry{Version: "0.46.6"}),
			want:     []string{"0.46.6", "0.46.0", "0.45.0"},
		},
		{
			name:     "never-acked user is capped at three",
			current:  "0.46.6",
			lastSeen: "",
			entries:  history,
			want:     []string{"0.46.0", "0.45.0", "0.44.0"},
		},
		{
			name:     "fewer than three newer entries returns only those",
			current:  "0.46.6",
			lastSeen: "0.45.0",
			entries:  append(releasesFrom(45), Entry{Version: "0.46.6"}),
			want:     []string{"0.46.6", "0.46.0"},
		},
		{
			name:     "entries above the running version are excluded",
			current:  "0.45.0",
			lastSeen: "0.44.0",
			entries:  history,
			want:     []string{"0.45.0"},
		},
		{
			name:     "lastSeen with build metadata compares by core version",
			current:  "0.46.0",
			lastSeen: "0.45.0+nightly.abc1234",
			entries:  history,
			want:     []string{"0.46.0"},
		},
		{
			name:     "lastSeen equal to running returns nothing",
			current:  "0.46.0",
			lastSeen: "0.46.0",
			entries:  history,
			want:     []string{},
		},
		{
			name:     "unversioned build has no ceiling and returns all entries newer than lastSeen",
			current:  "dev",
			lastSeen: "0.44.0",
			entries:  history,
			want:     []string{"0.46.0", "0.45.0"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := NewService(tt.current)
			setEntriesForTest(t, tt.entries)

			got := svc.GetEntriesSince(tt.lastSeen)

			assert.Equal(t, tt.want, versionsOf(got))
		})
	}
}

func TestGetEntriesSince_NoResultIsEmptySliceNotNil(t *testing.T) {
	svc := NewService("0.46.0")
	setEntriesForTest(t, releasesFrom(40))

	got := svc.GetEntriesSince("0.46.0")

	require.NotNil(t, got, "empty result must marshal as [] rather than null")
	assert.Empty(t, got)
}

func TestGetAllEntries_CeilingWhenVersionedNoCapWhenUnversioned(t *testing.T) {
	tests := []struct {
		name    string
		current string
		want    []string
	}{
		{
			name:    "versioned build hides entries above running version",
			current: "0.45.0",
			want:    []string{"0.45.0", "0.44.0", "0.43.0", "0.42.0", "0.41.0", "0.40.0", "0.39.0", "0.38.0", "0.37.0", "0.36.0"},
		},
		{
			name:    "unversioned build returns everything",
			current: "dev",
			want:    []string{"0.46.0", "0.45.0", "0.44.0", "0.43.0", "0.42.0", "0.41.0", "0.40.0", "0.39.0", "0.38.0", "0.37.0", "0.36.0"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setEntriesForTest(t, releasesFrom(36))

			got := NewService(tt.current).GetAllEntries()

			assert.Equal(t, tt.want, versionsOf(got))
		})
	}
}

func TestAnchorVersion(t *testing.T) {
	tests := []struct {
		current string
		want    string
	}{
		{"0.46.5", "0.46.5"},
		{"0.46.5+nightly.abc1234", "0.46.5"},
		{"1.2.0+dev", "1.2.0"},
		{"0.47.0-rc.1", "0.47.0-rc.1"},
		{"dev", ""},
		{"", ""},
		{"nightly-a1b2c3d", ""},
		{"v1.2.3", ""},
		{"1.2", ""},
	}
	for _, tt := range tests {
		t.Run(tt.current, func(t *testing.T) {
			assert.Equal(t, tt.want, NewService(tt.current).AnchorVersion())
		})
	}
}

func TestGetEntriesSince_DropsInvalidSecuritySHA(t *testing.T) {
	tests := []struct {
		name string
		sha  string
		kept bool
	}{
		{"seven-char lowercase hex is kept", "a1b2c3d", true},
		{"forty-char lowercase hex is kept", strings.Repeat("a", 40), true},
		{"uppercase hex is dropped", "A1B2C3D", false},
		{"non-hex characters are dropped", "zzzzzzz", false},
		{"too short is dropped", "a1b2c3", false},
		{"too long is dropped", strings.Repeat("a", 41), false},
		{"empty is dropped", "", false},
		{"link injection attempt is dropped", "a1b2c3d/../../x", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setEntriesForTest(t, []Entry{{
				Version:  "0.46.0",
				Security: []SecurityEntry{{Summary: "Harden the API layer", SHA: tt.sha}},
			}})

			got := NewService("0.46.0").GetAllEntries()

			require.Len(t, got, 1)
			if tt.kept {
				assert.Equal(t, []SecurityEntry{{Summary: "Harden the API layer", SHA: tt.sha}}, got[0].Security)
			} else {
				assert.Empty(t, got[0].Security)
				assert.NotNil(t, got[0].Security, "dropped security list must still be a JSON array")
			}
		})
	}
}

func TestGetAllEntries_DedupesListsKeepingOrder(t *testing.T) {
	setEntriesForTest(t, []Entry{{
		Version:  "0.46.0",
		Features: []string{"b", "a", "b", "c", "a"},
		Fixes:    []string{"fix", "fix"},
		Other:    []string{},
		Security: []SecurityEntry{
			{Summary: "Harden", SHA: "a1b2c3d"},
			{Summary: "Harden", SHA: "a1b2c3d"},
			{Summary: "Harden", SHA: "f00ba42"},
		},
	}})

	got := NewService("0.46.0").GetAllEntries()

	require.Len(t, got, 1)
	assert.Equal(t, []string{"b", "a", "c"}, got[0].Features)
	assert.Equal(t, []string{"fix"}, got[0].Fixes)
	assert.Equal(t, []SecurityEntry{
		{Summary: "Harden", SHA: "a1b2c3d"},
		{Summary: "Harden", SHA: "f00ba42"},
	}, got[0].Security)
}
