package crowdsec

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type stubExec struct {
	responses map[string]error
	calls     []string
}

func (s *stubExec) Execute(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := strings.Join(append([]string{name}, args...), " ")
	s.calls = append(s.calls, cmd)
	for key, err := range s.responses {
		if strings.Contains(cmd, key) {
			return nil, err
		}
	}
	return []byte("ok"), nil
}

// TestPullThenApplyFlow verifies that pulling a preset and then applying it works correctly.
func TestPullThenApplyFlow(t *testing.T) {
	// Create temp directories for cache and data
	cacheDir := t.TempDir()
	dataDir := t.TempDir()

	// Create cache with 1 hour TTL
	cache, err := NewHubCache(cacheDir, time.Hour)
	require.NoError(t, err)

	// Create a test archive
	archive := makeTestArchive(t, map[string]string{
		"config.yaml":   "test: config\nvalue: 123",
		"profiles.yaml": "name: test",
	})

	// Create hub service with mock HTTP client
	hub := NewHubService(nil, cache, dataDir)
	hub.HubBaseURL = "http://test.example.com"
	hub.HTTPClient = &http.Client{
		Transport: mockTransport(func(req *http.Request) (*http.Response, error) {
			switch req.URL.String() {
			case "http://test.example.com/api/index.json":
				body := `{"items":[{"name":"test/preset","title":"Test Preset","description":"Test","etag":"etag123","download_url":"http://test.example.com/test.tgz","preview_url":"http://test.example.com/test.yaml"}]}`
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(body)),
					Header:     make(http.Header),
				}, nil
			case "http://test.example.com/test.yaml":
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader("test: preview\nkey: value")),
					Header:     make(http.Header),
				}, nil
			case "http://test.example.com/test.tgz":
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(bytes.NewReader(archive)),
					Header:     make(http.Header),
				}, nil
			default:
				return &http.Response{
					StatusCode: http.StatusNotFound,
					Body:       io.NopCloser(strings.NewReader("")),
					Header:     make(http.Header),
				}, nil
			}
		}),
	}

	ctx := context.Background()

	// Step 1: Pull the preset
	t.Log("Step 1: Pulling preset")
	pullResult, err := hub.Pull(ctx, "test/preset")
	require.NoError(t, err, "Pull should succeed")
	require.Equal(t, "test/preset", pullResult.Meta.Slug)
	require.NotEmpty(t, pullResult.Meta.CacheKey)
	require.NotEmpty(t, pullResult.Preview)

	// Verify cache files exist
	require.FileExists(t, pullResult.Meta.ArchivePath, "Archive should be cached")
	require.FileExists(t, pullResult.Meta.PreviewPath, "Preview should be cached")

	// Read the cached files to verify content
	cachedArchive, err := os.ReadFile(pullResult.Meta.ArchivePath)
	require.NoError(t, err)
	require.Equal(t, archive, cachedArchive, "Cached archive should match original")

	cachedPreview, err := os.ReadFile(pullResult.Meta.PreviewPath)
	require.NoError(t, err)
	require.Contains(t, string(cachedPreview), "preview", "Cached preview should contain expected content")

	t.Log("Step 2: Verifying cache can be loaded")
	// Verify we can load from cache
	loaded, err := cache.Load(ctx, "test/preset")
	require.NoError(t, err, "Should be able to load cached preset")
	require.Equal(t, pullResult.Meta.Slug, loaded.Slug)
	require.Equal(t, pullResult.Meta.CacheKey, loaded.CacheKey)

	t.Log("Step 3: Applying preset from cache")
	// Step 2: Apply the preset (should use cached version)
	applyResult, err := hub.Apply(ctx, "test/preset")
	require.NoError(t, err, "Apply should succeed after pull")
	require.Equal(t, "applied", applyResult.Status)
	require.NotEmpty(t, applyResult.BackupPath)
	require.Equal(t, "test/preset", applyResult.AppliedPreset)

	// Verify files were extracted to dataDir
	extractedConfig := filepath.Join(dataDir, "config.yaml")
	require.FileExists(t, extractedConfig, "Config should be extracted")
	// #nosec G304 -- Test reads from known extracted config path in test dataDir
	content, err := os.ReadFile(extractedConfig)
	require.NoError(t, err)
	require.Contains(t, string(content), "test: config")
}

func TestApplyRepullsOnCacheMissAfterCSCLIFailure(t *testing.T) {
	cacheDir := t.TempDir()
	dataDir := filepath.Join(t.TempDir(), "data")
	cache, err := NewHubCache(cacheDir, time.Hour)
	require.NoError(t, err)

	archive := makeTestArchive(t, map[string]string{"config.yaml": "test: repull"})

	exec := &stubExec{responses: map[string]error{"install": fmt.Errorf("install failed")}}
	hub := NewHubService(exec, cache, dataDir)
	hub.HubBaseURL = "http://test.example.com"
	hub.HTTPClient = &http.Client{Transport: mockTransport(func(req *http.Request) (*http.Response, error) {
		switch req.URL.String() {
		case "http://test.example.com/api/index.json":
			body := `{"items":[{"name":"test/preset","title":"Test","etag":"e1"}]}`
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
		case "http://test.example.com/test/preset.yaml":
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("preview")), Header: make(http.Header)}, nil
		case "http://test.example.com/test/preset.tgz":
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(archive)), Header: make(http.Header)}, nil
		default:
			return &http.Response{StatusCode: http.StatusNotFound, Body: io.NopCloser(strings.NewReader("")), Header: make(http.Header)}, nil
		}
	})}

	ctx := context.Background()
	res, err := hub.Apply(ctx, "test/preset")
	require.NoError(t, err)
	require.Equal(t, "applied", res.Status)
	require.False(t, res.UsedCSCLI)
	require.NotEmpty(t, res.CacheKey)

	meta, loadErr := cache.Load(ctx, "test/preset")
	require.NoError(t, loadErr)
	require.Equal(t, res.CacheKey, meta.CacheKey)
	require.FileExists(t, filepath.Join(dataDir, "config.yaml"))
}

func TestApplyRepullsOnCacheExpired(t *testing.T) {
	cacheDir := t.TempDir()
	dataDir := filepath.Join(t.TempDir(), "data")
	cache, err := NewHubCache(cacheDir, 5*time.Millisecond)
	require.NoError(t, err)

	archive := makeTestArchive(t, map[string]string{"config.yaml": "test: expired"})
	ctx := context.Background()
	_, err = cache.Store(ctx, "expired/preset", "etag-old", "hub", "old", archive)
	require.NoError(t, err)

	// wait for expiration
	time.Sleep(10 * time.Millisecond)

	hub := NewHubService(nil, cache, dataDir)
	hub.HubBaseURL = "http://test.example.com"
	hub.HTTPClient = &http.Client{Transport: mockTransport(func(req *http.Request) (*http.Response, error) {
		switch req.URL.String() {
		case "http://test.example.com/api/index.json":
			body := `{"items":[{"name":"expired/preset","title":"Expired","etag":"e2"}]}`
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
		case "http://test.example.com/expired/preset.yaml":
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("preview new")), Header: make(http.Header)}, nil
		case "http://test.example.com/expired/preset.tgz":
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(archive)), Header: make(http.Header)}, nil
		default:
			return &http.Response{StatusCode: http.StatusNotFound, Body: io.NopCloser(strings.NewReader("")), Header: make(http.Header)}, nil
		}
	})}

	res, err := hub.Apply(ctx, "expired/preset")
	require.NoError(t, err)
	require.Equal(t, "applied", res.Status)
	require.False(t, res.UsedCSCLI)

	meta, loadErr := cache.Load(ctx, "expired/preset")
	require.NoError(t, loadErr)
	require.Equal(t, "e2", meta.Etag)
	require.FileExists(t, filepath.Join(dataDir, "config.yaml"))
}

func TestPullAcceptsNamespacedIndexEntry(t *testing.T) {
	cacheDir := t.TempDir()
	dataDir := filepath.Join(t.TempDir(), "data")
	cache, err := NewHubCache(cacheDir, time.Hour)
	require.NoError(t, err)

	archive := makeTestArchive(t, map[string]string{"config.yaml": "test: namespaced"})

	hub := NewHubService(nil, cache, dataDir)
	hub.HubBaseURL = "http://test.example.com"
	hub.HTTPClient = &http.Client{Transport: mockTransport(func(req *http.Request) (*http.Response, error) {
		switch req.URL.String() {
		case "http://test.example.com/api/index.json":
			body := `{"items":[{"name":"crowdsecurity/bot-mitigation-essentials","title":"Bot Mitigation Essentials","etag":"etag-bme"}]}`
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
		case "http://test.example.com/crowdsecurity/bot-mitigation-essentials.yaml":
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("namespaced preview")), Header: make(http.Header)}, nil
		case "http://test.example.com/crowdsecurity/bot-mitigation-essentials.tgz":
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(archive)), Header: make(http.Header)}, nil
		default:
			return &http.Response{StatusCode: http.StatusNotFound, Body: io.NopCloser(strings.NewReader("")), Header: make(http.Header)}, nil
		}
	})}

	ctx := context.Background()
	res, err := hub.Pull(ctx, "bot-mitigation-essentials")
	require.NoError(t, err)
	require.Equal(t, "bot-mitigation-essentials", res.Meta.Slug)
	require.Equal(t, "etag-bme", res.Meta.Etag)
	require.Contains(t, res.Preview, "namespaced preview")
}

func TestHubFallbackToMirrorOnForbidden(t *testing.T) {
	cacheDir := t.TempDir()
	dataDir := t.TempDir()
	cache, err := NewHubCache(cacheDir, time.Hour)
	require.NoError(t, err)

	archive := makeTestArchive(t, map[string]string{"config.yaml": "mirror"})

	hub := NewHubService(nil, cache, dataDir)
	hub.HubBaseURL = "http://primary.example.com"
	hub.MirrorBaseURL = "http://mirror.example.com"
	hub.HTTPClient = &http.Client{Transport: mockTransport(func(req *http.Request) (*http.Response, error) {
		switch req.URL.String() {
		case "http://primary.example.com/api/index.json":
			return &http.Response{StatusCode: http.StatusForbidden, Body: io.NopCloser(strings.NewReader("blocked")), Header: make(http.Header)}, nil
		case "http://mirror.example.com/api/index.json":
			body := `{"items":[{"name":"fallback/preset","title":"Fallback","etag":"etag-mirror"}]}`
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
		case "http://primary.example.com/fallback/preset.yaml":
			return &http.Response{StatusCode: http.StatusForbidden, Body: io.NopCloser(strings.NewReader("blocked")), Header: make(http.Header)}, nil
		case "http://mirror.example.com/fallback/preset.yaml":
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("mirror preview")), Header: make(http.Header)}, nil
		case "http://primary.example.com/fallback/preset.tgz":
			return &http.Response{StatusCode: http.StatusForbidden, Body: io.NopCloser(strings.NewReader("blocked")), Header: make(http.Header)}, nil
		case "http://mirror.example.com/fallback/preset.tgz":
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(archive)), Header: make(http.Header)}, nil
		default:
			return &http.Response{StatusCode: http.StatusNotFound, Body: io.NopCloser(strings.NewReader("")), Header: make(http.Header)}, nil
		}
	})}

	ctx := context.Background()
	res, err := hub.Pull(ctx, "fallback/preset")
	require.NoError(t, err)
	require.Equal(t, "etag-mirror", res.Meta.Etag)
	require.Contains(t, res.Preview, "mirror preview")
}

// TestApplyWithoutPullFails verifies that applying without pulling first fails with proper error.
func TestApplyWithoutPullFails(t *testing.T) {
	cacheDir := t.TempDir()
	dataDir := t.TempDir()

	cache, err := NewHubCache(cacheDir, time.Hour)
	require.NoError(t, err)

	// Create hub service without cscli (nil executor) and empty cache
	hub := NewHubService(nil, cache, dataDir)
	hub.HubBaseURL = "http://test.example.com"
	hub.HTTPClient = &http.Client{Transport: mockTransport(func(req *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusInternalServerError, Body: io.NopCloser(strings.NewReader("")), Header: make(http.Header)}, nil
	})}

	ctx := context.Background()

	// Try to apply without pulling first
	_, err = hub.Apply(ctx, "nonexistent/preset")
	require.Error(t, err, "Apply should fail without cache and without cscli")
	require.ErrorIs(t, err, ErrCacheMiss, "Error should expose cache miss for guidance")
	require.Contains(t, err.Error(), "refresh cache", "Error should surface repull failure context")
}

// TestCacheExpiration verifies that expired cache is not used.
func TestCacheExpiration(t *testing.T) {
	cacheDir := t.TempDir()

	// Create cache with very short TTL
	cache, err := NewHubCache(cacheDir, 1*time.Millisecond)
	require.NoError(t, err)

	// Store a preset
	archive := makeTestArchive(t, map[string]string{"test.yaml": "content"})
	ctx := context.Background()
	cached, err := cache.Store(ctx, "test/preset", "etag1", "hub", "preview", archive)
	require.NoError(t, err)

	// Wait for expiration
	time.Sleep(10 * time.Millisecond)

	// Try to load - should get ErrCacheExpired
	_, err = cache.Load(ctx, "test/preset")
	require.ErrorIs(t, err, ErrCacheExpired, "Should get cache expired error")

	// Verify the cache files still exist on disk (not deleted)
	require.FileExists(t, cached.ArchivePath, "Archive file should still exist")
	require.FileExists(t, cached.PreviewPath, "Preview file should still exist")
}

// TestCacheListAfterPull verifies that pulled presets appear in cache list.
func TestCacheListAfterPull(t *testing.T) {
	cacheDir := t.TempDir()
	dataDir := t.TempDir()

	cache, err := NewHubCache(cacheDir, time.Hour)
	require.NoError(t, err)

	archive := makeTestArchive(t, map[string]string{"test.yaml": "content"})

	hub := NewHubService(nil, cache, dataDir)
	hub.HubBaseURL = "http://test.example.com"
	hub.HTTPClient = &http.Client{
		Transport: mockTransport(func(req *http.Request) (*http.Response, error) {
			switch req.URL.String() {
			case "http://test.example.com/api/index.json":
				body := `{"items":[{"name":"preset1","title":"Preset 1","etag":"e1"}]}`
				return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
			case "http://test.example.com/preset1.yaml":
				return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("preview1")), Header: make(http.Header)}, nil
			case "http://test.example.com/preset1.tgz":
				return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(archive)), Header: make(http.Header)}, nil
			default:
				return &http.Response{StatusCode: http.StatusNotFound, Body: io.NopCloser(strings.NewReader("")), Header: make(http.Header)}, nil
			}
		}),
	}

	ctx := context.Background()

	// Pull preset
	_, err = hub.Pull(ctx, "preset1")
	require.NoError(t, err)

	// List cache contents
	cached, err := cache.List(ctx)
	require.NoError(t, err)
	require.Len(t, cached, 1, "Should have one cached preset")
	require.Equal(t, "preset1", cached[0].Slug)
}

// makeTestArchive creates a test tar.gz archive.
func makeTestArchive(t *testing.T, files map[string]string) []byte {
	t.Helper()
	buf := &bytes.Buffer{}
	gw := gzip.NewWriter(buf)
	tw := tar.NewWriter(gw)

	for name, content := range files {
		hdr := &tar.Header{
			Name: name,
			Mode: 0o644,
			Size: int64(len(content)),
		}
		require.NoError(t, tw.WriteHeader(hdr))
		_, err := tw.Write([]byte(content))
		require.NoError(t, err)
	}

	require.NoError(t, tw.Close())
	require.NoError(t, gw.Close())
	return buf.Bytes()
}

// mockTransport is a mock http.RoundTripper for testing.
type mockTransport func(*http.Request) (*http.Response, error)

func (m mockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return m(req)
}

// TestApplyReadsArchiveBeforeBackup verifies the fix for the bug where Apply() would:
// 1. Load cache metadata (getting archive path inside DataDir/hub_cache)
// 2. Backup DataDir (moving the cache including the archive!)
// 3. Try to read archive from original path (FAIL - file no longer exists!)
//
// The fix reads the archive into memory BEFORE creating the backup, so the
// archive data is preserved even after the backup operation moves the files.
func TestApplyReadsArchiveBeforeBackup(t *testing.T) {
	// Create base directory structure that mirrors production:
	// baseDir/
	//   └── crowdsec/              <- DataDir (gets backed up)
	//       └── hub_cache/         <- Cache lives INSIDE DataDir (the bug!)
	//           └── test/preset/
	//               ├── bundle.tgz
	//               ├── preview.yaml
	//               └── metadata.json
	baseDir := t.TempDir()
	dataDir := filepath.Join(baseDir, "crowdsec")
	cacheDir := filepath.Join(dataDir, "hub_cache") // Cache INSIDE DataDir - this is key!

	// Create DataDir with some existing config to make backup realistic
	// #nosec G301 -- Test CrowdSec data directory needs standard Unix permissions
	require.NoError(t, os.MkdirAll(dataDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dataDir, "config.yaml"), []byte("existing: config"), 0o600))

	// Create cache inside DataDir
	cache, err := NewHubCache(cacheDir, time.Hour)
	require.NoError(t, err)

	// Create test archive
	archive := makeTestArchive(t, map[string]string{
		"config.yaml":   "test: applied_config\nvalue: 123",
		"profiles.yaml": "name: test_profile",
	})

	// Pre-populate cache (simulating a prior Pull operation)
	ctx := context.Background()
	cachedMeta, err := cache.Store(ctx, "test/preset", "etag-pre", "hub", "preview: content", archive)
	require.NoError(t, err)
	require.FileExists(t, cachedMeta.ArchivePath, "Archive should exist in cache")

	// Verify cache is inside DataDir (the scenario that triggers the bug)
	require.True(t, strings.HasPrefix(cachedMeta.ArchivePath, dataDir),
		"Cache archive must be inside DataDir for this test to be valid")

	// Create hub service WITHOUT cscli (nil executor) to force cache fallback path
	hub := NewHubService(nil, cache, dataDir)
	hub.HubBaseURL = "http://test.example.com"
	// HTTP client that fails everything - we don't want to hit network
	hub.HTTPClient = &http.Client{
		Transport: mockTransport(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusInternalServerError,
				Body:       io.NopCloser(strings.NewReader("intentionally failing")),
				Header:     make(http.Header),
			}, nil
		}),
	}

	// Apply - this SHOULD succeed because:
	// 1. Archive is read into memory BEFORE backup
	// 2. Backup moves DataDir (including cache) but we already have archive bytes
	// 3. Extract uses the in-memory archive bytes
	//
	// BEFORE THE FIX, this would fail with:
	//   "read archive: open /tmp/.../crowdsec/hub_cache/.../bundle.tgz: no such file or directory"
	result, err := hub.Apply(ctx, "test/preset")
	require.NoError(t, err, "Apply should succeed - archive must be read before backup")
	require.Equal(t, "applied", result.Status)
	require.NotEmpty(t, result.BackupPath, "Backup should have been created")
	require.False(t, result.UsedCSCLI, "Should have used cache fallback, not cscli")

	// Verify backup was created
	_, statErr := os.Stat(result.BackupPath)
	require.NoError(t, statErr, "Backup directory should exist")

	// Verify files were extracted to DataDir
	extractedConfig := filepath.Join(dataDir, "config.yaml")
	require.FileExists(t, extractedConfig, "Config should be extracted")
	// #nosec G304 -- Test reads from known extracted config path in test dataDir
	content, err := os.ReadFile(extractedConfig)
	require.NoError(t, err)
	require.Contains(t, string(content), "test: applied_config",
		"Extracted config should contain content from archive, not original")
}
