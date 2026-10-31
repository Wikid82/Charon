package crowdsec

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIsSecretPath(t *testing.T) {
	t.Parallel()
	cases := map[string]bool{
		"bouncer_key":                             true,
		"config/local_api_credentials.yaml":       true,
		"config/online_api_credentials.yaml":      true,
		"a/b/c/bouncer_key":                       true,
		"BOUNCER_KEY":                             true,
		"config/Local_API_Credentials.YAML":       true,
		"bouncer_key/x":                           true,
		"./config/online_api_credentials.yaml":    true,
		"config//local_api_credentials.yaml":      true,
		`config\local_api_credentials.yaml`:       filepath.Separator == '\\',
		"bouncer_key.bak":                         false,
		"bouncer_key.tmp":                         true,
		"bouncer_key.tmp-123456789":               true,
		"bouncer_key.tmp-":                        true,
		"config/bouncer_key.tmp-42":               true,
		"BOUNCER_KEY.TMP-7":                       true,
		"bouncer_key.tmp-1/x":                     true,
		"config/local_api_credentials.yaml.tmp-x": true,
		"config/online_api_credentials.yaml.tmp":  true,
		"bouncer_key.tmpx":                        false,
		"bouncer_key.tmp.bak":                     false,
		"xbouncer_key.tmp-1":                      false,
		"local_api_credentials.yaml.tmpx":         false,
		"my_local_api_credentials.yaml":           false,
		"local_api_credentials.yaml.bak":          false,
		"config/online_api_credentials.yml":       false,
		"config/config.yaml":                      false,
		"notifications/bouncer_key_notes.yaml":    false,
		"":                                        false,
		".":                                       false,
	}
	for rel, want := range cases {
		require.Equal(t, want, IsSecretPath(rel), rel)
	}
}

func TestIsPreservedPath(t *testing.T) {
	t.Parallel()
	require.True(t, IsPreservedPath("data/x"))
	require.True(t, IsPreservedPath("crowdsec.db"))
	require.True(t, IsPreservedPath("bouncer_key"))
	require.False(t, IsPreservedPath("config/config.yaml"))
}

func TestFindConfigFile(t *testing.T) {
	t.Parallel()
	t.Run("config dir preferred", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		writeFile(t, filepath.Join(dir, "config", "config.yaml"), "a")
		writeFile(t, filepath.Join(dir, "config.yaml"), "b")
		require.Equal(t, filepath.Join(dir, "config", "config.yaml"), FindConfigFile(dir))
	})
	t.Run("root fallback", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		writeFile(t, filepath.Join(dir, "config.yaml"), "b")
		require.Equal(t, filepath.Join(dir, "config.yaml"), FindConfigFile(dir))
	})
	t.Run("none", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		writeFile(t, filepath.Join(dir, "config", "acquis.yaml"), "x")
		require.Empty(t, FindConfigFile(dir))
		require.Empty(t, FindConfigFile(filepath.Join(dir, "missing")))
	})
}

func seedSecretTree(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "crowdsec")
	writeFile(t, filepath.Join(dir, "config", "config.yaml"), "api: {}\n")
	writeFile(t, filepath.Join(dir, "config", "local_api_credentials.yaml"), "live-lapi")
	writeFile(t, filepath.Join(dir, "config", "online_api_credentials.yaml"), "live-capi")
	writeFile(t, filepath.Join(dir, "bouncer_key"), "live-key")
	writeFile(t, filepath.Join(dir, "other.yaml"), "o")
	return dir
}

func TestClearConfigKeepingPreservesSecrets(t *testing.T) {
	t.Parallel()
	dir := seedSecretTree(t)
	require.NoError(t, ClearConfigKeeping(dir, IsPreservedPath))
	require.Equal(t, "live-lapi", readFile(t, filepath.Join(dir, "config", "local_api_credentials.yaml")))
	require.Equal(t, "live-capi", readFile(t, filepath.Join(dir, "config", "online_api_credentials.yaml")))
	require.Equal(t, "live-key", readFile(t, filepath.Join(dir, "bouncer_key")))
	require.NoFileExists(t, filepath.Join(dir, "config", "config.yaml"))
	require.NoFileExists(t, filepath.Join(dir, "other.yaml"))
}

func TestClearConfigKeepingKeepsSymlinkedSecret(t *testing.T) {
	t.Parallel()
	dir := seedSecretTree(t)
	target := filepath.Join(t.TempDir(), "docker-secret")
	writeFile(t, target, "from-secret-store")
	link := filepath.Join(dir, "config", "local_api_credentials.yaml")
	require.NoError(t, os.Remove(link))
	require.NoError(t, os.Symlink(target, link))

	require.NoError(t, ClearConfigKeeping(dir, IsPreservedPath))
	got, err := os.Readlink(link)
	require.NoError(t, err)
	require.Equal(t, target, got)
	require.Equal(t, "from-secret-store", readFile(t, target))
}

func TestClearConfigKeepingPlainClearConfigStillRemovesSecrets(t *testing.T) {
	t.Parallel()
	dir := seedSecretTree(t)
	require.NoError(t, ClearConfig(dir))
	require.NoFileExists(t, filepath.Join(dir, "bouncer_key"))
}

func TestRestoreKeepingLeavesLiveSecretsAlone(t *testing.T) {
	t.Parallel()
	t.Run("secret changed since snapshot keeps the newer value", func(t *testing.T) {
		t.Parallel()
		dir := seedSecretTree(t)
		snap, err := Snapshot(dir)
		require.NoError(t, err)
		writeFile(t, filepath.Join(dir, "bouncer_key"), "newer-key")
		writeFile(t, filepath.Join(dir, "config", "config.yaml"), "uploaded: true\n")
		writeFile(t, filepath.Join(dir, "extra.yaml"), "x")

		require.NoError(t, RestoreKeeping(snap, dir, IsPreservedPath))
		require.Equal(t, "newer-key", readFile(t, filepath.Join(dir, "bouncer_key")))
		require.Equal(t, "live-lapi", readFile(t, filepath.Join(dir, "config", "local_api_credentials.yaml")))
		require.Equal(t, "api: {}\n", readFile(t, filepath.Join(dir, "config", "config.yaml")))
		require.NoFileExists(t, filepath.Join(dir, "extra.yaml"))
	})
	t.Run("restore copy failure does not delete live secrets", func(t *testing.T) {
		t.Parallel()
		dir := seedSecretTree(t)
		err := RestoreKeeping(filepath.Join(t.TempDir(), "missing-snapshot"), dir, IsPreservedPath)
		require.ErrorContains(t, err, "restore backup")
		require.Equal(t, "live-key", readFile(t, filepath.Join(dir, "bouncer_key")))
		require.Equal(t, "live-capi", readFile(t, filepath.Join(dir, "config", "online_api_credentials.yaml")))
	})
	t.Run("plain Restore still restores the snapshot copy", func(t *testing.T) {
		t.Parallel()
		dir := seedSecretTree(t)
		snap, err := Snapshot(dir)
		require.NoError(t, err)
		writeFile(t, filepath.Join(dir, "bouncer_key"), "changed")
		require.NoError(t, Restore(snap, dir))
		require.Equal(t, "live-key", readFile(t, filepath.Join(dir, "bouncer_key")))
	})
}

func TestIsHubMirrorConfig(t *testing.T) {
	t.Parallel()
	cases := map[string]bool{
		".charon-hub-mirror-abc.yaml":              true,
		".charon-hub-mirror-abc.yaml.local":        true,
		"config/.charon-hub-mirror-abc.yaml":       true,
		"config/.charon-hub-mirror-abc.yaml.local": true,
		"./.charon-hub-mirror-abc.yaml":            true,
		"CONFIG/.Charon-Hub-Mirror-ABC.YAML":       true,
		".CHARON-HUB-MIRROR-ABC.YAML.LOCAL":        true,
		`config\.charon-hub-mirror-abc.yaml`:       filepath.Separator == '\\',
		"config.yaml":                              false,
		"config/config.yaml":                       false,
		"config/config.yaml.local":                 false,
		"charon-hub-mirror-abc.yaml":               false,
		".charon-hub-mirror-abc.txt":               false,
		".charon-hub-mirror-abc.yaml.bak":          false,
		"sub/.charon-hub-mirror-abc.yaml":          false,
		"config/sub/.charon-hub-mirror-abc.yaml":   false,
		"data/.charon-hub-mirror-abc.yaml":         false,
		"xcharon-hub-mirror-abc.yaml":              false,
		"":                                         false,
		".":                                        false,
	}
	for rel, want := range cases {
		require.Equal(t, want, IsHubMirrorConfig(rel), rel)
	}
}

func TestHubMirrorConfigIsPreservedAndExcluded(t *testing.T) {
	t.Parallel()
	dataDir := t.TempDir()
	for _, rel := range []string{
		".charon-hub-mirror-abc.yaml",
		".charon-hub-mirror-abc.yaml.local",
		"config/.charon-hub-mirror-abc.yaml",
		"config/.charon-hub-mirror-abc.yaml.local",
	} {
		require.True(t, IsPreservedPath(rel), "preserved %s", rel)
		require.True(t, IsExportExcluded(rel), "export excludes %s", rel)
		require.True(t, ImportKeeper(dataDir)(rel), "import keeps %s", rel)
	}
	require.False(t, IsPreservedPath("config/config.yaml"))
	require.False(t, IsExportExcluded("config/config.yaml"))
	require.False(t, ImportKeeper(dataDir)("config/config.yaml"))
}

func TestHubMirrorLeftoverIsLeftOutOfExportArchive(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeExportTree(t, dir, map[string]string{
		"config/config.yaml":                       "common: {}\n",
		"config/.charon-hub-mirror-abc.yaml":       "cscli:\n  password: s3cret\n",
		"config/.charon-hub-mirror-abc.yaml.local": "cscli: {}\n",
		".charon-hub-mirror-def.yaml":              "cscli:\n  password: s3cret\n",
		"config/acquis.yaml":                       "source: file\n",
	})

	var buf bytes.Buffer
	_, omitted, err := WriteExportArchive(context.Background(), dir, &buf)
	require.NoError(t, err)
	require.Equal(t, []string{"config/acquis.yaml", "config/config.yaml"}, exportNames(t, buf.Bytes()))
	require.NotContains(t, omitted, "config/.charon-hub-mirror-abc.yaml")
}
