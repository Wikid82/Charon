package crowdsec

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type roundTripperFunc func(*http.Request) (*http.Response, error)

type recordingExec struct {
	outputs map[string][]byte
	errors  map[string]error
	calls   []string
}

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func (r *recordingExec) Execute(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := name + " " + strings.Join(args, " ")
	r.calls = append(r.calls, cmd)
	if err, ok := r.errors[cmd]; ok {
		return nil, err
	}
	if out, ok := r.outputs[cmd]; ok {
		return out, nil
	}
	return nil, fmt.Errorf("unexpected command: %s", cmd)
}

func newResponse(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}
}

func makeTarGz(t *testing.T, files map[string]string) []byte {
	t.Helper()
	buf := &bytes.Buffer{}
	gw := gzip.NewWriter(buf)
	tw := tar.NewWriter(gw)
	// Sort keys for deterministic order in archive
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		content := files[name]
		hdr := &tar.Header{Name: name, Mode: 0o644, Size: int64(len(content))}
		require.NoError(t, tw.WriteHeader(hdr))
		_, err := tw.Write([]byte(content))
		require.NoError(t, err)
	}
	require.NoError(t, tw.Close())
	require.NoError(t, gw.Close())
	return buf.Bytes()
}

func readFixture(t *testing.T, name string) string {
	t.Helper()
	// #nosec G304 -- Test reads from testdata directory with known fixture names
	data, err := os.ReadFile(filepath.Join("testdata", name))
	require.NoError(t, err)
	return string(data)
}

func TestFetchIndexPrefersCSCLI(t *testing.T) {
	t.Parallel()
	exec := &recordingExec{outputs: map[string][]byte{"cscli hub list -o json": []byte(`{"collections":[{"name":"crowdsecurity/test","description":"desc","version":"1.0"}]}`)}}
	svc := NewHubService(exec, nil, t.TempDir())
	svc.HTTPClient = nil

	idx, err := svc.FetchIndex(context.Background())
	require.NoError(t, err)
	require.Len(t, idx.Items, 1)
	require.Equal(t, "crowdsecurity/test", idx.Items[0].Name)
	require.Contains(t, exec.calls, "cscli hub list -o json")
}

func TestFetchIndexFallbackHTTP(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping network I/O test in short mode")
	}
	t.Parallel()
	exec := &recordingExec{errors: map[string]error{"cscli hub list -o json": fmt.Errorf("boom")}}
	cacheDir := t.TempDir()
	svc := NewHubService(exec, nil, cacheDir)
	svc.HubBaseURL = "http://example.com"
	indexBody := readFixture(t, "hub_index.json")
	svc.HTTPClient = &http.Client{Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.String() == "http://example.com"+defaultHubIndexPath {
			resp := newResponse(http.StatusOK, indexBody)
			resp.Header.Set("Content-Type", "application/json")
			return resp, nil
		}
		return newResponse(http.StatusNotFound, ""), nil
	})}

	idx, err := svc.FetchIndex(context.Background())
	require.NoError(t, err)
	require.Len(t, idx.Items, 1)
	require.Equal(t, "crowdsecurity/demo", idx.Items[0].Name)
}

func TestFetchIndexHTTPRejectsRedirect(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping network I/O test in short mode")
	}
	t.Parallel()
	svc := NewHubService(nil, nil, t.TempDir())
	svc.HubBaseURL = "http://hub.example"
	svc.HTTPClient = &http.Client{Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		resp := newResponse(http.StatusMovedPermanently, "")
		resp.Header.Set("Location", "https://hub.crowdsec.net/")
		return resp, nil
	})}

	_, err := svc.fetchIndexHTTP(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "redirect")
}

func TestFetchIndexHTTPRejectsHTML(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping network I/O test in short mode")
	}
	t.Parallel()
	svc := NewHubService(nil, nil, t.TempDir())
	htmlBody := readFixture(t, "hub_index_html.html")
	svc.HTTPClient = &http.Client{Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		resp := newResponse(http.StatusOK, htmlBody)
		resp.Header.Set("Content-Type", "text/html")
		return resp, nil
	})}

	_, err := svc.fetchIndexHTTP(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "HTML")
}

func TestFetchIndexHTTPFallsBackToDefaultHub(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping network I/O test in short mode")
	}
	t.Parallel()
	svc := NewHubService(nil, nil, t.TempDir())
	svc.HubBaseURL = "https://hub.crowdsec.net"
	calls := make([]string, 0)

	indexBody := `{"items":[{"name":"crowdsecurity/demo","title":"Demo","type":"collection"}]}`
	svc.HTTPClient = &http.Client{Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		calls = append(calls, req.URL.String())
		switch req.URL.String() {
		case "https://hub.crowdsec.net/api/index.json":
			resp := newResponse(http.StatusMovedPermanently, "")
			resp.Header.Set("Location", "https://hub-data.crowdsec.net/api/index.json")
			return resp, nil
		case "https://hub-data.crowdsec.net/api/index.json":
			resp := newResponse(http.StatusOK, indexBody)
			resp.Header.Set("Content-Type", "application/json")
			return resp, nil
		default:
			return newResponse(http.StatusNotFound, ""), nil
		}
	})}

	idx, err := svc.fetchIndexHTTP(context.Background())
	require.NoError(t, err)
	require.Len(t, idx.Items, 1)
	require.Equal(t, "crowdsecurity/demo", idx.Items[0].Name)
	require.Equal(t, []string{"https://hub.crowdsec.net/api/index.json", "https://hub-data.crowdsec.net/api/index.json"}, calls)
}

func TestFetchIndexFallsBackToMirrorOnForbidden(t *testing.T) {
	t.Parallel()
	svc := NewHubService(nil, nil, t.TempDir())
	svc.HubBaseURL = "https://hub-data.crowdsec.net"
	svc.MirrorBaseURL = defaultHubMirrorBaseURL

	calls := make([]string, 0)
	indexBody := `{"items":[{"name":"crowdsecurity/demo","title":"Demo","type":"collection"}]}`
	svc.HTTPClient = &http.Client{Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		calls = append(calls, req.URL.String())
		switch req.URL.String() {
		case "https://hub-data.crowdsec.net/api/index.json":
			return newResponse(http.StatusForbidden, ""), nil
		case defaultHubMirrorBaseURL + "/.index.json":
			resp := newResponse(http.StatusOK, indexBody)
			resp.Header.Set("Content-Type", "application/json")
			return resp, nil
		default:
			return newResponse(http.StatusNotFound, ""), nil
		}
	})}

	idx, err := svc.FetchIndex(context.Background())
	require.NoError(t, err)
	require.Len(t, idx.Items, 1)
	require.Contains(t, calls, defaultHubMirrorBaseURL+"/.index.json")
}

func TestPullCachesPreview(t *testing.T) {
	t.Parallel()
	cacheDir := t.TempDir()
	dataDir := filepath.Join(t.TempDir(), "crowdsec")
	cache, err := NewHubCache(cacheDir, time.Hour)
	require.NoError(t, err)

	archiveBytes := makeTarGz(t, map[string]string{"config.yaml": "value: 1"})

	svc := NewHubService(nil, cache, dataDir)
	svc.HubBaseURL = "http://example.com"
	svc.HTTPClient = &http.Client{Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		switch req.URL.String() {
		case "http://example.com" + defaultHubIndexPath:
			return newResponse(http.StatusOK, `{"items":[{"name":"crowdsecurity/demo","title":"Demo","description":"desc","type":"collection","etag":"etag1","download_url":"http://example.com/demo.tgz","preview_url":"http://example.com/demo.yaml"}]}`), nil
		case "http://example.com/demo.yaml":
			return newResponse(http.StatusOK, "preview-body"), nil
		case "http://example.com/demo.tgz":
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(archiveBytes)), Header: make(http.Header)}, nil
		default:
			return newResponse(http.StatusNotFound, ""), nil
		}
	})}

	res, err := svc.Pull(context.Background(), "crowdsecurity/demo")
	require.NoError(t, err)
	require.Equal(t, "preview-body", res.Preview)
	require.NotEmpty(t, res.Meta.CacheKey)
	require.FileExists(t, res.Meta.ArchivePath)
}

func TestApplyUsesCacheWhenCSCLIFails(t *testing.T) {
	t.Parallel()
	cache, err := NewHubCache(t.TempDir(), time.Hour)
	require.NoError(t, err)
	dataDir := filepath.Join(t.TempDir(), "data")

	archive := makeTarGz(t, map[string]string{"a/b.yaml": "ok: 1"})
	_, err = cache.Store(context.Background(), "crowdsecurity/demo", "etag1", "hub", "preview", archive)
	require.NoError(t, err)

	exec := &recordingExec{outputs: map[string][]byte{"cscli version": []byte("v"), "cscli hub update": []byte("ok")}, errors: map[string]error{"cscli hub install crowdsecurity/demo": fmt.Errorf("install failed")}}
	svc := NewHubService(exec, cache, dataDir)

	res, err := svc.Apply(context.Background(), "crowdsecurity/demo")
	require.NoError(t, err)
	require.False(t, res.UsedCSCLI)
	require.Equal(t, "applied", res.Status)
	require.FileExists(t, filepath.Join(dataDir, "a", "b.yaml"))
}

func TestApplyRollsBackOnBadArchive(t *testing.T) {
	t.Parallel()
	cache, err := NewHubCache(t.TempDir(), time.Hour)
	require.NoError(t, err)
	baseDir := filepath.Join(t.TempDir(), "data")
	// #nosec G301 -- Test data directory needs standard Unix permissions
	require.NoError(t, os.MkdirAll(baseDir, 0o755))
	keep := filepath.Join(baseDir, "keep.txt")
	require.NoError(t, os.WriteFile(keep, []byte("before"), 0o600))

	badArchive := makeTarGz(t, map[string]string{"../evil.txt": "boom"})
	_, err = cache.Store(context.Background(), "crowdsecurity/demo", "etag1", "hub", "preview", badArchive)
	require.NoError(t, err)

	svc := NewHubService(nil, cache, baseDir)
	_, err = svc.Apply(context.Background(), "crowdsecurity/demo")
	require.Error(t, err)

	// #nosec G304 -- Reading test fixture file with known path
	content, readErr := os.ReadFile(keep)
	require.NoError(t, readErr)
	require.Equal(t, "before", string(content))
}

func TestApplyUsesCacheWhenCscliMissing(t *testing.T) {
	t.Parallel()
	cache, err := NewHubCache(t.TempDir(), time.Hour)
	require.NoError(t, err)
	dataDir := filepath.Join(t.TempDir(), "data")

	archive := makeTarGz(t, map[string]string{"config.yml": "hello: world"})
	_, err = cache.Store(context.Background(), "crowdsecurity/demo", "etag1", "hub", "preview", archive)
	require.NoError(t, err)

	svc := NewHubService(nil, cache, dataDir)
	res, err := svc.Apply(context.Background(), "crowdsecurity/demo")
	require.NoError(t, err)
	require.False(t, res.UsedCSCLI)
	require.FileExists(t, filepath.Join(dataDir, "config.yml"))
}

func TestPullReturnsCachedPreviewWithoutNetwork(t *testing.T) {
	t.Parallel()
	cache, err := NewHubCache(t.TempDir(), time.Hour)
	require.NoError(t, err)

	archive := makeTarGz(t, map[string]string{"demo.yaml": "x: 1"})
	_, err = cache.Store(context.Background(), "crowdsecurity/demo", "etag1", "hub", "cached-preview", archive)
	require.NoError(t, err)

	svc := NewHubService(nil, cache, t.TempDir())
	svc.HTTPClient = nil

	res, err := svc.Pull(context.Background(), "crowdsecurity/demo")
	require.NoError(t, err)
	require.Equal(t, "cached-preview", res.Preview)
}

func TestPullEvictsExpiredCacheAndRefreshes(t *testing.T) {
	t.Parallel()
	cache, err := NewHubCache(t.TempDir(), time.Second)
	require.NoError(t, err)

	fixed := time.Now().Add(-2 * time.Second)
	cache.nowFn = func() time.Time { return fixed }
	archive := makeTarGz(t, map[string]string{"a.yaml": "v: 1"})
	initial, err := cache.Store(context.Background(), "crowdsecurity/demo", "etag1", "hub", "old", archive)
	require.NoError(t, err)

	cache.nowFn = func() time.Time { return fixed.Add(3 * time.Second) }
	svc := NewHubService(nil, cache, t.TempDir())
	svc.HubBaseURL = "http://example.com"
	svc.HTTPClient = &http.Client{Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		switch req.URL.String() {
		case "http://example.com" + defaultHubIndexPath:
			return newResponse(http.StatusOK, `{"items":[{"name":"crowdsecurity/demo","title":"Demo","description":"desc","type":"collection","etag":"etag2","download_url":"http://example.com/demo.tgz","preview_url":"http://example.com/demo.yaml"}]}`), nil
		case "http://example.com/demo.yaml":
			return newResponse(http.StatusOK, "fresh-preview"), nil
		case "http://example.com/demo.tgz":
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(archive)), Header: make(http.Header)}, nil
		default:
			return newResponse(http.StatusNotFound, ""), nil
		}
	})}

	res, err := svc.Pull(context.Background(), "crowdsecurity/demo")
	require.NoError(t, err)
	require.NotEqual(t, initial.CacheKey, res.Meta.CacheKey)
	require.Equal(t, "fresh-preview", res.Preview)
}

func TestPullFallsBackToArchivePreview(t *testing.T) {
	t.Parallel()
	cache, err := NewHubCache(t.TempDir(), time.Hour)
	require.NoError(t, err)
	archive := makeTarGz(t, map[string]string{"scenarios/demo.yaml": "title: demo"})

	svc := NewHubService(nil, cache, t.TempDir())
	svc.HubBaseURL = "http://example.com"
	svc.HTTPClient = &http.Client{Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.String() == "http://example.com"+defaultHubIndexPath {
			return newResponse(http.StatusOK, `{"items":[{"name":"crowdsecurity/demo","title":"Demo","etag":"etag1","download_url":"http://example.com/demo.tgz","preview_url":"http://example.com/demo.yaml"}]}`), nil
		}
		if req.URL.String() == "http://example.com/demo.yaml" {
			return newResponse(http.StatusInternalServerError, ""), nil
		}
		if req.URL.String() == "http://example.com/demo.tgz" {
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(archive)), Header: make(http.Header)}, nil
		}
		return newResponse(http.StatusNotFound, ""), nil
	})}

	res, err := svc.Pull(context.Background(), "crowdsecurity/demo")
	require.NoError(t, err)
	require.Contains(t, res.Preview, "title: demo")
}

func TestPullFallsBackToMirrorArchiveOnForbidden(t *testing.T) {
	t.Parallel()
	cache, err := NewHubCache(t.TempDir(), time.Hour)
	require.NoError(t, err)
	dataDir := filepath.Join(t.TempDir(), "crowdsec")

	archiveBytes := makeTarGz(t, map[string]string{"config.yml": "foo: bar"})
	svc := NewHubService(nil, cache, dataDir)
	svc.HubBaseURL = "https://primary.example"
	svc.MirrorBaseURL = defaultHubMirrorBaseURL

	calls := make([]string, 0)
	indexBody := `{"items":[{"name":"crowdsecurity/demo","title":"Demo","etag":"etag1","type":"collection"}]}`
	svc.HTTPClient = &http.Client{Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		calls = append(calls, req.URL.String())
		switch req.URL.String() {
		case "https://primary.example/api/index.json":
			resp := newResponse(http.StatusOK, indexBody)
			resp.Header.Set("Content-Type", "application/json")
			return resp, nil
		case "https://primary.example/crowdsecurity/demo.tgz":
			return newResponse(http.StatusForbidden, ""), nil
		case "https://primary.example/crowdsecurity/demo.yaml":
			return newResponse(http.StatusForbidden, ""), nil
		case defaultHubMirrorBaseURL + "/.index.json":
			resp := newResponse(http.StatusOK, indexBody)
			resp.Header.Set("Content-Type", "application/json")
			return resp, nil
		case defaultHubMirrorBaseURL + "/crowdsecurity/demo.tgz":
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(archiveBytes)), Header: make(http.Header)}, nil
		case defaultHubMirrorBaseURL + "/crowdsecurity/demo.yaml":
			return newResponse(http.StatusOK, "mirror-preview"), nil
		case defaultHubBaseURL + "/api/index.json":
			return newResponse(http.StatusInternalServerError, ""), nil
		default:
			return newResponse(http.StatusNotFound, ""), nil
		}
	})}

	res, err := svc.Pull(context.Background(), "crowdsecurity/demo")
	require.NoError(t, err)
	require.Contains(t, calls, defaultHubMirrorBaseURL+"/crowdsecurity/demo.tgz")
	require.Equal(t, "mirror-preview", res.Preview)
	require.FileExists(t, res.Meta.ArchivePath)
}

func TestFetchWithLimitRejectsLargePayload(t *testing.T) {
	t.Parallel()
	svc := NewHubService(nil, nil, t.TempDir())
	big := bytes.Repeat([]byte("a"), int(maxArchiveSize+10))
	svc.HTTPClient = &http.Client{Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(big)), Header: make(http.Header)}, nil
	})}

	_, err := svc.fetchWithLimitFromURL(context.Background(), "http://example.com/large.tgz")
	require.Error(t, err)
	require.Contains(t, err.Error(), "payload too large")
}

func makeSymlinkTar(t *testing.T, linkName string) []byte {
	t.Helper()
	buf := &bytes.Buffer{}
	gw := gzip.NewWriter(buf)
	tw := tar.NewWriter(gw)
	hdr := &tar.Header{Name: linkName, Mode: 0o777, Typeflag: tar.TypeSymlink, Linkname: "target"}
	require.NoError(t, tw.WriteHeader(hdr))
	require.NoError(t, tw.Close())
	require.NoError(t, gw.Close())
	return buf.Bytes()
}

func TestExtractTarGzRejectsSymlink(t *testing.T) {
	t.Parallel()
	svc := NewHubService(nil, nil, t.TempDir())
	archive := makeSymlinkTar(t, "bad.symlink")

	err := svc.extractTarGz(context.Background(), archive, filepath.Join(t.TempDir(), "data"))
	require.Error(t, err)
	require.Contains(t, err.Error(), "symlinks not allowed")
}

func TestExtractTarGzRejectsAbsolutePath(t *testing.T) {
	t.Parallel()
	svc := NewHubService(nil, nil, t.TempDir())

	buf := &bytes.Buffer{}
	gw := gzip.NewWriter(buf)
	tw := tar.NewWriter(gw)
	hdr := &tar.Header{Name: "/etc/passwd", Mode: 0o644, Size: int64(len("x"))}
	require.NoError(t, tw.WriteHeader(hdr))
	_, err := tw.Write([]byte("x"))
	require.NoError(t, err)
	require.NoError(t, tw.Close())
	require.NoError(t, gw.Close())

	err = svc.extractTarGz(context.Background(), buf.Bytes(), filepath.Join(t.TempDir(), "data"))
	require.Error(t, err)
	require.Contains(t, err.Error(), "unsafe path")
}

func TestFetchIndexHTTPError(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping network I/O test in short mode")
	}
	t.Parallel()
	svc := NewHubService(nil, nil, t.TempDir())
	svc.HTTPClient = &http.Client{Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		return newResponse(http.StatusServiceUnavailable, ""), nil
	})}

	_, err := svc.fetchIndexHTTP(context.Background())
	require.Error(t, err)
}

func TestPullValidatesSlugAndMissingPreset(t *testing.T) {
	t.Parallel()
	svc := NewHubService(nil, nil, t.TempDir())

	_, err := svc.Pull(context.Background(), " ")
	require.Error(t, err)

	cache, cacheErr := NewHubCache(t.TempDir(), time.Hour)
	require.NoError(t, cacheErr)
	svc.Cache = cache
	svc.HubBaseURL = "http://hub.example"
	svc.HTTPClient = &http.Client{Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		return newResponse(http.StatusOK, `{"items":[{"name":"crowdsecurity/other","title":"Other","description":"d","type":"collection"}]}`), nil
	})}

	_, err = svc.Pull(context.Background(), "crowdsecurity/missing")
	require.Error(t, err)
}

func TestFetchPreviewRequiresURL(t *testing.T) {
	t.Parallel()
	svc := NewHubService(nil, nil, t.TempDir())
	_, err := svc.fetchPreview(context.Background(), nil)
	require.Error(t, err)
}

func TestFetchWithLimitRequiresClient(t *testing.T) {
	t.Parallel()
	svc := NewHubService(nil, nil, t.TempDir())
	svc.HTTPClient = nil
	_, err := svc.fetchWithLimitFromURL(context.Background(), "http://example.com/demo.tgz")
	require.Error(t, err)
}

func TestRunCSCLIRejectsUnsafeSlug(t *testing.T) {
	t.Parallel()
	exec := &recordingExec{}
	svc := NewHubService(exec, nil, t.TempDir())

	err := svc.runCSCLI(context.Background(), "../bad")
	require.Error(t, err)
}

func TestApplyUsesCSCLISuccess(t *testing.T) {
	t.Parallel()
	cache, err := NewHubCache(t.TempDir(), time.Hour)
	require.NoError(t, err)
	_, err = cache.Store(context.Background(), "crowdsecurity/demo", "etag1", "hub", "preview", makeTarGz(t, map[string]string{"config.yml": "val: 1"}))
	require.NoError(t, err)

	exec := &recordingExec{outputs: map[string][]byte{
		"cscli version":                        []byte("v1"),
		"cscli hub update":                     []byte("ok"),
		"cscli hub install crowdsecurity/demo": []byte("installed"),
	}}

	svc := NewHubService(exec, cache, t.TempDir())
	res, applyErr := svc.Apply(context.Background(), "crowdsecurity/demo")
	require.NoError(t, applyErr)
	require.True(t, res.UsedCSCLI)
	require.Equal(t, "applied", res.Status)
}

func TestFetchIndexCSCLIParseError(t *testing.T) {
	t.Parallel()
	exec := &recordingExec{outputs: map[string][]byte{"cscli hub list -o json": []byte("not-json")}}
	svc := NewHubService(exec, nil, t.TempDir())
	svc.HubBaseURL = "http://hub.example"
	svc.HTTPClient = &http.Client{Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		return newResponse(http.StatusInternalServerError, ""), nil
	})}

	_, err := svc.FetchIndex(context.Background())
	require.Error(t, err)
}

func TestFetchWithLimitStatusError(t *testing.T) {
	t.Parallel()
	svc := NewHubService(nil, nil, t.TempDir())
	svc.HubBaseURL = "http://hub.example"
	svc.HTTPClient = &http.Client{Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		return newResponse(http.StatusNotFound, ""), nil
	})}

	_, err := svc.fetchWithLimitFromURL(context.Background(), "http://hub.example/demo.tgz")
	require.Error(t, err)
}

func TestApplyRollsBackWhenCacheMissing(t *testing.T) {
	t.Parallel()
	baseDir := t.TempDir()
	dataDir := filepath.Join(baseDir, "crowdsec")
	// #nosec G301 -- Test fixture directory with standard permissions
	require.NoError(t, os.MkdirAll(dataDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dataDir, "keep.txt"), []byte("before"), 0o600))

	svc := NewHubService(nil, nil, dataDir)
	res, err := svc.Apply(context.Background(), "crowdsecurity/demo")
	require.Error(t, err)
	require.Contains(t, err.Error(), "cache unavailable")
	require.NotEmpty(t, res.BackupPath)
	require.Equal(t, "failed", res.Status)

	content, readErr := os.ReadFile(filepath.Join(dataDir, "keep.txt")) //nolint:gosec // G304: Test file in temp directory
	require.NoError(t, readErr)
	require.Equal(t, "before", string(content))
}

func TestNormalizeHubBaseURL(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"empty uses default", "", defaultHubBaseURL},
		{"whitespace uses default", "   ", defaultHubBaseURL},
		{"removes trailing slash", "https://hub.crowdsec.net/", "https://hub.crowdsec.net"},
		{"removes multiple trailing slashes", "https://hub.crowdsec.net///", "https://hub.crowdsec.net"},
		{"trims spaces", "  https://hub.crowdsec.net  ", "https://hub.crowdsec.net"},
		{"no slash unchanged", "https://hub.crowdsec.net", "https://hub.crowdsec.net"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := normalizeHubBaseURL(tt.input)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestBuildIndexURL(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		base string
		want string
	}{
		{"empty base uses default", "", defaultHubBaseURL + defaultHubIndexPath},
		{"standard base appends path", "https://hub.crowdsec.net", "https://hub.crowdsec.net" + defaultHubIndexPath},
		{"trailing slash removed", "https://hub.crowdsec.net/", "https://hub.crowdsec.net" + defaultHubIndexPath},
		{"direct json url unchanged", "https://custom.hub/index.json", "https://custom.hub/index.json"},
		{"case insensitive json", "https://custom.hub/INDEX.JSON", "https://custom.hub/INDEX.JSON"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := buildIndexURL(tt.base)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestUniqueStrings(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		input []string
		want  []string
	}{
		{"empty slice", []string{}, []string{}},
		{"no duplicates", []string{"a", "b", "c"}, []string{"a", "b", "c"}},
		{"with duplicates", []string{"a", "b", "a", "c", "b"}, []string{"a", "b", "c"}},
		{"all duplicates", []string{"x", "x", "x"}, []string{"x"}},
		{"preserves order", []string{"z", "a", "m", "a"}, []string{"z", "a", "m"}},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := uniqueStrings(tt.input)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestFirstNonEmpty(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		values []string
		want   string
	}{
		{"first non-empty", []string{"", "second", "third"}, "second"},
		{"all empty", []string{"", "", ""}, ""},
		{"first is non-empty", []string{"first", "second"}, "first"},
		{"whitespace treated as empty", []string{"  ", "second"}, "second"},
		{"whitespace with content", []string{"  hello  ", "second"}, "  hello  "},
		{"empty slice", []string{}, ""},
		{"tabs and newlines", []string{"\t\n", "third"}, "third"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := firstNonEmpty(tt.values...)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestCleanShellArg(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		input string
		safe  bool
	}{
		{"clean slug", "crowdsecurity/demo", true},
		{"with dash", "crowdsecurity/demo-v1", true},
		{"with underscore", "crowdsecurity/demo_parser", true},
		{"with dot", "crowdsecurity/demo.yaml", true},
		{"path traversal", "../etc/passwd", false},
		{"absolute path", "/etc/passwd", false},
		{"backslash converted", "bad\\path", true},
		{"colon not allowed", "demo:1.0", false},
		{"semicolon", "foo;rm -rf", false},
		{"pipe", "foo|bar", false},
		{"ampersand", "foo&bar", false},
		{"backtick", "foo`cmd`", false},
		{"dollar", "foo$var", false},
		{"parenthesis", "foo()", false},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := cleanShellArg(tt.input)
			if tt.safe {
				require.NotEmpty(t, got, "safe input should not be empty")
				// Note: backslashes are converted to forward slashes by filepath.Clean
			} else {
				require.Empty(t, got, "unsafe input should return empty string")
			}
		})
	}
}

func TestHasCSCLI(t *testing.T) {
	t.Parallel()
	t.Run("cscli available", func(t *testing.T) {
		t.Parallel()
		exec := &recordingExec{outputs: map[string][]byte{"cscli version": []byte("v1.5.0")}}
		svc := NewHubService(exec, nil, t.TempDir())
		got := svc.hasCSCLI(context.Background())
		require.True(t, got)
	})

	t.Run("cscli not found", func(t *testing.T) {
		t.Parallel()
		exec := &recordingExec{errors: map[string]error{"cscli version": fmt.Errorf("executable not found")}}
		svc := NewHubService(exec, nil, t.TempDir())
		got := svc.hasCSCLI(context.Background())
		require.False(t, got)
	})
}

func TestFindPreviewFileFromArchive(t *testing.T) {
	t.Parallel()
	svc := NewHubService(nil, nil, t.TempDir())

	t.Run("finds yaml in archive", func(t *testing.T) {
		t.Parallel()
		archive := makeTarGz(t, map[string]string{
			"scenarios/test.yaml": "name: test-scenario\ndescription: test",
		})
		preview := svc.findPreviewFile(archive)
		require.Contains(t, preview, "test-scenario")
	})

	t.Run("returns empty for no yaml", func(t *testing.T) {
		t.Parallel()
		archive := makeTarGz(t, map[string]string{
			"readme.txt": "no yaml here",
		})
		preview := svc.findPreviewFile(archive)
		require.Empty(t, preview)
	})

	t.Run("returns empty for invalid archive", func(t *testing.T) {
		t.Parallel()
		preview := svc.findPreviewFile([]byte("not a gzip archive"))
		require.Empty(t, preview)
	})
}

func TestApplyWithCopyBasedBackup(t *testing.T) {
	t.Parallel()
	cache, err := NewHubCache(t.TempDir(), time.Hour)
	require.NoError(t, err)

	dataDir := filepath.Join(t.TempDir(), "data")
	require.NoError(t, os.MkdirAll(dataDir, 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(dataDir, "existing.txt"), []byte("old data"), 0o600))

	// Create subdirectory with files
	subDir := filepath.Join(dataDir, "subdir")
	require.NoError(t, os.MkdirAll(subDir, 0o750))
	// #nosec G306 -- Test fixture file in subdirectory
	require.NoError(t, os.WriteFile(filepath.Join(subDir, "nested.txt"), []byte("nested"), 0o644))

	archive := makeTarGz(t, map[string]string{"new/config.yaml": "new: config"})
	_, err = cache.Store(context.Background(), "test/preset", "etag1", "hub", "preview", archive)
	require.NoError(t, err)

	svc := NewHubService(nil, cache, dataDir)

	res, err := svc.Apply(context.Background(), "test/preset")
	require.NoError(t, err)
	require.Equal(t, "applied", res.Status)
	require.NotEmpty(t, res.BackupPath)

	// Verify backup was created with copy-based approach
	require.FileExists(t, filepath.Join(res.BackupPath, "existing.txt"))
	require.FileExists(t, filepath.Join(res.BackupPath, "subdir", "nested.txt"))
	// Verify new config was applied
	require.FileExists(t, filepath.Join(dataDir, "new", "config.yaml"))
}

func TestIndexURLCandidates_GitHubMirror(t *testing.T) {
	t.Parallel()

	candidates := indexURLCandidates("https://raw.githubusercontent.com/crowdsecurity/hub/master")
	require.Len(t, candidates, 2)
	require.Contains(t, candidates, "https://raw.githubusercontent.com/crowdsecurity/hub/master/.index.json")
	require.Contains(t, candidates, "https://raw.githubusercontent.com/crowdsecurity/hub/master/api/index.json")
}

func TestBuildResourceURLs_DeduplicatesExplicitAndBases(t *testing.T) {
	t.Parallel()

	urls := buildResourceURLs("https://hub.example/preset.tgz", "crowdsecurity/demo", "/%s.tgz", []string{"https://hub.example", "https://hub.example"})
	require.NotEmpty(t, urls)
	require.Equal(t, "https://hub.example/preset.tgz", urls[0])
	require.Len(t, urls, 2)
}

func TestHubHTTPErrorMethods(t *testing.T) {
	t.Parallel()

	inner := errors.New("inner")
	err := hubHTTPError{url: "https://hub.example", statusCode: 404, inner: inner, fallback: true}

	require.Contains(t, err.Error(), "https://hub.example")
	require.ErrorIs(t, err, inner)
	require.True(t, err.CanFallback())
}

func TestBackupExistingHandlesDeviceBusy(t *testing.T) {
	t.Parallel()
	dataDir := filepath.Join(t.TempDir(), "data")
	require.NoError(t, os.MkdirAll(dataDir, 0o750))
	// #nosec G306 -- Test fixture file used for copy-based backup verification
	require.NoError(t, os.WriteFile(filepath.Join(dataDir, "file.txt"), []byte("content"), 0o644))

	svc := NewHubService(nil, nil, dataDir)
	backupPath := dataDir + ".backup.test"

	// Even if rename fails, copy-based backup should work
	err := svc.backupExisting(backupPath)
	require.NoError(t, err)
	require.FileExists(t, filepath.Join(backupPath, "file.txt"))
}

func TestCopyFile(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	srcFile := filepath.Join(tmpDir, "source.txt")
	dstFile := filepath.Join(tmpDir, "dest.txt")

	// Create source file
	content := []byte("test file content")
	// #nosec G306 -- Test fixture source file for copyFile test
	require.NoError(t, os.WriteFile(srcFile, content, 0o644))

	// Test successful copy
	err := copyFile(srcFile, dstFile)
	require.NoError(t, err)
	require.FileExists(t, dstFile)

	// Verify content
	dstContent, err := os.ReadFile(dstFile) //nolint:gosec // G304: Test file in temp directory
	require.NoError(t, err)
	require.Equal(t, content, dstContent)

	// Test copy non-existent file
	err = copyFile(filepath.Join(tmpDir, "nonexistent.txt"), dstFile)
	require.Error(t, err)
	require.Contains(t, err.Error(), "open src")

	// Test copy to invalid destination
	err = copyFile(srcFile, filepath.Join(tmpDir, "nonexistent", "dest.txt"))
	require.Error(t, err)
	require.Contains(t, err.Error(), "create dst")
}

func TestCopyDir(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	srcDir := filepath.Join(tmpDir, "source")
	dstDir := filepath.Join(tmpDir, "dest")

	// Create source directory structure
	require.NoError(t, os.MkdirAll(filepath.Join(srcDir, "subdir"), 0o750)) // #nosec G301 -- test fixture
	require.NoError(t, os.WriteFile(filepath.Join(srcDir, "file1.txt"), []byte("file1"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(srcDir, "subdir", "file2.txt"), []byte("file2"), 0o600))

	// Create destination directory
	require.NoError(t, os.MkdirAll(dstDir, 0o750)) // #nosec G301 -- test fixture

	// Test successful copy
	err := copyDir(srcDir, dstDir)
	require.NoError(t, err)

	// Verify files were copied
	require.FileExists(t, filepath.Join(dstDir, "file1.txt"))
	require.FileExists(t, filepath.Join(dstDir, "subdir", "file2.txt"))

	// Verify content
	content1, err := os.ReadFile(filepath.Join(dstDir, "file1.txt")) //nolint:gosec // G304: Test file in temp directory
	require.NoError(t, err)
	require.Equal(t, []byte("file1"), content1)

	content2, err := os.ReadFile(filepath.Join(dstDir, "subdir", "file2.txt")) //nolint:gosec // G304: Test file in temp directory
	require.NoError(t, err)
	require.Equal(t, []byte("file2"), content2)

	// Test copy non-existent directory
	err = copyDir(filepath.Join(tmpDir, "nonexistent"), dstDir)
	require.Error(t, err)
	require.Contains(t, err.Error(), "stat src")

	// Test copy file as directory (should fail)
	fileNotDir := filepath.Join(tmpDir, "file.txt")
	require.NoError(t, os.WriteFile(fileNotDir, []byte("test"), 0o600))
	err = copyDir(fileNotDir, dstDir)
	require.Error(t, err)
	require.Contains(t, err.Error(), "not a directory")
}

func TestFetchIndexHTTPAcceptsTextPlain(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping network I/O test in short mode")
	}
	t.Parallel()
	svc := NewHubService(nil, nil, t.TempDir())
	indexBody := `{"items":[{"name":"crowdsecurity/demo","title":"Demo","type":"collection"}]}`
	svc.HTTPClient = &http.Client{Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		resp := newResponse(http.StatusOK, indexBody)
		resp.Header.Set("Content-Type", "text/plain; charset=utf-8")
		return resp, nil
	})}

	idx, err := svc.fetchIndexHTTP(context.Background())
	require.NoError(t, err)
	require.Len(t, idx.Items, 1)
	require.Equal(t, "crowdsecurity/demo", idx.Items[0].Name)
}

// ============================================
// Phase 2.1: SSRF Validation & Hub Sync Tests
// ============================================

func TestValidateHubURL_ValidHTTPSProduction(t *testing.T) {
	t.Parallel()
	validURLs := []string{
		"https://hub-data.crowdsec.net/api/index.json",
		"https://hub.crowdsec.net/api/index.json",
		"https://raw.githubusercontent.com/crowdsecurity/hub/master/.index.json",
	}

	for _, url := range validURLs {
		t.Run(url, func(t *testing.T) {
			t.Parallel()
			err := validateHubURL(url)
			require.NoError(t, err, "Expected valid production hub URL to pass validation")
		})
	}
}

func TestValidateHubURL_InvalidSchemes(t *testing.T) {
	t.Parallel()
	invalidSchemes := []string{
		"ftp://hub.crowdsec.net/index.json",
		"file:///etc/passwd",
		"gopher://attacker.com",
		"data:text/html,<script>alert('xss')</script>",
	}

	for _, url := range invalidSchemes {
		t.Run(url, func(t *testing.T) {
			t.Parallel()
			err := validateHubURL(url)
			require.Error(t, err, "Expected invalid scheme to be rejected")
			require.Contains(t, err.Error(), "unsupported scheme")
		})
	}
}

func TestValidateHubURL_LocalhostExceptions(t *testing.T) {
	t.Parallel()
	localhostURLs := []string{
		"http://localhost:8080/index.json",
		"http://127.0.0.1:8080/index.json",
		"http://[::1]:8080/index.json",
		"http://test.hub/api/index.json",
		"http://example.com/api/index.json",
		"http://test.example.com/api/index.json",
		"http://server.local/api/index.json",
	}

	for _, url := range localhostURLs {
		t.Run(url, func(t *testing.T) {
			t.Parallel()
			err := validateHubURL(url)
			require.NoError(t, err, "Expected localhost/test domain to be allowed")
		})
	}
}

func TestValidateHubURL_UnknownDomainRejection(t *testing.T) {
	t.Parallel()
	unknownURLs := []string{
		"https://evil.com/index.json",
		"https://attacker.net/hub/index.json",
		"https://hub.evil.com/index.json",
	}

	for _, url := range unknownURLs {
		t.Run(url, func(t *testing.T) {
			t.Parallel()
			err := validateHubURL(url)
			require.Error(t, err, "Expected unknown domain to be rejected")
			require.Contains(t, err.Error(), "unknown hub domain")
		})
	}
}

func TestValidateHubURL_HTTPRejectedForProduction(t *testing.T) {
	t.Parallel()
	httpURLs := []string{
		"http://hub-data.crowdsec.net/api/index.json",
		"http://hub.crowdsec.net/api/index.json",
		"http://raw.githubusercontent.com/crowdsecurity/hub/master/.index.json",
	}

	for _, url := range httpURLs {
		t.Run(url, func(t *testing.T) {
			t.Parallel()
			err := validateHubURL(url)
			require.Error(t, err, "Expected HTTP to be rejected for production domains")
			require.Contains(t, err.Error(), "must use HTTPS")
		})
	}
}

func TestBuildResourceURLs(t *testing.T) {
	t.Parallel()
	t.Run("with explicit URL", func(t *testing.T) {
		t.Parallel()
		urls := buildResourceURLs("https://explicit.com/file.tgz", "demo/slug", "/%s.tgz", []string{"https://base1.com", "https://base2.com"})
		require.Contains(t, urls, "https://explicit.com/file.tgz")
		require.Contains(t, urls, "https://base1.com/demo/slug.tgz")
		require.Contains(t, urls, "https://base2.com/demo/slug.tgz")
	})

	t.Run("without explicit URL", func(t *testing.T) {
		t.Parallel()
		urls := buildResourceURLs("", "demo/preset", "/%s.yaml", []string{"https://hub1.com", "https://hub2.com"})
		require.Len(t, urls, 2)
		require.Contains(t, urls, "https://hub1.com/demo/preset.yaml")
		require.Contains(t, urls, "https://hub2.com/demo/preset.yaml")
	})

	t.Run("removes duplicates", func(t *testing.T) {
		t.Parallel()
		urls := buildResourceURLs("", "test", "/%s.tgz", []string{"https://hub.com", "https://hub.com", "https://mirror.com"})
		require.Len(t, urls, 2)
	})

	t.Run("handles empty bases", func(t *testing.T) {
		t.Parallel()
		urls := buildResourceURLs("", "test", "/%s.tgz", []string{"", "https://hub.com", ""})
		require.Len(t, urls, 1)
		require.Equal(t, "https://hub.com/test.tgz", urls[0])
	})
}

func TestParseRawIndex(t *testing.T) {
	t.Parallel()
	t.Run("parses valid raw index", func(t *testing.T) {
		t.Parallel()
		rawJSON := `{
			"collections": {
				"crowdsecurity/demo": {
					"path": "collections/crowdsecurity/demo.tgz",
					"version": "1.0",
					"description": "Demo collection"
				}
			},
			"scenarios": {
				"crowdsecurity/test-scenario": {
					"path": "scenarios/crowdsecurity/test-scenario.yaml",
					"version": "2.0",
					"description": "Test scenario"
				}
			}
		}`

		idx, err := parseRawIndex([]byte(rawJSON), "https://hub.example.com/api/index.json")
		require.NoError(t, err)
		require.Len(t, idx.Items, 2)

		// Verify collection entry
		var demoFound bool
		for _, item := range idx.Items {
			if item.Name != "crowdsecurity/demo" {
				continue
			}
			demoFound = true
			require.Equal(t, "collections", item.Type)
			require.Equal(t, "1.0", item.Version)
			require.Equal(t, "Demo collection", item.Description)
			require.Contains(t, item.DownloadURL, "collections/crowdsecurity/demo.tgz")
		}
		require.True(t, demoFound)
	})

	t.Run("returns error on invalid JSON", func(t *testing.T) {
		t.Parallel()
		_, err := parseRawIndex([]byte("not json"), "https://hub.example.com")
		require.Error(t, err)
		require.Contains(t, err.Error(), "parse raw index")
	})

	t.Run("returns error on empty index", func(t *testing.T) {
		t.Parallel()
		_, err := parseRawIndex([]byte("{}"), "https://hub.example.com")
		require.Error(t, err)
		require.Contains(t, err.Error(), "empty raw index")
	})
}

func TestFetchIndexHTTPFromURL_HTMLDetection(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping network I/O test in short mode")
	}
	t.Parallel()
	svc := NewHubService(nil, nil, t.TempDir())

	htmlResponse := `<!DOCTYPE html>
<html>
<head><title>CrowdSec Hub</title></head>
<body><h1>Welcome to CrowdSec Hub</h1></body>
</html>`

	svc.HTTPClient = &http.Client{Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		resp := newResponse(http.StatusOK, htmlResponse)
		resp.Header.Set("Content-Type", "text/html; charset=utf-8")
		return resp, nil
	})}

	_, err := svc.fetchIndexHTTPFromURL(context.Background(), "http://test.hub/index.json")
	require.Error(t, err)
	require.Contains(t, err.Error(), "HTML")
}

func TestHubService_Apply_ArchiveReadBeforeBackup(t *testing.T) {
	t.Parallel()
	cache, err := NewHubCache(t.TempDir(), time.Hour)
	require.NoError(t, err)

	dataDir := t.TempDir()
	archive := makeTarGz(t, map[string]string{"config.yml": "test: value"})
	_, err = cache.Store(context.Background(), "test/preset", "etag1", "hub", "preview", archive)
	require.NoError(t, err)

	svc := NewHubService(nil, cache, dataDir)

	// Apply should read archive before backup to avoid path issues
	res, err := svc.Apply(context.Background(), "test/preset")
	require.NoError(t, err)
	require.Equal(t, "applied", res.Status)
	require.FileExists(t, filepath.Join(dataDir, "config.yml"))
}

func TestHubService_Apply_CacheRefresh(t *testing.T) {
	t.Parallel()
	cache, err := NewHubCache(t.TempDir(), time.Second)
	require.NoError(t, err)

	dataDir := t.TempDir()

	// Store expired entry
	fixed := time.Now().Add(-5 * time.Second)
	cache.nowFn = func() time.Time { return fixed }
	archive := makeTarGz(t, map[string]string{"config.yml": "old"})
	_, err = cache.Store(context.Background(), "test/preset", "etag1", "hub", "old-preview", archive)
	require.NoError(t, err)

	// Reset time to trigger expiration
	cache.nowFn = time.Now

	indexBody := `{"items":[{"name":"test/preset","title":"Test","etag":"etag2","download_url":"http://test.hub/preset.tgz"}]}`
	newArchive := makeTarGz(t, map[string]string{"config.yml": "new"})

	svc := NewHubService(nil, cache, dataDir)
	svc.HubBaseURL = "http://test.hub"
	svc.HTTPClient = &http.Client{Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		if strings.Contains(req.URL.String(), "index.json") {
			return newResponse(http.StatusOK, indexBody), nil
		}
		if strings.Contains(req.URL.String(), "preset.tgz") {
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(newArchive)), Header: make(http.Header)}, nil
		}
		return newResponse(http.StatusNotFound, ""), nil
	})}

	res, err := svc.Apply(context.Background(), "test/preset")
	require.NoError(t, err)
	require.Equal(t, "applied", res.Status)

	// Verify new content was applied
	content, err := os.ReadFile(filepath.Join(dataDir, "config.yml")) //nolint:gosec // G304: Test file in temp directory
	require.NoError(t, err)
	require.Equal(t, "new", string(content))
}

func TestHubService_Apply_RollbackOnExtractionFailure(t *testing.T) {
	t.Parallel()
	cache, err := NewHubCache(t.TempDir(), time.Hour)
	require.NoError(t, err)

	dataDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dataDir, "important.txt"), []byte("preserve me"), 0o600))

	// Create archive with path traversal attempt
	badArchive := makeTarGz(t, map[string]string{"../escape.txt": "evil"})
	_, err = cache.Store(context.Background(), "test/preset", "etag1", "hub", "preview", badArchive)
	require.NoError(t, err)

	svc := NewHubService(nil, cache, dataDir)

	_, err = svc.Apply(context.Background(), "test/preset")
	require.Error(t, err)

	// Verify rollback preserved original file
	content, err := os.ReadFile(filepath.Join(dataDir, "important.txt")) // #nosec G304 -- test fixture path
	require.NoError(t, err)
	require.Equal(t, "preserve me", string(content))
}

func TestCopyDirAndCopyFile(t *testing.T) {
	t.Parallel()
	t.Run("copyFile success", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		srcFile := filepath.Join(tmpDir, "source.txt")
		dstFile := filepath.Join(tmpDir, "dest.txt")

		content := []byte("test content with special chars: !@#$%")
		require.NoError(t, os.WriteFile(srcFile, content, 0o600))

		err := copyFile(srcFile, dstFile)
		require.NoError(t, err)

		dstContent, err := os.ReadFile(dstFile) //nolint:gosec // G304: Test file in temp directory
		require.NoError(t, err)
		require.Equal(t, content, dstContent)
	})

	t.Run("copyFile preserves permissions", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		srcFile := filepath.Join(tmpDir, "executable.sh")
		dstFile := filepath.Join(tmpDir, "copy.sh")

		require.NoError(t, os.WriteFile(srcFile, []byte("#!/bin/bash\necho test"), 0o750)) // #nosec G306 -- test fixture for executable

		err := copyFile(srcFile, dstFile)
		require.NoError(t, err)

		srcInfo, err := os.Stat(srcFile)
		require.NoError(t, err)
		dstInfo, err := os.Stat(dstFile)
		require.NoError(t, err)

		require.Equal(t, srcInfo.Mode(), dstInfo.Mode())
	})

	t.Run("copyDir with nested structure", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		srcDir := filepath.Join(tmpDir, "source")
		dstDir := filepath.Join(tmpDir, "dest")

		// Create complex directory structure
		require.NoError(t, os.MkdirAll(filepath.Join(srcDir, "a", "b", "c"), 0o750)) // #nosec G301 -- test fixture
		require.NoError(t, os.WriteFile(filepath.Join(srcDir, "root.txt"), []byte("root"), 0o600))
		require.NoError(t, os.WriteFile(filepath.Join(srcDir, "a", "level1.txt"), []byte("level1"), 0o600))
		require.NoError(t, os.WriteFile(filepath.Join(srcDir, "a", "b", "level2.txt"), []byte("level2"), 0o600))
		require.NoError(t, os.WriteFile(filepath.Join(srcDir, "a", "b", "c", "level3.txt"), []byte("level3"), 0o600))

		require.NoError(t, os.MkdirAll(dstDir, 0o750)) // #nosec G301 -- test fixture

		err := copyDir(srcDir, dstDir)
		require.NoError(t, err)

		// Verify all files copied correctly
		require.FileExists(t, filepath.Join(dstDir, "root.txt"))
		require.FileExists(t, filepath.Join(dstDir, "a", "level1.txt"))
		require.FileExists(t, filepath.Join(dstDir, "a", "b", "level2.txt"))
		require.FileExists(t, filepath.Join(dstDir, "a", "b", "c", "level3.txt"))

		content, err := os.ReadFile(filepath.Join(dstDir, "a", "b", "c", "level3.txt")) // #nosec G304 -- test fixture path
		require.NoError(t, err)
		require.Equal(t, "level3", string(content))
	})

	t.Run("copyDir fails on non-directory source", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		srcFile := filepath.Join(tmpDir, "file.txt")
		dstDir := filepath.Join(tmpDir, "dest")

		require.NoError(t, os.WriteFile(srcFile, []byte("test"), 0o600))
		require.NoError(t, os.MkdirAll(dstDir, 0o750)) // #nosec G301 -- test fixture

		err := copyDir(srcFile, dstDir)
		require.Error(t, err)
		require.Contains(t, err.Error(), "not a directory")
	})
}

// ============================================
// emptyDir Tests
// ============================================

func TestEmptyDir(t *testing.T) {
	t.Parallel()
	t.Run("empties directory with files", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, "file1.txt"), []byte("content1"), 0o600))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "file2.txt"), []byte("content2"), 0o600))

		err := emptyDir(dir)
		require.NoError(t, err)

		// Directory should still exist
		require.DirExists(t, dir)

		// But be empty
		entries, err := os.ReadDir(dir)
		require.NoError(t, err)
		require.Empty(t, entries)
	})

	t.Run("empties directory with subdirectories", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		subDir := filepath.Join(dir, "subdir")
		require.NoError(t, os.MkdirAll(subDir, 0o750)) // #nosec G301 -- test fixture
		require.NoError(t, os.WriteFile(filepath.Join(subDir, "nested.txt"), []byte("nested"), 0o600))

		err := emptyDir(dir)
		require.NoError(t, err)

		require.DirExists(t, dir)
		entries, err := os.ReadDir(dir)
		require.NoError(t, err)
		require.Empty(t, entries)
	})

	t.Run("handles non-existent directory", func(t *testing.T) {
		t.Parallel()
		err := emptyDir(filepath.Join(t.TempDir(), "nonexistent"))
		require.NoError(t, err, "should not error on non-existent directory")
	})

	t.Run("handles empty directory", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		err := emptyDir(dir)
		require.NoError(t, err)
		require.DirExists(t, dir)
	})
}

// ============================================
// extractTarGz Tests
// ============================================

func TestExtractTarGz(t *testing.T) {
	t.Parallel()
	svc := NewHubService(nil, nil, t.TempDir())

	t.Run("extracts valid archive", func(t *testing.T) {
		t.Parallel()
		targetDir := t.TempDir()
		archive := makeTarGz(t, map[string]string{
			"file1.txt":        "content1",
			"subdir/file2.txt": "content2",
		})

		err := svc.extractTarGz(context.Background(), archive, targetDir)
		require.NoError(t, err)

		require.FileExists(t, filepath.Join(targetDir, "file1.txt"))
		require.FileExists(t, filepath.Join(targetDir, "subdir", "file2.txt"))

		content1, err := os.ReadFile(filepath.Join(targetDir, "file1.txt")) // #nosec G304 -- test fixture path
		require.NoError(t, err)
		require.Equal(t, "content1", string(content1))
	})

	t.Run("rejects path traversal", func(t *testing.T) {
		t.Parallel()
		targetDir := t.TempDir()

		// Create malicious archive with path traversal
		buf := &bytes.Buffer{}
		gw := gzip.NewWriter(buf)
		tw := tar.NewWriter(gw)

		hdr := &tar.Header{Name: "../escape.txt", Mode: 0o644, Size: 7}
		require.NoError(t, tw.WriteHeader(hdr))
		_, err := tw.Write([]byte("escaped"))
		require.NoError(t, err)
		require.NoError(t, tw.Close())
		require.NoError(t, gw.Close())

		err = svc.extractTarGz(context.Background(), buf.Bytes(), targetDir)
		require.Error(t, err)
		require.Contains(t, err.Error(), "unsafe path")
	})

	t.Run("rejects symlinks", func(t *testing.T) {
		t.Parallel()
		targetDir := t.TempDir()

		buf := &bytes.Buffer{}
		gw := gzip.NewWriter(buf)
		tw := tar.NewWriter(gw)

		hdr := &tar.Header{
			Name:     "symlink",
			Mode:     0o777,
			Size:     0,
			Typeflag: tar.TypeSymlink,
			Linkname: "/etc/passwd",
		}
		require.NoError(t, tw.WriteHeader(hdr))
		require.NoError(t, tw.Close())
		require.NoError(t, gw.Close())

		err := svc.extractTarGz(context.Background(), buf.Bytes(), targetDir)
		require.Error(t, err)
		require.Contains(t, err.Error(), "symlinks not allowed")
	})

	t.Run("handles corrupted gzip", func(t *testing.T) {
		t.Parallel()
		targetDir := t.TempDir()
		err := svc.extractTarGz(context.Background(), []byte("not a gzip"), targetDir)
		require.Error(t, err)
		require.Contains(t, err.Error(), "gunzip")
	})

	t.Run("handles context cancellation", func(t *testing.T) {
		t.Parallel()
		targetDir := t.TempDir()
		archive := makeTarGz(t, map[string]string{"file.txt": "content"})

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		err := svc.extractTarGz(ctx, archive, targetDir)
		require.Error(t, err)
		require.ErrorIs(t, err, context.Canceled)
	})

	t.Run("creates nested directories", func(t *testing.T) {
		t.Parallel()
		targetDir := t.TempDir()
		archive := makeTarGz(t, map[string]string{
			"a/b/c/deep.txt": "deep content",
		})

		err := svc.extractTarGz(context.Background(), archive, targetDir)
		require.NoError(t, err)

		require.FileExists(t, filepath.Join(targetDir, "a", "b", "c", "deep.txt"))
	})
}

// ============================================
// backupExisting Tests
// ============================================

func TestBackupExisting(t *testing.T) {
	t.Parallel()
	t.Run("handles non-existent directory", func(t *testing.T) {
		t.Parallel()
		dataDir := filepath.Join(t.TempDir(), "nonexistent")
		svc := NewHubService(nil, nil, dataDir)
		backupPath := dataDir + ".backup"

		err := svc.backupExisting(backupPath)
		require.NoError(t, err)
		require.NoDirExists(t, backupPath)
	})

	t.Run("creates backup of existing directory", func(t *testing.T) {
		t.Parallel()
		dataDir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dataDir, "config.txt"), []byte("config data"), 0o600))

		subDir := filepath.Join(dataDir, "subdir")
		require.NoError(t, os.MkdirAll(subDir, 0o750)) // #nosec G301 -- test fixture
		require.NoError(t, os.WriteFile(filepath.Join(subDir, "nested.txt"), []byte("nested data"), 0o600))

		svc := NewHubService(nil, nil, dataDir)
		backupPath := filepath.Join(t.TempDir(), "backup")

		err := svc.backupExisting(backupPath)
		require.NoError(t, err)

		// Verify backup exists
		require.FileExists(t, filepath.Join(backupPath, "config.txt"))
		require.FileExists(t, filepath.Join(backupPath, "subdir", "nested.txt"))
	})

	t.Run("backup contents match original", func(t *testing.T) {
		t.Parallel()
		dataDir := t.TempDir()
		originalContent := "important config"
		require.NoError(t, os.WriteFile(filepath.Join(dataDir, "config.txt"), []byte(originalContent), 0o600)) // #nosec G306 -- test fixture

		svc := NewHubService(nil, nil, dataDir)
		backupPath := filepath.Join(t.TempDir(), "backup")

		err := svc.backupExisting(backupPath)
		require.NoError(t, err)

		backupContent, err := os.ReadFile(filepath.Join(backupPath, "config.txt")) // #nosec G304 -- test fixture path
		require.NoError(t, err)
		require.Equal(t, originalContent, string(backupContent))
	})
}

// ============================================
// rollback Tests
// ============================================

func TestRollback(t *testing.T) {
	t.Parallel()
	t.Run("rollback with backup", func(t *testing.T) {
		t.Parallel()
		parentDir := t.TempDir()
		dataDir := filepath.Join(parentDir, "data")
		backupPath := filepath.Join(parentDir, "backup")

		// Create backup first
		require.NoError(t, os.MkdirAll(backupPath, 0o750))                                                            // #nosec G301 -- test fixture
		require.NoError(t, os.WriteFile(filepath.Join(backupPath, "backed_up.txt"), []byte("backup content"), 0o600)) // #nosec G306 -- test fixture

		// Create data dir with different content
		require.NoError(t, os.MkdirAll(dataDir, 0o750))                                                           // #nosec G301 -- test fixture
		require.NoError(t, os.WriteFile(filepath.Join(dataDir, "current.txt"), []byte("current content"), 0o600)) // #nosec G306 -- test fixture

		svc := NewHubService(nil, nil, dataDir)

		err := svc.rollback(backupPath)
		require.NoError(t, err)

		// Data dir should now have backup contents
		require.FileExists(t, filepath.Join(dataDir, "backed_up.txt"))
		// Backup path should no longer exist (renamed to dataDir)
		require.NoDirExists(t, backupPath)
	})

	t.Run("rollback with empty backup path", func(t *testing.T) {
		t.Parallel()
		dataDir := t.TempDir()
		svc := NewHubService(nil, nil, dataDir)

		err := svc.rollback("")
		require.NoError(t, err)
	})

	t.Run("rollback with non-existent backup", func(t *testing.T) {
		t.Parallel()
		dataDir := t.TempDir()
		svc := NewHubService(nil, nil, dataDir)

		err := svc.rollback(filepath.Join(t.TempDir(), "nonexistent"))
		require.NoError(t, err)
	})
}

// ============================================
// hubHTTPError Tests
// ============================================

func TestHubHTTPErrorError(t *testing.T) {
	t.Parallel()
	t.Run("error with inner error", func(t *testing.T) {
		t.Parallel()
		inner := errors.New("connection refused")
		err := hubHTTPError{
			url:        "https://hub.example.com/index.json",
			statusCode: 503,
			inner:      inner,
			fallback:   true,
		}

		msg := err.Error()
		require.Contains(t, msg, "https://hub.example.com/index.json")
		require.Contains(t, msg, "503")
		require.Contains(t, msg, "connection refused")
	})

	t.Run("error without inner error", func(t *testing.T) {
		t.Parallel()
		err := hubHTTPError{
			url:        "https://hub.example.com/index.json",
			statusCode: 404,
			inner:      nil,
			fallback:   false,
		}

		msg := err.Error()
		require.Contains(t, msg, "https://hub.example.com/index.json")
		require.Contains(t, msg, "404")
		require.NotContains(t, msg, "nil")
	})
}

func TestHubHTTPErrorUnwrap(t *testing.T) {
	t.Parallel()
	t.Run("unwrap returns inner error", func(t *testing.T) {
		t.Parallel()
		inner := errors.New("underlying error")
		err := hubHTTPError{
			url:        "https://hub.example.com",
			statusCode: 500,
			inner:      inner,
		}

		unwrapped := err.Unwrap()
		require.Equal(t, inner, unwrapped)
	})

	t.Run("unwrap returns nil when no inner", func(t *testing.T) {
		t.Parallel()
		err := hubHTTPError{
			url:        "https://hub.example.com",
			statusCode: 500,
			inner:      nil,
		}

		unwrapped := err.Unwrap()
		require.Nil(t, unwrapped)
	})

	t.Run("errors.Is works through Unwrap", func(t *testing.T) {
		t.Parallel()
		inner := context.Canceled
		err := hubHTTPError{
			url:        "https://hub.example.com",
			statusCode: 0,
			inner:      inner,
		}

		// errors.Is should work through Unwrap chain
		require.True(t, errors.Is(err, context.Canceled))
	})
}

func TestHubHTTPErrorCanFallback(t *testing.T) {
	t.Parallel()
	t.Run("returns true when fallback is true", func(t *testing.T) {
		t.Parallel()
		err := hubHTTPError{
			url:        "https://hub.example.com",
			statusCode: 503,
			fallback:   true,
		}

		require.True(t, err.CanFallback())
	})

	t.Run("returns false when fallback is false", func(t *testing.T) {
		t.Parallel()
		err := hubHTTPError{
			url:        "https://hub.example.com",
			statusCode: 404,
			fallback:   false,
		}

		require.False(t, err.CanFallback())
	})
}

// TestValidateHubURL_EdgeCases tests additional edge cases for SSRF protection
func TestValidateHubURL_EdgeCases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		url       string
		wantError bool
		errorMsg  string
	}{
		{
			name:      "Missing hostname",
			url:       "https://",
			wantError: true,
			errorMsg:  "missing hostname",
		},
		{
			name:      "Invalid URL format - unsupported scheme caught",
			url:       "not-a-url",
			wantError: true,
			errorMsg:  "unsupported scheme",
		},
		{
			name:      "FTP scheme rejected",
			url:       "ftp://hub-data.crowdsec.net/file.tgz",
			wantError: true,
			errorMsg:  "unsupported scheme",
		},
		{
			name:      "File scheme rejected",
			url:       "file:///etc/passwd",
			wantError: true,
			errorMsg:  "unsupported scheme",
		},
		{
			name:      "Test domain allowed",
			url:       "http://test.hub/api/index.json",
			wantError: false,
		},
		{
			name:      "Example.com allowed for testing",
			url:       "http://example.com/index.json",
			wantError: false,
		},
		{
			name:      ".local domain allowed",
			url:       "http://myserver.local/index.json",
			wantError: false,
		},
		{
			name:      "Subdomain of example.com allowed",
			url:       "http://test.example.com/index.json",
			wantError: false,
		},
		{
			name:      "IPv6 loopback allowed",
			url:       "http://[::1]:8080/index.json",
			wantError: false,
		},
		{
			name:      "Unknown production domain rejected",
			url:       "https://malicious-hub.com/index.json",
			wantError: true,
			errorMsg:  "unknown hub domain",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := validateHubURL(tt.url)
			if tt.wantError {
				require.Error(t, err)
				if tt.errorMsg != "" {
					require.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// ============================================
// NewHubService Constructor Tests
// ============================================

func TestNewHubService_DefaultTimeouts(t *testing.T) {
	t.Parallel()
	exec := &recordingExec{}
	cache, err := NewHubCache(t.TempDir(), time.Hour)
	require.NoError(t, err)

	svc := NewHubService(exec, cache, t.TempDir())

	require.NotNil(t, svc)
	require.Equal(t, defaultPullTimeout, svc.PullTimeout)
	require.Equal(t, defaultApplyTimeout, svc.ApplyTimeout)
	require.NotNil(t, svc.HTTPClient)
	require.Equal(t, defaultHubBaseURL, svc.HubBaseURL)
}

func TestNewHubService_EnvVarTimeouts_Valid(t *testing.T) {
	// Note: Cannot use t.Parallel() with t.Setenv()
	t.Setenv("HUB_PULL_TIMEOUT_SECONDS", "30")
	t.Setenv("HUB_APPLY_TIMEOUT_SECONDS", "60")

	svc := NewHubService(nil, nil, t.TempDir())

	require.Equal(t, 30*time.Second, svc.PullTimeout)
	require.Equal(t, 60*time.Second, svc.ApplyTimeout)
}

func TestNewHubService_EnvVarTimeouts_Invalid(t *testing.T) {
	// Note: Cannot use t.Parallel() with t.Setenv()
	t.Setenv("HUB_PULL_TIMEOUT_SECONDS", "invalid")
	t.Setenv("HUB_APPLY_TIMEOUT_SECONDS", "not-a-number")

	svc := NewHubService(nil, nil, t.TempDir())

	// Should fall back to defaults for invalid values
	require.Equal(t, defaultPullTimeout, svc.PullTimeout)
	require.Equal(t, defaultApplyTimeout, svc.ApplyTimeout)
}

func TestNewHubService_EnvVarTimeouts_Negative(t *testing.T) {
	// Note: Cannot use t.Parallel() with t.Setenv()
	t.Setenv("HUB_PULL_TIMEOUT_SECONDS", "-10")
	t.Setenv("HUB_APPLY_TIMEOUT_SECONDS", "0")

	svc := NewHubService(nil, nil, t.TempDir())

	// Should fall back to defaults for non-positive values
	require.Equal(t, defaultPullTimeout, svc.PullTimeout)
	require.Equal(t, defaultApplyTimeout, svc.ApplyTimeout)
}

func TestNewHubService_EnvVarTimeouts_Whitespace(t *testing.T) {
	// Note: Cannot use t.Parallel() with t.Setenv()
	t.Setenv("HUB_PULL_TIMEOUT_SECONDS", "  45  ")
	t.Setenv("HUB_APPLY_TIMEOUT_SECONDS", "\t90\n")

	svc := NewHubService(nil, nil, t.TempDir())

	// Should trim whitespace and parse correctly
	require.Equal(t, 45*time.Second, svc.PullTimeout)
	require.Equal(t, 90*time.Second, svc.ApplyTimeout)
}

func TestNewHubService_CustomHubBaseURL(t *testing.T) {
	// Note: Cannot use t.Parallel() with t.Setenv()
	t.Setenv("HUB_BASE_URL", "https://custom.hub.example.com")

	svc := NewHubService(nil, nil, t.TempDir())

	require.Equal(t, "https://custom.hub.example.com", svc.HubBaseURL)
}

func TestNewHubService_CustomMirrorBaseURL(t *testing.T) {
	// Note: Cannot use t.Parallel() with t.Setenv()
	t.Setenv("HUB_MIRROR_BASE_URL", "https://mirror.example.com")

	svc := NewHubService(nil, nil, t.TempDir())

	require.Equal(t, "https://mirror.example.com", svc.MirrorBaseURL)
}

// ============================================
// backupExisting Additional Tests
// ============================================

func TestBackupExisting_CopyFallback_Success(t *testing.T) {
	t.Parallel()
	dataDir := t.TempDir()

	// Create complex directory structure
	require.NoError(t, os.MkdirAll(filepath.Join(dataDir, "configs", "scenarios"), 0o750))                                   // #nosec G301 -- test fixture
	require.NoError(t, os.WriteFile(filepath.Join(dataDir, "main.yaml"), []byte("main config"), 0o600))                      // #nosec G306 -- test fixture
	require.NoError(t, os.WriteFile(filepath.Join(dataDir, "configs", "sub.yaml"), []byte("sub config"), 0o600))             // #nosec G306 -- test fixture
	require.NoError(t, os.WriteFile(filepath.Join(dataDir, "configs", "scenarios", "s1.yaml"), []byte("scenario 1"), 0o600)) // #nosec G306 -- test fixture

	svc := NewHubService(nil, nil, dataDir)
	backupPath := filepath.Join(t.TempDir(), "backup")

	err := svc.backupExisting(backupPath)
	require.NoError(t, err)

	// Verify all files were backed up
	require.FileExists(t, filepath.Join(backupPath, "main.yaml"))
	require.FileExists(t, filepath.Join(backupPath, "configs", "sub.yaml"))
	require.FileExists(t, filepath.Join(backupPath, "configs", "scenarios", "s1.yaml"))

	// Verify content integrity
	content, err := os.ReadFile(filepath.Join(backupPath, "configs", "scenarios", "s1.yaml")) // #nosec G304 -- test fixture path
	require.NoError(t, err)
	require.Equal(t, "scenario 1", string(content))
}

func TestBackupExisting_RenameSuccess(t *testing.T) {
	t.Parallel()
	baseDir := t.TempDir()
	dataDir := filepath.Join(baseDir, "data")
	require.NoError(t, os.MkdirAll(dataDir, 0o750))                                                // #nosec G301 -- test fixture
	require.NoError(t, os.WriteFile(filepath.Join(dataDir, "file.txt"), []byte("content"), 0o600)) // #nosec G306 -- test fixture

	svc := NewHubService(nil, nil, dataDir)
	backupPath := filepath.Join(baseDir, "backup")

	err := svc.backupExisting(backupPath)
	require.NoError(t, err)

	// Original should be gone (renamed, not copied)
	require.NoDirExists(t, dataDir)
	// Backup should exist with content
	require.FileExists(t, filepath.Join(backupPath, "file.txt"))
}

func TestBackupExisting_EmptyDirectory(t *testing.T) {
	t.Parallel()
	dataDir := t.TempDir()

	svc := NewHubService(nil, nil, dataDir)
	backupPath := filepath.Join(t.TempDir(), "backup")

	err := svc.backupExisting(backupPath)
	require.NoError(t, err)

	// Backup should exist even for empty dir
	require.DirExists(t, backupPath)
}

func TestBackupExisting_PreservesPermissions(t *testing.T) {
	t.Parallel()
	dataDir := t.TempDir()
	execFile := filepath.Join(dataDir, "executable.sh")
	require.NoError(t, os.WriteFile(execFile, []byte("#!/bin/bash"), 0o750)) // #nosec G306 -- test fixture for executable script

	svc := NewHubService(nil, nil, dataDir)
	backupPath := filepath.Join(t.TempDir(), "backup")

	err := svc.backupExisting(backupPath)
	require.NoError(t, err)

	// Check permissions were preserved
	origInfo, err := os.Stat(execFile)
	if err == nil {
		// If original still exists (rename succeeded)
		backupInfo, err := os.Stat(filepath.Join(backupPath, "executable.sh"))
		require.NoError(t, err)
		require.Equal(t, origInfo.Mode(), backupInfo.Mode())
	} else {
		// If original was renamed (which removes it)
		backupInfo, err := os.Stat(filepath.Join(backupPath, "executable.sh"))
		require.NoError(t, err)
		require.Equal(t, os.FileMode(0o750), backupInfo.Mode()&0o777)
	}
}

// ============================================
// extractTarGz Security Tests
// ============================================

func TestExtractTarGz_NestedPathTraversal(t *testing.T) {
	t.Parallel()
	svc := NewHubService(nil, nil, t.TempDir())
	targetDir := t.TempDir()

	// Create archive with nested path traversal
	buf := &bytes.Buffer{}
	gw := gzip.NewWriter(buf)
	tw := tar.NewWriter(gw)

	hdr := &tar.Header{Name: "dir/../../etc/shadow", Mode: 0o644, Size: 7}
	require.NoError(t, tw.WriteHeader(hdr))
	_, err := tw.Write([]byte("hacked!"))
	require.NoError(t, err)
	require.NoError(t, tw.Close())
	require.NoError(t, gw.Close())

	err = svc.extractTarGz(context.Background(), buf.Bytes(), targetDir)
	require.Error(t, err)
	require.Contains(t, err.Error(), "unsafe path")
}

func TestExtractTarGz_AbsolutePathWithDots(t *testing.T) {
	t.Parallel()
	svc := NewHubService(nil, nil, t.TempDir())
	targetDir := t.TempDir()

	buf := &bytes.Buffer{}
	gw := gzip.NewWriter(buf)
	tw := tar.NewWriter(gw)

	hdr := &tar.Header{Name: "/tmp/../etc/passwd", Mode: 0o644, Size: 4}
	require.NoError(t, tw.WriteHeader(hdr))
	_, err := tw.Write([]byte("root"))
	require.NoError(t, err)
	require.NoError(t, tw.Close())
	require.NoError(t, gw.Close())

	err = svc.extractTarGz(context.Background(), buf.Bytes(), targetDir)
	require.Error(t, err)
	require.Contains(t, err.Error(), "unsafe path")
}

func TestExtractTarGz_EmptyArchive(t *testing.T) {
	t.Parallel()
	svc := NewHubService(nil, nil, t.TempDir())
	targetDir := t.TempDir()

	// Create empty tar.gz
	buf := &bytes.Buffer{}
	gw := gzip.NewWriter(buf)
	tw := tar.NewWriter(gw)
	require.NoError(t, tw.Close())
	require.NoError(t, gw.Close())

	err := svc.extractTarGz(context.Background(), buf.Bytes(), targetDir)
	require.NoError(t, err)

	// Target directory should exist but be empty
	require.DirExists(t, targetDir)
	entries, err := os.ReadDir(targetDir)
	require.NoError(t, err)
	require.Empty(t, entries)
}

func TestExtractTarGz_InvalidTarAfterGzip(t *testing.T) {
	t.Parallel()
	svc := NewHubService(nil, nil, t.TempDir())
	targetDir := t.TempDir()

	// Create gzipped data that is not a valid tar
	buf := &bytes.Buffer{}
	gw := gzip.NewWriter(buf)
	_, err := gw.Write([]byte("this is not a tar archive"))
	require.NoError(t, err)
	require.NoError(t, gw.Close())

	err = svc.extractTarGz(context.Background(), buf.Bytes(), targetDir)
	require.Error(t, err)
	require.Contains(t, err.Error(), "tar")
}

func TestExtractTarGz_LargeNestedStructure(t *testing.T) {
	t.Parallel()
	svc := NewHubService(nil, nil, t.TempDir())
	targetDir := t.TempDir()

	// Create archive with deeply nested directories
	files := map[string]string{
		"a/b/c/d/e/f/g/h/file.txt": "deep file",
		"x/y/z/file.yaml":          "another file",
	}

	archive := makeTarGz(t, files)

	err := svc.extractTarGz(context.Background(), archive, targetDir)
	require.NoError(t, err)

	require.FileExists(t, filepath.Join(targetDir, "a", "b", "c", "d", "e", "f", "g", "h", "file.txt"))
	require.FileExists(t, filepath.Join(targetDir, "x", "y", "z", "file.yaml"))
}

func TestExtractTarGz_SpecialCharactersInFilenames(t *testing.T) {
	t.Parallel()
	svc := NewHubService(nil, nil, t.TempDir())
	targetDir := t.TempDir()

	files := map[string]string{
		"file with spaces.txt":      "content 1",
		"file-with-dashes.yaml":     "content 2",
		"file_with_underscores.yml": "content 3",
		"file.multiple.dots.txt":    "content 4",
	}

	archive := makeTarGz(t, files)

	err := svc.extractTarGz(context.Background(), archive, targetDir)
	require.NoError(t, err)

	require.FileExists(t, filepath.Join(targetDir, "file with spaces.txt"))
	require.FileExists(t, filepath.Join(targetDir, "file-with-dashes.yaml"))
	require.FileExists(t, filepath.Join(targetDir, "file_with_underscores.yml"))
	require.FileExists(t, filepath.Join(targetDir, "file.multiple.dots.txt"))
}

func TestExtractTarGz_DirectoriesWithoutFiles(t *testing.T) {
	t.Parallel()
	svc := NewHubService(nil, nil, t.TempDir())
	targetDir := t.TempDir()

	buf := &bytes.Buffer{}
	gw := gzip.NewWriter(buf)
	tw := tar.NewWriter(gw)

	// Add directory entry without files
	hdr := &tar.Header{
		Name:     "empty-dir/",
		Mode:     0o755,
		Typeflag: tar.TypeDir,
	}
	require.NoError(t, tw.WriteHeader(hdr))

	require.NoError(t, tw.Close())
	require.NoError(t, gw.Close())

	err := svc.extractTarGz(context.Background(), buf.Bytes(), targetDir)
	require.NoError(t, err)

	require.DirExists(t, filepath.Join(targetDir, "empty-dir"))
}

func TestExtractTarGz_SkipsSpecialFileTypes(t *testing.T) {
	t.Parallel()
	svc := NewHubService(nil, nil, t.TempDir())
	targetDir := t.TempDir()

	buf := &bytes.Buffer{}
	gw := gzip.NewWriter(buf)
	tw := tar.NewWriter(gw)

	// Add a character device (should be skipped)
	hdr := &tar.Header{
		Name:     "dev-null",
		Mode:     0o666,
		Typeflag: tar.TypeChar,
	}
	require.NoError(t, tw.WriteHeader(hdr))

	// Add a regular file
	regularHdr := &tar.Header{
		Name: "regular.txt",
		Mode: 0o644,
		Size: 7,
	}
	require.NoError(t, tw.WriteHeader(regularHdr))
	_, err := tw.Write([]byte("content"))
	require.NoError(t, err)

	require.NoError(t, tw.Close())
	require.NoError(t, gw.Close())

	err = svc.extractTarGz(context.Background(), buf.Bytes(), targetDir)
	require.NoError(t, err)

	// Special file should not exist
	require.NoFileExists(t, filepath.Join(targetDir, "dev-null"))
	// Regular file should exist
	require.FileExists(t, filepath.Join(targetDir, "regular.txt"))
}

// ============================================
// asString Tests
// ============================================

func TestAsString_Nil(t *testing.T) {
	t.Parallel()
	result := asString(nil)
	require.Equal(t, "", result)
}

func TestAsString_String(t *testing.T) {
	t.Parallel()
	result := asString("hello")
	require.Equal(t, "hello", result)
}

func TestAsString_Int(t *testing.T) {
	t.Parallel()
	result := asString(42)
	require.Equal(t, "42", result)
}

func TestAsString_Float(t *testing.T) {
	t.Parallel()
	result := asString(3.14)
	require.Contains(t, result, "3.14")
}

func TestAsString_Bool(t *testing.T) {
	t.Parallel()
	require.Equal(t, "true", asString(true))
	require.Equal(t, "false", asString(false))
}

func TestAsString_Struct(t *testing.T) {
	t.Parallel()
	type testStruct struct {
		Name string
		Age  int
	}
	result := asString(testStruct{Name: "Alice", Age: 30})
	require.Contains(t, result, "Alice")
	require.Contains(t, result, "30")
}

func TestAsString_EmptyString(t *testing.T) {
	t.Parallel()
	result := asString("")
	require.Equal(t, "", result)
}

// ============================================
// fetchIndexHTTPFromURL Additional Tests
// ============================================

func TestFetchIndexHTTPFromURL_ParseRawIndexFallback(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping network I/O test in short mode")
	}
	t.Parallel()
	svc := NewHubService(nil, nil, t.TempDir())

	rawIndexBody := `{
		"collections": {
			"crowdsecurity/nginx": {
				"path": "collections/crowdsecurity/nginx.tgz",
				"version": "1.5",
				"description": "Nginx collection"
			}
		}
	}`

	svc.HTTPClient = &http.Client{Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		resp := newResponse(http.StatusOK, rawIndexBody)
		resp.Header.Set("Content-Type", "application/json")
		return resp, nil
	})}

	idx, err := svc.fetchIndexHTTPFromURL(context.Background(), "http://test.hub/.index.json")
	require.NoError(t, err)
	require.Len(t, idx.Items, 1)
	require.Equal(t, "crowdsecurity/nginx", idx.Items[0].Name)
	require.Equal(t, "collections", idx.Items[0].Type)
}

func TestFetchIndexHTTPFromURL_EmptyJSONArray(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping network I/O test in short mode")
	}
	t.Parallel()
	svc := NewHubService(nil, nil, t.TempDir())

	svc.HTTPClient = &http.Client{Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		// Return empty items array which will trigger raw index parsing,
		// which will also fail because there are no sections
		resp := newResponse(http.StatusOK, `{"items":[]}`)
		resp.Header.Set("Content-Type", "application/json")
		return resp, nil
	})}

	// Empty items array triggers raw index parsing (map[string]map[string]...), which succeeds
	// but returns empty index. This is actually valid JSON but semantically empty.
	// The code returns idx even if empty in this case (no error), so we should not expect an error.
	idx, err := svc.fetchIndexHTTPFromURL(context.Background(), "http://test.hub/index.json")
	require.NoError(t, err)
	require.Empty(t, idx.Items, "should parse successfully but return empty items")
}

func TestFetchIndexHTTPFromURL_InvalidJSON(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping network I/O test in short mode")
	}
	t.Parallel()
	svc := NewHubService(nil, nil, t.TempDir())

	svc.HTTPClient = &http.Client{Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		resp := newResponse(http.StatusOK, `{invalid json`)
		resp.Header.Set("Content-Type", "application/json")
		return resp, nil
	})}

	_, err := svc.fetchIndexHTTPFromURL(context.Background(), "http://test.hub/index.json")
	require.Error(t, err)
}

// ============================================
// isGzip Tests
// ============================================

func TestIsGzip_ValidGzip(t *testing.T) {
	t.Parallel()
	buf := &bytes.Buffer{}
	gw := gzip.NewWriter(buf)
	_, err := gw.Write([]byte("test data"))
	require.NoError(t, err)
	require.NoError(t, gw.Close())

	require.True(t, isGzip(buf.Bytes()))
}

func TestIsGzip_NotGzip(t *testing.T) {
	t.Parallel()
	require.False(t, isGzip([]byte("plain text")))
	require.False(t, isGzip([]byte{}))
	require.False(t, isGzip([]byte{0x00}))
}

func TestIsGzip_TooShort(t *testing.T) {
	t.Parallel()
	require.False(t, isGzip([]byte{0x1f}))
	require.False(t, isGzip([]byte{}))
}

// ============================================
// peekFirstYAML Tests
// ============================================

func TestPeekFirstYAML_FindsYAML(t *testing.T) {
	t.Parallel()
	svc := NewHubService(nil, nil, t.TempDir())
	archive := makeTarGz(t, map[string]string{
		"readme.txt":    "readme content",
		"aaa.yaml":      "name: test\nversion: 1.0",
		"zzz-other.yml": "other: config",
	})

	result := svc.peekFirstYAML(archive)
	require.NotEmpty(t, result)
	require.Contains(t, result, "name: test")
}

func TestPeekFirstYAML_NoYAMLFiles(t *testing.T) {
	t.Parallel()
	svc := NewHubService(nil, nil, t.TempDir())
	archive := makeTarGz(t, map[string]string{
		"readme.txt":  "readme",
		"config.json": "{}",
	})

	result := svc.peekFirstYAML(archive)
	require.Empty(t, result)
}

func TestPeekFirstYAML_InvalidArchive(t *testing.T) {
	t.Parallel()
	svc := NewHubService(nil, nil, t.TempDir())
	result := svc.peekFirstYAML([]byte("not a gzip archive"))
	require.Empty(t, result)
}

// ============================================
// findIndexEntry Tests
// ============================================

func TestFindIndexEntry_ExactMatch(t *testing.T) {
	t.Parallel()
	idx := HubIndex{
		Items: []HubIndexEntry{
			{Name: "crowdsecurity/nginx", Title: "Nginx"},
			{Name: "crowdsecurity/apache", Title: "Apache"},
		},
	}

	entry, found := findIndexEntry(idx, "crowdsecurity/nginx")
	require.True(t, found)
	require.Equal(t, "crowdsecurity/nginx", entry.Name)
}

func TestFindIndexEntry_ShortName(t *testing.T) {
	t.Parallel()
	idx := HubIndex{
		Items: []HubIndexEntry{
			{Name: "crowdsecurity/nginx", Title: "Nginx"},
		},
	}

	entry, found := findIndexEntry(idx, "nginx")
	require.True(t, found)
	require.Equal(t, "crowdsecurity/nginx", entry.Name)
}

func TestFindIndexEntry_AmbiguousShortName(t *testing.T) {
	t.Parallel()
	idx := HubIndex{
		Items: []HubIndexEntry{
			{Name: "crowdsecurity/test", Title: "Test 1"},
			{Name: "vendor/test", Title: "Test 2"},
		},
	}

	_, found := findIndexEntry(idx, "test")
	require.False(t, found, "ambiguous short name should not match")
}

func TestFindIndexEntry_NotFound(t *testing.T) {
	t.Parallel()
	idx := HubIndex{
		Items: []HubIndexEntry{
			{Name: "crowdsecurity/nginx", Title: "Nginx"},
		},
	}

	_, found := findIndexEntry(idx, "nonexistent")
	require.False(t, found)
}

func TestFindIndexEntry_EmptySlug(t *testing.T) {
	t.Parallel()
	idx := HubIndex{
		Items: []HubIndexEntry{
			{Name: "crowdsecurity/test", Title: "Test"},
		},
	}

	_, found := findIndexEntry(idx, "  ")
	require.False(t, found)
}
