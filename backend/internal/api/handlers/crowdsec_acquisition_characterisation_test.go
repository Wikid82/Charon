package handlers

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/crowdsec"
)

// These characterisation tests pin how the acquisition GET and PUT handlers answer today: the happy path, the
// missing-file and unreadable-file cases, and the read-only and failed-write cases. Permission failures still answer
// a generic 500 with no error_code; the #1708 fix changes that, so these tests are the baseline for its diff.

// newAcquisitionCharacterisationRouter serves GET and PUT on one router with the acquisition file at acquis.
func newAcquisitionCharacterisationRouter(t *testing.T, acquis string) *gin.Engine {
	t.Helper()
	t.Setenv("CHARON_CROWDSEC_ACQUIS_PATH", acquis)
	h := newTestCrowdsecHandler(t, OpenTestDB(t), &fakeExec{}, "/bin/false", t.TempDir())
	r := gin.New()
	r.GET("/acquisition", h.GetAcquisitionConfig)
	r.PUT("/acquisition", h.UpdateAcquisitionConfig)
	return r
}

func getAcquisitionCharacterisation(r http.Handler) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/acquisition", http.NoBody))
	return w
}

// requireGenericAcquisitionError pins the current 500 shape: the generic message and no error_code.
func requireGenericAcquisitionError(t *testing.T, w *httptest.ResponseRecorder, msg string) {
	t.Helper()
	require.Equal(t, http.StatusInternalServerError, w.Code, w.Body.String())
	require.JSONEq(t, fmt.Sprintf(`{"error":%q}`, msg), w.Body.String())
	require.NotContains(t, w.Body.String(), "error_code")
}

// requireNotRoot skips a test that needs the directory or file mode to bind, which root ignores.
func requireNotRoot(t *testing.T) {
	t.Helper()
	if os.Geteuid() == 0 {
		t.Skip("file and directory permissions do not bind root")
	}
}

func TestAcquisitionCharacterisationGetHappyPathAndMissingFile(t *testing.T) {
	acquis := filepath.Join(t.TempDir(), "acquis.yaml")
	r := newAcquisitionCharacterisationRouter(t, acquis)

	w := getAcquisitionCharacterisation(r)
	require.Equal(t, http.StatusNotFound, w.Code)
	require.JSONEq(t, `{"error":"acquisition config not found"}`, w.Body.String())

	require.NoError(t, os.WriteFile(acquis, []byte("source: file\n"), 0o600))
	w = getAcquisitionCharacterisation(r)
	require.Equal(t, http.StatusOK, w.Code)
	require.JSONEq(t, `{"content":"source: file\n"}`, w.Body.String())
}

func TestAcquisitionCharacterisationPutHappyPathBody(t *testing.T) {
	t.Run("existing file is backed up and named in the response", func(t *testing.T) {
		dir := t.TempDir()
		acquis := filepath.Join(dir, "acquis.yaml")
		require.NoError(t, os.WriteFile(acquis, []byte("old\n"), 0o600))
		r := newAcquisitionCharacterisationRouter(t, acquis)

		w := putAcquisition(r, "new\n")
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())

		var resp map[string]any
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		require.Equal(t, "updated", resp["status"])
		require.Equal(t, true, resp["reload_hint"])
		backup, ok := resp["backup"].(string)
		require.True(t, ok, "backup is a string")
		require.True(t, strings.HasPrefix(backup, "acquis.yaml.backup."), backup)
		require.NotContains(t, backup, dir, "the response names the backup, never its absolute path")

		saved, err := os.ReadFile(filepath.Join(dir, backup)) //nolint:gosec // G304: t.TempDir-derived path
		require.NoError(t, err)
		require.Equal(t, "old\n", string(saved))
	})

	t.Run("absent file has no backup", func(t *testing.T) {
		acquis := filepath.Join(t.TempDir(), "acquis.yaml")
		r := newAcquisitionCharacterisationRouter(t, acquis)

		w := putAcquisition(r, "fresh\n")
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		require.JSONEq(t, `{"status":"updated","backup":"","reload_hint":true}`, w.Body.String())
	})
}

// TestAcquisitionCharacterisationGetDirectoryIsGeneric500 pins that a path that is not a readable file answers the
// generic 500 with no error_code (a directory reads as an error, not as not-found).
func TestAcquisitionCharacterisationGetDirectoryIsGeneric500(t *testing.T) {
	acquis := filepath.Join(t.TempDir(), "acquis.yaml")
	require.NoError(t, os.Mkdir(acquis, 0o700))
	r := newAcquisitionCharacterisationRouter(t, acquis)

	requireGenericAcquisitionError(t, getAcquisitionCharacterisation(r), "failed to read acquisition config")
}

func TestAcquisitionCharacterisationGetUnreadableFileIsGeneric500(t *testing.T) {
	requireNotRoot(t)
	acquis := filepath.Join(t.TempDir(), "acquis.yaml")
	require.NoError(t, os.WriteFile(acquis, []byte("secret\n"), 0o000)) //nolint:gosec // G306: deliberately unreadable file under test
	r := newAcquisitionCharacterisationRouter(t, acquis)

	requireGenericAcquisitionError(t, getAcquisitionCharacterisation(r), "failed to read acquisition config")
}

func TestAcquisitionCharacterisationPutReadOnlyFolderIsGeneric500(t *testing.T) {
	requireNotRoot(t)
	dir := t.TempDir()
	acquis := filepath.Join(dir, "acquis.yaml")
	require.NoError(t, os.WriteFile(acquis, []byte("original\n"), 0o600))
	r := newAcquisitionCharacterisationRouter(t, acquis)
	require.NoError(t, os.Chmod(dir, 0o500))       //nolint:gosec // G302: read-only directory is the failure under test
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) }) //nolint:gosec // G302: restore directory so TempDir cleanup works

	requireGenericAcquisitionError(t, putAcquisition(r, "new\n"), "failed to write acquisition config")
	got, err := os.ReadFile(acquis) //nolint:gosec // G304: t.TempDir-derived path
	require.NoError(t, err)
	require.Equal(t, "original\n", string(got))
	requireOnlyEntries(t, dir, "acquis.yaml")
}

// TestAcquisitionCharacterisationPutWriteFailuresAreGeneric500 runs without root and without file modes, through
// the existing seams, so the generic 500 is pinned on every platform.
func TestAcquisitionCharacterisationPutWriteFailuresAreGeneric500(t *testing.T) {
	for name, fail := range map[string]error{
		"permission denied on replace": fmt.Errorf("replace acquisition config: %w", fs.ErrPermission),
		"read-only mount on replace":   fmt.Errorf("replace acquisition config: %w", syscall.EROFS),
		"other rename failure":         fmt.Errorf("replace acquisition config: %w", &os.LinkError{Op: "rename", Err: syscall.EXDEV}),
	} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			acquis := filepath.Join(dir, "acquis.yaml")
			require.NoError(t, os.WriteFile(acquis, []byte("original\n"), 0o600))
			r := newAcquisitionCharacterisationRouter(t, acquis)
			stubWriteFileAtomic(t, func(string, []byte, fs.FileMode, crowdsec.AtomicWriteOptions) (crowdsec.AtomicWriteResult, error) {
				return crowdsec.AtomicWriteResult{}, fail
			})

			requireGenericAcquisitionError(t, putAcquisition(r, "new\n"), "failed to write acquisition config")
			got, err := os.ReadFile(acquis) //nolint:gosec // G304: t.TempDir-derived path
			require.NoError(t, err)
			require.Equal(t, "original\n", string(got))
			requireOnlyEntries(t, dir, "acquis.yaml")
		})
	}
}

// TestAcquisitionCharacterisationPutPartialInPlaceWriteKeepsBackup pins that a half-written live file answers the
// generic 500 and keeps the backup, the only intact copy.
func TestAcquisitionCharacterisationPutPartialInPlaceWriteKeepsBackup(t *testing.T) {
	dir := t.TempDir()
	acquis := filepath.Join(dir, "acquis.yaml")
	require.NoError(t, os.WriteFile(acquis, []byte("original\n"), 0o600))
	r := newAcquisitionCharacterisationRouter(t, acquis)
	stubWriteFileAtomic(t, func(string, []byte, fs.FileMode, crowdsec.AtomicWriteOptions) (crowdsec.AtomicWriteResult, error) {
		return crowdsec.AtomicWriteResult{}, fmt.Errorf("write in place: %w", crowdsec.ErrPartialInPlaceWrite)
	})

	requireGenericAcquisitionError(t, putAcquisition(r, "new\n"), "failed to write acquisition config")
	backups, err := filepath.Glob(acquis + ".backup.*")
	require.NoError(t, err)
	require.Len(t, backups, 1)
}

// TestAcquisitionCharacterisationPutBackupCreateFailureIsGeneric500 forces the backup create step to fail through its
// seam: the live file stays as it was and no backup is left.
func TestAcquisitionCharacterisationPutBackupCreateFailureIsGeneric500(t *testing.T) {
	dir := t.TempDir()
	acquis := filepath.Join(dir, "acquis.yaml")
	require.NoError(t, os.WriteFile(acquis, []byte("original\n"), 0o600))
	r := newAcquisitionCharacterisationRouter(t, acquis)
	origOpen := openAcquisitionBackupFile
	t.Cleanup(func() { openAcquisitionBackupFile = origOpen })
	openAcquisitionBackupFile = func(string, fs.FileMode) (acquisitionFile, error) {
		return nil, fmt.Errorf("create backup: %w", fs.ErrPermission)
	}

	requireGenericAcquisitionError(t, putAcquisition(r, "new\n"), "failed to write acquisition config")
	got, err := os.ReadFile(acquis) //nolint:gosec // G304: t.TempDir-derived path
	require.NoError(t, err)
	require.Equal(t, "original\n", string(got))
	requireOnlyEntries(t, dir, "acquis.yaml")
}
