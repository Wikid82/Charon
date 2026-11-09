package handlers

import (
	"fmt"
	"io/fs"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// TestRespondImportWriteFailure pins the answer for every write stage of an import: only a permission or
// read-only failure in the configuration folder gets the coded answer; everything else keeps its stage text,
// and every failure logs the stage name.
func TestRespondImportWriteFailure(t *testing.T) {
	causes := map[string]error{
		"permission": fmt.Errorf("remove config/x: %w", fs.ErrPermission),
		"read-only":  fmt.Errorf("remove config/x: %w", syscall.EROFS),
		"disk full":  fmt.Errorf("write config/x: %w", syscall.ENOSPC),
	}
	stages := []struct {
		stage        importStage
		configFolder bool
	}{
		{importStageTempDir, false},
		{importStageUpload, false},
		{importStageBackup, true},
		{importStageClear, true},
		{importStageExtract, true},
	}
	for _, st := range stages {
		for causeName, cause := range causes {
			t.Run(st.stage.name+"/"+causeName, func(t *testing.T) {
				buf := captureLogs(t)
				gin.SetMode(gin.TestMode)
				w := httptest.NewRecorder()
				c, _ := gin.CreateTestContext(w)

				respondImportWriteFailure(c, st.stage, cause)

				require.Equal(t, http.StatusInternalServerError, w.Code, w.Body.String())
				require.Contains(t, buf.String(), `"stage":"`+st.stage.name+`"`)
				coded := st.stage.configFolder && causeName != "disk full"
				if coded {
					require.JSONEq(t, `{"error":"`+msgImportConfigNotWritable+`","error_code":"import_config_not_writable"}`, w.Body.String())
					return
				}
				require.JSONEq(t, `{"error":"`+st.stage.text+`"}`, w.Body.String())
				require.NotContains(t, w.Body.String(), "error_code")
			})
		}
	}
}

// TestImportConfigClearPermissionFailureAnswersNotWritable forces the clear step to fail with a permission error
// (through the clearConfigKeeping seam, so it works for root too) and checks the coded answer, the rollback, and
// the logged stage and cause.
func TestImportConfigClearPermissionFailureAnswersNotWritable(t *testing.T) {
	dir := t.TempDir()
	seedEngineState(t, dir)
	orig := clearConfigKeeping
	clearConfigKeeping = func(string, func(string) bool) error {
		return fmt.Errorf("empty data dir: %w", fs.ErrPermission)
	}
	t.Cleanup(func() { clearConfigKeeping = orig })
	buf := captureLogs(t)
	_, r := newImportRouter(t, dir)

	w := importStatus(t, r, map[string]string{"config.yaml": importableConfig})

	require.Equal(t, http.StatusInternalServerError, w.Code, w.Body.String())
	body := decodeRefusal(t, w)
	require.Equal(t, "import_config_not_writable", body.ErrorCode)
	require.Equal(t, msgImportConfigNotWritable, body.Error)
	require.Contains(t, buf.String(), `"stage":"clear"`)
	require.Contains(t, buf.String(), "permission denied")
	assertEngineStateIntact(t, dir)
}

// TestImportConfigLockedEntryKeepsTheFolderComplete makes a real locked directory stop the clear partway: the
// refusal is unchanged and the rollback still returns the whole folder to its state before the import.
func TestImportConfigLockedEntryKeepsTheFolderComplete(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("permission-based failure cannot be provoked as root")
	}
	dir := t.TempDir()
	seedRealLayout(t, dir)
	writeTree(t, dir, map[string]string{
		"config/config.yaml":         "live: true\n",
		"config/zz-locked/f.yaml":    "live",
		"config/zz-locked/keep.yaml": "live",
	})
	before := countConfigFiles(t, dir)
	locked := filepath.Join(dir, "config", "zz-locked")
	require.NoError(t, os.Chmod(locked, 0o500))       //nolint:gosec // G302: test needs a restricted dir
	t.Cleanup(func() { _ = os.Chmod(locked, 0o700) }) //nolint:gosec // G302: restore owner access so cleanup can remove it
	_, r := newImportRouter(t, dir)

	w := importStatus(t, r, map[string]string{"config/config.yaml": importableConfig})

	require.Equal(t, http.StatusInternalServerError, w.Code, w.Body.String())
	require.Equal(t, "import_config_not_writable", decodeRefusal(t, w).ErrorCode)
	require.Equal(t, before, countConfigFiles(t, dir), "the rollback restores every entry the failed clear removed")
	require.Equal(t, "live: true\n", readTreeFile(t, filepath.Join(dir, "config", "config.yaml")))
	require.Equal(t, "filenames: [x]\n", readTreeFile(t, filepath.Join(dir, "config", "acquis.yaml")))
}

// countConfigFiles counts the regular files under the configuration folder.
func countConfigFiles(t *testing.T, dir string) int {
	t.Helper()
	count := 0
	require.NoError(t, filepath.WalkDir(filepath.Join(dir, "config"), func(_ string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.Type().IsRegular() {
			count++
		}
		return nil
	}))
	return count
}

// TestImportConfigClearFailureWithoutAccessCauseKeepsStageText covers a clear failure that is not a permission
// problem: the answer keeps the stage text and carries no code.
func TestImportConfigClearFailureWithoutAccessCauseKeepsStageText(t *testing.T) {
	dir := t.TempDir()
	seedEngineState(t, dir)
	orig := clearConfigKeeping
	clearConfigKeeping = func(string, func(string) bool) error {
		return fmt.Errorf("empty data dir: %w", syscall.EIO)
	}
	t.Cleanup(func() { clearConfigKeeping = orig })
	_, r := newImportRouter(t, dir)

	w := importStatus(t, r, map[string]string{"config.yaml": importableConfig})

	require.Equal(t, http.StatusInternalServerError, w.Code, w.Body.String())
	require.JSONEq(t, `{"error":"failed to clear the CrowdSec configuration"}`, w.Body.String())
	assertEngineStateIntact(t, dir)
}

// TestImportConfigNotWritableMessageNamesTheFolder pins that the coded text points at the folder and not at the
// file.
func TestImportConfigNotWritableMessageNamesTheFolder(t *testing.T) {
	require.True(t, strings.HasPrefix(msgImportConfigNotWritable, "Charon cannot change the CrowdSec configuration folder"))
	require.NotContains(t, msgImportConfigNotWritable, "failed to create config dir")
}

// TestImportConfigReadOnlyFolderIsCoded uses a real read-only config folder (the directory cannot be emptied)
// and checks the coded answer; skipped for root, where permission checks do not apply.
func TestImportConfigReadOnlyFolderIsCoded(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("permission checks do not apply to root")
	}
	dir := t.TempDir()
	seedEngineState(t, dir)
	locked := filepath.Join(dir, "locked")
	writeTree(t, locked, map[string]string{"f.yaml": "x"})
	require.NoError(t, os.Chmod(locked, 0o500))       // #nosec G302 -- test fixture
	t.Cleanup(func() { _ = os.Chmod(locked, 0o750) }) // #nosec G302 -- test fixture
	_, r := newImportRouter(t, dir)

	w := importStatus(t, r, map[string]string{"config.yaml": importableConfig})

	require.Equal(t, http.StatusInternalServerError, w.Code, w.Body.String())
	require.Equal(t, "import_config_not_writable", decodeRefusal(t, w).ErrorCode)
}

// TestImportConfigTempDirFailureKeepsStageText points the temp directory at a regular file so the import cannot
// create its work folder; the answer keeps the stage text and carries no code.
func TestImportConfigTempDirFailureKeepsStageText(t *testing.T) {
	blocker := filepath.Join(t.TempDir(), "blocker")
	require.NoError(t, os.WriteFile(blocker, []byte("x"), 0o600))
	t.Setenv("TMPDIR", blocker)
	_, r := newImportRouter(t, t.TempDir())

	w := importStatus(t, r, map[string]string{"config.yaml": importableConfig})

	require.Equal(t, http.StatusInternalServerError, w.Code, w.Body.String())
	require.JSONEq(t, `{"error":"failed to create temp dir"}`, w.Body.String())
}

// TestImportConfigUploadFailureKeepsStageText forces the upload write to fail (through the saveUploadedFile seam);
// the answer keeps the upload stage text and carries no code.
func TestImportConfigUploadFailureKeepsStageText(t *testing.T) {
	orig := saveUploadedFile
	saveUploadedFile = func(*gin.Context, *multipart.FileHeader, string, ...fs.FileMode) error {
		return fmt.Errorf("write upload: %w", syscall.ENOSPC)
	}
	t.Cleanup(func() { saveUploadedFile = orig })
	_, r := newImportRouter(t, t.TempDir())

	w := importStatus(t, r, map[string]string{"config.yaml": importableConfig})

	require.Equal(t, http.StatusInternalServerError, w.Code, w.Body.String())
	require.JSONEq(t, `{"error":"failed to save upload"}`, w.Body.String())
}
