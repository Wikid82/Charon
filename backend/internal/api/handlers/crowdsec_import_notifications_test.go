package handlers

import (
	"encoding/json"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

const (
	importProfilesNaming = "name: p\nfilters:\n - Alert.Remediation == true\ndecisions:\n - type: ban\n   duration: 4h\nnotifications:\n - %s\n"
	importNotification   = "type: http\nname: %s\nurl: https://example.invalid/\nmethod: POST\n"
)

// treeState captures every entry below root (kind, mode, content, link target) so a test can assert
// that a refused import left the tree byte-identical.
func treeState(t *testing.T, root string) map[string]string {
	t.Helper()
	state := map[string]string{}
	require.NoError(t, filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		require.NoError(t, err)
		rel, relErr := filepath.Rel(root, p)
		require.NoError(t, relErr)
		info, infoErr := d.Info()
		require.NoError(t, infoErr)
		entry := info.Mode().String()
		switch {
		case info.Mode()&os.ModeSymlink != 0:
			target, linkErr := os.Readlink(p)
			require.NoError(t, linkErr)
			entry += " -> " + target
		case info.Mode().IsRegular():
			entry += " " + readTreeFile(t, p)
		}
		state[rel] = entry
		return nil
	}))
	return state
}

func importNotificationBody(t *testing.T, w *httptest.ResponseRecorder) string {
	t.Helper()
	var body map[string]string
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	return body["error"]
}

func TestImportConfigRefusesUndefinedNotificationsBeforeAnyChange(t *testing.T) {
	t.Parallel()
	parent := t.TempDir()
	dir := filepath.Join(parent, "crowdsec")
	require.NoError(t, os.MkdirAll(dir, 0o750))
	seedRealLayout(t, dir)
	writeTree(t, dir, map[string]string{
		"config/profiles.yaml":         "live profiles\n",
		"config/notifications/l.yaml":  "live notification\n",
		"config/notifications/l2.yaml": "live notification 2\n",
	})
	h, r := newImportRouter(t, dir)
	before := treeState(t, dir)
	parentBefore := treeState(t, parent)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, importRequest(t, map[string]string{
		"config/config.yaml":   importableConfig,
		"config/profiles.yaml": strings.Replace(importProfilesNaming, "%s", "x", 1),
	}))

	require.Equal(t, http.StatusUnprocessableEntity, w.Code, w.Body.String())
	require.Equal(t, "config validation failed: profiles.yaml names notifications that have no matching file: x", importNotificationBody(t, w))
	require.Equal(t, before, treeState(t, dir), "the live tree must be byte-identical")
	require.Equal(t, parentBefore, treeState(t, parent), "no snapshot may be created")
	require.True(t, h.dataMu.TryLock(), "the data lock must not be held")
	h.dataMu.Unlock()
}

func TestImportConfigNotificationReferences(t *testing.T) {
	t.Parallel()
	profiles := strings.Replace(importProfilesNaming, "%s", "x", 1)
	cases := []struct {
		name  string
		files map[string]string
		code  int
	}{
		{"satisfied by a notification file", map[string]string{"config/config.yaml": importableConfig, "config/profiles.yaml": profiles, "config/notifications/x.yaml": strings.Replace(importNotification, "%s", "x", 1)}, http.StatusOK},
		{"satisfied by name in a differently named file", map[string]string{"config/config.yaml": importableConfig, "config/profiles.yaml": profiles, "config/notifications/other.yml": strings.Replace(importNotification, "%s", "x", 1)}, http.StatusOK},
		{"defined only by file name", map[string]string{"config/config.yaml": importableConfig, "config/profiles.yaml": profiles, "config/notifications/x.yaml": strings.Replace(importNotification, "%s", "y", 1)}, http.StatusUnprocessableEntity},
		{"legacy archive without profiles", map[string]string{"config/config.yaml": importableConfig}, http.StatusOK},
		{"root layout refused", map[string]string{"config.yaml": importableConfig, "profiles.yaml": profiles}, http.StatusUnprocessableEntity},
		{"redirected paths pass", map[string]string{"config/config.yaml": "api:\n  server:\n    profiles_path: /elsewhere/profiles.yaml\n", "config/profiles.yaml": profiles}, http.StatusOK},
		{"unparsable profiles pass the check", map[string]string{"config/config.yaml": importableConfig, "config/profiles.yaml": "name: [unclosed\n"}, http.StatusOK},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			seedRealLayout(t, dir)
			_, r := newImportRouter(t, dir)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, importRequest(t, tc.files))
			require.Equal(t, tc.code, w.Code, w.Body.String())
			if tc.code == http.StatusOK {
				for rel, content := range tc.files {
					require.Equal(t, content, readTreeFile(t, filepath.Join(dir, rel)))
				}
			}
		})
	}
}

func TestImportConfigNotificationMessageIsBoundedAndSanitized(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	seedRealLayout(t, dir)
	_, r := newImportRouter(t, dir)
	var names []string
	for _, n := range []string{"n01", "n02", "n03", "n04", "n05", "n06", "n07", "n08", "n09", "n10", "n11", "n12"} {
		names = append(names, "- "+n)
	}
	profiles := "name: p\nnotifications:\n " + strings.Join(names, "\n ") + "\n"
	w := httptest.NewRecorder()
	r.ServeHTTP(w, importRequest(t, map[string]string{"config/config.yaml": importableConfig, "config/profiles.yaml": profiles}))
	require.Equal(t, http.StatusUnprocessableEntity, w.Code, w.Body.String())
	msg := importNotificationBody(t, w)
	require.Equal(t, "config validation failed: profiles.yaml names notifications that have no matching file: n01, n02, n03, n04, n05, n06, n07, n08, n09, n10, and 2 more", msg)
	require.NotContains(t, msg, dir)
}
