package handlers

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// isolateTempDir points the process temp dir at a fresh directory and returns it.
func isolateTempDir(t *testing.T) string {
	t.Helper()
	tmp := t.TempDir()
	t.Setenv("TMPDIR", tmp)
	return tmp
}

func requireNoExportTempFiles(t *testing.T, tmp string) {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(tmp, "crowdsec-export-*"))
	require.NoError(t, err)
	require.Empty(t, matches)
}

func getExport(r *gin.Engine) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/admin/crowdsec/export", http.NoBody))
	return w
}

func requireJSONError(t *testing.T, w *httptest.ResponseRecorder, msg string) {
	t.Helper()
	require.Equal(t, http.StatusInternalServerError, w.Code)
	require.Empty(t, w.Header().Get("Content-Disposition"))
	require.NotEqual(t, "application/gzip", w.Header().Get("Content-Type"))
	var body map[string]string
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body), w.Body.String())
	require.Equal(t, msg, body["error"])
	require.False(t, bytes.HasPrefix(w.Body.Bytes(), []byte{0x1f, 0x8b}), "body must not start a gzip stream")
}

func TestExportConfigContentLengthAndValidArchive(t *testing.T) {
	tmp := isolateTempDir(t)
	dir := t.TempDir()
	seedRealLayout(t, dir)
	_, r := newImportRouter(t, dir)

	w := getExport(r)
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "application/gzip", w.Header().Get("Content-Type"))
	require.Contains(t, w.Header().Get("Content-Disposition"), "attachment; filename=crowdsec-config-")
	require.Equal(t, strconv.Itoa(w.Body.Len()), w.Header().Get("Content-Length"))

	gr, err := gzip.NewReader(bytes.NewReader(w.Body.Bytes()))
	require.NoError(t, err)
	tr := tar.NewReader(gr)
	n := 0
	for {
		_, nextErr := tr.Next()
		if errors.Is(nextErr, io.EOF) {
			break
		}
		require.NoError(t, nextErr)
		n++
	}
	require.Positive(t, n)
	requireNoExportTempFiles(t, tmp)
}

func TestExportConfigUnreadableFileReturnsJSONError(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("permission checks do not apply to root")
	}
	tmp := isolateTempDir(t)
	dir := t.TempDir()
	writeTree(t, dir, map[string]string{"a.yaml": "a", "restricted.conf": "x"})
	p := filepath.Join(dir, "restricted.conf")
	require.NoError(t, os.Chmod(p, 0o000)) // #nosec G302 -- intentional test permission
	t.Cleanup(func() { _ = os.Chmod(p, 0o600) })
	_, r := newImportRouter(t, dir)

	requireJSONError(t, getExport(r), "a file in the CrowdSec folder could not be read; see server logs")
	requireNoExportTempFiles(t, tmp)
}

func TestExportConfigRatioFailureIsNotImportable(t *testing.T) {
	tmp := isolateTempDir(t)
	dir := t.TempDir()
	// Highly compressible content: well above the 100x ratio an import accepts.
	require.NoError(t, os.WriteFile(filepath.Join(dir, "zeros.yaml"), make([]byte, 20<<20), 0o600))
	_, r := newImportRouter(t, dir)

	requireJSONError(t, getExport(r), "crowdsec config cannot be exported in an importable form")
	requireNoExportTempFiles(t, tmp)
}

func TestExportConfigOverCompressedLimitIsTooLarge(t *testing.T) {
	if testing.Short() {
		t.Skip("writes a 51 MiB incompressible fixture")
	}
	tmp := isolateTempDir(t)
	dir := t.TempDir()
	buf := make([]byte, 51<<20)
	_, err := rand.Read(buf)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "big.bin"), buf, 0o600))
	_, r := newImportRouter(t, dir)

	requireJSONError(t, getExport(r), "crowdsec config is too large to export")
	requireNoExportTempFiles(t, tmp)
}

func TestExportConfigTempFileFailureReturnsJSONError(t *testing.T) {
	t.Setenv("TMPDIR", filepath.Join(t.TempDir(), "missing"))
	dir := t.TempDir()
	writeTree(t, dir, map[string]string{"a.yaml": "a"})
	_, r := newImportRouter(t, dir)

	requireJSONError(t, getExport(r), "failed to export crowdsec config")
}

// blockingWriter holds the response write until released, standing in for a slow client.
type blockingWriter struct {
	gin.ResponseWriter
	started chan struct{}
	release chan struct{}
	once    sync.Once
}

func (b *blockingWriter) Write(p []byte) (int, error) {
	b.once.Do(func() { close(b.started) })
	<-b.release
	return b.ResponseWriter.Write(p)
}

func TestExportConfigDownloadDoesNotHoldDataLock(t *testing.T) {
	isolateTempDir(t)
	dir := t.TempDir()
	seedRealLayout(t, dir)
	h := newTestCrowdsecHandler(t, OpenTestDB(t), &fakeExec{}, "/bin/false", dir)
	bw := &blockingWriter{started: make(chan struct{}), release: make(chan struct{})}
	r := gin.New()
	r.Use(func(c *gin.Context) {
		if strings.HasSuffix(c.Request.URL.Path, "/export") {
			bw.ResponseWriter = c.Writer
			c.Writer = bw
		}
		c.Next()
	})
	h.RegisterRoutes(r.Group("/api/v1"))

	// An archive to import while the download is stalled.
	_, plain := newImportRouter(t, dir)
	archive := exportBytes(t, plain)

	done := make(chan struct{})
	go func() {
		defer close(done)
		getExport(r)
	}()
	select {
	case <-bw.started:
	case <-time.After(10 * time.Second):
		t.Fatal("download never started")
	}

	require.True(t, h.dataMu.TryLock(), "a stalled download must not hold the data lock")
	h.dataMu.Unlock()

	importDone := make(chan int, 1)
	go func() {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, importBytesRequest(t, archive))
		importDone <- w.Code
	}()
	select {
	case code := <-importDone:
		require.Equal(t, http.StatusOK, code)
	case <-time.After(20 * time.Second):
		t.Fatal("import blocked behind a stalled download")
	}

	close(bw.release)
	<-done
}

func TestExportConfigConcurrentWithImports(t *testing.T) {
	isolateTempDir(t)
	dir := t.TempDir()
	seedRealLayout(t, dir)
	_, r := newImportRouter(t, dir)
	archive := exportBytes(t, r)

	const workers = 4
	var wg sync.WaitGroup
	errs := make(chan error, workers*4)
	finished := make(chan struct{})
	for i := 0; i < workers; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			for j := 0; j < 3; j++ {
				w := getExport(r)
				if w.Code != http.StatusOK {
					errs <- errors.New("export status " + strconv.Itoa(w.Code))
					return
				}
				if !archiveHasValidConfig(w.Body.Bytes()) {
					errs <- errors.New("export is not a complete archive")
					return
				}
			}
		}()
		go func() {
			defer wg.Done()
			for j := 0; j < 3; j++ {
				w := httptest.NewRecorder()
				r.ServeHTTP(w, importBytesRequest(t, archive))
				if w.Code != http.StatusOK {
					errs <- errors.New("import status " + strconv.Itoa(w.Code))
					return
				}
			}
		}()
	}
	go func() { wg.Wait(); close(finished) }()
	select {
	case <-finished:
	case <-time.After(60 * time.Second):
		t.Fatal("concurrent import/export deadlocked")
	}
	close(errs)
	for err := range errs {
		t.Error(err)
	}
}

// archiveHasValidConfig reports whether data is a readable archive holding config/config.yaml.
func archiveHasValidConfig(data []byte) bool {
	gr, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return false
	}
	tr := tar.NewReader(gr)
	found := false
	for {
		hdr, nextErr := tr.Next()
		if errors.Is(nextErr, io.EOF) {
			return found
		}
		if nextErr != nil {
			return false
		}
		body, readErr := io.ReadAll(tr)
		if readErr != nil {
			return false
		}
		if hdr.Name == "config/config.yaml" {
			found = string(body) == importableConfig
		}
	}
}

func TestExportConfigCancelledRequestAbortsWithoutBody(t *testing.T) {
	tmp := isolateTempDir(t)
	dir := t.TempDir()
	writeTree(t, dir, map[string]string{"a.yaml": "a"})
	_, r := newImportRouter(t, dir)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/crowdsec/export", http.NoBody).WithContext(ctx)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Empty(t, w.Body.Bytes(), "no JSON error and no archive for a client that is gone")
	require.Empty(t, w.Header().Get("Content-Disposition"))
	requireNoExportTempFiles(t, tmp)
}

func TestExtractArchiveFailures(t *testing.T) {
	dir := t.TempDir()
	h, _ := newImportRouter(t, dir)
	noKeep := func(string) bool { return false }

	require.ErrorContains(t, h.extractArchive(filepath.Join(dir, "missing.tar.gz"), dir, noKeep), "failed to open archive")

	notGzip := filepath.Join(t.TempDir(), "plain.tar.gz")
	require.NoError(t, os.WriteFile(notGzip, []byte("not gzip"), 0o600))
	require.ErrorContains(t, h.extractArchive(notGzip, dir, noKeep), "failed to create gzip reader")

	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	require.NoError(t, tar.NewWriter(gw).Close())
	require.NoError(t, gw.Close())
	valid := filepath.Join(t.TempDir(), "empty.tar.gz")
	require.NoError(t, os.WriteFile(valid, buf.Bytes(), 0o600))
	require.ErrorContains(t, h.extractArchive(valid, filepath.Join(dir, "missing-dest"), noKeep), "list live directories")
}
