package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

type importRefusal struct {
	Error     string   `json:"error"`
	ErrorCode string   `json:"error_code"`
	Names     []string `json:"names"`
}

func decodeRefusal(t *testing.T, w *httptest.ResponseRecorder) importRefusal {
	t.Helper()
	var body importRefusal
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body), w.Body.String())
	return body
}

func TestImportRefusalFlatArchiveCarriesACode(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	seedRealLayout(t, dir)
	_, r := newImportRouter(t, dir)

	w := importStatus(t, r, map[string]string{"config.yaml": importableConfig})

	require.Equal(t, http.StatusUnprocessableEntity, w.Code, w.Body.String())
	body := decodeRefusal(t, w)
	require.Equal(t, "import_flat_archive", body.ErrorCode)
	require.Equal(t, msgImportFlatArchive, body.Error)
}

func TestImportRefusalKeptProfileUndefinedListsNamesAsAnArray(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	seedRealLayout(t, dir)
	writeTree(t, dir, map[string]string{"config/profiles.yaml": strings.Replace(importProfilesNaming, "%s", "ghost", 1)})
	_, r := newImportRouter(t, dir)

	w := importStatus(t, r, map[string]string{"config/config.yaml": importableConfig})

	require.Equal(t, http.StatusUnprocessableEntity, w.Code, w.Body.String())
	body := decodeRefusal(t, w)
	require.Equal(t, "import_kept_profile_undefined", body.ErrorCode)
	require.Equal(t, []string{"ghost"}, body.Names)
	require.Equal(t, msgKeptProfileUndefined+"ghost", body.Error)
}

func TestImportRefusalKeptProfileDuplicateCarriesACode(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	seedRealLayout(t, dir)
	writeTree(t, dir, map[string]string{
		"config/profiles.yaml":            strings.Replace(importProfilesNaming, "%s", "alpha", 1),
		"config/notifications/alpha.yaml": strings.Replace(importNotification, "%s", "alpha", 1) + "---\n" + strings.Replace(importNotification, "%s", "beta", 1),
	})
	_, r := newImportRouter(t, dir)

	w := importStatus(t, r, map[string]string{
		"config/config.yaml":          importableConfig,
		"config/notifications/b.yaml": strings.Replace(importNotification, "%s", "beta", 1),
	})

	require.Equal(t, http.StatusUnprocessableEntity, w.Code, w.Body.String())
	body := decodeRefusal(t, w)
	require.Equal(t, "import_kept_profile_duplicate", body.ErrorCode)
	require.Equal(t, msgKeptProfileDuplicate, body.Error)
	require.Empty(t, body.Names)
}

func TestImportRefusalKeepCurrentFailedCarriesACode(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	seedRealLayout(t, dir)
	h, r := newImportRouter(t, dir)
	h.mergeNotifications = func(string, string) error { return errors.New("disk problem") }

	w := importStatus(t, r, map[string]string{"config/config.yaml": importableConfig})

	require.Equal(t, http.StatusInternalServerError, w.Code, w.Body.String())
	body := decodeRefusal(t, w)
	require.Equal(t, "import_keep_current_failed", body.ErrorCode)
	require.Equal(t, msgKeepCurrentFiles, body.Error)
}

func TestImportRefusalConfigNotFoundCarriesACode(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	// A dangling link keeps the archive's config.yaml from being written and leaves nothing to read.
	require.NoError(t, os.Symlink(filepath.Join(t.TempDir(), "missing.yaml"), filepath.Join(dir, "config.yaml")))
	_, r := newImportRouter(t, dir)

	w := importStatus(t, r, map[string]string{"config.yaml": importableConfig})

	require.Equal(t, http.StatusUnprocessableEntity, w.Code, w.Body.String())
	body := decodeRefusal(t, w)
	require.Equal(t, "import_config_not_found", body.ErrorCode)
	require.Contains(t, body.Error, "config.yaml was not found")
}

func TestImportRefusalConfigMergeFailureRollsBack(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	seedRealLayout(t, dir)
	live := filepath.Join(dir, "config", "config.yaml")
	before, err := os.ReadFile(live) //nolint:gosec // G304: path inside the test directory
	require.NoError(t, err)
	h, r := newImportRouter(t, dir)
	h.mergeConfig = func(string, string) error { return errors.New("disk problem") }

	w := importStatus(t, r, map[string]string{"config/config.yaml": importableConfig})

	require.Equal(t, http.StatusInternalServerError, w.Code, w.Body.String())
	body := decodeRefusal(t, w)
	require.Equal(t, "import_keep_current_failed", body.ErrorCode)
	after, err := os.ReadFile(live) //nolint:gosec // G304: path inside the test directory
	require.NoError(t, err)
	require.Equal(t, string(before), string(after), "the current configuration is restored")
}
