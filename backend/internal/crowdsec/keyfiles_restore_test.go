package crowdsec

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

const keyConfig = "api:\n  server:\n    listen_uri: 0.0.0.0:8080\n"

func TestRestoreKeyFilesPutsBackEveryKeyRule(t *testing.T) {
	t.Parallel()
	snapDir, liveDir := companionTrees(t,
		map[string]string{
			"config/config.yaml": keyConfig,
			"config/notifications/http_default.yaml": httpPlugin("key_path: /etc/crowdsec/ssl/client.pem\n" +
				"cert_path: /etc/crowdsec/ssl/client.crt\n"),
			"config/ssl/client.pem":     "opaque private material\n",
			"config/ssl/client.crt":     "certificate\n",
			"config/ssl/named.key":      "opaque",
			"config/ssl/bundle.P12":     "opaque",
			"config/ssl/armored.txt":    pemCert + pemRSA,
			"config/ssl/public.txt":     pemPublicKey,
			"config/other.yaml":         "not a key\n",
			"config/notifications/x.k":  keyArmor("EC PRIVATE KEY"), // not read by CrowdSec
			"config/notifications/x.ky": "plain",
		},
		map[string]string{"config/config.yaml": keyConfig})
	require.NoError(t, os.Chmod(filepath.Join(snapDir, "config", "ssl", "client.pem"), 0o640)) // #nosec G302 -- test file, group-readable on purpose to prove the cap

	restored, err := RestoreKeyFiles(snapDir, liveDir)

	require.NoError(t, err)
	require.Equal(t, []string{"config/ssl/armored.txt", "config/ssl/bundle.P12", "config/ssl/client.pem", "config/ssl/named.key"}, restored)
	require.Equal(t, "opaque private material\n", readFile(t, filepath.Join(liveDir, "config", "ssl", "client.pem")))
	info, err := os.Stat(filepath.Join(liveDir, "config", "ssl", "client.pem"))
	require.NoError(t, err)
	require.Zero(t, info.Mode().Perm()&^0o600, "mode is capped at 0600")
	for _, absent := range []string{"ssl/client.crt", "ssl/public.txt", "other.yaml", "notifications/x.k", "notifications/x.ky"} {
		require.NoFileExists(t, filepath.Join(liveDir, "config", filepath.FromSlash(absent)))
	}
}

func TestRestoreKeyFilesResolvesAbsoluteDataFolderPaths(t *testing.T) {
	t.Parallel()
	snapDir, liveDir := t.TempDir(), t.TempDir()
	putFiles(t, snapDir, map[string]string{
		"config/config.yaml":                     keyConfig,
		"config/notifications/http_default.yaml": httpPlugin("key_path: " + filepath.Join(liveDir, "certs", "client.pem") + "\n"),
		"certs/client.pem":                       "opaque",
	})
	putFiles(t, liveDir, map[string]string{"config/config.yaml": keyConfig})

	restored, err := RestoreKeyFiles(snapDir, liveDir)

	require.NoError(t, err)
	require.Equal(t, []string{"certs/client.pem"}, restored)
	require.Equal(t, "opaque", readFile(t, filepath.Join(liveDir, "certs", "client.pem")))
}

func TestRestoreKeyFilesNeverReplacesWhatTheArchiveProvided(t *testing.T) {
	t.Parallel()
	snapDir, liveDir := companionTrees(t,
		map[string]string{"config/ssl/a.key": "snapshot", "config/ssl/b.key": "snapshot", "config/ssl/c.key": "snapshot"},
		map[string]string{"config/ssl/a.key": "archive"})
	outside := t.TempDir()
	putFiles(t, outside, map[string]string{"b.key": "outside"})
	require.NoError(t, os.Symlink(filepath.Join(outside, "b.key"), filepath.Join(liveDir, "config", "ssl", "b.key")))

	restored, err := RestoreKeyFiles(snapDir, liveDir)

	require.NoError(t, err)
	require.Equal(t, []string{"config/ssl/c.key"}, restored)
	require.Equal(t, "archive", readFile(t, filepath.Join(liveDir, "config", "ssl", "a.key")))
	require.Equal(t, "outside", readFile(t, filepath.Join(outside, "b.key")))
}

func TestRestoreKeyFilesSymlinkCases(t *testing.T) {
	t.Parallel()
	snapDir, liveDir := companionTrees(t,
		map[string]string{"config/linked/a.key": "snapshot", "config/file/b.key": "snapshot", "config/real.key": "snapshot"},
		map[string]string{"config/file": "a file where the folder belongs"})
	outside := t.TempDir()
	require.NoError(t, os.Symlink(outside, filepath.Join(liveDir, "config", "linked")))
	// A key file that is itself a link in the snapshot is not followed.
	require.NoError(t, os.Symlink(filepath.Join(outside, "target"), filepath.Join(snapDir, "config", "link.key")))
	putFiles(t, outside, map[string]string{"target": "secret"})

	restored, err := RestoreKeyFiles(snapDir, liveDir)

	require.NoError(t, err)
	require.Equal(t, []string{"config/real.key"}, restored)
	require.NoFileExists(t, filepath.Join(outside, "a.key"))
	require.NoFileExists(t, filepath.Join(liveDir, "config", "link.key"))
	require.Equal(t, "a file where the folder belongs", readFile(t, filepath.Join(liveDir, "config", "file")))
}

func TestRestoreKeyFilesHostileNames(t *testing.T) {
	t.Parallel()
	hostile := "we\nird\x1b[31m.key"
	snapDir, liveDir := companionTrees(t,
		map[string]string{"config/" + hostile: "snapshot", "config/-----BEGIN RSA PRIVATE KEY-----.key": "snapshot"},
		nil)

	restored, err := RestoreKeyFiles(snapDir, liveDir)

	require.NoError(t, err)
	require.Len(t, restored, 2)
	require.Equal(t, "snapshot", readFile(t, filepath.Join(liveDir, "config", hostile)))
}

func TestRestoreKeyFilesSkipsExcludedAndOversizedFiles(t *testing.T) {
	t.Parallel()
	snapDir, liveDir := companionTrees(t,
		map[string]string{
			"config/hub/parsers/x.key": "hub item",
			"config/big.key":           strings.Repeat("x", int(maxKeyRestoreBytes)+1),
			"config/small.key":         "ok",
		}, nil)

	restored, err := RestoreKeyFiles(snapDir, liveDir)

	require.NoError(t, err)
	require.Equal(t, []string{"config/small.key"}, restored)
}

func TestRestoreKeyFilesLimits(t *testing.T) {
	t.Parallel()
	files := map[string]string{}
	for i := range maxKeyRestoreFiles + 1 {
		files["config/k/"+strings.Repeat("a", i+1)+".key"] = "x"
	}
	snapDir, liveDir := companionTrees(t, files, nil)

	_, err := RestoreKeyFiles(snapDir, liveDir)

	require.ErrorIs(t, err, ErrKeyRestoreLimit)
}

func TestRestoreKeyFilesNothingToRestore(t *testing.T) {
	t.Parallel()
	snapDir, liveDir := companionTrees(t, map[string]string{"config/config.yaml": keyConfig}, nil)

	restored, err := RestoreKeyFiles(snapDir, liveDir)

	require.NoError(t, err)
	require.Empty(t, restored)
}

func TestWriteKeyFileRefusesGrowthAndExistingPath(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	dest := filepath.Join(dir, "k")

	wrote, err := writeKeyFile(dest, strings.NewReader(strings.Repeat("x", int(maxKeyRestoreBytes)+1)), 0o600)
	require.ErrorIs(t, err, ErrKeyRestoreLimit)
	require.False(t, wrote)
	require.NoFileExists(t, dest)

	require.NoError(t, os.WriteFile(dest, []byte("existing"), 0o600))
	wrote, err = writeKeyFile(dest, strings.NewReader("new"), 0o600)
	require.NoError(t, err)
	require.False(t, wrote)
	require.Equal(t, "existing", readFile(t, dest))
}

func TestRestoreKeyFilesFailsWhenTheSnapshotCannotBeRead(t *testing.T) {
	t.Parallel()
	snapDir, liveDir := companionTrees(t, map[string]string{"config/sealed/a.key": "x"}, nil)
	sealed := filepath.Join(snapDir, "config", "sealed")
	require.NoError(t, os.Chmod(sealed, 0))
	t.Cleanup(func() { _ = os.Chmod(sealed, 0o700) }) // #nosec G302 -- lets the temp dir be removed

	_, err := RestoreKeyFiles(snapDir, liveDir)

	if os.Geteuid() != 0 {
		require.Error(t, err)
	}
}
