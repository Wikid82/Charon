package crowdsec

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"embed"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
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

//go:embed testdata/hub_index_fixture.json testdata/hub_index_html.html testdata/hub_index_irregular_names.json
var hubTestFixtures embed.FS

func readFixture(t *testing.T, name string) string {
	t.Helper()
	data, err := hubTestFixtures.ReadFile(filepath.Join("testdata", name))
	require.NoError(t, err)
	return string(data)
}

func TestFetchIndexListsFromCSCLIWhenHubUnreachable(t *testing.T) {
	t.Parallel()
	exec := &recordingExec{outputs: map[string][]byte{"cscli hub list -a -o json": []byte(`{"collections":[{"name":"crowdsecurity/test","description":"desc","version":"1.0"}]}`)}}
	svc := NewHubService(exec, nil, t.TempDir())
	svc.HTTPClient = nil

	idx, err := svc.FetchIndex(context.Background())
	require.NoError(t, err)
	require.Len(t, idx.Items, 1)
	require.Equal(t, "crowdsecurity/test", idx.Items[0].Name)
	require.Contains(t, exec.calls, "cscli hub list -a -o json")
}

func TestFetchIndexFallbackHTTP(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping network I/O test in short mode")
	}
	exec := &recordingExec{errors: map[string]error{"cscli hub list -a -o json": fmt.Errorf("boom")}}
	cacheDir := t.TempDir()
	svc := NewHubService(exec, nil, cacheDir)
	indexBody := readFixture(t, "hub_index_fixture.json")
	hubServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != defaultHubIndexPath {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(indexBody))
	}))
	defer hubServer.Close()

	svc.HubBaseURL = hubServer.URL
	svc.validateURL = func(string) error { return nil } // local httptest server is not an allowlisted hub host
	svc.HTTPClient = hubServer.Client()

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
	svc.HubBaseURL = "https://hub.crowdsec.net"
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
	svc.HubBaseURL = "https://hub.crowdsec.net"
	svc.HTTPClient = &http.Client{Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		return newResponse(http.StatusOK, `{"items":[{"name":"crowdsecurity/other","title":"Other","description":"d","type":"collection"}]}`), nil
	})}

	_, err = svc.Pull(context.Background(), "crowdsecurity/missing")
	require.Error(t, err)
}

func TestFetchIndexCSCLIParseError(t *testing.T) {
	t.Parallel()
	exec := &recordingExec{outputs: map[string][]byte{"cscli hub list -o json": []byte("not-json")}}
	svc := NewHubService(exec, nil, t.TempDir())
	svc.HubBaseURL = "https://hub.crowdsec.net"
	svc.HTTPClient = &http.Client{Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		return newResponse(http.StatusInternalServerError, ""), nil
	})}

	_, err := svc.FetchIndex(context.Background())
	require.Error(t, err)
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

func TestIndexURLCandidates_GitHubMirror(t *testing.T) {
	t.Parallel()

	candidates := indexURLCandidates("https://raw.githubusercontent.com/crowdsecurity/hub/master")
	require.Len(t, candidates, 2)
	require.Contains(t, candidates, "https://raw.githubusercontent.com/crowdsecurity/hub/master/.index.json")
	require.Contains(t, candidates, "https://raw.githubusercontent.com/crowdsecurity/hub/master/api/index.json")
}

func TestHubHTTPErrorMethods(t *testing.T) {
	t.Parallel()

	inner := errors.New("inner")
	err := hubHTTPError{url: "https://hub.crowdsec.net", statusCode: 404, inner: inner, fallback: true}

	require.Contains(t, err.Error(), "https://hub.crowdsec.net")
	require.ErrorIs(t, err, inner)
	require.True(t, err.CanFallback())
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
	err := copyTree(srcDir, dstDir, nil)
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
	err = copyTree(filepath.Join(tmpDir, "nonexistent"), dstDir, nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "stat src")

	// Test copy file as directory (should fail)
	fileNotDir := filepath.Join(tmpDir, "file.txt")
	require.NoError(t, os.WriteFile(fileNotDir, []byte("test"), 0o600))
	err = copyTree(fileNotDir, dstDir, nil)
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

func TestValidateHubURL_RejectsNonProductionHosts(t *testing.T) {
	t.Parallel()
	nonProductionURLs := []string{
		"http://localhost:8080/index.json",
		"http://127.0.0.1:8080/index.json",
		"http://[::1]:8080/index.json",
		"https://localhost/index.json",
		"https://127.0.0.1/index.json",
		"https://[::1]/index.json",
		"https://test.hub/api/index.json",
		"https://example.com/api/index.json",
		"https://test.example.com/api/index.json",
		"https://server.local/api/index.json",
		"http://example.com/api/index.json",
	}

	for _, url := range nonProductionURLs {
		t.Run(url, func(t *testing.T) {
			t.Parallel()
			require.Error(t, validateHubURL(url), "Expected non-production host to be rejected")
		})
	}
}

func TestNewHubHTTPClient_RejectsLoopback(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	resp, err := newHubHTTPClient(2 * time.Second).Get(srv.URL)
	if err == nil {
		_ = resp.Body.Close()
	}
	require.Error(t, err, "hub client must reject loopback")
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

		idx, err := parseRawIndex([]byte(rawJSON), "https://hub.crowdsec.net/api/index.json")
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
		_, err := parseRawIndex([]byte("not json"), "https://hub.crowdsec.net")
		require.Error(t, err)
		require.Contains(t, err.Error(), "parse raw index")
	})

	t.Run("returns error on empty index", func(t *testing.T) {
		t.Parallel()
		_, err := parseRawIndex([]byte("{}"), "https://hub.crowdsec.net")
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

	_, err := svc.fetchIndexHTTPFromURL(context.Background(), "https://hub.crowdsec.net/index.json")
	require.Error(t, err)
	require.Contains(t, err.Error(), "HTML")
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

		err := copyTree(srcDir, dstDir, nil)
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

		err := copyTree(srcFile, dstDir, nil)
		require.Error(t, err)
		require.Contains(t, err.Error(), "not a directory")
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
			url:        "https://hub.crowdsec.net/index.json",
			statusCode: 503,
			inner:      inner,
			fallback:   true,
		}

		msg := err.Error()
		require.Contains(t, msg, "https://hub.crowdsec.net/index.json")
		require.Contains(t, msg, "503")
		require.Contains(t, msg, "connection refused")
	})

	t.Run("error without inner error", func(t *testing.T) {
		t.Parallel()
		err := hubHTTPError{
			url:        "https://hub.crowdsec.net/index.json",
			statusCode: 404,
			inner:      nil,
			fallback:   false,
		}

		msg := err.Error()
		require.Contains(t, msg, "https://hub.crowdsec.net/index.json")
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
			url:        "https://hub.crowdsec.net",
			statusCode: 500,
			inner:      inner,
		}

		unwrapped := err.Unwrap()
		require.Equal(t, inner, unwrapped)
	})

	t.Run("unwrap returns nil when no inner", func(t *testing.T) {
		t.Parallel()
		err := hubHTTPError{
			url:        "https://hub.crowdsec.net",
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
			url:        "https://hub.crowdsec.net",
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
			url:        "https://hub.crowdsec.net",
			statusCode: 503,
			fallback:   true,
		}

		require.True(t, err.CanFallback())
	})

	t.Run("returns false when fallback is false", func(t *testing.T) {
		t.Parallel()
		err := hubHTTPError{
			url:        "https://hub.crowdsec.net",
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
			name:      "Allowlisted hosts over HTTPS accepted",
			url:       "https://raw.githubusercontent.com/crowdsecurity/hub/master/.index.json",
			wantError: false,
		},
		{
			name:      "Test domain rejected",
			url:       "https://test.hub/api/index.json",
			wantError: true,
			errorMsg:  "unknown hub domain",
		},
		{
			name:      "Example.com rejected",
			url:       "https://example.com/index.json",
			wantError: true,
			errorMsg:  "unknown hub domain",
		},
		{
			name:      ".local domain rejected",
			url:       "https://myserver.local/index.json",
			wantError: true,
			errorMsg:  "unknown hub domain",
		},
		{
			name:      "Plain HTTP loopback rejected",
			url:       "http://localhost:8080/index.json",
			wantError: true,
			errorMsg:  "must use HTTPS",
		},
		{
			name:      "IPv6 loopback rejected",
			url:       "http://[::1]:8080/index.json",
			wantError: true,
			errorMsg:  "must use HTTPS",
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
	require.Equal(t, 120*time.Second, defaultApplyTimeout)
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
	t.Setenv("HUB_BASE_URL", "https://hub.crowdsec.net")

	svc := NewHubService(nil, nil, t.TempDir())

	require.Equal(t, "https://hub.crowdsec.net", svc.HubBaseURL)
}

func TestNewHubService_CustomMirrorBaseURL(t *testing.T) {
	// Note: Cannot use t.Parallel() with t.Setenv()
	t.Setenv("HUB_MIRROR_BASE_URL", "https://hub-data.crowdsec.net")

	svc := NewHubService(nil, nil, t.TempDir())

	require.Equal(t, "https://hub-data.crowdsec.net", svc.MirrorBaseURL)
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

	idx, err := svc.fetchIndexHTTPFromURL(context.Background(), "https://hub.crowdsec.net/.index.json")
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
	idx, err := svc.fetchIndexHTTPFromURL(context.Background(), "https://hub.crowdsec.net/index.json")
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

	_, err := svc.fetchIndexHTTPFromURL(context.Background(), "https://hub.crowdsec.net/index.json")
	require.Error(t, err)
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

func TestFindIndexEntry_ShortNameDoesNotMatch(t *testing.T) {
	t.Parallel()
	idx := HubIndex{
		Items: []HubIndexEntry{
			{Name: "crowdsecurity/nginx", Title: "Nginx"},
		},
	}

	_, found := findIndexEntry(idx, "nginx")
	require.False(t, found)
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

func TestHubBaseCandidates_DistinctRolesKeepOrder(t *testing.T) {
	t.Parallel()
	svc := NewHubService(nil, nil, t.TempDir())
	svc.HubBaseURL = "https://hub.crowdsec.net"
	svc.MirrorBaseURL = defaultHubMirrorBaseURL

	require.Equal(t, []string{
		"https://hub.crowdsec.net",
		defaultHubMirrorBaseURL,
		defaultHubBaseURL,
	}, svc.hubBaseCandidates())
}

func TestParseCSCLIIndexKeepsOnlyUsableNames(t *testing.T) {
	t.Parallel()
	raw := `{"collections":[
		{"name":"crowdsecurity/ok","title":"","description":"","version":"1.0","hash":"abc"},
		{"name":"solo","description":"no owner"},
		{"name":" crowdsecurity/padded ","title":"Padded","description":"d","etag":"e1","download_url":"https://hub.crowdsec.net/p.yaml","preview_url":"https://hub.crowdsec.net/p.yaml"},
		{"name":"owner/name+plus"}
	]}`
	idx, err := parseCSCLIIndex([]byte(raw))
	require.NoError(t, err)
	require.Len(t, idx.Items, 2)
	byName := map[string]HubIndexEntry{}
	for _, it := range idx.Items {
		byName[it.Name] = it
	}
	ok := byName["crowdsecurity/ok"]
	require.Equal(t, "crowdsecurity/ok", ok.Title)
	require.Equal(t, "crowdsecurity/ok", ok.Description)
	require.Equal(t, "abc", ok.Etag)
	require.Equal(t, "collections", ok.Type)
	require.Equal(t, "Padded", byName["crowdsecurity/padded"].Title)

	_, err = parseCSCLIIndex([]byte(`{"collections":[{"name":"solo"}]}`))
	require.ErrorContains(t, err, "empty cscli index")
	_, err = parseCSCLIIndex([]byte(`nope`))
	require.Error(t, err)
}
