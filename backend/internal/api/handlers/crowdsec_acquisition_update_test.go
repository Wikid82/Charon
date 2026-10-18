package handlers

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/crowdsec"
)

func putAcquisition(r http.Handler, content string) *httptest.ResponseRecorder {
	body := fmt.Sprintf(`{"content": %q}`, content)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/acquisition", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	return w
}

func newAcquisitionRouter(t *testing.T) http.Handler {
	t.Helper()
	h := newTestCrowdsecHandler(t, OpenTestDB(t), &fakeExec{}, "/bin/false", t.TempDir())
	r := gin.New()
	r.PUT("/acquisition", h.UpdateAcquisitionConfig)
	return r
}

func TestUpdateAcquisitionConfigConcurrentPutsStayConsistent(t *testing.T) {
	dir := t.TempDir()
	acquis := filepath.Join(dir, "acquis.yaml")
	require.NoError(t, os.WriteFile(acquis, []byte("original\n"), 0o640)) //nolint:gosec // G306: deliberate 0640 to verify mode preservation
	t.Setenv("CHARON_CROWDSEC_ACQUIS_PATH", acquis)
	r := newAcquisitionRouter(t)

	const n = crowdsec.DefaultBackupRetention
	payloads := make([]string, n)
	codes := make([]int, n)
	var wg sync.WaitGroup
	for i := range n {
		payloads[i] = fmt.Sprintf("payload-%d\n%s\n", i, strings.Repeat("x", 4096))
		wg.Add(1)
		go func() {
			defer wg.Done()
			codes[i] = putAcquisition(r, payloads[i]).Code
		}()
	}
	wg.Wait()
	for _, code := range codes {
		require.Equal(t, http.StatusOK, code)
	}

	got, err := os.ReadFile(acquis) //nolint:gosec // G304: t.TempDir-derived path
	require.NoError(t, err)
	require.Contains(t, payloads, string(got))

	backups, err := filepath.Glob(acquis + ".backup.*")
	require.NoError(t, err)
	require.Len(t, backups, n)

	info, err := os.Stat(acquis)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o640), info.Mode().Perm())

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	require.Len(t, entries, n+1, "no temp files may be left behind")
}

func TestUpdateAcquisitionConfigKeepsPreviousFileOnFailure(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("directory permissions do not bind root")
	}
	dir := t.TempDir()
	acquis := filepath.Join(dir, "acquis.yaml")
	require.NoError(t, os.WriteFile(acquis, []byte("original\n"), 0o600))
	t.Setenv("CHARON_CROWDSEC_ACQUIS_PATH", acquis)
	r := newAcquisitionRouter(t)

	require.NoError(t, os.Chmod(dir, 0o500))       //nolint:gosec // G302: read-only directory is the failure under test
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) }) //nolint:gosec // G302: restore directory so TempDir cleanup works

	w := putAcquisition(r, "new\n")
	require.Equal(t, http.StatusInternalServerError, w.Code)

	got, err := os.ReadFile(acquis) //nolint:gosec // G304: t.TempDir-derived path
	require.NoError(t, err)
	require.Equal(t, "original\n", string(got))
}

func TestUpdateAcquisitionConfigCreatesFileWithoutBackupWhenAbsent(t *testing.T) {
	acquis := filepath.Join(t.TempDir(), "acquis.yaml")
	t.Setenv("CHARON_CROWDSEC_ACQUIS_PATH", acquis)
	r := newAcquisitionRouter(t)

	require.Equal(t, http.StatusOK, putAcquisition(r, "fresh\n").Code)

	got, err := os.ReadFile(acquis) //nolint:gosec // G304: t.TempDir-derived path
	require.NoError(t, err)
	require.Equal(t, "fresh\n", string(got))
	backups, err := filepath.Glob(acquis + ".backup.*")
	require.NoError(t, err)
	require.Empty(t, backups)
}

// requireOnlyEntries fails when dir holds anything but the live file (and the given extra names).
func requireOnlyEntries(t *testing.T, dir string, names ...string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	got := make([]string, 0, len(entries))
	for _, e := range entries {
		got = append(got, e.Name())
	}
	require.ElementsMatch(t, names, got)
}

func TestReplaceAcquisitionConfigFallsBackInPlaceOnBusyRename(t *testing.T) {
	for _, errno := range []syscall.Errno{syscall.EBUSY, syscall.EXDEV, syscall.EROFS} {
		t.Run(errno.Error(), func(t *testing.T) {
			dir := t.TempDir()
			acquis := filepath.Join(dir, "acquis.yaml")
			require.NoError(t, os.WriteFile(acquis, []byte("original\n"), 0o640)) //nolint:gosec // G306: deliberate 0640 to verify mode preservation
			orig := renameFile
			renameFile = func(oldpath, newpath string) error {
				return &os.LinkError{Op: "rename", Old: oldpath, New: newpath, Err: errno}
			}
			t.Cleanup(func() { renameFile = orig })

			now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
			backup, err := replaceAcquisitionConfig(acquis, "new\n", now)
			require.NoError(t, err)
			require.Equal(t, crowdsec.BackupFilePath(acquis, now), backup)

			got, err := os.ReadFile(acquis) //nolint:gosec // G304: t.TempDir-derived path
			require.NoError(t, err)
			require.Equal(t, "new\n", string(got))
			saved, err := os.ReadFile(backup) //nolint:gosec // G304: t.TempDir-derived path
			require.NoError(t, err)
			require.Equal(t, "original\n", string(saved))
			requireOnlyEntries(t, dir, "acquis.yaml", filepath.Base(backup))
		})
	}
}

func TestReplaceAcquisitionConfigInPlaceFallbackFailureKeepsBackup(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("file permissions do not bind root")
	}
	dir := t.TempDir()
	acquis := filepath.Join(dir, "acquis.yaml")
	require.NoError(t, os.WriteFile(acquis, []byte("original\n"), 0o400))
	orig := renameFile
	renameFile = func(oldpath, newpath string) error {
		return &os.LinkError{Op: "rename", Old: oldpath, New: newpath, Err: syscall.EBUSY}
	}
	t.Cleanup(func() { renameFile = orig })

	backup, err := replaceAcquisitionConfig(acquis, "new\n", time.Now())
	require.Error(t, err)
	require.Empty(t, backup)
	backups, globErr := filepath.Glob(acquis + ".backup.*")
	require.NoError(t, globErr)
	require.Len(t, backups, 1, "the backup is the only intact copy after a failed in-place write")
	requireOnlyEntries(t, dir, "acquis.yaml", filepath.Base(backups[0]))
}

func TestUpdateAcquisitionConfigFailedReplaceLeaksNoFiles(t *testing.T) {
	dir := t.TempDir()
	acquis := filepath.Join(dir, "acquis.yaml")
	require.NoError(t, os.WriteFile(acquis, []byte("original\n"), 0o600))
	t.Setenv("CHARON_CROWDSEC_ACQUIS_PATH", acquis)
	r := newAcquisitionRouter(t)
	orig := renameFile
	renameFile = func(string, string) error { return errors.New("boom") }
	t.Cleanup(func() { renameFile = orig })

	w := putAcquisition(r, "new\n")
	require.Equal(t, http.StatusInternalServerError, w.Code)

	got, err := os.ReadFile(acquis) //nolint:gosec // G304: t.TempDir-derived path
	require.NoError(t, err)
	require.Equal(t, "original\n", string(got))
	requireOnlyEntries(t, dir, "acquis.yaml")
}

func TestReplaceAcquisitionConfigBackupNameCollisionAdvances(t *testing.T) {
	dir := t.TempDir()
	acquis := filepath.Join(dir, "acquis.yaml")
	require.NoError(t, os.WriteFile(acquis, []byte("original\n"), 0o600))
	now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	taken := crowdsec.BackupFilePath(acquis, now)
	require.NoError(t, os.WriteFile(taken, []byte("older backup\n"), 0o600))

	backup, err := replaceAcquisitionConfig(acquis, "new\n", now)
	require.NoError(t, err)
	require.Equal(t, crowdsec.BackupFilePath(acquis, now.Add(time.Microsecond)), backup)

	kept, err := os.ReadFile(taken) //nolint:gosec // G304: t.TempDir-derived path
	require.NoError(t, err)
	require.Equal(t, "older backup\n", string(kept), "an existing backup must never be overwritten")
	saved, err := os.ReadFile(backup) //nolint:gosec // G304: t.TempDir-derived path
	require.NoError(t, err)
	require.Equal(t, "original\n", string(saved))
}

func TestReplaceAcquisitionConfigBackupCollisionExhausted(t *testing.T) {
	dir := t.TempDir()
	acquis := filepath.Join(dir, "acquis.yaml")
	require.NoError(t, os.WriteFile(acquis, []byte("original\n"), 0o600))
	now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	names := []string{"acquis.yaml"}
	for i := range backupCreateAttempts {
		p := crowdsec.BackupFilePath(acquis, now.Add(time.Duration(i)*time.Microsecond))
		require.NoError(t, os.WriteFile(p, []byte("x"), 0o600))
		names = append(names, filepath.Base(p))
	}

	backup, err := replaceAcquisitionConfig(acquis, "new\n", now)
	require.ErrorContains(t, err, "write acquisition backup")
	require.Empty(t, backup)
	requireOnlyEntries(t, dir, names...)
	got, err := os.ReadFile(acquis) //nolint:gosec // G304: t.TempDir-derived path
	require.NoError(t, err)
	require.Equal(t, "original\n", string(got))
}

func TestReplaceAcquisitionConfigReadError(t *testing.T) {
	dir := t.TempDir()
	acquis := filepath.Join(dir, "acquis.yaml")
	require.NoError(t, os.Mkdir(acquis, 0o700)) // a directory: stat succeeds, read fails

	backup, err := replaceAcquisitionConfig(acquis, "new\n", time.Now())
	require.ErrorContains(t, err, "read acquisition config")
	require.Empty(t, backup)
}

func TestReplaceAcquisitionConfigStatError(t *testing.T) {
	dir := t.TempDir()
	notDir := filepath.Join(dir, "file")
	require.NoError(t, os.WriteFile(notDir, []byte("x"), 0o600))

	_, err := replaceAcquisitionConfig(filepath.Join(notDir, "acquis.yaml"), "new\n", time.Now())
	require.ErrorContains(t, err, "stat acquisition config")
}

func TestReplaceAcquisitionConfigTempCreateError(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing-dir", "acquis.yaml")

	_, err := replaceAcquisitionConfig(missing, "new\n", time.Now())
	require.ErrorContains(t, err, "create temporary acquisition config")
}

func TestUpdateAcquisitionConfigCancelledRequestDoesNotWrite(t *testing.T) {
	dir := t.TempDir()
	acquis := filepath.Join(dir, "acquis.yaml")
	require.NoError(t, os.WriteFile(acquis, []byte("original\n"), 0o600))
	t.Setenv("CHARON_CROWDSEC_ACQUIS_PATH", acquis)
	r := newAcquisitionRouter(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	req := httptest.NewRequest(http.MethodPut, "/acquisition", strings.NewReader(`{"content":"new\n"}`)).WithContext(ctx)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Empty(t, w.Body.String(), "an aborted request writes no response body")
	got, err := os.ReadFile(acquis) //nolint:gosec // G304: t.TempDir-derived path
	require.NoError(t, err)
	require.Equal(t, "original\n", string(got))
	requireOnlyEntries(t, dir, "acquis.yaml")
}
