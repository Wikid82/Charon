package bouncerkey

import (
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSourceMemoryName(t *testing.T) {
	t.Parallel()

	require.Equal(t, Source("memory"), SourceMemory)
}

func TestHeldEmptyWhenNothingHeld(t *testing.T) {
	t.Parallel()

	require.Empty(t, Held(filepath.Join(t.TempDir(), "bouncer_key")))
}

func TestHoldThenHeldThenRelease(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "bouncer_key")

	Hold(path, testRegistered)
	require.Equal(t, testRegistered, Held(path))

	Release(path)
	require.Empty(t, Held(path))
}

func TestHoldReplacesPreviousKey(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "bouncer_key")

	Hold(path, testEnvValid)
	Hold(path, testRegistered)

	require.Equal(t, testRegistered, Held(path))
}

// TestHeldIsKeyedByPath checks that a key held for one path is invisible to another.
func TestHeldIsKeyedByPath(t *testing.T) {
	t.Parallel()

	first := filepath.Join(t.TempDir(), "bouncer_key")
	second := filepath.Join(t.TempDir(), "bouncer_key")

	Hold(first, testRegistered)
	t.Cleanup(func() { Release(first) })

	require.Equal(t, testRegistered, Held(first))
	require.Empty(t, Held(second))

	Release(second)
	require.Equal(t, testRegistered, Held(first), "releasing another path must not drop this key")
}

// TestHoldReleaseConcurrent runs Hold, Held and Release from many goroutines on
// the same and on different paths. It must pass under -race and never return a
// value that was not held.
func TestHoldReleaseConcurrent(t *testing.T) {
	t.Parallel()

	const workers = 16
	const rounds = 200
	dir := t.TempDir()
	shared := filepath.Join(dir, "shared_key")

	var wg sync.WaitGroup
	for w := range workers {
		own := filepath.Join(dir, fmt.Sprintf("own_%d", w))
		want := fmt.Sprintf("concurrent-key-%02d-0000000000000000", w)
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range rounds {
				Hold(shared, want)
				// Other workers hold the shared path too, so the value is any worker's key or
				// empty (released); it must never be a value nobody wrote.
				if got := Held(shared); got != "" && !strings.HasPrefix(got, "concurrent-key-") {
					t.Errorf("shared path returned a foreign value %q", got)
				}
				Hold(own, want)
				if got := Held(own); got != want {
					t.Errorf("own path returned %q, want %q", got, want)
				}
				Release(own)
				if got := Held(own); got != "" {
					t.Errorf("own path still holds %q after Release", got)
				}
				Release(shared)
			}
		}()
	}
	wg.Wait()

	require.Empty(t, Held(shared))
}

func TestPathForOverrideWins(t *testing.T) {
	t.Setenv(PathEnvName, "  /custom/dir/../bouncer_key  ")

	require.Equal(t, "/custom/bouncer_key", PathFor("/app/data/crowdsec"))
}

func TestPathForOverrideWithoutDataDir(t *testing.T) {
	t.Setenv(PathEnvName, "/custom/bouncer_key")

	require.Equal(t, "/custom/bouncer_key", PathFor(""))
}

func TestPathForBlankOverrideFallsBackToDataDir(t *testing.T) {
	t.Setenv(PathEnvName, "   ")

	require.Equal(t, "/app/data/crowdsec/bouncer_key", PathFor("/app/data/crowdsec"))
}

func TestPathForDataDir(t *testing.T) {
	t.Setenv(PathEnvName, "")

	tests := []struct {
		name    string
		dataDir string
		want    string
	}{
		{name: "absolute dir", dataDir: "/app/data/crowdsec", want: "/app/data/crowdsec/bouncer_key"},
		{name: "trailing slash", dataDir: "/srv/cs/", want: "/srv/cs/bouncer_key"},
		{name: "relative dir", dataDir: "data/crowdsec", want: "data/crowdsec/bouncer_key"},
		{name: "relative dir with trailing slash", dataDir: "data/crowdsec/", want: "data/crowdsec/bouncer_key"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, filepath.FromSlash(tt.want), PathFor(tt.dataDir))
		})
	}
}

func TestPathForNoDataDirUsesDefault(t *testing.T) {
	t.Setenv(PathEnvName, "")

	require.Equal(t, DefaultFile, PathFor(""))
}
