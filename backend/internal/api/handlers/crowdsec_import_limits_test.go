package handlers

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/rand"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/crowdsec"
)

// archiveWithEntryCount builds a tar.gz with config.yaml and n zero-size files after it.
func archiveWithEntryCount(t *testing.T, n int) []byte {
	t.Helper()
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)
	writeArchiveEntry := func(name string, data []byte) {
		require.NoError(t, tw.WriteHeader(&tar.Header{Name: name, Typeflag: tar.TypeReg, Size: int64(len(data)), Mode: 0o600}))
		_, err := tw.Write(data)
		require.NoError(t, err)
	}
	writeArchiveEntry("config/config.yaml", []byte(importableConfig))
	for i := range n {
		writeArchiveEntry(fmt.Sprintf("e%05d", i), nil)
	}
	require.NoError(t, tw.Close())
	require.NoError(t, gw.Close())
	return buf.Bytes()
}

func TestImportRefusesArchiveAboveEntryLimit(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	seedRealLayout(t, dir)
	_, r := newImportRouter(t, dir)

	// config.yaml plus MaxImportEntries more entries is one entry over the limit.
	w := httptest.NewRecorder()
	r.ServeHTTP(w, importBytesRequest(t, archiveWithEntryCount(t, crowdsec.MaxImportEntries)))

	require.Equal(t, http.StatusUnprocessableEntity, w.Code, w.Body.String())
	require.Equal(t, "import_archive_too_large", decodeRefusal(t, w).ErrorCode)
}

func TestImportUploadAboveBodyLimit(t *testing.T) {
	// Not parallel: the test redirects the temp directory with t.Setenv.
	tmp := t.TempDir()
	t.Setenv("TMPDIR", tmp)
	dir := t.TempDir()
	seedRealLayout(t, dir)
	_, r := newImportRouter(t, dir)

	// The upload is one byte over the body limit; the content is never inspected.
	w := httptest.NewRecorder()
	r.ServeHTTP(w, importBytesRequest(t, make([]byte, crowdsec.MaxImportCompressedBytes+(1<<20)+1)))

	require.Equal(t, http.StatusRequestEntityTooLarge, w.Code, w.Body.String())
	require.Equal(t, "request_too_large", decodeRefusal(t, w).ErrorCode)
	entries, err := os.ReadDir(tmp)
	require.NoError(t, err)
	require.Empty(t, entries, "no spool file or import directory is left behind")
}

func TestImportRefusesArchiveAboveUncompressedLimit(t *testing.T) {
	t.Parallel()
	validator := &ConfigArchiveValidator{
		MaxSize:             50 << 20,
		MaxUncompressed:     10 << 10,
		MaxCompressionRatio: 100,
		RequiredFiles:       []string{"config.yaml"},
	}
	// Random bytes do not compress, so only the size bound applies.
	content := make([]byte, 20<<10)
	_, err := rand.Read(content)
	require.NoError(t, err)
	archivePath := createTestArchive(t, "tar.gz", map[string]string{"config.yaml": string(content)}, true)

	require.ErrorIs(t, validator.Validate(archivePath), crowdsec.ErrArchiveTooLarge)
}

func TestImportRefusesArchiveAboveCompressionRatio(t *testing.T) {
	t.Parallel()
	validator := &ConfigArchiveValidator{
		MaxSize:             50 << 20,
		MaxUncompressed:     500 << 20,
		MaxCompressionRatio: 10,
		RequiredFiles:       []string{"config.yaml"},
	}
	archivePath := createTestArchive(t, "tar.gz", map[string]string{"config.yaml": fmt.Sprintf("%0100000d", 0)}, true)

	require.ErrorIs(t, validator.Validate(archivePath), crowdsec.ErrArchiveTooLarge)
}
