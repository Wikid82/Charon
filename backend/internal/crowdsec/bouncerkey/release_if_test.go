package bouncerkey

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestReleaseIfDropsMatchingHold checks that a compare-and-release clears a hold that still holds the key.
func TestReleaseIfDropsMatchingHold(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "bouncer_key")
	Hold(path, testRegistered)

	releaseIf(path, testRegistered)

	require.Empty(t, Held(path))
}

// TestReleaseIfKeepsHoldOfAnotherKey checks that releasing a key that was read earlier does not drop a
// different key held for the same path in the meantime.
func TestReleaseIfKeepsHoldOfAnotherKey(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "bouncer_key")
	Hold(path, testHeldInvalid)
	// Another goroutine holds a different key between the read of testHeldInvalid and the release.
	Hold(path, testRegistered)

	releaseIf(path, testHeldInvalid)

	require.Equal(t, testRegistered, Held(path))
	t.Cleanup(func() { Release(path) })
}

// TestEnsureKeepsHoldSetDuringValidation drives Ensure so that a different key is held while Ensure
// validates the key it read. Ensure must not drop the new hold when it releases its own key.
func TestEnsureKeepsHoldSetDuringValidation(t *testing.T) {
	clearEnv(t)

	cases := []struct {
		name    string
		verdict Verdict
	}{
		{name: "rejected held key", verdict: VerdictRejected},
		{name: "accepted held key", verdict: VerdictAccepted},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := seedKeyFile(t, testSavedValid)
			Hold(path, testHeldInvalid)
			t.Cleanup(func() { Release(path) })

			fake := newFakeLAPI()
			validate := func(ctx context.Context, key string) Verdict {
				if key == testHeldInvalid {
					// A concurrent failed save holds a different key while the held key is checked.
					Hold(path, testRegistered)
					return tc.verdict
				}
				return fake.validate(ctx, key)
			}

			_, err := Ensure(context.Background(), path, validate, fake.register, fake.save)
			require.NoError(t, err)
			require.Equal(t, testRegistered, Held(path))
		})
	}
}

// TestResolveKeepsHoldSetAfterFileRead runs Resolve against a file that matches the held key while another
// goroutine holds a different key for the same path. The race detector must stay quiet, and the last hold
// must survive once the workers finish.
func TestResolveKeepsHoldSetAfterFileRead(t *testing.T) {
	t.Parallel()

	const rounds = 200
	path := seedKeyFile(t, testSavedValid)
	t.Cleanup(func() { Release(path) })

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for range rounds {
			Hold(path, testSavedValid)
			_, _ = Resolve(path)
		}
	}()
	go func() {
		defer wg.Done()
		for i := range rounds {
			Hold(path, fmt.Sprintf("concurrent-held-key-%03d-0000000", i))
		}
	}()
	wg.Wait()

	Hold(path, testRegistered)
	key, src := Resolve(path)
	require.Equal(t, SourceMemory, src)
	require.Equal(t, testRegistered, key)
}
