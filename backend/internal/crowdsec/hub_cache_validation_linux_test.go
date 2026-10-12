//go:build linux

package crowdsec

import (
	"context"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/crowdsec/hubcachetest"
)

// namedPipeMode is the file type bit for a named pipe, used with mknod.
const namedPipeMode = 0o010000

func TestLoadRejectsUnexpectedFileTypeNamedPipe(t *testing.T) {
	t.Skip("enabled by a later commit in this series")
	cache, base := newValidationCache(t)
	hubcachetest.Write(t, base, hubcachetest.Entry{Slug: cacheTestSlug, Item: cacheTestItem})
	item := filepath.Join(entryDir(base, cacheTestSlug), "item.yaml")
	require.NoError(t, os.Remove(item))
	require.NoError(t, syscall.Mknod(item, namedPipeMode|0o600, 0))

	done := make(chan error, 1)
	go func() {
		_, err := cache.Load(context.Background(), cacheTestSlug)
		done <- err
	}()
	select {
	case err := <-done:
		require.Error(t, err)
		require.NoDirExists(t, entryDir(base, cacheTestSlug))
	case <-time.After(5 * time.Second):
		t.Fatal("loading an entry with an unexpected file type blocked")
	}
}
