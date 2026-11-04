package crowdsec

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/logger"
)

// Each skipped non-regular entry gets one Debug line, and the export ends with one Info line carrying the total.
func TestWriteExportArchiveLogsSkippedSymlinksOnce(t *testing.T) {
	dir := t.TempDir()
	writeExportTree(t, dir, map[string]string{"a.yaml": "a", "z.yaml": "z"})
	for _, name := range []string{"l1.yaml", "l2.yaml", "l3.yaml"} {
		require.NoError(t, os.Symlink("/nonexistent/target", filepath.Join(dir, name)))
	}

	var logs bytes.Buffer
	logger.Init(true, &logs)
	t.Cleanup(func() { logger.Init(false, io.Discard) })

	var archive bytes.Buffer
	_, _, err := WriteExportArchive(context.Background(), dir, &archive)
	require.NoError(t, err)

	out := logs.String()
	assert.Equal(t, 3, strings.Count(out, "skipping non-regular entry"), "one Debug line per skipped entry")
	assert.Equal(t, 1, strings.Count(out, "skipped 3 non-regular"), "exactly one summary line with the count")
	assert.NotContains(t, out, "level=warning", "skipped links are not warnings")

	summary := ""
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, "skipped 3 non-regular") {
			summary = line
		}
	}
	assert.Contains(t, summary, "level=info")
	assert.Equal(t, []string{"a.yaml", "z.yaml"}, exportNames(t, archive.Bytes()), "the archive keeps only regular files")
}

// An export with no skipped entries writes no summary line.
func TestWriteExportArchiveNoSummaryWhenNothingSkipped(t *testing.T) {
	dir := t.TempDir()
	writeExportTree(t, dir, map[string]string{"a.yaml": "a"})

	var logs bytes.Buffer
	logger.Init(true, &logs)
	t.Cleanup(func() { logger.Init(false, io.Discard) })

	var archive bytes.Buffer
	_, _, err := WriteExportArchive(context.Background(), dir, &archive)
	require.NoError(t, err)
	assert.NotContains(t, logs.String(), "non-regular")
}
