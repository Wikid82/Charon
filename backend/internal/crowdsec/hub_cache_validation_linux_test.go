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

// specialFileMode is a file type bit that is neither a regular file nor a directory, used with mknod.
const specialFileMode = 0o010000

func TestLoadRejectsUnexpectedFileType(t *testing.T) {
	cache, base := newValidationCache(t)
	hubcachetest.Write(t, base, hubcachetest.Entry{Slug: cacheTestSlug, Item: cacheTestItem})
	item := filepath.Join(entryDir(base, cacheTestSlug), "item.yaml")
	require.NoError(t, os.Remove(item))
	require.NoError(t, syscall.Mknod(item, specialFileMode|0o600, 0))

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
