package crowdsec

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestHubCacheStoreLoadAndExpire(t *testing.T) {
	t.Parallel()
	cacheDir := t.TempDir()
	cache, err := NewHubCache(cacheDir, time.Minute)
	require.NoError(t, err)

	ctx := context.Background()
	meta, err := cache.Store(ctx, "crowdsecurity/demo", "etag1", "hub", "preview-text", []byte("archive-bytes"))
	require.NoError(t, err)
	require.NotEmpty(t, meta.CacheKey)

	loaded, err := cache.Load(ctx, "crowdsecurity/demo")
	require.NoError(t, err)
	require.Equal(t, meta.CacheKey, loaded.CacheKey)
	require.Equal(t, "etag1", loaded.Etag)

	cache.nowFn = func() time.Time { return meta.RetrievedAt.Add(2 * time.Minute) }
	_, err = cache.Load(ctx, "crowdsecurity/demo")
	require.ErrorIs(t, err, ErrCacheExpired)
}

func TestHubCacheRejectsBadSlug(t *testing.T) {
	t.Parallel()
	cacheDir := t.TempDir()
	cache, err := NewHubCache(cacheDir, time.Hour)
	require.NoError(t, err)

	_, err = cache.Store(context.Background(), "../bad", "etag", "hub", "preview", []byte("data"))
	require.Error(t, err)

	_, err = cache.Store(context.Background(), "..\\bad", "etag", "hub", "preview", []byte("data"))
	require.Error(t, err)
}

func TestHubCacheListAndEvict(t *testing.T) {
	t.Parallel()
	cacheDir := t.TempDir()
	cache, err := NewHubCache(cacheDir, time.Hour)
	require.NoError(t, err)

	ctx := context.Background()
	_, err = cache.Store(ctx, "crowdsecurity/demo", "etag1", "hub", "preview", []byte("data1"))
	require.NoError(t, err)
	_, err = cache.Store(ctx, "crowdsecurity/other", "etag2", "hub", "preview", []byte("data2"))
	require.NoError(t, err)

	entries, err := cache.List(ctx)
	require.NoError(t, err)
	require.Len(t, entries, 2)

	require.NoError(t, cache.Evict(ctx, "crowdsecurity/demo"))
	entries, err = cache.List(ctx)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	require.Equal(t, "crowdsecurity/other", entries[0].Slug)
}

func TestHubCacheTouchUpdatesTTL(t *testing.T) {
	t.Parallel()
	cacheDir := t.TempDir()
	cache, err := NewHubCache(cacheDir, time.Minute)
	require.NoError(t, err)

	ctx := context.Background()
	meta, err := cache.Store(ctx, "crowdsecurity/demo", "etag1", "hub", "preview", []byte("data1"))
	require.NoError(t, err)

	cache.nowFn = func() time.Time { return meta.RetrievedAt.Add(30 * time.Second) }
	require.NoError(t, cache.Touch(ctx, "crowdsecurity/demo"))

	cache.nowFn = func() time.Time { return meta.RetrievedAt.Add(2 * time.Minute) }
	_, err = cache.Load(ctx, "crowdsecurity/demo")
	require.ErrorIs(t, err, ErrCacheExpired)
}

func TestHubCachePreviewExistsAndSize(t *testing.T) {
	t.Parallel()
	cacheDir := t.TempDir()
	cache, err := NewHubCache(cacheDir, time.Hour)
	require.NoError(t, err)

	ctx := context.Background()
	archive := []byte("archive-bytes-here")
	_, err = cache.Store(ctx, "crowdsecurity/demo", "etag123", "hub", "preview-content", archive)
	require.NoError(t, err)

	preview, err := cache.LoadPreview(ctx, "crowdsecurity/demo")
	require.NoError(t, err)
	require.Equal(t, "preview-content", preview)
	require.True(t, cache.Exists(ctx, "crowdsecurity/demo"))
	require.GreaterOrEqual(t, cache.Size(ctx), int64(len(archive)))
}

func TestHubCacheExistsHonorsTTL(t *testing.T) {
	t.Parallel()
	cacheDir := t.TempDir()
	cache, err := NewHubCache(cacheDir, time.Second)
	require.NoError(t, err)

	ctx := context.Background()
	meta, err := cache.Store(ctx, "crowdsecurity/demo", "etag123", "hub", "preview", []byte("data"))
	require.NoError(t, err)

	cache.nowFn = func() time.Time { return meta.RetrievedAt.Add(3 * time.Second) }
	require.False(t, cache.Exists(ctx, "crowdsecurity/demo"))
}

func TestSanitizeSlugCases(t *testing.T) {
	t.Parallel()
	require.Equal(t, "demo/preset", sanitizeSlug(" demo/preset "))
	require.Equal(t, "", sanitizeSlug("../traverse"))
	require.Equal(t, "", sanitizeSlug("/abs/path"))
	require.Equal(t, "", sanitizeSlug("\\windows\\bad"))
	require.Equal(t, "", sanitizeSlug("bad spaces %"))
}

func TestNewHubCacheRequiresBaseDir(t *testing.T) {
	t.Parallel()
	_, err := NewHubCache("", time.Hour)
	require.Error(t, err)
}

func TestHubCacheTouchMissing(t *testing.T) {
	t.Parallel()
	cache, err := NewHubCache(t.TempDir(), time.Hour)
	require.NoError(t, err)

	err = cache.Touch(context.Background(), "missing")
	require.ErrorIs(t, err, ErrCacheMiss)
}

func TestHubCacheTouchInvalidSlug(t *testing.T) {
	t.Parallel()
	cache, err := NewHubCache(t.TempDir(), time.Hour)
	require.NoError(t, err)

	err = cache.Touch(context.Background(), "../bad")
	require.Error(t, err)
}

func TestHubCacheStoreContextCanceled(t *testing.T) {
	t.Parallel()
	cache, err := NewHubCache(t.TempDir(), time.Hour)
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err = cache.Store(ctx, "demo", "etag", "hub", "preview", []byte("data"))
	require.ErrorIs(t, err, context.Canceled)
}

func TestHubCacheLoadInvalidSlug(t *testing.T) {
	t.Parallel()
	cache, err := NewHubCache(t.TempDir(), time.Hour)
	require.NoError(t, err)

	_, err = cache.Load(context.Background(), "../bad")
	require.Error(t, err)
}

func TestHubCacheLoadMetadataReadError(t *testing.T) {
	t.Parallel()

	baseDir := t.TempDir()
	cache, err := NewHubCache(baseDir, time.Hour)
	require.NoError(t, err)

	slugDir := filepath.Join(baseDir, "crowdsecurity", "demo")
	require.NoError(t, os.MkdirAll(slugDir, 0o750))
	require.NoError(t, os.Mkdir(filepath.Join(slugDir, "metadata.json"), 0o750))

	_, err = cache.Load(context.Background(), "crowdsecurity/demo")
	require.Error(t, err)
	require.False(t, errors.Is(err, ErrCacheMiss))
}

func TestHubCacheExistsContextCanceled(t *testing.T) {
	t.Parallel()
	cache, err := NewHubCache(t.TempDir(), time.Hour)
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	require.False(t, cache.Exists(ctx, "demo"))
}

func TestHubCacheListSkipsExpired(t *testing.T) {
	t.Parallel()
	cacheDir := t.TempDir()
	cache, err := NewHubCache(cacheDir, time.Second)
	require.NoError(t, err)
	ctx := context.Background()
	fixed := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	cache.nowFn = func() time.Time { return fixed }
	_, err = cache.Store(ctx, "crowdsecurity/demo", "etag", "hub", "preview", []byte("data"))
	require.NoError(t, err)

	cache.nowFn = func() time.Time { return fixed.Add(3 * time.Second) }
	entries, err := cache.List(ctx)
	require.NoError(t, err)
	require.Len(t, entries, 0)
}

func TestHubCacheEvictInvalidSlug(t *testing.T) {
	t.Parallel()
	cache, err := NewHubCache(t.TempDir(), time.Hour)
	require.NoError(t, err)
	err = cache.Evict(context.Background(), "../bad")
	require.Error(t, err)
}

func TestHubCacheListContextCanceled(t *testing.T) {
	t.Parallel()
	cache, err := NewHubCache(t.TempDir(), time.Hour)
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = cache.List(ctx)
	require.ErrorIs(t, err, context.Canceled)
}

// ============================================
// TTL Tests
// ============================================

func TestHubCacheTTL(t *testing.T) {
	t.Parallel()
	t.Run("returns configured TTL", func(t *testing.T) {
		t.Parallel()
		cache, err := NewHubCache(t.TempDir(), 2*time.Hour)
		require.NoError(t, err)
		require.Equal(t, 2*time.Hour, cache.TTL())
	})

	t.Run("returns minute TTL", func(t *testing.T) {
		t.Parallel()
		cache, err := NewHubCache(t.TempDir(), time.Minute)
		require.NoError(t, err)
		require.Equal(t, time.Minute, cache.TTL())
	})

	t.Run("returns zero TTL if configured", func(t *testing.T) {
		t.Parallel()
		cache, err := NewHubCache(t.TempDir(), 0)
		require.NoError(t, err)
		require.Equal(t, time.Duration(0), cache.TTL())
	})
}
