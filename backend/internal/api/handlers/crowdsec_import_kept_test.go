package handlers

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Wikid82/charon/backend/internal/crowdsec"
	"github.com/stretchr/testify/require"
)

func keptFilesOf(t *testing.T, body []byte) []string {
	t.Helper()
	var resp struct {
		Status    string   `json:"status"`
		KeptFiles []string `json:"kept_files"`
	}
	require.NoError(t, json.Unmarshal(body, &resp))
	require.Equal(t, "imported", resp.Status)
	return resp.KeptFiles
}

// seedLiveCompanions adds the four required companions to a real-layout tree.
func seedLiveCompanions(t *testing.T, dir string) {
	t.Helper()
	writeTree(t, dir, map[string]string{
		"config/acquis.yaml":     "live-acquis",
		"config/profiles.yaml":   "live-profiles\n",
		"config/simulation.yaml": "live-sim",
		"config/patterns/linux":  "live-pattern",
	})
}

func TestImportConfigKeepsCurrentCompanionsTheArchiveLacks(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	seedRealLayout(t, dir)
	seedLiveCompanions(t, dir)
	_, r := newImportRouter(t, dir)

	w := importStatus(t, r, map[string]string{"config/config.yaml": importableConfig})

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.Equal(t, []string{"acquis.yaml", "patterns", "profiles.yaml", "simulation.yaml"}, keptFilesOf(t, w.Body.Bytes()))
	require.Equal(t, "live-acquis", readTreeFile(t, filepath.Join(dir, "config", "acquis.yaml")))
	require.Equal(t, "live-profiles\n", readTreeFile(t, filepath.Join(dir, "config", "profiles.yaml")))
	require.Equal(t, "live-sim", readTreeFile(t, filepath.Join(dir, "config", "simulation.yaml")))
	require.Equal(t, "live-pattern", readTreeFile(t, filepath.Join(dir, "config", "patterns", "linux")))
}

func TestImportConfigArchiveCompanionsWinAndAreNotListed(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	seedRealLayout(t, dir)
	seedLiveCompanions(t, dir)
	_, r := newImportRouter(t, dir)

	w := importStatus(t, r, map[string]string{
		"config/config.yaml":     importableConfig,
		"config/acquis.yaml":     "archive-acquis",
		"config/profiles.yaml":   "name: p\nfilters:\n - Alert.Remediation == true\ndecisions:\n - type: ban\n   duration: 4h\n",
		"config/patterns/custom": "archive-pattern",
	})

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.Equal(t, []string{"simulation.yaml"}, keptFilesOf(t, w.Body.Bytes()))
	require.Equal(t, "archive-acquis", readTreeFile(t, filepath.Join(dir, "config", "acquis.yaml")))
	require.NoFileExists(t, filepath.Join(dir, "config", "patterns", "linux"))
}

func TestImportConfigWithoutCompanionsToKeepOmitsKeptFiles(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	seedRealLayout(t, dir)
	require.NoError(t, os.Remove(filepath.Join(dir, "config", "acquis.yaml")))
	_, r := newImportRouter(t, dir)

	w := importStatus(t, r, map[string]string{"config/config.yaml": importableConfig})

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var body map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.NotContains(t, body, "kept_files")
}

func TestImportConfigFlatLiveTreeWithNestedArchive(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeTree(t, dir, map[string]string{"config.yaml": "old: true\n", "acquis.yaml": "flat-acquis"})
	_, r := newImportRouter(t, dir)

	w := importStatus(t, r, map[string]string{"config/config.yaml": importableConfig})

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.Equal(t, []string{"acquis.yaml"}, keptFilesOf(t, w.Body.Bytes()))
	require.Equal(t, "flat-acquis", readTreeFile(t, filepath.Join(dir, "acquis.yaml")))
	require.NoFileExists(t, filepath.Join(dir, "config", "acquis.yaml"))
}

func TestImportConfigLinkedCompanionIsKeptUntouched(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	seedRealLayout(t, dir)
	outside := t.TempDir()
	writeTree(t, outside, map[string]string{"profiles.yaml": "outside-profiles", "patterns/linux": "outside"})
	require.NoError(t, os.Remove(filepath.Join(dir, "config", "acquis.yaml")))
	require.NoError(t, os.Symlink(filepath.Join(outside, "profiles.yaml"), filepath.Join(dir, "config", "profiles.yaml")))
	require.NoError(t, os.Symlink(filepath.Join(outside, "patterns"), filepath.Join(dir, "config", "patterns")))
	_, r := newImportRouter(t, dir)

	w := importStatus(t, r, map[string]string{"config/config.yaml": importableConfig})

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var body map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.NotContains(t, body, "kept_files")
	require.Equal(t, "outside-profiles", readTreeFile(t, filepath.Join(outside, "profiles.yaml")))
	require.Equal(t, "outside", readTreeFile(t, filepath.Join(outside, "patterns", "linux")))
	target, err := os.Readlink(filepath.Join(dir, "config", "profiles.yaml"))
	require.NoError(t, err)
	require.Equal(t, filepath.Join(outside, "profiles.yaml"), target)
}

func seedLiveProfileWithNotification(t *testing.T, dir string) {
	t.Helper()
	writeTree(t, dir, map[string]string{
		"config/profiles.yaml":            strings.Replace(importProfilesNaming, "%s", "alpha", 1),
		"config/notifications/alpha.yaml": strings.Replace(importNotification, "%s", "alpha", 1),
	})
}

func TestImportConfigRestoresNotificationFilesTheKeptProfileNeeds(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	seedRealLayout(t, dir)
	seedLiveProfileWithNotification(t, dir)
	_, r := newImportRouter(t, dir)

	w := importStatus(t, r, map[string]string{"config/config.yaml": importableConfig})

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.Equal(t, []string{"acquis.yaml", "notifications/alpha.yaml", "profiles.yaml"}, keptFilesOf(t, w.Body.Bytes()))
	require.Equal(t, strings.Replace(importNotification, "%s", "alpha", 1), readTreeFile(t, filepath.Join(dir, "config", "notifications", "alpha.yaml")))
}

func TestImportConfigRefusesKeptProfileWithUndefinedNotification(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	seedRealLayout(t, dir)
	seedLiveProfileWithNotification(t, dir)
	// The archive's own file of the same name defines another notification and must not be replaced.
	_, r := newImportRouter(t, dir)
	before := treeState(t, dir)

	w := importStatus(t, r, map[string]string{
		"config/config.yaml":              importableConfig,
		"config/notifications/alpha.yaml": strings.Replace(importNotification, "%s", "other", 1),
	})

	require.Equal(t, http.StatusUnprocessableEntity, w.Code, w.Body.String())
	require.Equal(t, msgKeptProfileUndefined+"alpha", importNotificationBody(t, w))
	requireSameTree(t, before, dir)
}

func TestImportConfigRefusesKeptProfileWhenRestoredFileClashes(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	seedRealLayout(t, dir)
	writeTree(t, dir, map[string]string{
		"config/profiles.yaml":            strings.Replace(importProfilesNaming, "%s", "alpha", 1),
		"config/notifications/alpha.yaml": strings.Replace(importNotification, "%s", "alpha", 1) + "---\n" + strings.Replace(importNotification, "%s", "beta", 1),
	})
	_, r := newImportRouter(t, dir)
	before := treeState(t, dir)

	w := importStatus(t, r, map[string]string{
		"config/config.yaml":          importableConfig,
		"config/notifications/b.yaml": strings.Replace(importNotification, "%s", "beta", 1),
	})

	require.Equal(t, http.StatusUnprocessableEntity, w.Code, w.Body.String())
	require.Equal(t, msgKeptProfileDuplicate, importNotificationBody(t, w))
	requireSameTree(t, before, dir)
}

func TestImportConfigRefusesKeptProfileNamingNoSnapshotFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	seedRealLayout(t, dir)
	writeTree(t, dir, map[string]string{"config/profiles.yaml": strings.Replace(importProfilesNaming, "%s", "ghost", 1)})
	_, r := newImportRouter(t, dir)
	before := treeState(t, dir)

	w := importStatus(t, r, map[string]string{"config/config.yaml": importableConfig})

	require.Equal(t, http.StatusUnprocessableEntity, w.Code, w.Body.String())
	require.Equal(t, msgKeptProfileUndefined+"ghost", importNotificationBody(t, w))
	requireSameTree(t, before, dir)
}

// requireSameTree asserts the tree below dir matches a treeState taken earlier. A rollback recreates
// directories with the snapshot's private mode, so directory permission bits are not compared.
func requireSameTree(t *testing.T, before map[string]string, dir string) {
	t.Helper()
	normalize := func(state map[string]string) map[string]string {
		out := make(map[string]string, len(state))
		for rel, entry := range state {
			if strings.HasPrefix(entry, "d") {
				entry = "dir"
			}
			out[rel] = entry
		}
		return out
	}
	require.Equal(t, normalize(before), normalize(treeState(t, dir)), "a refused import must roll back to the same tree")
}

func TestKeptProfileRefusal(t *testing.T) {
	t.Parallel()
	require.Equal(t, msgKeptProfileDuplicate, keptProfileRefusal(crowdsec.ErrDuplicateNotifications))
	require.Equal(t, msgKeptProfileUndefined+"a, b", keptProfileRefusal(&crowdsec.UndefinedNotificationsError{Names: []string{"a", "b"}}))
}

func TestLiveConfigPrefix(t *testing.T) {
	t.Parallel()
	require.Equal(t, "config", liveConfigPrefix(1))
	require.Empty(t, liveConfigPrefix(0))
}
