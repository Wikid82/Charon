package handlers

import (
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

const importPlugin = "type: http\nname: http_default\nurl: https://hooks.example.invalid/notify\nmethod: POST\n"

// exportedWithoutKeys is the archive an export of a tree with a key_path file would hold: the key file
// itself is not part of it.
func exportedWithoutKeys() map[string]string {
	return map[string]string{
		"config/config.yaml":                     importableConfig,
		"config/notifications/http_default.yaml": importPlugin + "key_path: /etc/crowdsec/ssl/client.pem\n",
	}
}

func seedLiveKeys(t *testing.T, dir string) {
	t.Helper()
	seedRealLayout(t, dir)
	writeTree(t, dir, map[string]string{
		"config/notifications/http_default.yaml": importPlugin + "key_path: /etc/crowdsec/ssl/client.pem\n",
		"config/ssl/client.pem":                  "live private material\n",
		"config/ssl/extra.key":                   "live key\n",
		"config/ssl/unrelated.txt":               "gone as before\n",
	})
}

func TestImportConfigOnTheSameHostKeepsTheCurrentKeyFiles(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	seedLiveKeys(t, dir)
	_, r := newImportRouter(t, dir)

	w := importStatus(t, r, exportedWithoutKeys())

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.Contains(t, keptFilesOf(t, w.Body.Bytes()), "config/ssl/client.pem")
	require.Contains(t, keptFilesOf(t, w.Body.Bytes()), "config/ssl/extra.key")
	require.Equal(t, "live private material\n", readTreeFile(t, filepath.Join(dir, "config", "ssl", "client.pem")))
	require.Equal(t, "live key\n", readTreeFile(t, filepath.Join(dir, "config", "ssl", "extra.key")))
	require.NoFileExists(t, filepath.Join(dir, "config", "ssl", "unrelated.txt"))
}

func TestImportConfigArchiveKeyFileWinsOverTheCurrentOne(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	seedLiveKeys(t, dir)
	_, r := newImportRouter(t, dir)
	archive := exportedWithoutKeys()
	archive["config/ssl/client.pem"] = "archive material\n"

	w := importStatus(t, r, archive)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.NotContains(t, keptFilesOf(t, w.Body.Bytes()), "config/ssl/client.pem")
	require.Equal(t, "archive material\n", readTreeFile(t, filepath.Join(dir, "config", "ssl", "client.pem")))
}

func TestImportConfigKeyRestoreFailureRollsBack(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	seedLiveKeys(t, dir)
	for i := range 70 {
		writeTree(t, dir, map[string]string{"config/many/" + strings.Repeat("k", i+1) + ".key": "live"})
	}
	_, r := newImportRouter(t, dir)

	w := importStatus(t, r, exportedWithoutKeys())

	require.Equal(t, http.StatusInternalServerError, w.Code, w.Body.String())
	require.Contains(t, w.Body.String(), msgKeepCurrentFiles)
	require.Equal(t, "live private material\n", readTreeFile(t, filepath.Join(dir, "config", "ssl", "client.pem")))
	require.FileExists(t, filepath.Join(dir, "config", "ssl", "unrelated.txt"))
}
