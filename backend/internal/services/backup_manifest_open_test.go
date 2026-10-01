package services

import (
	"archive/zip"
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestReadManifestEntry_OpenError verifies that an entry whose compression
// method is unsupported surfaces as a wrapped "open manifest entry" error.
func TestReadManifestEntry_OpenError(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.CreateRaw(&zip.FileHeader{Name: "manifest.json", Method: 99})
	require.NoError(t, err)
	_, err = w.Write([]byte("{}"))
	require.NoError(t, err)
	require.NoError(t, zw.Close())

	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	require.NoError(t, err)
	require.Len(t, zr.File, 1)

	parsed, err := readManifestEntry(zr.File[0])
	assert.Nil(t, parsed)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "open manifest entry")
	assert.ErrorIs(t, err, zip.ErrAlgorithm)
}
