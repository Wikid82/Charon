package handlers

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

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
	require.NoError(t, os.WriteFile(acquis, []byte("original\n"), 0o640))
	t.Setenv("CHARON_CROWDSEC_ACQUIS_PATH", acquis)
	r := newAcquisitionRouter(t)

	const n = crowdsec.DefaultBackupRetention
	payloads := make([]string, n)
	var wg sync.WaitGroup
	for i := range n {
		payloads[i] = fmt.Sprintf("payload-%d\n%s\n", i, strings.Repeat("x", 4096))
		wg.Add(1)
		go func() {
			defer wg.Done()
			require.Equal(t, http.StatusOK, putAcquisition(r, payloads[i]).Code)
		}()
	}
	wg.Wait()

	got, err := os.ReadFile(acquis)
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

	require.NoError(t, os.Chmod(dir, 0o500))
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })

	w := putAcquisition(r, "new\n")
	require.Equal(t, http.StatusInternalServerError, w.Code)

	got, err := os.ReadFile(acquis)
	require.NoError(t, err)
	require.Equal(t, "original\n", string(got))
}

func TestUpdateAcquisitionConfigCreatesFileWithoutBackupWhenAbsent(t *testing.T) {
	acquis := filepath.Join(t.TempDir(), "acquis.yaml")
	t.Setenv("CHARON_CROWDSEC_ACQUIS_PATH", acquis)
	r := newAcquisitionRouter(t)

	require.Equal(t, http.StatusOK, putAcquisition(r, "fresh\n").Code)

	got, err := os.ReadFile(acquis)
	require.NoError(t, err)
	require.Equal(t, "fresh\n", string(got))
	backups, err := filepath.Glob(acquis + ".backup.*")
	require.NoError(t, err)
	require.Empty(t, backups)
}
