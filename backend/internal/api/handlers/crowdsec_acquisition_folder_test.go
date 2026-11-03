package handlers

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestAcquisitionNotWritableMessageLeadsWithFolder pins the wording the save failure shows: the folder is named
// before the file's own mode, and the text says a read-only file alone does not block a save.
func TestAcquisitionNotWritableMessageLeadsWithFolder(t *testing.T) {
	folderAt := strings.Index(msgAcquisitionNotWritable, "folder")
	noBlockAt := strings.Index(msgAcquisitionNotWritable, "A read-only file alone does not block a save")

	require.Positive(t, folderAt, msgAcquisitionNotWritable)
	require.Positive(t, noBlockAt, msgAcquisitionNotWritable)
	require.Less(t, folderAt, noBlockAt, "the folder is named before the file's mode is ruled out")
	require.NotContains(t, msgAcquisitionNotWritable, "owner and mode", "the message must not point at the file's mode as the cause")
}

// TestSaveIntoReadOnlyFileInWritableFolderSucceeds is the fact the save-failure text rests on: a save replaces the
// file by renaming a new copy over it, so a 0444 file in a writable folder still saves. The check holds for root too,
// since root writes past the mode.
func TestSaveIntoReadOnlyFileInWritableFolderSucceeds(t *testing.T) {
	dir := t.TempDir()
	acquis := filepath.Join(dir, "acquis.yaml")
	require.NoError(t, os.WriteFile(acquis, []byte("original\n"), 0o444)) //nolint:gosec // G306: deliberate read-only file to prove a save is not blocked by it
	t.Setenv("CHARON_CROWDSEC_ACQUIS_PATH", acquis)
	r := newAcquisitionRouter(t)

	w := putAcquisition(r, "replaced\n")

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	got, err := os.ReadFile(filepath.Clean(acquis))
	require.NoError(t, err)
	require.Equal(t, "replaced\n", string(got))
}
