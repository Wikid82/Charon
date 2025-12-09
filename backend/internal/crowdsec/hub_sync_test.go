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
	for name, content := range files {
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
	data, err := os.ReadFile(filepath.Join("testdata", name))
	require.NoError(t, err)
	return string(data)
}

func TestFetchIndexPrefersCSCLI(t *testing.T) {
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

func TestPullCachesPreview(t *testing.T) {
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
	cache, err := NewHubCache(t.TempDir(), time.Hour)
	require.NoError(t, err)
	baseDir := filepath.Join(t.TempDir(), "data")
	require.NoError(t, os.MkdirAll(baseDir, 0o755))
	keep := filepath.Join(baseDir, "keep.txt")
	require.NoError(t, os.WriteFile(keep, []byte("before"), 0o644))

	badArchive := makeTarGz(t, map[string]string{"../evil.txt": "boom"})
	_, err = cache.Store(context.Background(), "crowdsecurity/demo", "etag1", "hub", "preview", badArchive)
	require.NoError(t, err)

	svc := NewHubService(nil, cache, baseDir)
	_, err = svc.Apply(context.Background(), "crowdsecurity/demo")
	require.Error(t, err)

	content, readErr := os.ReadFile(keep)
	require.NoError(t, readErr)
	require.Equal(t, "before", string(content))
}

func TestApplyUsesCacheWhenCscliMissing(t *testing.T) {
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

func TestFetchWithLimitRejectsLargePayload(t *testing.T) {
	svc := NewHubService(nil, nil, t.TempDir())
	big := bytes.Repeat([]byte("a"), int(maxArchiveSize+10))
	svc.HTTPClient = &http.Client{Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(big)), Header: make(http.Header)}, nil
	})}

	_, err := svc.fetchWithLimit(context.Background(), "http://example.com/large.tgz")
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
	svc := NewHubService(nil, nil, t.TempDir())
	archive := makeSymlinkTar(t, "bad.symlink")

	err := svc.extractTarGz(context.Background(), archive, filepath.Join(t.TempDir(), "data"))
	require.Error(t, err)
	require.Contains(t, err.Error(), "symlinks not allowed")
}

func TestExtractTarGzRejectsAbsolutePath(t *testing.T) {
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
	svc := NewHubService(nil, nil, t.TempDir())
	svc.HTTPClient = &http.Client{Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		return newResponse(http.StatusServiceUnavailable, ""), nil
	})}

	_, err := svc.fetchIndexHTTP(context.Background())
	require.Error(t, err)
}

func TestPullValidatesSlugAndMissingPreset(t *testing.T) {
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
	svc := NewHubService(nil, nil, t.TempDir())
	_, err := svc.fetchPreview(context.Background(), "")
	require.Error(t, err)
}

func TestFetchWithLimitRequiresClient(t *testing.T) {
	svc := NewHubService(nil, nil, t.TempDir())
	svc.HTTPClient = nil
	_, err := svc.fetchWithLimit(context.Background(), "http://example.com/demo.tgz")
	require.Error(t, err)
}

func TestRunCSCLIRejectsUnsafeSlug(t *testing.T) {
	exec := &recordingExec{}
	svc := NewHubService(exec, nil, t.TempDir())

	err := svc.runCSCLI(context.Background(), "../bad")
	require.Error(t, err)
}

func TestApplyUsesCSCLISuccess(t *testing.T) {
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
	svc := NewHubService(nil, nil, t.TempDir())
	svc.HubBaseURL = "http://hub.example"
	svc.HTTPClient = &http.Client{Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		return newResponse(http.StatusNotFound, ""), nil
	})}

	_, err := svc.fetchWithLimit(context.Background(), "http://hub.example/demo.tgz")
	require.Error(t, err)
}

func TestApplyRollsBackWhenCacheMissing(t *testing.T) {
	baseDir := t.TempDir()
	dataDir := filepath.Join(baseDir, "crowdsec")
	require.NoError(t, os.MkdirAll(dataDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dataDir, "keep.txt"), []byte("before"), 0o644))

	svc := NewHubService(nil, nil, dataDir)
	res, err := svc.Apply(context.Background(), "crowdsecurity/demo")
	require.Error(t, err)
	require.Contains(t, err.Error(), "cscli unavailable")
	require.Empty(t, res.BackupPath)
	require.Equal(t, "failed", res.Status)

	content, readErr := os.ReadFile(filepath.Join(dataDir, "keep.txt"))
	require.NoError(t, readErr)
	require.Equal(t, "before", string(content))
}
